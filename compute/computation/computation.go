// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package computation defines a common API for the Partition-Compute-Accumulate
// pattern.
package computation

import (
	"io"
	"time"
)

// BinaryData represents a byte slice with a computation-specific encoding for
// partial input and output (result) data. In output data, an empty byte slice
// represents a partial computation error.
type BinaryData []byte

// A ComputeRequest represents the invocation of a named computation with
// specific input arguments and an output stream to write the final result.
type ComputeRequest struct {
	Name         string    // name of computation
	Args         []string  // ordered list of input arguments
	OutputWriter io.Writer // non-nil stream to write final output (e.g. os.Stdout)
}

// A Computation defines the individual functions invoked by the overall
// business logic of coordinator and worker components to process a specific
// compute request with the Partition-Compute-Accumulate distribution pattern.
// Realizations of this interface implement the logic of specific computations.
type Computation interface {
	// Name uniquely identifies the computation.
	Name() string

	// Description provides a short one-line description of the computation.
	Description() string

	// Partition implements the partitioning logic on the side of a coordinator.
	//
	// This function is invoked by a coordinator on the given incoming
	// ComputeRequest returning a receive-only channel from which successive
	// input data for partial computations can be pulled on demand. An error
	// must be returned along with a nil channel if the given ComputeRequest is
	// not valid, i.e. if it has invalid arguments.
	//
	// Whenever partitioning is completed the input channel must be closed by
	// this function signaling the coordinator that all partial input has been
	// emitted.
	//
	// Input data is encoded as BinaryData in a computation-specific encoding.
	Partition(request ComputeRequest) (input <-chan BinaryData, err error)

	// PartialCompute implements the computation logic on the side of a worker.
	//
	// This function is invoked by a worker each time a new partial computation
	// should be performed with the given input data returning output data to be
	// sent back to the coordinator.
	//
	// If the partial computation fails with a computational error it must be
	// specified as empty output data, i.e. as a byte slice of zero length. If
	// this function fails due to invalid input or output data encoding, output
	// must also be set to a zero length byte array. In both cases, the
	// receiving coordinator fails fast as the partial computation cannot be
	// completed successfully by any worker.
	//
	// If the worker cannot perform the given partial computation for other
	// reasons, such as required resources not being available currently, nil
	// should be returned to indicate that no output data should be sent back to
	// the coordinator. In this case, PartialComputeTimeout will trigger
	// eventually on the coordinator side causing resubmission of the
	// unresponsive partial computation.
	//
	// Input and output data is encoded as BinaryData in a computation-specific
	// encoding.
	PartialCompute(input BinaryData) (output BinaryData)

	// PartialComputeTimeout gets a computation-specific timeout indicating when
	// a coordinator should stop waiting for partial result data from a worker.
	//
	// Carefully chose a timeout value that covers the running time of any
	// potential partial computation including the roundtrip time of associated
	// communication data transfer.
	//
	// Whenever this timeout is triggered on the side of a coordinator it should
	// try to resubmit the unresponsive partial computation. This also ensures
	// that partial compute messages which are lost on the network
	// infrastructure are resubmitted.
	PartialComputeTimeout() time.Duration

	// Accumulate implements the accumulation logic on the side of a
	// coordinator.
	//
	// This function is invoked by a coordinator whenever partial output data
	// from a worker is received. The caller must ensure that multiple
	// goroutines do not invoke this function concurrently.
	//
	// Output data is encoded as BinaryData in a computation-specific encoding.
	Accumulate(output BinaryData)

	// Finalize yields the final result of the computation and writes it to the
	// output stream given in the initial ComputeRequest passed to Partition.
	//
	// This function is invoked once by a coordinator when the computation has
	// finished successfully, i.e. when all partial inputs emitted by Partition
	// have been accumulated. The given start time records the time when the
	// Partition-Compute-Accumulate pattern has been started by the coordinator.
	// It may be used to output the total running time of the computation.
	Finalize(start time.Time)
}
