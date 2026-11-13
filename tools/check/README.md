# tempo-check

An exact finite-state checker for a constructive TLA+ fragment. It explores every reachable state, checks invariants, eventuality, recurrence, and response, and emits replayed safety traces or weakly fair lassos. Successful liveness checks validate ranked certificates independently of cycle discovery. It is a standalone Go command with no external dependencies.

```sh
cd tools/check
GOWORK=off go run -buildvcs=false . -config ../../spec/Progress.cfg ../../spec/Lifetime.tla
```

Flags precede the module path. From this directory, `../../verify.sh tla` checks both bundled models and their counterexample controls. `GOWORK=off go test -buildvcs=false -timeout 120s -race ./...` runs the command tests and the independent small-graph fairness oracle.

## Results

The command writes one JSON object:

| Exit status | JSON `status` | Meaning |
|-------------|---------------|---------|
| 0 | `ok` | Exploration completed and all requested checks passed |
| 1 | `violation` | A named property has a counterexample replayed against the source relations |
| 2 | `not_checked` | Unsupported input, an undefined expression, a checking limit, or an error prevented a result |

`complete` reports whether state exploration finished; it does not override `status`.

A safety trace contains a shortest reachable prefix ending in a violating state. A liveness trace contains `stem`, `cycle`, and a zero-based `trigger` index into the stem; the last stem state is the first cycle state, and the cycle repeats its first state at the end. The goal is false from the trigger onward. Earlier goals are permitted for response and recurrence; eventuality has trigger zero. Fairness refers to nonstuttering action occurrences. The checker re-evaluates initial membership, every edge, the trigger and failed predicates, and every fairness witness before emitting a trace.

## Accepted Fragment

One module declares constants, state variables, and pure nonrecursive operators. `EXTENDS Naturals` and `EXTENDS Integers` supply the supported arithmetic. Configuration directives are `CONSTANTS`, `SPECIFICATION`, `INVARIANTS`, and `PROPERTIES`, including their singular forms. Constant values are closed finite expressions; overrides, constraints, symmetry, and external modules are unsupported.

Module and configuration files accept LF, CRLF, and CR line endings. Raw line endings inside strings are rejected; escaped `\r` and `\n` remain string data. Proof hashes identify the original source bytes.

Identifiers start with a letter or underscore and contain at least one letter. Unsupported tokens such as `\intersect`, `\AA`, `\EE`, and `--` are rejected intact, without interpreting their shorter supported prefixes. Separate subtraction and unary negation with whitespace, comments, or parentheses.

Values are Booleans, exact integers, strings, finite sets, tuples, and records. Expressions include equality, Boolean operators, integer `+`, `-`, `*`, comparisons and ranges, membership, field selection, record sets, `IF`, unambiguous `CASE`, sequential value-only `LET`, finite `\E`/`\A`, and nonoverlapping record `EXCEPT` updates with `@`. `Nat` and `Int` support membership but cannot be enumerated or stored.

Cross-sort comparisons and heterogeneous sets with incomparable elements are rejected. Reachable values retain the same sort at every variable, record-field, tuple-position, and set-element path, including paths first populated after an empty set.

