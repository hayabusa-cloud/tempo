// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package tempo

// Residual describes the remaining contract after an observed event history.
// R is the concrete state, E the event type, and O the observation type.
//
// Observe and Step must be total, deterministic, and pure, including through
// references stored in R or E. Repeated calls with equal inputs have equal
// results. Step must not change its receiver or perform the event it describes.
// Events supply external facts; invalid requests have explicit modeled behavior.
//
// The type constraint does not enforce these laws. Implementations define their
// valid initial states and zero-value meaning. Values carrying live ownership,
// locks, or one-shot resumptions do not satisfy this copyable-state contract.
type Residual[R Residual[R, E, O], E, O any] interface {
	Observe() O
	Step(E) R
}
