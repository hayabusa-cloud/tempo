# Temporal Contracts

A model consists of state R, events E, observations O, a pure total transition δ:R×E→R, and observation o:R→O. Events contain locally available facts or explicitly modeled requests. An observer supplies their order; no global order or synchronized clock is assumed.

`Residual.Step` represents δ and `Residual.Observe` represents o. Implementations must be deterministic and pure, including through aliases. A rejected request has explicit behavior. An unrelated event may leave state unchanged. Termination, acceptable stopping points, failure, and outstanding obligations belong to the selected observation signature.

## Histories and Canonical State

For a finite event sequence w:

```text
run(r, [])       = r
run(r, e :: es)  = run(δ(r,e), es)
r ≈ s           iff ∀w. o(run(r,w)) = o(run(s,w))
```

This equivalence is the greatest observation-preserving relation closed under every transition. The quotient R/≈ has well-defined transitions and observations. Every exact summary identifies only states related by ≈, making the quotient the coarsest exact summary for the fixed model. A finite encoding and efficient executable transitions are additional requirements.

Include every relevant future operation before reducing state. If event-dependent return values matter, preserve them in the signature, such as through a response table or an explicitly recorded result. Identical current status flags alone do not establish equivalent futures.

## Product

`Both(left,right)` has state R×S and observations O×P:

```text
δ×((r,s),e) = (δL(r,e), δR(s,e))
o×(r,s)     = (oL(r), oR(s))
```

Each component depends only on its own pure state and the explicit event. Product execution projects to the corresponding component execution. Products associate and exchange up to tuple reassociation and permutation. The mathematical unit is a one-state constant-observation model; no additional Go export is required.

Shared mutable state, hidden environmental reads, and effectful methods violate this product contract. Separately minimizing components against different operation alphabets does not establish correctness of their combination. Cancellation, application completion, target settlement, and loan return remain distinct when future behavior distinguishes them.

## Exact State Changes

A mapping f from M to N preserves behavior when, for all admitted states and events:

```text
oN(f(r))     = oM(r)
f(δM(r,e))  = δN(f(r),e)
```

The Lean `Hom` structure records these equations. They preserve every finite observed continuation and the observations along any supplied infinite event stream. A live migration additionally needs resource-ownership and local-knowledge correspondence. It retains original temporal origins and deadlines, or supplies a justified clock translation. New operations can require information absent from the old summary.

`Product.Parts` exposes its two concrete states. A caller can apply a justified mapping and reconstruct the product with `Both`; the method supplies no mapping proof or ownership transfer.

## Conditional Response

`Response(P,Q)` means every position satisfying P has a position at or after it satisfying Q. A response at the trigger itself counts; a response before it does not. Invocation identities belong to P and Q when separate requests must be distinguished. Application response, physical settlement, and loan return have separate predicates.

`response_of_refinement` requires one admissible abstract run for each admissible concrete run, shared across all invocation identities. It maps each concrete admission to an abstract admission and reflects every corresponding future abstract response to a future concrete response. Abstract liveness then implies concrete liveness. Complete-run selection, silent physical steps, scheduling, and clock assumptions remain explicit realization obligations.

`response_of_weakFair` proves service when pending work persists until the goal, enables the task throughout that interval, and each task occurrence delivers the goal. Fairness of individual owners does not imply service for an obligation moving forever between them. The service model makes that distinction executable.

## Observation and Execution

The Go core and Lean model use the same pointwise definitions for product stepping and observation. Their correspondence is a source-level mathematical specification, not a mechanized proof of Go execution. Go methods can violate purity or allocate; the interface cannot prevent that.

The TLA+ lifetime model and Go example select one finite event alphabet, bounded clock, and concrete state representation. The correspondence test, enabled by `./verify.sh go`, enumerates each component's reachable states and checks every state/event transition against the original TLA+ operators using the included checker. It also checks the initial state and event alphabet. These finite correspondence checks use explicit field-preserving record encodings; the checker remains part of their trusted harness.

The checker separately explores the admitted finite model and its explicit weak-fairness conditions. It checks invariants, eventuality, recurrence, and response; successful liveness checks validate a ranked certificate independently of SCC discovery. Optional Lean artifacts prove the encoded graph claims using `Certificate.sound`. The parser, evaluator, source-to-graph correspondence, SCC completeness, and Go implementation remain outside that mechanized result. Neither layer supplies missing observations or establishes its own environment assumptions.

Deadline claims require explicit clock meaning, sufficiently fresh observations, and service assumptions. Logical steps and sampled ticks do not establish elapsed physical time. Preserving a chosen signature preserves a complete-execution property only when that property is proved to depend on the signature.

Use concrete owner state for direct execution. Keep state needed by future behavior, and erase verification-only state only with appropriate preservation evidence. The generic API has no dynamic storage of its own, but dictionary calls, copying, and component costs can remain. Zero allocation and zero additional runtime cost are separate claims.

## Concrete Execution

For a fixed composition, extract its concrete states with `Parts` before a hot loop, call their `Step` methods directly, and reconstruct with `Both` when the caller needs the product again. Preserve the event order and every required observation point. The [Parts example](../example_test.go) demonstrates this pattern without changing either component's contract.

Small value events avoid creating pointer-backed snapshots. When an event must contain pointers, a generic component call can make a fresh event escape even if the concrete method only reads it. Keeping that method visible to the compiler can avoid the escape. Purity still requires immutable observations through aliases; reusing mutable event storage is valid only when no residual retains it. A concrete helper can also fail to inline, so check the entire caller loop, copies and allocations.

The [comparative benchmarks](../product_test.go) cover nested products, an eight-event request/loan history, and fresh pointer events. Every workload preserves the corresponding observations. The lifetime workloads emit an observation after each event; one iteration processes the whole history. The pointer workloads compare generic composition, concrete specialization through `Parts`/`Both`, and direct components. They do not assert a fixed allocation count or speedup across compilers.

```sh
GOWORK=off GOMAXPROCS=1 go test -timeout 120s -run '^$' -bench '^Benchmark(Nested|Lifetime|Pointer)' -benchmem -benchtime=100ms -count=10 -cpu=1 .
```

Inspect escape diagnostics and emitted code for the same build. Compare only equivalent events, observations and retained state; neither these pure workloads nor their timings establish execution-owner or physical-lifetime guarantees.
