# tempo

[![Go Reference](https://pkg.go.dev/badge/code.hybscloud.com/tempo.svg)](https://pkg.go.dev/code.hybscloud.com/tempo)
[![Codecov](https://codecov.io/gh/hayabusa-cloud/tempo/graph/badge.svg)](https://codecov.io/gh/hayabusa-cloud/tempo)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Benchmarks](https://github.com/hayabusa-cloud/tempo/actions/workflows/benchmarks.yml/badge.svg)](https://github.com/hayabusa-cloud/tempo/actions/workflows/benchmarks.yml)

[English](README.md) | **简体中文** | [Español](README.es.md) | [日本語](README.ja.md) | [Français](README.fr.md)

`tempo` 描述每个事件发生后尚待履行的时序契约。它在一个无依赖的 Go 包中提供纯状态转换及其组合。

包的导入路径为 `code.hybscloud.com/tempo`。

```go
type Residual[R Residual[R, E, O], E, O any] interface {
	Observe() O
	Step(E) R
}
```

为状态、事件和观察结果使用具体类型。通过 `Both` 组合相互独立的契约：

```go
state := tempo.Both(requestContract, loanContract)
state = state.Step(event)
observation := state.Observe()
```

`observation.Left` 描述请求；`observation.Right` 描述其物理资源借用。请求完成并不意味着借用的资源已经归还。[可执行示例](example_test.go) 保留了这两种状态之间的区别。

四个顶层导出标识符为 `Residual`、`Pair`、`Product` 和 `Both`。`Product.Parts` 返回各分量状态，供显式重建或经过验证的状态映射使用。

时间和其他外部事实由事件提供。具体实现负责定义准入、拒绝、截止时间和终止行为。`tempo` 不执行 I/O、调度、取消或资源释放。

## 集成

在使用方包的事件与观察类型中复用技术栈现有类型。保留 `iox` 的结果语义、`kont` 的续延恢复所有权、`cove` 的上下文、`sess` 的协议进展状态，以及 `takt` 的关联信息和物理生命周期。核心不需要适配器包，也不依赖这些职责的所属包。

相互独立的纯状态逐分量组合。涉及共享状态的交互需要单独定义联合转换。Go 类型无法强制保证纯性，复制残余状态也不会转移端点、续延或缓冲区的责任。

## 验证

需要 Go 1.26 或更新版本。运行包测试：

```sh
GOWORK=off go test -timeout 120s -race ./...
```

[CI 工作流](.github/workflows/ci.yml) 执行 Go、TLA+、Lean 证明和证书检查。覆盖率徽章衡量根目录运行时包的测试覆盖率。

完整验证需要包含 `tools/check` 的仓库副本、Elan 以及其中已安装的 [Lean 4.35.x 工具链](proofs/lean-version)。运行 `./verify.sh` 执行全部四个阶段，也可以单独选择：

| 命令 | 检查内容 |
|------|----------|
| `./verify.sh go` | 竞态测试、vet，以及通过 `tempo_verify` 将所有可达生命周期分量转换与 TLA+ 对照 |
| `./verify.sh lean` | 残余状态定律、条件响应、弱公平服务、证书可靠性和传递公理依赖 |
| `./verify.sh tla` | 有界请求生命周期模型和重复服务模型，包括重放反例的负向检查 |
| `./verify.sh certificates` | 使用 Lean 检查成立的图性质声明，并拒绝损坏的证书 |

验证工具不会安装任何软件。自包含的 [Lean 库](proofs/README.md) 没有外部依赖。[检查器](tools/check/README.md) 是独立的 Go 命令模块，仅使用标准库，支持文档中声明的有限 TLA+ 子集。不支持的构造和耗尽的检查限额均返回 `not_checked`。

[模型](spec/README.md) 显式声明公平性假设；[语义文档](docs/semantics.md) 定义模型与 Go 的对应关系。内核证书证明编码后的图性质；源代码解释仍是通过测试和审查验证的边界。

## 开销

核心存储具体值，不引入动态存储。分配和复制取决于各分量的实现。编译后的代码仍可能保留泛型调用；在作出开销结论之前，应将选定的实现与等价的直接代码进行比较。

使用 `Parts` 和 `Both` 在热点循环中保留具体分量类型。[具体执行](docs/semantics.md#concrete-execution)说明了指针事件的逃逸，并提供嵌套组合、生命周期和指针事件的额外对比工作负载。

在自己的环境中测量提供的组合工作负载与直接工作负载：

```sh
GOWORK=off GOMAXPROCS=1 go test -timeout 120s -run '^$' -bench '^Benchmark(Direct|Product)$' -benchmem -benchtime=100ms -count=10 -cpu=1 .
```

[基准测试工作流](.github/workflows/benchmarks.yml) 运行这两个工作负载，要求每个工作负载产生十个完整样本，并在作业摘要和产物中发布环境信息及原始输出。徽章表示采集成功，不代表达到某个性能阈值。比较测量结果时，应使用等价的环境和编译器设置。

## 许可证

MIT。Copyright ©2026 Robin He and Hayabusa Cloud Co., Ltd. 参见 [LICENSE](LICENSE)。
