// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tempo_test

import (
	"fmt"

	"code.hybscloud.com/tempo"
)

type event uint8

const (
	cancel event = iota
	complete
	settle
	returnLoan
	move
	tick
	invalid event = 255
)

// This bounded clock is model input, not an elapsed-time measurement.
const (
	origin   = 0
	deadline = 2
	maxTime  = 3
)

type request struct {
	Cancelled, Complete, Expired, Rejected bool
	Now, Origin, Deadline, Owner           int
}

func (r request) Observe() request { return r }

func (r request) Step(e event) request {
	switch e {
	case cancel:
		r.Cancelled = true
	case complete:
		r.Complete = true
	case move:
		r.Owner = 1
	case tick:
		if r.Now < maxTime {
			r.Now++
		}
		r.Expired = r.Expired || (!r.Complete && r.Now > r.Deadline)
	case settle, returnLoan:
	default:
		r.Rejected = true
	}
	return r
}

type loan struct {
	Settled, Returned, Rejected bool
	Custodian                   int
}

func (l loan) Observe() loan { return l }

func (l loan) Step(e event) loan {
	switch e {
	case settle:
		l.Settled = true
	case returnLoan:
		if l.Settled {
			l.Returned = true
		} else {
			l.Rejected = true
		}
	case cancel, complete, move, tick:
	default:
		l.Rejected = true
	}
	return l
}

func ExampleBoth() {
	r := tempo.Both(request{Origin: origin, Deadline: deadline}, loan{})
	r = r.Step(cancel)
	r = r.Step(complete)
	o := r.Observe()
	fmt.Println(o.Left.Complete, o.Right.Returned)

	// Target settlement permits returning the physical loan independently.
	r = r.Step(settle)
	r = r.Step(returnLoan)
	o = r.Observe()
	fmt.Println(o.Left.Complete, o.Right.Returned)
	// Output:
	// true false
	// true true
}

func ExampleProduct_Parts() {
	r := tempo.Both(request{Origin: origin, Deadline: deadline}, loan{})
	app, resource := r.Parts()
	for _, e := range []event{cancel, complete, settle, returnLoan} {
		app, resource = app.Step(e), resource.Step(e)
	}
	r = tempo.Both(app, resource)
	o := r.Observe()
	fmt.Println(o.Left.Complete, o.Right.Settled, o.Right.Returned)
	// Output: true true true
}
