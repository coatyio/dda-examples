// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package components

import (
	"bytes"
	"context"
	"time"

	"github.com/coatyio/dda-examples/compute/clog"
	"github.com/coatyio/dda-examples/compute/registry"
	"github.com/coatyio/dda/config"
	"github.com/coatyio/dda/dda"
	"github.com/coatyio/dda/services/com/api"
	"github.com/google/uuid"
)

// A Worker is an application component that realizes the Compute part of the
// Partition-Compute-Accumulate distribution pattern. It performs a partial
// computation requested by a coordinator returning partial result data. To
// communicate with a coordinator it uses the DDA communication service as a
// library.
type Worker struct {
	*clog.CLogger                    // conditional logger
	id            string             // identifies this worker in communication requests
	reg           *registry.Registry // registry of predefined computations
	dda           *dda.Dda           // DDA instance
	finalized     chan struct{}      // signals finalization of operations
}

// NewWorker creates and returns a semi-initialized Worker object ready for use
// with Start.
func NewWorker() *Worker {
	id := uuid.NewString()
	return &Worker{
		CLogger:   clog.New("%v %s ", RoleWorker, UuidShort(id)),
		id:        id,
		reg:       registry.NewRegistry(),
		finalized: make(chan struct{}),
	}
}

// Start a Worker initialized by NewWorker.
func (w *Worker) Start(ctx context.Context, brokerUrl string, completed chan<- struct{}) {
	defer func() {
		if w.dda != nil {
			w.dda.Close()
		}
		completed <- struct{}{} // signal that shutdown has completed
	}()

	// Fail fast if one of the following initializations yields an error.

	if err := w.initDda(brokerUrl); err != nil {
		w.Errorf("Failed initializing DDA: %v", err)
		return
	}

	if err := w.dda.Open(0); err != nil {
		w.Errorf("Failed opening DDA: %v", err)
		return
	}

	if err := w.trackCoordinators(ctx); err != nil {
		w.Errorf("Failed tracking coordinators: %v", err)
		return
	}

	if err := w.subscribePartialComputations(ctx); err != nil {
		w.Errorf("Failed subscribing computations: %v", err)
		return
	}

	w.announce(true)

	<-ctx.Done() // await shutdown by termination signal

	w.announce(false)

	// Give publication of leave announcement time before closing DDA connection.
	<-time.After(500 * time.Millisecond)

	// Await finalization of both tracking and compute operations.
	<-w.finalized
	<-w.finalized
}

// finalize signals that an operation to be finalized, i.e. tracking or compute,
// has finished.
func (w *Worker) finalize() {
	w.finalized <- struct{}{}
}

// initDda sets up a DDA instance for communication.
func (w *Worker) initDda(brokerUrl string) error {
	cfg := config.New() // create DDA configuration with default options

	// Set non-default DDA configuration options. Do not change default cluster
	// name as DDA sidecar for coordinators also uses default cluster name.
	cfg.Services.Com.Url = brokerUrl
	cfg.Identity.Name = "worker"
	cfg.Identity.Id = w.id
	cfg.Apis.Grpc.Disabled = true    // do not expose gRPC API
	cfg.Apis.GrpcWeb.Disabled = true // do not expose gRPC-Web

	var err error

	// Create DDA instance to access communication service.
	if w.dda, err = dda.New(cfg); err != nil {
		return err
	}
	return nil
}

// announce is part of distributed lifecycle tracking. It publishes an
// announcement event to signal coordinator components that this worker is
// either joining/alive (if join is true) or leaving (if join is false).
func (w *Worker) announce(join bool) {
	evt := api.Event{
		Type:   EventTypeAnnounceWorker,
		Id:     w.id,
		Source: RoleWorker.String(),
		Data:   DataAnnounceJoin,
	}
	if !join {
		evt.Data = DataAnnounceLeave
	}

	w.Printf("Send announcement %s", string(evt.Data))

	if err := w.dda.PublishEvent(evt); err != nil {
		w.Errorf("Failed announcing %s event for id %s: %v", evt.Type, UuidShort(evt.Id), err)
	}
}

// trackCoordinators is part of distributed lifecycle tracking. It subscribes to
// announcement actions by coordinators and acknowledges all those actions that
// are not indicating a leave to signal that this worker is alive.
func (w *Worker) trackCoordinators(ctx context.Context) error {
	acts, err := w.dda.SubscribeAction(ctx, api.SubscriptionFilter{Type: ActionEventTypeAnnounceCoordinator})
	if err != nil {
		return err
	}

	go func() {
		defer w.finalize()

		// Note that the acts channel is closed and announce actions by
		// coordinators are unsubscribed as soon as the worker shuts down,
		// triggered when the given context is canceled.
		for ac := range acts {
			w.Printf("Announcement from %s %s: %s", ac.Source, UuidShort(ac.Id), string(ac.Params))
			if bytes.Equal(ac.Params, DataAnnounceJoin) {
				// Reply so that joining coordinator can track this worker.
				if err := ac.Callback(api.ActionResult{Context: RoleWorker.String(), Data: []byte(w.id)}); err != nil {
					w.Errorf("Failed publishing %s action result: %v", ac.Type, err)
				}
			}
		}
	}()

	return nil
}

// subscribePartialComputations receives compute actions sent by coordinators
// and handles them one by one.
func (w *Worker) subscribePartialComputations(ctx context.Context) error {
	// Register a shared compute subscription to receive partial computations
	// exclusively.
	acs, err := w.dda.SubscribeAction(ctx, api.SubscriptionFilter{
		Type:  ActionTypeCompute,
		Share: ActionShareCompute,
	})
	if err != nil {
		return err
	}

	go func() {
		defer w.finalize()

		// Handle incoming partial computations one by one in this goroutine. No
		// need to cope with backpressure here as coordinators make provision
		// against overloading workers. Note that the acs channel is closed and
		// compute actions are unsubscribed as soon as the worker shuts down,
		// triggered when the given context is canceled.
		for ac := range acs {
			w.handlePartialComputation(ac)
		}
	}()

	return nil
}

// handlePartialComputation invokes the partial computation with the input from
// the given compute action and sends back the output of the computation.
func (w *Worker) handlePartialComputation(ac api.ActionWithCallback) {
	cm := w.reg.ComputationByName(ac.Id)
	if cm == nil {
		w.Errorf("Skip computation %s which is not defined", ac.Id)
		return
	}

	w.Printf("Partial input from %v %s: %s%v", RoleCoordinator, UuidShort(ac.Source), ac.Id, ac.Params)

	output := cm.PartialCompute(ac.Params)
	if output == nil {
		w.Errorf("Skip publishing %s action result", ac.Type)
		return
	}

	w.Printf("Partial output to %v %s: %s%v", RoleCoordinator, UuidShort(ac.Source), ac.Id, output)

	if err := ac.Callback(api.ActionResult{
		Context: w.id, // identifies the worker that handled the partial computation
		Data:    output,
	}); err != nil {
		w.Errorf("Failed publishing %s action result: %v", ac.Type, err)
	}
}
