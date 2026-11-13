// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tempo_test

import (
	"testing"

	"code.hybscloud.com/tempo"
)

type sum uint64

func (s sum) Observe() uint64   { return uint64(s) }
func (s sum) Step(e uint64) sum { return s + sum(e) }

type parity uint64

func (p parity) Observe() uint64      { return uint64(p) }
func (p parity) Step(e uint64) parity { return p ^ parity(e) }

var result tempo.Pair[uint64, uint64]
var nestedResult tempo.Pair[tempo.Pair[uint64, uint64], uint64]
var lifetimeResult tempo.Pair[request, loan]
var lifetimeTrace = [...]event{tick, move, cancel, returnLoan, complete, settle, returnLoan, invalid}

type pointerSum uint64

func (s pointerSum) Observe() uint64 { return uint64(s) }
func (s pointerSum) Step(e *uint64) pointerSum {
	if e == nil {
		return s
	}
	return s + pointerSum(*e)
}

type pointerParity uint64

func (p pointerParity) Observe() uint64 { return uint64(p) }
func (p pointerParity) Step(e *uint64) pointerParity {
	if e == nil {
		return p
	}
	return p ^ pointerParity(*e)
}

type pointerProduct = tempo.Product[pointerSum, pointerParity, *uint64, uint64, uint64]

func stepPointerProduct(p pointerProduct, e *uint64) pointerProduct {
	left, right := p.Parts()
	return tempo.Both(left.Step(e), right.Step(e))
}

func observePointerProduct(p pointerProduct) tempo.Pair[uint64, uint64] {
	left, right := p.Parts()
	return tempo.Pair[uint64, uint64]{Left: left.Observe(), Right: right.Observe()}
}

func TestProductHistory(t *testing.T) {
	initial := tempo.Both(sum(7), parity(3))
	a := initial
	var wantSum, wantParity uint64 = 7, 3
	for _, e := range []uint64{0, 1, 3, 7, ^uint64(0), 13} {
		a = a.Step(e)
		wantSum += e
		wantParity ^= e
		if got := a.Observe(); got.Left != wantSum || got.Right != wantParity {
			t.Fatalf("observation %+v, want (%d, %d)", got, wantSum, wantParity)
		}
	}
	if got := initial.Observe(); got != (tempo.Pair[uint64, uint64]{Left: 7, Right: 3}) {
		t.Fatalf("initial value changed: %+v", got)
	}
}

func TestProductReassociation(t *testing.T) {
	a := tempo.Both(tempo.Both(sum(1), parity(2)), sum(3))
	b := tempo.Both(sum(1), tempo.Both(parity(2), sum(3)))
	for _, e := range []uint64{4, 9, 17} {
		a, b = a.Step(e), b.Step(e)
		x, y := a.Observe(), b.Observe()
		if x.Left.Left != y.Left || x.Left.Right != y.Right.Left || x.Right != y.Right.Right {
			t.Fatalf("reassociation changed observations: %+v, %+v", x, y)
		}
	}
}

func TestProductParts(t *testing.T) {
	r := tempo.Both(sum(7), parity(3)).Step(9)
	left, right := r.Parts()
	if tempo.Both(left, right) != r {
		t.Fatal("reconstruction changed the product")
	}
	if left.Observe() != 16 || right.Observe() != 10 {
		t.Fatal("component projections lost state")
	}
}

func TestLifetimeBoundaries(t *testing.T) {
	r := tempo.Both(request{Origin: origin, Deadline: deadline}, loan{})
	before := r
	r = r.Step(returnLoan)
	if o := r.Observe(); !o.Right.Rejected || o.Right.Returned || o.Right.Settled {
		t.Fatalf("premature return: %+v", o)
	}
	r = before.Step(cancel).Step(complete).Step(move)
	if o := r.Observe(); !o.Left.Complete || o.Right.Returned || o.Right.Settled || o.Right.Custodian != 0 || o.Left.Owner != 1 {
		t.Fatalf("application events changed physical lifetime: %+v", o)
	}
	r = before.Step(tick).Step(tick).Step(move).Step(tick)
	if o := r.Observe(); !o.Left.Expired || o.Left.Deadline != deadline || o.Left.Origin != origin {
		t.Fatalf("move reset the original deadline: %+v", o)
	}
}

