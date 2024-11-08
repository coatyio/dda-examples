// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

// Package pi provides a Partition-Compute-Accumulate distribution pattern that
// calculates π (Pi).
package pi

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"time"

	cmpt "github.com/coatyio/dda-examples/compute/computation"
)

// PiComputeData represents input or output data of PiComputation to be
// transmitted in binary gob encoding, a go only binary encoding format.
type PiComputeData struct {
	K    int64      // input: iteration value
	Prec uint       // input: precision for big.Floats
	Sum  *big.Float // output: partial sum
}

// PiComputation implements the Computation interface to calculate π (Pi) up to
// a given number of decimal digits using the Chudnovsky algorithm
// (https://en.wikipedia.org/wiki/Chudnovsky_algorithm). The workload of
// computing partial sums in the Chudnovsky formula is evenly split among
// available workers.
type PiComputation struct {
	request cmpt.ComputeRequest // only available in Partition, Accumulate, Finalize
	sum     *big.Float          // only available in Partition, Accumulate, Finalize
	prec    uint                // only available in Partition, Accumulate, Finalize
	digits  uint64              // only available in Partition, Accumulate, Finalize
	n       int64               // only available in Partition, Accumulate, Finalize
}

func (c *PiComputation) Name() string {
	return "pi"
}

func (c *PiComputation) Description() string {
	return "computes π (Pi) up to a given number of decimal digits"
}

func (c *PiComputation) Partition(request cmpt.ComputeRequest) (input <-chan cmpt.BinaryData, err error) {
	if len(request.Args) != 1 {
		return nil, fmt.Errorf("one positive integer argument required")
	}
	c.digits, err = strconv.ParseUint(request.Args[0], 10, 0)
	if err != nil {
		return nil, fmt.Errorf("one positive integer argument required")
	}
	if c.digits == 0 {
		return nil, fmt.Errorf("one positive integer argument required")
	}

	c.request = request

	// Number of decimal digits per iteration 14.181647462725477...
	dpi := math.Log10(151931373056000) // log10(640320^3 / (24 * 6 * 2 * 6))

	// Calculate number of iterations n = ceil(digits/dpi)
	d := float64(c.digits)
	c.n = int64(math.Ceil(d / dpi))

	// Calculate big.Float mantissa precision required for the given decimal places.
	c.prec = uint(int(math.Ceil(math.Log2(10)*d)) + int(math.Ceil(math.Log10(d))) + 2)

	c.sum = new(big.Float).SetPrec(c.prec).SetFloat64(0)

	in := make(chan cmpt.BinaryData, 1)

	go func() {
		defer close(in)
		// for k := c.n - 1; k >= 0; k-- { // k = n-1..0
		for k := int64(0); k < c.n; k++ { // k = 0..n-1
			if bytes, err := c.encodeData(PiComputeData{K: k, Prec: c.prec}); err != nil {
				fmt.Fprintf(c.request.OutputWriter, "Error encoding input data: %v", err)
				return
			} else {
				in <- bytes
			}
		}
	}()

	return in, nil
}

func (c *PiComputation) PartialCompute(input cmpt.BinaryData) (output cmpt.BinaryData) {
	data, err := c.decodeData(input)
	if err != nil {
		return []byte{}
	} // This algorithm is a rewrite of the original Chudnovsy formula as
	// described here:
	//
	// https://www.craig-wood.com/nick/articles/pi-chudnovsky/
	//
	// The partial sum is calculated as follows:
	//
	// ps = mk * lk / xk mk = (6k)! / ((3k)! * (k)!^3) lk = 545140134*k +
	// 13591409 xk = -262537412640768000^k
	//
	// Note that this algorithm could be further improved using binary splitting
	// and reusing the partial sum calculated by preceding iteration k-1.
	// However, interdepending inputs are not supported by the
	// Partition-Compute-Accumulate, so pi calculation is not a good example for
	// this distribution pattern.
	tmp1 := new(big.Float).SetPrec(data.Prec)
	tmp2 := new(big.Float).SetPrec(data.Prec)

	// mk = (6k)! / ((3k)! * (k)!^3)
	// Performance optimization: use big.Float instead of big.Int
	kf := new(big.Int).MulRange(2, data.K)
	k3f := new(big.Int).MulRange(data.K+1, 3*data.K)
	k3f.Mul(kf, k3f)
	k6f := new(big.Int).MulRange(3*data.K+1, 6*data.K)
	k6f.Mul(k3f, k6f)
	mkd := new(big.Int)
	mkd.Exp(kf, big.NewInt(3), nil)
	mkd.Mul(k3f, mkd)
	tmp1.SetInt(k6f)
	tmp2.SetInt(mkd)
	mk := new(big.Float).SetPrec(data.Prec)
	mk.Quo(tmp1, tmp2)

	// lk = 545140134*k + 13591409
	tmp1.SetInt64(13591409)
	tmp2.Mul(
		new(big.Float).SetPrec(data.Prec).SetFloat64(545140134),
		new(big.Float).SetPrec(data.Prec).SetFloat64(float64(data.K)))
	lk := new(big.Float).SetPrec(data.Prec)
	lk.Add(tmp1, tmp2)

	// xk = -262537412640768000^k
	exp := func(a *big.Float, b int64) *big.Float {
		res := new(big.Float).SetPrec(data.Prec).SetFloat64(1)
		for i := int64(1); i <= b; i++ {
			res.Mul(res, a)
		}
		return res
	}
	tmp1.SetInt64(-262537412640768000)
	xk := exp(tmp1, data.K)

	// ps = mk * lk / xk
	ps := new(big.Float).SetPrec(data.Prec)
	ps.Mul(mk, lk)
	ps.Quo(ps, xk)

	bytes, err := c.encodeData(PiComputeData{Sum: ps})
	if err != nil {
		return []byte{}
	}
	return bytes
}

