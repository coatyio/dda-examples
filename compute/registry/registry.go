// SPDX-FileCopyrightText: © 2023 Siemens AG
// SPDX-License-Identifier: MIT

package registry

import (
	"slices"

	cmpt "github.com/coatyio/dda-examples/compute/computation"
	"github.com/coatyio/dda-examples/compute/registry/fac"
	"github.com/coatyio/dda-examples/compute/registry/pi"
	"github.com/coatyio/dda-examples/compute/registry/wf"
)

// A Registry manages predefined computations for lookup by coordinators and
// workers.
type Registry struct {
	computations map[string]cmpt.Computation
}

// NewRegistry returns a Registry with all predefined computations.
func NewRegistry() *Registry {
	reg := &Registry{make(map[string]cmpt.Computation)}

	// Register predefined computations in subpackages.
	reg.Register(&pi.PiComputation{})
	reg.Register(&wf.WordFrequencyComputation{})
	reg.Register(&fac.FacComputation{})

	return reg
}

// Register the given computation.
func (r *Registry) Register(cmp cmpt.Computation) {
	r.computations[cmp.Name()] = cmp
}

// ComputationByName gets the computation of the given name if it has been
// registered; otherwise nil.
func (r *Registry) ComputationByName(name string) cmpt.Computation {
	if v, ok := r.computations[name]; ok {
		return v
	}
	return nil
}

// Names gets a slice of all defined computation names ordered ascendingly.
func (r *Registry) Names() []string {
	names := make([]string, len(r.computations))
	i := 0
	for k := range r.computations {
		names[i] = k
		i++
	}
	slices.Sort(names)
	return names
}