func TestLifetimeReachability(t *testing.T) {
	type state = tempo.Product[request, loan, event, request, loan]
	initial := tempo.Both(request{Origin: origin, Deadline: deadline}, loan{})
	seen := map[state]bool{initial: true}
	queue := []state{initial}
	edges := 0
	for head := 0; head < len(queue); head++ {
		current := queue[head]
		before := current.Observe()
		for _, e := range []event{cancel, complete, settle, returnLoan, move, tick, invalid} {
			edges++
			next := current.Step(e)
			o := next.Observe()
			if o.Right.Returned && !o.Right.Settled {
				t.Fatal("loan returned before settlement")
			}
			if o.Left.Origin != origin || o.Left.Deadline != deadline || o.Right.Custodian != 0 {
				t.Fatal("original obligation or physical custodian changed")
			}
			if o.Left.Now < before.Left.Now || o.Left.Now > maxTime {
				t.Fatal("clock observation left its bounded monotone domain")
			}
			if (e == cancel || e == complete || e == move) && o.Right != before.Right {
				t.Fatal("application event changed the physical contract")
			}
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
	t.Logf("%d reachable states; %d transitions", len(seen), edges)
}

func TestProductAllocations(t *testing.T) {
	allocs := testing.AllocsPerRun(1000, func() {
		r := tempo.Both(sum(0), parity(0))
		for i := range uint64(100) {
			r = r.Step(i)
		}
		result = r.Observe()
	})
	if allocs != 0 {
		t.Fatalf("%g allocations/run", allocs)
	}
	if n := testing.AllocsPerRun(1000, func() {
		p := tempo.Both(tempo.Both(sum(0), parity(0)), sum(0))
		for i := range uint64(100) {
			p = p.Step(i)
			nestedResult = p.Observe()
		}
	}); n != 0 {
		t.Fatalf("nested product: %g allocations/run", n)
	}
	if n := testing.AllocsPerRun(1000, func() {
		p := tempo.Both(request{Origin: origin, Deadline: deadline}, loan{})
		for _, e := range lifetimeTrace {
			p = p.Step(e)
			lifetimeResult = p.Observe()
		}
	}); n != 0 {
		t.Fatalf("lifetime product: %g allocations/run", n)
	}
}

func TestConcretePointerProduct(t *testing.T) {
	p := tempo.Both(pointerSum(7), pointerParity(3))
	reference := p
	a, b := uint64(7), uint64(3)
	for i := range uint64(1000) {
		value := i ^ (^uint64(0) >> (i % 64))
		e := &value
		if i%5 == 0 {
			e = nil
		} else {
			a, b = a+value, b^value
		}
		p, reference = stepPointerProduct(p, e), reference.Step(e)
		if p != reference || observePointerProduct(p) != (tempo.Pair[uint64, uint64]{Left: a, Right: b}) {
			t.Fatalf("concrete transition differs at event %d", i)
		}
	}
	if n := testing.AllocsPerRun(1000, func() {
		p := tempo.Both(pointerSum(0), pointerParity(0))
		for i := range uint64(100) {
			p = stepPointerProduct(p, &i)
			result = observePointerProduct(p)
		}
	}); n != 0 {
		t.Fatalf("concrete pointer product: %g allocations/run", n)
	}
}

func FuzzProduct(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0, 1, 255, 3, 17})
	f.Fuzz(func(t *testing.T, events []byte) {
		r := tempo.Both(sum(0), parity(0))
		var a, b uint64
		for _, e := range events {
			r = r.Step(uint64(e))
			a += uint64(e)
			b ^= uint64(e)
		}
		if got := r.Observe(); got.Left != a || got.Right != b {
			t.Fatalf("got %+v, want (%d, %d)", got, a, b)
		}
	})
}

func BenchmarkProduct(b *testing.B) {
	r := tempo.Both(sum(0), parity(0))
	b.ReportAllocs()
	for b.Loop() {
		r = r.Step(1)
		result = r.Observe()
	}
}

func BenchmarkDirect(b *testing.B) {
	var a, c uint64
	b.ReportAllocs()
	for b.Loop() {
		a++
		c ^= 1
		result = tempo.Pair[uint64, uint64]{Left: a, Right: c}
	}
}

func BenchmarkNestedProduct(b *testing.B) {
	p := tempo.Both(tempo.Both(sum(0), parity(0)), sum(0))
	b.ReportAllocs()
	for b.Loop() {
		p = p.Step(1)
		nestedResult = p.Observe()
	}
}

func BenchmarkNestedDirect(b *testing.B) {
	var a, c, d uint64
	b.ReportAllocs()
	for b.Loop() {
		a, c, d = a+1, c^1, d+1
		nestedResult = tempo.Pair[tempo.Pair[uint64, uint64], uint64]{Left: tempo.Pair[uint64, uint64]{Left: a, Right: c}, Right: d}
	}
}

func BenchmarkLifetimeProduct(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		p := tempo.Both(request{Origin: origin, Deadline: deadline}, loan{})
		for _, e := range lifetimeTrace {
			p = p.Step(e)
			lifetimeResult = p.Observe()
		}
	}
}

func BenchmarkLifetimeDirect(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		app, resource := request{Origin: origin, Deadline: deadline}, (loan{})
		for _, e := range lifetimeTrace {
			app, resource = app.Step(e), resource.Step(e)
			lifetimeResult = tempo.Pair[request, loan]{Left: app.Observe(), Right: resource.Observe()}
		}
	}
}

func BenchmarkPointerProduct(b *testing.B) {
	p := tempo.Both(pointerSum(0), pointerParity(0))
	b.ReportAllocs()
	for b.Loop() {
		e := uint64(1)
		p = p.Step(&e)
		result = p.Observe()
	}
}

func BenchmarkPointerConcrete(b *testing.B) {
	p := tempo.Both(pointerSum(0), pointerParity(0))
	b.ReportAllocs()
	for b.Loop() {
		e := uint64(1)
		p = stepPointerProduct(p, &e)
		result = observePointerProduct(p)
	}
}

func BenchmarkPointerDirect(b *testing.B) {
	var a pointerSum
	var c pointerParity
	b.ReportAllocs()
	for b.Loop() {
		e := uint64(1)
		a, c = a.Step(&e), c.Step(&e)
		result = tempo.Pair[uint64, uint64]{Left: a.Observe(), Right: c.Observe()}
	}
}
