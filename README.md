# tempo

[![Go Reference](https://pkg.go.dev/badge/code.hybscloud.com/tempo.svg)](https://pkg.go.dev/code.hybscloud.com/tempo)
[![Go Report Card](https://goreportcard.com/badge/github.com/hayabusa-cloud/tempo)](https://goreportcard.com/report/github.com/hayabusa-cloud/tempo)
[![Codecov](https://codecov.io/gh/hayabusa-cloud/tempo/graph/badge.svg)](https://codecov.io/gh/hayabusa-cloud/tempo)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Benchmarks](https://github.com/hayabusa-cloud/tempo/actions/workflows/benchmarks.yml/badge.svg)](https://github.com/hayabusa-cloud/tempo/actions/workflows/benchmarks.yml)

**English** | [简体中文](README.zh-CN.md) | [Español](README.es.md) | [日本語](README.ja.md) | [Français](README.fr.md)

`tempo` describes what remains of a temporal contract after each event. It provides pure state transitions and their composition in one dependency-free Go package.

Import the package as `code.hybscloud.com/tempo`.

```go
type Residual[R Residual[R, E, O], E, O any] interface {
	Observe() O
	Step(E) R
}
```

Use concrete types for state, events, and observations. Compose independent contracts with `Both`:

```go
state := tempo.Both(requestContract, loanContract)
state = state.Step(event)
observation := state.Observe()
```

`observation.Left` describes the request; `observation.Right` describes its physical loan. Completing the request need not return the loan. The [executable example](example_test.go) keeps these frontiers separate.

The four top-level exports are `Residual`, `Pair`, `Product`, and `Both`. `Product.Parts` returns its component states for explicit reconstruction or a checked state mapping.

Events supply time and other external facts. Implementations define admission, rejection, deadlines, and termination. `tempo` performs no I/O, scheduling, cancellation, or resource release.

## Integration

Use existing stack types in the consuming package's event and observation types. Preserve `iox` outcomes, `kont` resumption ownership, `cove` context, `sess` protocol frontiers, and `takt`/`urex` correlation and physical lifetimes. The core needs no adapter packages or dependencies on those owners.

Independent pure states compose pointwise. Shared-state interactions require their own joint transition. Go types do not enforce purity, and copying a residual never transfers an endpoint, continuation, or buffer obligation.

## Verification

Requires Go 1.26+. Run the package tests with:

```sh
GOWORK=off go test -timeout 120s -race ./...
```

The [CI workflow](.github/workflows/ci.yml) runs the Go, TLA+, Lean proof, and certificate checks. The coverage badge measures the root runtime package.

Full verification requires a repository checkout including `tools/check` and Elan with an installed [Lean 4.35.x toolchain](proofs/lean-version). Run `./verify.sh` for all four phases, or select one:

| Command | Checks |
|---------|--------|
| `./verify.sh go` | Race tests, vet, and every reachable lifetime component transition against TLA+ with `tempo_verify` |
| `./verify.sh lean` | Residual laws, conditional response, weak-fair service, certificate soundness, and transitive axioms |
| `./verify.sh tla` | Bounded request-lifetime and repeated-service models, including replayed counterexample controls |
| `./verify.sh certificates` | Lean checking of successful graph claims and rejection of corrupted certificates |

The harness installs nothing. The self-contained [Lean library](proofs/README.md) has no external dependencies. The [checker](tools/check/README.md) is a separate Go command module using only the standard library and supporting a documented finite TLA+ fragment. Unsupported constructs and exhausted bounds return `not_checked`.

The [models](spec/README.md) state fairness assumptions explicitly; the [semantics](docs/semantics.md) specify their correspondence to Go. Kernel certificates prove encoded graph claims; source interpretation remains a tested and reviewed boundary.

## Cost

The core stores concrete values and introduces no dynamic storage. Allocation and copying depend on component implementations. Generic calls can remain in compiled code; compare the selected realization with equivalent direct code before making a cost claim.

Keep concrete component types in hot loops using `Parts` and `Both`. See [concrete execution](docs/semantics.md#concrete-execution) for pointer-event escapes and additional nested, lifetime and pointer-event comparisons.

Measure the supplied composed and direct workloads in your environment:

```sh
GOWORK=off GOMAXPROCS=1 go test -timeout 120s -run '^$' -bench '^Benchmark(Direct|Product)$' -benchmem -benchtime=100ms -count=10 -cpu=1 .
```

The [benchmark workflow](.github/workflows/benchmarks.yml) runs this pair, requires ten complete samples per workload, and publishes the environment and raw output in its job summary and artifact. Its badge reports successful collection, not a performance threshold. Compare measurements from equivalent environments and compiler settings.

## License

MIT. Copyright ©2026 Robin He and Hayabusa Cloud Co., Ltd. See [LICENSE](LICENSE).
