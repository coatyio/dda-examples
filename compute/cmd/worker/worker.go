// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

/*
Starts a specific number of worker components that perform distributed
partial computations requested by a coordinator.

For usage details, run worker with the command line flag -h or --help.
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	"github.com/coatyio/dda-examples/compute/clog"
	comp "github.com/coatyio/dda-examples/compute/components"
	"github.com/coatyio/dda/plog"
)

const (
	defaultWorkers = 10  // default number of workers
	maxWorkers     = 100 // maximum number of workers
)

func main() {
	var brokerUrl string
	var help bool
	var log bool

	flag.Usage = usage
	flag.StringVar(&brokerUrl, "b", "tcp://localhost:1883", "MQTT 5 Broker URL for DDA communication service")
	flag.BoolVar(&help, "h", false, "Show usage information")
	flag.BoolVar(&log, "l", false, "Show logging output (for debugging)")
	flag.Parse()

	if flag.Arg(1) != "" || help {
		usage()
		os.Exit(0)
	}

	if !log {
		plog.Disable() // disable DDA logging
	} else {
		clog.Enable() // turn on application logging
	}

	// Accept any number of workers between 1 and maxWorkers.
	count, err := strconv.Atoi(flag.Arg(0))
	if err != nil && flag.Arg(0) == "" {
		count = 10
	} else if err != nil || count < 1 || count > maxWorkers {
		fmt.Printf("Number of workers must be between 1 and %d\n", maxWorkers)
		return
	}

	// Handle SIGTERM.
	signaled := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer close(signaled)
		fmt.Printf("Terminating workers on signal %v...\n", <-sigCh)
	}()

	fmt.Printf("Starting %d workers...\n", count)

	ctx, cancel := context.WithCancel(context.Background()) // triggers graceful shutdown of workers
	completed := make(chan struct{})                        // signals completion of worker shutdowns
	for i := 0; i < count; i++ {
		go comp.NewWorker().Start(ctx, brokerUrl, completed)
	}

	// Wait for all workers to shut down gracefully, triggered either on their
	// own or after first termination signal is received.
	for sw := count; sw > 0; {
		select {
		case <-signaled:
			signaled = nil // skip this case after first termination signal
			cancel()       // start shutting down workers gracefully
		case <-completed:
			sw--
		}
	}
}

func usage() {
	fmt.Printf(`usage: worker [-h|--help] [-l] [-b brokerUrl] [count]

Starts the given number of worker components (default %d, maximum %d).

Flags:
`, defaultWorkers, maxWorkers)
	flag.PrintDefaults()
}
