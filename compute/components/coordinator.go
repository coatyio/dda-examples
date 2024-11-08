// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package components

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/coatyio/dda-examples/compute/clog"
	cmpt "github.com/coatyio/dda-examples/compute/computation"
	"github.com/coatyio/dda-examples/compute/registry"
	stubs "github.com/coatyio/dda/apis/grpc/stubs/golang"
	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"
)

// A Coordinator is an application component that handles a compute request
// using the Partition-Compute-Accumulate distribution pattern. It splits the
// request into partial computations that are dispatched to workers, accumulates
// partial results from workers, and outputs the final result to a specified
// destination. It communicates with workers and other coordinators using the
// DDA sidecar communication service over gRPC.
type Coordinator struct {
	*clog.CLogger                        // conditional logger
	id            string                 // identifies this coordinator in communication requests
	reg           *registry.Registry     // registry of predefined computations
	tracker       *Tracker               // tracks alive workers and coordinators
	client        stubs.ComServiceClient // gRPC client for DDA communication service
	clientCloser  func()                 // closes gRPC client connection
	finalized     chan struct{}          // signals finalization of operations
	workerFree    chan struct{}          // signals availability of worker for computation
}

// NewCoordinator creates and returns a semi-initialized Coordinator object
// ready for use with Start.
func NewCoordinator() *Coordinator {
	id := uuid.NewString()
	return &Coordinator{
		CLogger:    clog.New("%v %s ", RoleCoordinator, UuidShort(id)),
		id:         id,
		reg:        registry.NewRegistry(),
		tracker:    NewTracker(),
		finalized:  make(chan struct{}),
		workerFree: make(chan struct{}, 1),
	}
}

// Start a Coordinator initialized by NewCoordinator().
func (c *Coordinator) Start(ctx context.Context, req cmpt.ComputeRequest, ddaAddress string, completed chan<- struct{}) {
	defer func() {
		if c.clientCloser != nil {
			c.clientCloser()
		}
		close(completed) // signal that shutdown has completed
	}()

	var err error

	ctx, cancel := context.WithCancel(ctx) // child context to be canceled if computation has completed
	defer cancel()

	c.tracker.TryJoin(RoleCoordinator, c.id) // preregister as coordinator

	// Fail fast if one of the following initializations yields an error.

	c.client, c.clientCloser, err = c.openGrpcClient(ddaAddress)
	if err != nil {
		c.Errorf("Failed opening DDA gRPC client connection: %v", err)
		return
	}

	// Set up lifecycle tracking in own gouroutines.
	if err = c.trackCoordinators(ctx); err != nil {
		c.Errorf("Failed tracking coordinators: %v", err)
		return
	}
	if err = c.trackWorkers(ctx); err != nil {
		c.Errorf("Failed tracking workers: %v", err)
		return
	}

	c.announce(true)

	// Process compute request in its own goroutine.
	if err = c.partitionAccumulate(ctx, cancel, req); err != nil {
		return
	}

	// Await shutdown by termination signal (by parent context) or completion of
	// computation (by child context).
	<-ctx.Done()

	c.announce(false)

	// Give publication of leave announcement time before closing DDA connection.
	<-time.After(500 * time.Millisecond)

	// Await finalization of two tracking and one computation operations.
	<-c.finalized
	<-c.finalized
	<-c.finalized
}

// finalize signals that an operation to be finalized, i.e. tracking or
// partitioning/accumulating, has finished.
func (c *Coordinator) finalize() {
	c.finalized <- struct{}{}
}

// openGrpcClient connects to the gRPC service of the co-located DDA sidecar.
func (c *Coordinator) openGrpcClient(address string) (stubs.ComServiceClient, func(), error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	c.Printf("Connecting to DDA sidecar with gRPC address %s...", address)

	conn, err := grpc.Dial(address, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to dial gRPC Client on address %s: %v", address, err)
	}
	return stubs.NewComServiceClient(conn), func() { defer conn.Close() }, nil
}

