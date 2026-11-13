// ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

// Package tempo describes temporal contracts as explicit state transitions.
//
// A [Residual] observes its current contract state and steps to a successor when
// supplied an event. [Both] composes two independent residuals over the same event
// type. Implementations provide pure, deterministic methods; the package does
// not read clocks, perform effects, schedule work, or release resources.
//
// Observations retain distinctions required by future behavior. Cancellation,
// application completion, target settlement, and physical resource return can
// be separate facts. A residual describes these facts without granting authority
// to perform the corresponding actions.
//
// Generic composition introduces no dynamic storage of its own. Implementations
// determine allocation, copying, and concurrency safety; generic method dispatch
// may remain in compiled code and cause pointer-bearing events to escape. Known
// compositions can use [Product.Parts] and concrete component methods in a hot
// loop, reconstructing with [Both] when a product is needed again.
package tempo