func (c *PiComputation) PartialComputeTimeout() time.Duration {
	// As compute time for partial sums is increasing non-linear with increasing
	// k, let's provide the maximum value for a timeout.
	return time.Duration(math.MaxInt64) // infinity
}

func (c *PiComputation) Accumulate(output cmpt.BinaryData) {
	if data, err := c.decodeData(output); err != nil {
		fmt.Fprintf(c.request.OutputWriter, "Skipping undecodable output: %v\n", err)
	} else {
		c.sum.Add(c.sum, data.Sum)
	}
}

func (c *PiComputation) Finalize(start time.Time) {
	fmt.Fprintf(c.request.OutputWriter, "Computation time: %v\n", time.Since(start))
	fmt.Fprintf(c.request.OutputWriter, "  Decimal digits: %d\n", c.digits)
	fmt.Fprintf(c.request.OutputWriter, "       Precision: %d floating point mantissa bits\n", c.prec)
	fmt.Fprintf(c.request.OutputWriter, "      Iterations: %d\n", c.n)

	// pi = 426880 * 10005^0.5 / c.sum
	tmp1 := new(big.Float).SetPrec(c.prec)
	tmp1.SetInt64(426880)
	tmp2 := new(big.Float).SetPrec(c.prec)
	tmp2.SetInt64(10005)
	tmp2.Sqrt(tmp2)
	pi := new(big.Float).SetPrec(c.prec)
	pi.Mul(tmp1, tmp2)
	pi.Quo(pi, c.sum)

	pit := pi.Text('f', -1)
	fmt.Fprintf(c.request.OutputWriter, "%s\n", pit[0:2])
	tens, rest := c.digits/10, c.digits%10
	i := uint64(2)
	for cnt := uint64(1); cnt <= tens; cnt++ {
		fmt.Fprintf(c.request.OutputWriter, "%s ", pit[i:i+10])
		if cnt%5 == 0 {
			fmt.Fprintln(c.request.OutputWriter)
		}
		i += 10
	}
	fmt.Fprintf(c.request.OutputWriter, "%s\n", pit[i:i+rest])
}

func (c *PiComputation) encodeData(data PiComputeData) (cmpt.BinaryData, error) {
	// Note that a gob encoder cannot be reused by a worker. On the first
	// encoding, it encodes type information of WordFrequency, i.e
	// map[string]int, and assigns a type id. In subsequent encodings only the
	// type id is encoded. A gob decoder running in a late joining coordinator
	// cannot decode the unknown type id of such encodings as the initial type
	// information has been missed.
	encodeBuf := &bytes.Buffer{}
	encoder := gob.NewEncoder(encodeBuf)
	if err := encoder.Encode(data); err != nil {
		return nil, err
	}
	bytes := encodeBuf.Bytes()
	return bytes, nil
}

func (c *PiComputation) decodeData(inout cmpt.BinaryData) (*PiComputeData, error) {
	// Note that a gob decoder cannot be reused by a coordinator, as a worker
	// uses a new gob encoder for each encoding (see encodeOutput). If a gob
	// decoder receives an already known type information (and not its type id
	// only), it will error with "duplicate type received".
	decodeBuf := &bytes.Buffer{}
	decodeBuf.Write(inout)
	decoder := gob.NewDecoder(decodeBuf)

	var data PiComputeData
	if err := decoder.Decode(&data); err != nil {
		fmt.Println("decode error:", err)
		return nil, err
	}
	return &data, nil
}