// announce is part of distributed lifecycle tracking. It publishes an
// announcement action to signal coordinator and worker components that this
// coordinator is joining (join is true) or leaving (join is false). On joining,
// it receives responses by alive coordinators and workers which are tracked.
func (c *Coordinator) announce(join bool) {
	act := &stubs.Action{
		Type:   ActionEventTypeAnnounceCoordinator,
		Id:     c.id,
		Source: RoleCoordinator.String(),
		Params: DataAnnounceJoin,
	}

	ctx := context.Background()
	var cancel context.CancelFunc = func() {}
	if join {
		ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
	} else {
		act.Params = DataAnnounceLeave
	}

	c.Printf("Send announcement %s", string(act.Params))

	stream, err := c.client.PublishAction(ctx, act)
	if err != nil {
		c.Errorf("Failed announcing %s action for id %s: %v", act.Type, UuidShort(act.Id), err)
	}
	_, _ = stream.Header() // await dda-suback to ensure subsequent reesponses are not lost

	if !join {
		defer cancel()
		return // leaving coordinator is not interested in responses
	}

	go func() {
		defer c.finalize()
		defer cancel()

		for {
			ar, err := stream.Recv() // blocks waiting for result or error
			if err != nil {
				// Skip log output if call has been canceled or deadline exceeded.
				if status.Code(err) != codes.Canceled && status.Code(err) != codes.DeadlineExceeded {
					c.Errorf("Error receiving announce responses: %v", err)
				}
				return
			}
			id := string(ar.Data)
			c.Printf("Announcement from %s %s: %s", ar.Context, UuidShort(id), string(DataAnnounceJoin))
			c.tracker.TryJoin(ParseComponentRole(ar.Context), id)
		}
	}()
}

// trackCoordinators is part of distributed lifecycle tracking. It subscribes to
// announcement actions by coordinators and acknowledges all those actions that
// are not indicating a leave to signal that this coordinator is alive.
func (c *Coordinator) trackCoordinators(ctx context.Context) error {
	stream, err := c.client.SubscribeAction(ctx, &stubs.SubscriptionFilter{Type: ActionEventTypeAnnounceCoordinator})
	if err != nil {
		return err
	}

	go func() {
		defer c.finalize()

		for {
			ac, err := stream.Recv() // blocks waiting for result or error
			if err != nil {
				if status.Code(err) != codes.Canceled { // skip log output if call has been canceled
					c.Errorf("Error tracking coordinator announcements: %v", err)
				}
				return
			}
			src, id, params := ac.Action.Source, ac.Action.Id, ac.Action.Params
			c.Printf("Announcement from %s %s: %s", src, UuidShort(id), string(params))
			if bytes.Equal(params, DataAnnounceJoin) {
				if id != c.id { // skip echo
					c.tracker.TryJoin(ParseComponentRole(src), id)

					// Reply so that joining coordinator can track this coordinator.
					if _, err := c.client.PublishActionResult(context.Background(), &stubs.ActionResultCorrelated{
						CorrelationId: ac.CorrelationId,
						Result:        &stubs.ActionResult{Context: RoleCoordinator.String(), Data: []byte(c.id)},
					}); err != nil {
						c.Errorf("Failed publishing %s action result: %v", ac.Action.Type, err)
					}
				}
			} else {
				c.tracker.Leave(ParseComponentRole(src), id)
			}
		}
	}()

	return nil
}

// trackWorkers is part of distributed lifecycle tracking. It subscribes to
// announcement events by joining or leaving workers and tracks them.
func (c *Coordinator) trackWorkers(ctx context.Context) error {
	stream, err := c.client.SubscribeEvent(ctx, &stubs.SubscriptionFilter{Type: EventTypeAnnounceWorker})
	if err != nil {
		return err
	}

	go func() {
		defer c.finalize()

		for {
			evt, err := stream.Recv() // blocks waiting for result or error
			if err != nil {
				if status.Code(err) != codes.Canceled { // skip log output if call has been canceled
					c.Errorf("Error tracking worker announcements: %v", err)
				}
				return
			}
			src := evt.Source
			c.Printf("Announcement from %s %s: %s", src, UuidShort(evt.Id), string(evt.Data))
			if bytes.Equal(evt.Data, DataAnnounceJoin) {
				c.tracker.TryJoin(ParseComponentRole(src), evt.Id)
			} else {
				c.tracker.Leave(ParseComponentRole(src), evt.Id)
			}
		}
	}()

	return nil
}

