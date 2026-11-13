# tempo

Pure, composable temporal contracts for Go.

- Implement `Residual` with total, deterministic, pure `Observe` and `Step` methods. `Step` returns a successor without mutating existing state or aliases.
- Supply external facts through events. Callers own clocks, I/O, scheduling, and resource lifetimes; residuals carry no execution or ownership authority.
- Use `Both` to compose independent residuals over the same events; use `Parts` to access their concrete states.
- Define valid initial states, zero-value meaning, and rejection behavior. Preserve distinctions needed by future events, including cancellation, completion, settlement, and resource return.
- Measure allocation and dispatch costs in the actual caller.

The [semantics](docs/semantics.md) define the contract. [Lean proofs](proofs/README.md), [TLA+ models](spec/README.md), and the [finite checker](tools/check/README.md) document verification scope. Run `./verify.sh` to verify the repository.