Use infix Boolean expressions, such as `(A) /\ (B)`; leading `/\` or `\/` lists depend on indentation and are rejected. Parenthesize mixed conjunction/disjunction and chained implications or comparisons.

Precedence follows TLA+ operator ranges: write `<> (x = 1)` and `1 * (-2)`. Overlapping ranges require parentheses unless repeating the same left-associative infix operator.

Initial and next predicates are finite alternatives of assignments and guards. Use equality or finite membership to assign a whole variable; use `UNCHANGED` for unchanged next-state variables. Every viable alternative assigns every declared variable exactly once. Initial right-hand sides depend only on constants and finite bound values; next right-hand sides depend only on the current state and such values. Conjunction combines simultaneous assignments. State guards run before updates, and post-state guards run after the complete update. Finite existential choices and state-guarded `IF` actions are supported. Implicit relational solving and primed right-hand dependencies are unsupported.

Specifications have the form `Init /\ [][Next]_vars`, optionally conjoined with `WF_vars(A)` assumptions. The subscript names a variable or tuple containing every declared state variable exactly once. All fairness actions must generate only Next edges when they change the state. Invariants are state predicates. Temporal properties are `[]P`, `<>P`, `[]<>P`, or `[](P => <>Q)` for state predicates P and Q. A response at its trigger position satisfies the obligation. Strong fairness, other temporal nesting, temporal quantification, and refinement mappings are unsupported.

Operators take value parameters with lexical scope and refer only to earlier declarations. Every definition is checked, including unused definitions. Parameters, quantified names, and `LET` names cannot shadow names already visible at their declaration. Later declarations do not capture earlier local bindings. Value parameters cannot be applied as operators. `LET` bindings cannot define parameterized operators, refer forward, or recurse. Prime applies only to declared variables; `UNCHANGED` names declared variables or their tuple aliases. These restrictions make transition enumeration constructive and explicit. An empty initial set is reported as `not_checked`.

## Algorithm and Bounds

Breadth-first exploration uses canonical structural values and exact state keys, with no probabilistic fingerprints. It adds stuttering at every state and does not use invariants to prune the graph. Duplicate state edges are merged. Liveness search follows goal-avoiding suffixes from selected seeds: initial states for eventuality, all reachable states for recurrence, and reachable trigger states for response. Each recurrent strongly connected component must contain, for every weak-fairness action, either a disabled state or an internal nonstuttering action edge. A closed walk visits all selected witnesses before returning to its entry. Component membership uses one reusable bitmap; rejected components clear only their own members.

When no fair component exists, a separate pass validates a certificate covering all seed states without the goal. The avoiding region is closed until the goal, its natural rank never increases, and each stable rank has a continuously enabled fair task absent from its edges. Enabledness comes from the full graph, including edges reaching the goal. Every nonincreasing natural rank eventually stabilizes, so no fair path can remain in this region forever.

| Flag | Default | Bound |
|------|---------|-------|
| `-max-states` | 50000 | Reachable states |
| `-max-transitions` | 1000000 | Distinct state edges, including stuttering |
| `-max-values` | 10000 | Enumeration size, action alternatives or clauses, and stored value nodes |
| `-max-steps` | 10000000 | Expression evaluations and action expansions |

Each source file is limited to 1 MiB, syntax and stored values to depth 256, operator expansion to depth 128, and integers to 65536 bits. Record-set expansion checks the total materialized nodes before constructing the Cartesian product, including nested record sets; compact descriptors cannot bypass the value bound. A limit aborts checking; it never truncates arithmetic or changes the modeled state space. The command owns heap storage for parsing, values, graph exploration, and counterexamples. It runs outside the runtime package.

## Kernel Certificates

Pass `-proof /absolute/path/Check.lean` to emit a Lean artifact after all requested properties pass. The destination must not exist; write errors return `not_checked`. Violations and incomplete checks produce no positive artifact. The output records source hashes, the exact finite graph, predicate tables, certificates, specialized theorems, and a transitive axiom audit.

Requires Elan with an installed Lean 4.35.x toolchain. Build the local library with `../../verify.sh lean`. From `proofs/`, run `./run.sh lake env lean /absolute/path/Check.lean`. Exporting a file does not run Lean; the JSON `proof` field names an artifact awaiting this check. Kernel reduction establishes its finite conditions and the local [Lean library](../../proofs/README.md) proves certificate soundness. Only the standard axioms `propext`, `Classical.choice`, and `Quot.sound` are permitted.

These proofs concern the encoded graph and predicates. Source hashes identify input bytes; they do not prove interpretation. The parser, evaluator, source-to-graph correspondence, SCC completeness, and Go implementation remain tested and reviewed boundaries. Language semantics follow [Specifying Systems](https://lamport.azurewebsites.net/tla/book-02-08-08.pdf) within the stated fragment. Strong fairness and arbitrary temporal logic require separate methods.

For repeated workload measurements, run `GOWORK=off go test -buildvcs=false -timeout 120s -run '^$' -bench BenchmarkLifetime -count 10`. This measures the complete command, including process startup. Set `TEMPO_CHECK_COVERDIR` to an existing directory during tests to collect coverage from the child checker processes; a Go distribution containing `covdata` can inspect it with `go tool covdata`.
