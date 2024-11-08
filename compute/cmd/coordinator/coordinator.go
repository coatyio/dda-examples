// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

/*
Starts a coordinator that receives a compute request from the command line,
splits it into partial computations that are dispatched to workers, accumulates
partial results from workers, and yields the final result.

For usage details, run coordinator with the command line flag -h or --help.
*/
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"syscall"

	"github.com/coatyio/dda-examples/compute/clog"
	comp "github.com/coatyio/dda-examples/compute/components"
	cmpt "github.com/coatyio/dda-examples/compute/computation"
	"github.com/coatyio/dda-examples/compute/registry"
)

func main() {
	var ddaAddress string
	var help bool
	var log bool

	// Parsing command line flags and arguments.
	flag.Usage = usage
	flag.StringVar(&ddaAddress, "d", ":8900", "address (host:port) of DDA sidecar gRPC API")
	flag.BoolVar(&help, "h", false, "Show usage information")
	flag.BoolVar(&log, "l", false, "Show logging output (for debugging)")
	flag.Parse()

	computation := flag.Arg(0)

	if help || computation == "" {
		usage()
		os.Exit(0)
	}

	if log {
		clog.Enable() // turn on application logging
	}

	// Handle SIGTERM.
	signaled := make(chan struct{})
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer close(signaled)
		fmt.Printf("Terminating coordinator on signal %v...\n", <-sigCh)
	}()

	request := cmpt.ComputeRequest{
		Name:         computation,
		Args:         slices.Delete(flag.Args(), 0, 1),
		OutputWriter: os.Stdout,
	}

	fmt.Printf("Starting a coordinator to compute %s%v...\n", request.Name, request.Args)

	ctx, cancel := context.WithCancel(context.Background()) // triggers graceful shutdown of coordinator
	completed := make(chan struct{})                        // signals completion of coordinator shutdown
	go comp.NewCoordinator().Start(ctx, request, ddaAddress, completed)

	// Wait for coordinator to shut down gracefully, triggered either on its own
	// or after first termination signal is received.
	for {
		select {
		case <-signaled:
			signaled = nil // skip this case after first termination signal
			cancel()       // start shutting down coordinator gracefully
		case <-completed:
			return
		}
	}
}

func usage() {
	fmt.Printf(`usage: coordinator [-h|--help] [-l] [-d ddaAddress] computation [arguments...]

Starts a coordinator component for a computation with specific input arguments.

The following distributed computations are predefined:

`)
	printComputations()
	fmt.Println("\nFlags:")
	flag.PrintDefaults()
}

func printComputations() {
	reg := registry.NewRegistry()
	maxLen := 0
	for _, name := range reg.Names() {
		l := len(name)
		if len(name) > maxLen {
			maxLen = l
		}
	}
	for _, name := range reg.Names() {
		fmt.Printf("  %*s: %s\n", maxLen, name, reg.ComputationByName(name).Description())
	}
}
