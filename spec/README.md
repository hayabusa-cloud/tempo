# Temporal Models

[Lifetime.tla](Lifetime.tla) composes independent request and physical-loan transitions over the same events. The [Go example](../example_test.go) uses the same state fields and branches. This is a bounded example of `Product`, not a kernel, protocol, or Go implementation proof.

| Configuration | Expected result | Meaning |
|---------------|-----------------|---------|
| `Safety.cfg` | Pass | Return requires settlement; original obligation and physical custodian remain unchanged |
| `Progress.cfg` | Pass | Completion and return follow under the three stated weak-fairness assumptions |
| `Unsafe.cfg` | Invariant counterexample | Returning before settlement breaks `ReturnedAfterSettlement` |
| `Unfair.cfg` | Temporal counterexample | Stuttering or starving return prevents unconditional eventual return |

`move` changes the request owner while retaining the original deadline and loan custodian. `cancel` changes neither completion nor physical custody. `tick` supplies one bounded logical-clock observation; it does not read wall time. The model records expiry, but assumes no deadline guarantee or physical-time service bound.

The positive configurations use deadline 2 and a clock bounded by 3. Terminal states admit stuttering. `FairSpec` explicitly requires weak fairness of completion, settlement, and return; these are environment assumptions, not properties established by the Go package.

[Service.tla](Service.tla) adds a repeated idle → pending → answered lifecycle with two possible owners. `Serve` follows the obligation across owner changes; `ServeAt(w)` names one owner. Every new admission follows retirement, so an earlier answer cannot discharge a later pending interval.

| Configuration | Expected result | Meaning |
|---------------|-----------------|---------|
| `Service.cfg` | Pass | Weak fairness of admission, service, and retirement establishes repeated response and recurring answers |
| `Owner.cfg` | Temporal counterexample | Repeated owner changes can starve service while each owner's task remains weakly fair |
| `Starved.cfg` | Temporal counterexample | Without fairness, an infinite stuttering suffix need not answer |

From the repository root, run all seven checks with the included [Go checker](../tools/check/README.md):

```sh
./verify.sh tla
```

The harness requires the three positive checks to succeed and the four negative checks to return replayed counterexamples for their named properties. Unsupported input, resource exhaustion, and unrelated errors do not count as counterexamples. It installs nothing and builds the checker in a temporary directory. `./verify.sh certificates` additionally checks positive graph certificates with Lean and rejects corrupted evidence.

The files use ordinary TLA+ notation. The included checker implements the constructive finite fragment described in its documentation, including stuttering and weak fairness; it is not a full-language TLA+ implementation. See the official [model configuration guide](https://tla.msr-inria.inria.fr/tlatoolbox/doc/model/overview-page.html) for the distinction between invariants, temporal properties, and fairness assumptions.
