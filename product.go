// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tempo

// Pair retains both component observations without merging their meaning.
type Pair[A, B any] struct {
	Left  A
	Right B
}

// Product describes two independent contracts over the same event type.
// Each component must satisfy [Residual]'s purity laws. Shared mutable state is
// outside this contract. The zero value is usable when both component zero
// values are usable.
type Product[L Residual[L, E, A], R Residual[R, E, B], E, A, B any] struct {
	left  L
	right R
}

// Both pairs left and right, retaining their concrete types and state.
// Composition describes a shared event history; it does not execute work.
func Both[L Residual[L, E, A], R Residual[R, E, B], E, A, B any](left L, right R) Product[L, R, E, A, B] {
	return Product[L, R, E, A, B]{left: left, right: right}
}

// Observe returns both current observations.
func (p Product[L, R, E, A, B]) Observe() Pair[A, B] {
	return Pair[A, B]{Left: p.left.Observe(), Right: p.right.Observe()}
}

// Parts returns both concrete component states for explicit transformation or
// reconstruction with [Both]. They remain pure values, with no resource ownership.
func (p Product[L, R, E, A, B]) Parts() (L, R) {
	return p.left, p.right
}

// Step returns the product of both successors after e, leaving p unchanged.
// Each component interprets e under its own contract, including explicit
// self-loops for unrelated events and modeled rejection for invalid events.
func (p Product[L, R, E, A, B]) Step(e E) Product[L, R, E, A, B] {
	p.left = step(p.left, e, L.Step)
	p.right = step(p.right, e, R.Step)
	return p
}

// step exposes component application to inlining without storing a callback.
func step[R, E any](r R, e E, next func(R, E) R) R { return next(r, e) }