// partitionAccumulate realizes the Partition-Compute-Accumulate distribution
// pattern on the coordinator side.
func (c *Coordinator) partitionAccumulate(ctx context.Context, cancel context.CancelFunc, req cmpt.ComputeRequest) error {
	cm := c.reg.ComputationByName(req.Name)
	if cm == nil {
		err := fmt.Errorf("%s is not defined", req.Name)
		fmt.Println(err)
		return err
	}

	in, err := cm.Partition(req)
	if err != nil {
		fmt.Printf("Invalid input arguments: %v\n", err)
		return err
	}

	go func(start time.Time) {
		defer c.finalize()

		// Trigger graceful shutdown by completion of computation; no-op if
		// already canceled by termination signal.
		defer cancel()

		pcProc := 0                                   // current number of processing partial computations
		pcCompleted := make(chan partialResult)       // signals completion of a partial computation
		pcResubmit := make(chan cmpt.BinaryData, 100) // queue of partial computations to be resubmitted
		failFast := false                             // indicates fast failure

		var inFree <-chan cmpt.BinaryData         // input channel if a worker is free; nil otherwise
		var pcResubmitFree <-chan cmpt.BinaryData // pcResubmit channel if worker is free; nil otherwise
		var lastFreeState workerFreeState
		var currentFreeState workerFreeState

		// Whenever a worker is available for processing input, the for-select
		// loop reads the input channel (or the channel of resubmit input) and
		// publishes a partial computation using the DDA Action pattern.
		// Whenever such a processing partial computation completes its result
		// is accumulated. The overall computation is finalized when all partial
		// inputs have been processed.
		//
		// The loop fails fast if context ctx is canceled by a termination
		// signal, or if a partial result indicates a computational error. Other
		// types of errors such as partial computation timeout or communication
		// errors are handled by trying to resubmit the input data of the failed
		// partial computation. If too many partial computations to be
		// resubmitted pile up in a queue (max 100), the loop also fails fast.
	loop:
		for {
			// First, check if a worker is available for processing input.
			cc, cw := c.tracker.Count()
			currentFreeState = workerFreeState{cc: cc, cw: cw, pcProc: pcProc}
			if currentFreeState != lastFreeState {
				c.Printf("%s", currentFreeState) // only log changes
			}
			lastFreeState = currentFreeState
			if currentFreeState.Free() > 0 {
				inFree = in                 // unblock input channel if not fully drained
				pcResubmitFree = pcResubmit // unblock pcResubmit channel
			} else {
				inFree = nil         // block input channel
				pcResubmitFree = nil // block pcResubmit channel
			}

			// Then, process (resubmitted) input and completions of partial
			// computations.
			select {
			case <-ctx.Done(): // canceled by termination signal
				failFast = true
				break loop
			case input, ok := <-inFree:
				if !ok {
					in = nil // skip this case forever from now on as channel is fully drained
					if pcProc == 0 && len(pcResubmit) == 0 {
						break loop // all partial computations completed
					}
					break
				}
				pcProc++
				c.Printf("Sending partial input to a %v: %v", RoleWorker, input)
				go c.performPartialComputation(ctx, cm, input, pcCompleted)
			case input := <-pcResubmitFree:
				pcProc++
				c.Printf("Sending resubmitted partial input to a %v: %v", RoleWorker, input)
				go c.performPartialComputation(ctx, cm, input, pcCompleted)
			case res := <-pcCompleted: // ongoing partial computation completed
				pcProc--
				if res.resubmit != nil {
					c.Errorf("Failed sending/receiving partial input/output for input %v: %v", res.data, res.resubmit)
					select {
					case pcResubmit <- res.data: // try queuing input data
						c.Errorf("Queuing partial input for resubmission: %v", res.data)
					default:
						c.Errorf("Failed queuing partial input for resubmission %v: queue overflow: cannot resubmit more than %d inputs", res.data, len(pcResubmit))
						failFast = true // fail fast if queue is full
						break loop
					}
					break
				}
				if len(res.data) == 0 { // fail fast on computational or encoding error
					c.Errorf("Received computational or encoding error from %v %s", RoleWorker, res.workerId)
					failFast = true
					break loop
				}
				c.Printf("Received partial output from %v %s: %v", RoleWorker, res.workerId, res.data)
				cm.Accumulate(res.data)
				if in == nil && pcProc == 0 && len(pcResubmit) == 0 {
					break loop // all partial computations completed
				}
			default: // if all cases block recheck whether a worker is free
			}
		}

		if !failFast {
			cm.Finalize(start)
		} else {
			fmt.Fprintf(req.OutputWriter, "Computation %s%v failed\n", req.Name, req.Args)
		}
	}(time.Now())

	return nil
}

