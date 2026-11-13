# tempo

[![Go Reference](https://pkg.go.dev/badge/code.hybscloud.com/tempo.svg)](https://pkg.go.dev/code.hybscloud.com/tempo)
[![Go Report Card](https://goreportcard.com/badge/github.com/hayabusa-cloud/tempo)](https://goreportcard.com/report/github.com/hayabusa-cloud/tempo)
[![Codecov](https://codecov.io/gh/hayabusa-cloud/tempo/graph/badge.svg)](https://codecov.io/gh/hayabusa-cloud/tempo)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Benchmarks](https://github.com/hayabusa-cloud/tempo/actions/workflows/benchmarks.yml/badge.svg)](https://github.com/hayabusa-cloud/tempo/actions/workflows/benchmarks.yml)

[English](README.md) | [简体中文](README.zh-CN.md) | [Español](README.es.md) | **日本語** | [Français](README.fr.md)

`tempo` は、各イベントの後に残る時間的な契約を記述します。依存関係のない単一の Go パッケージで、純粋な状態遷移とその合成を提供します。

インポートパスは `code.hybscloud.com/tempo` です。

```go
type Residual[R Residual[R, E, O], E, O any] interface {
	Observe() O
	Step(E) R
}
```

状態、イベント、観測には具体型を使います。独立した契約は `Both` で合成できます。

```go
state := tempo.Both(requestContract, loanContract)
state = state.Step(event)
observation := state.Observe()
```

`observation.Left` はリクエストを、`observation.Right` はそれに伴う物理リソースの貸し出しを記述します。リクエストの完了は、リソースの返却を意味しません。[実行可能な例](example_test.go) は、この二つの進行状態を区別しています。

エクスポートされるトップレベルの宣言は `Residual`、`Pair`、`Product`、`Both` の四つです。`Product.Parts` は各構成要素の状態を返し、明示的な再構築や検証済みの状態写像に利用できます。

時刻などの外部の事実はイベントで与えます。受け入れ、拒否、期限、終了の振る舞いは各実装が定義します。`tempo` は I/O、スケジューリング、キャンセル、リソース解放を行いません。

## 統合

利用側パッケージのイベント型と観測型には、スタックの既存の型を使います。`iox` の結果、`kont` の再開継続の所有権、`cove` のコンテキスト、`sess` のプロトコル進行状態、`takt`/`urex` の対応付けと物理リソースの生存期間を保持してください。コアにはアダプターパッケージも、これらの責務を持つパッケージへの依存も不要です。

独立した純粋な状態は、各構成要素ごとに合成します。共有状態を介する相互作用には、専用の結合遷移が必要です。Go の型は純粋性を保証しません。また、残余状態をコピーしても、エンドポイント、継続、バッファーに関する責務は移転しません。

## 検証

Go 1.26 以降が必要です。パッケージのテストは次のコマンドで実行します。

```sh
GOWORK=off go test -timeout 120s -race ./...
```

[CI ワークフロー](.github/workflows/ci.yml) は Go と TLA+ の検査を実行します。カバレッジバッジは、ルートのランタイムパッケージを対象とします。

完全な検証には、`tools/check` を含むリポジトリのコピー、Elan、および Elan にインストール済みの [Lean 4.35.x ツールチェーン](proofs/lean-version) が必要です。`./verify.sh` で全四段階を実行するか、個別に選択します。

| コマンド | 検査内容 |
|----------|----------|
| `./verify.sh go` | 競合検査、vet、および `tempo_verify` による到達可能な全ライフサイクル構成要素の遷移と TLA+ との照合 |
| `./verify.sh lean` | 残余状態の法則、条件付き応答、弱公平なサービス、証明書の健全性、推移的な公理依存 |
| `./verify.sh tla` | 有界なリクエスト生存期間モデルと反復サービスモデル、および反例を再現する負の検査 |
| `./verify.sh certificates` | 成立するグラフ上の主張を Lean で検査し、破損した証明書を拒否 |

検証ツールは何もインストールしません。自己完結した [Lean ライブラリー](proofs/README.md) に外部依存はありません。[チェッカー](tools/check/README.md) は標準ライブラリーのみを使う独立した Go コマンドモジュールで、文書化された有限の TLA+ 断片をサポートします。未対応の構文や検査上限への到達は `not_checked` を返します。

[モデル](spec/README.md) は公平性の仮定を明示し、[意味論](docs/semantics.md) は Go との対応を定義します。カーネル証明書は符号化されたグラフ上の主張を証明しますが、ソースの解釈はテストとレビューで検証する境界として残ります。

## コスト

コアは具体的な値を格納し、動的な記憶領域を導入しません。割り当てとコピーは構成要素の実装に依存します。コンパイル後にもジェネリック呼び出しが残る場合があるため、コストを主張する前に、選択した実装を等価な直接実装と比較してください。

ホットループでは `Parts` と `Both` を使い、構成要素の具体的な型を保持してください。[具体的な実行](docs/semantics.md#concrete-execution)では、ポインターを含むイベントのエスケープと、入れ子の積、ライフサイクル、ポインターイベントの追加比較を説明しています。

付属の合成版と直接版のワークロードを、ご自身の環境で測定できます。

```sh
GOWORK=off GOMAXPROCS=1 go test -timeout 120s -run '^$' -bench '^Benchmark(Direct|Product)$' -benchmem -benchtime=100ms -count=10 -cpu=1 .
```

[ベンチマークワークフロー](.github/workflows/benchmarks.yml) はこの二つを実行し、各ワークロードに十個の完全なサンプルを要求します。環境情報と生の出力は、ジョブのサマリーとアーティファクトに保存します。バッジは測定結果の収集成功を示すもので、性能のしきい値を示すものではありません。測定値の比較には、同等の環境とコンパイラー設定を使ってください。

## ライセンス

MIT。Copyright ©2026 Robin He and Hayabusa Cloud Co., Ltd. [LICENSE](LICENSE) を参照してください。
