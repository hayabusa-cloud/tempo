# Temporal Laws

This directory is an independent Lean project for [Lean 4.35.x](lean-version), with no external package dependencies. `Tempo.lean` defines the complete library; `Audit.lean` checks required theorem entry points and transitive axioms.

Requires Elan with an installed toolchain from this series:

```sh
cd proofs
./run.sh
```

The launcher selects the highest stable patch installed in Elan for the declared series and downloads nothing. Use `./run.sh lake env lean /absolute/path/Check.lean` for other proof checks. From the repository root, `./verify.sh lean` runs the same build.

Warnings are errors. The axiom check permits only `propext`, `Classical.choice`, and `Quot.sound`, rejects local axioms, and checks every theorem originating in the `Tempo` module, including generated declarations and declarations outside its namespace.

| Result | Statement |
|--------|-----------|
| `run_append` | Concatenated event histories compose sequentially |
| `equivalent_observe`, `equivalent_step` | Behavioral equality preserves present observations and every successor |
| `equivalent_of_closed` | Every observation-preserving transition-closed relation preserves behavior |
| `Hom.run_map`, `Hom.observes_run` | An exact state mapping preserves finite execution and its observations |
| `Hom.kernel_le_equivalent` | An exact summary identifies only behaviorally equivalent states |
| `canonical_exact` | Equality in the behavioral quotient is exactly behavioral equivalence |
| `run_product`, `equivalent_product` | Product execution and behavioral equality decompose componentwise |
| `weakFair_iff` | Weak fairness is recurrent disablement or recurrent action occurrence |
| `infinitelyOften_shift`, `weakFair_shift` | Removing a finite prefix preserves recurrence and weak fairness |
| `infinitelyOften_periodic`, `weakFair_periodic` | A positive-period repetition has recurrence and fairness exactly when its finite phases contain the required witnesses |
| `Hom.trace_map`, `Hom.observes_trace`, `Hom.trace_property`, `equivalent_trace` | Exact mappings and behavioral equivalence preserve observations over a supplied infinite event stream |
| `response_of_refinement` | One admissible abstract run per concrete run transports response for every invocation identity, with future-order correspondence |
| `response_of_weakFair` | Persistent pending work, continuous enabledness, and service on occurrence imply response under weak fairness |
| `rank_stabilizes` | Every nonincreasing natural-valued sequence is eventually constant |
| `Certificate.sound` | An enabled blocking task at every stable rank excludes a fair goal-avoiding suffix |

The event alphabet and observation signature are fixed. The quotient need not have a finite or computable representation. A migration mapping preserves modeled behavior; it does not transfer physical authority. These theorems prove the mathematical model, with the [Go correspondence](../docs/semantics.md) specified separately. They establish neither real-time progress nor a Go compiler cost theorem.

For a repeated cycle, `enabled` denotes existence of a nonstuttering action successor and `occurs` denotes a nonstuttering action edge. `weakFair_periodic` proves the checker's finite fairness witness criterion under that interpretation; `weakFair_shift` extends it past any finite stem. These laws do not verify parsing, state enumeration, SCC discovery, cycle construction, or their Go implementation.

`Graph` records finite successors, action labels, and enabledness. A `Certificate` covers every triggered state without the goal, preserves that region until the goal, and assigns nonincreasing natural ranks. At each rank, one weak-fair task is continuously enabled but absent from edges preserving that rank. Rank stabilization contradicts weak fairness, establishing `Response(seed, goal)`.

The checker can emit these graph tables, certificates, and specialized theorems as a Lean file. `./verify.sh certificates` checks the shipped positive models and rejects missing-region, increasing-rank, disabled-task, and added-axiom controls. Finite conditions are proved by kernel reduction; native evaluation and added trust axioms are unnecessary. The exported artifact carries source hashes for identification, not a mechanized source-to-graph correspondence proof.