func (c *Coordinator) performPartialComputation(ctx context.Context, cm cmpt.Computation, input cmpt.BinaryData, completed chan partialResult) {
	ac := &stubs.Action{
		Type:   ActionTypeCompute,
		Id:     cm.Name(),
		Source: c.id,
		Params: input,
	}

	ctx, cancel := context.WithTimeout(ctx, cm.PartialComputeTimeout())
	defer cancel() // clean up server-side stream and response subscription when timeout expires
	stream, err := c.client.PublishAction(ctx, ac)
	if err != nil {
		if status.Code(err) == codes.Canceled {
			return // skip log output, do not signal failure on completed
		}

		// Error code also includes codes.DeadlineExceeded for expired context timeout.
		completed <- partialResult{data: input, resubmit: err}
		return
	}

	// Wait for single result or error, including timeout and parent context cancelation.
	ar, err := stream.Recv()

	if err != nil {
		// Error includes "canceled" condition (gRPC error with status code
		// codes.Canceled) and "stream ended" condition (err == io.EOF)
		if status.Code(err) == codes.Canceled {
			return // skip log output, do not signal failure on completed
		}

		// Error code also includes codes.DeadlineExceeded for expired context timeout.
		completed <- partialResult{data: input, resubmit: err}
		return
	}

	// Send result data of partial computation.
	completed <- partialResult{data: ar.Data, workerId: ar.Context}
}

// partialResult represents information about the result of a partial computation.
type partialResult struct {
	data     cmpt.BinaryData // output data if resubmit is nil; otherwise input data to be resubmitted
	workerId string          // ID of worker that performed partial computation
	resubmit error           // error causing input to be resubmitted; nil if output data is available
}

// workerFreeState maintains the parameters of the latest check for a free worker.
type workerFreeState struct {
	cc     int // number of coordinators
	cw     int // number of workers
	pcProc int // number of processing partial computations
}

// Free estimates the number of workers currently free to perform the next
// pending partial computation.
//
// The estimation assumes that the workload of partial computations produced by
// all coordinators in the system is evenly distributed to all workers in a
// manner that doesn't overload workers: Each coordinator should dispatch a
// partial computation at hand not until its current number of ongoing, i.e.
// published but not yet completed partial computations is less than the average
// number of alive workers per coordinator.
//
// This uniform distribution logic works best with a DDA communication service
// whose underlying transport protocol uses a round-robin strategy to evenly
// dispatch publications on shared subscription groups. For example, if MQTT 5
// protocol is configured in the DDA communication service the installed MQTT 5
// broker should use such a strategy for the shared subscription group
// "pcompute".
func (s workerFreeState) Free() int {
	if s.cc == 0 {
		return 0
	}
	return s.cw/s.cc - s.pcProc
}

// String makes workerFreeState satisfy the fmt.Stringer interface.
func (s workerFreeState) String() string {
	if free := s.Free(); free > 0 {
		return fmt.Sprintf("%d workers free: %d partial computations on %d workers; %d coordinators", free, s.pcProc, s.cw, s.cc)
	}
	return fmt.Sprintf("No workers free: %d partial computations on %d workers; %d coordinators", s.pcProc, s.cw, s.cc)
}
