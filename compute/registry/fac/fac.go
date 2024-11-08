// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package fac provides a Partition-Compute-Accumulate distribution pattern that
// calculates the factorial function of a given non-negative integer.
package fac

import (
	"fmt"
	"math/big"
	"strconv"
	"time"

	cmpt "github.com/coatyio/dda-examples/compute/computation"
)

// FacComputation implements the Computation interface to compute the factorial
// function n! of a given non-negative integer.
//
// Note that this implementation is just meant for demonstration and testing
// purposes to show and understand the principle behind the
// Partition-Compute-Accumulate distribution pattern.
type FacComputation struct {
	request cmpt.ComputeRequest // only available in Partition, Accumulate, Finalize
	result  *big.Int            // only available in Partition, Accumulate, Finalize
}

func (c *FacComputation) Name() string {
	return "fac"
}

func (c *FacComputation) Description() string {
	return "computes factorial of a given non-negative integer (for demonstration and testing purposes)"
}

func (c *FacComputation) Partition(request cmpt.ComputeRequest) (input <-chan cmpt.BinaryData, err error) {
	if len(request.Args) != 1 {
		return nil, fmt.Errorf("one integer argument required")
	}
	n, err := strconv.ParseUint(request.Args[0], 10, 0)
	if err != nil {
		return nil, fmt.Errorf("one non-negative integer argument required")
	}

	c.request = request
	c.result = big.NewInt(1)

	// Note: to speed up provisioning of inputs with longer lasting partitioning
	// operations you may use a buffered channel of a certain size to provide a
	// fixed-size FIFO queue with limited concurrent send/receive operations.
	in := make(chan cmpt.BinaryData, 1)

	go func() {
		defer close(in)
		for i := uint64(2); i <= n; i++ {
			// Transmit input in UTF-8 encoded binary serialization format.
			in <- []byte(strconv.FormatUint(i, 10))
		}
	}()

	return in, nil
}

func (c *FacComputation) PartialCompute(input cmpt.BinaryData) (output cmpt.BinaryData) {
	time.Sleep(1 * time.Second) // use constant delay for demonstration purposes
	return input                // identity function
}

func (c *FacComputation) PartialComputeTimeout() time.Duration {
	return 5 * time.Second
}

func (c *FacComputation) Accumulate(output cmpt.BinaryData) {
	if n, err := strconv.ParseUint(string(output), 10, 0); err != nil {
		fmt.Fprintf(c.request.OutputWriter, "Skipping undecodable output: %v\n", err)
	} else {
		c.result.Mul(c.result, big.NewInt(int64(n)))
	}
}

func (c *FacComputation) Finalize(start time.Time) {
	fmt.Fprintf(c.request.OutputWriter, "Computation time: %v\n", time.Since(start))
	fmt.Fprintf(c.request.OutputWriter, "Computation %s%v = %v\n", c.request.Name, c.request.Args, c.result)
}
