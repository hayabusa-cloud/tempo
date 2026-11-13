# tempo

[![Go Reference](https://pkg.go.dev/badge/code.hybscloud.com/tempo.svg)](https://pkg.go.dev/code.hybscloud.com/tempo)
[![Go Report Card](https://goreportcard.com/badge/github.com/hayabusa-cloud/tempo)](https://goreportcard.com/report/github.com/hayabusa-cloud/tempo)
[![Codecov](https://codecov.io/gh/hayabusa-cloud/tempo/graph/badge.svg)](https://codecov.io/gh/hayabusa-cloud/tempo)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Benchmarks](https://github.com/hayabusa-cloud/tempo/actions/workflows/benchmarks.yml/badge.svg)](https://github.com/hayabusa-cloud/tempo/actions/workflows/benchmarks.yml)

[English](README.md) | [简体中文](README.zh-CN.md) | **Español** | [日本語](README.ja.md) | [Français](README.fr.md)

`tempo` describe lo que queda de un contrato temporal después de cada evento. Ofrece transiciones de estado puras y su composición en un único paquete Go sin dependencias.

La ruta de importación es `code.hybscloud.com/tempo`.

```go
type Residual[R Residual[R, E, O], E, O any] interface {
	Observe() O
	Step(E) R
}
```

Use tipos concretos para los estados, eventos y observaciones. Componga contratos independientes con `Both`:

```go
state := tempo.Both(requestContract, loanContract)
state = state.Step(event)
observation := state.Observe()
```

`observation.Left` describe la solicitud; `observation.Right` describe el préstamo del recurso físico asociado. Completar la solicitud no implica devolver el recurso. El [ejemplo ejecutable](example_test.go) mantiene separados estos estados de avance.

Las cuatro declaraciones exportadas de nivel superior son `Residual`, `Pair`, `Product` y `Both`. `Product.Parts` devuelve los estados de sus componentes para reconstruirlos explícitamente o aplicar una transformación de estado verificada.

Los eventos aportan el tiempo y otros hechos externos. Cada implementación define la admisión, el rechazo, los plazos y la terminación. `tempo` no realiza E/S, planificación, cancelación ni liberación de recursos.

## Integración

Reutilice los tipos existentes de la pila en los tipos de eventos y observaciones del paquete consumidor. Preserve los resultados de `iox`, la propiedad de las reanudaciones de `kont`, el contexto de `cove`, los estados de avance del protocolo de `sess` y la correlación y los ciclos de vida físicos de `takt`/`urex`. El núcleo no necesita paquetes adaptadores ni dependencias de los paquetes responsables de esas funciones.

Los estados puros independientes se componen punto a punto. Las interacciones con estado compartido requieren su propia transición conjunta. Los tipos de Go no garantizan la pureza, y copiar un estado residual nunca transfiere obligaciones sobre un extremo de comunicación, una continuación o un búfer.

## Verificación

Requiere Go 1.26 o posterior. Ejecute las pruebas del paquete con:

```sh
GOWORK=off go test -timeout 120s -race ./...
```

El [flujo de CI](.github/workflows/ci.yml) ejecuta las comprobaciones de Go, TLA+, las pruebas formales de Lean y los certificados. La insignia de cobertura mide el paquete de ejecución situado en la raíz.

La verificación completa requiere una copia del repositorio que incluya `tools/check` y Elan con una [cadena de herramientas Lean 4.35.x](proofs/lean-version) instalada. Ejecute `./verify.sh` para las cuatro fases o seleccione una:

| Comando | Comprobaciones |
|---------|----------------|
| `./verify.sh go` | Pruebas de carreras, vet y comparación con TLA+ de cada transición alcanzable de los componentes del ciclo de vida mediante `tempo_verify` |
| `./verify.sh lean` | Leyes de estados residuales, respuesta condicional, servicio con equidad débil, corrección de certificados y dependencias transitivas de axiomas |
| `./verify.sh tla` | Modelos acotados del ciclo de vida de solicitudes y del servicio repetido, con controles negativos que reproducen contraejemplos |
| `./verify.sh certificates` | Comprobación en Lean de afirmaciones válidas sobre grafos y rechazo de certificados corruptos |

Las herramientas de verificación no instalan nada. La [biblioteca Lean](proofs/README.md) es autónoma y no tiene dependencias externas. El [verificador](tools/check/README.md) es un módulo de comandos Go independiente que utiliza solo la biblioteca estándar y admite un fragmento finito documentado de TLA+. Las construcciones no admitidas y el agotamiento de los límites devuelven `not_checked`.

Los [modelos](spec/README.md) declaran explícitamente los supuestos de equidad; la [semántica](docs/semantics.md) especifica su correspondencia con Go. Los certificados comprobados por el núcleo demuestran afirmaciones sobre los grafos codificados; la interpretación del código fuente sigue siendo un límite validado mediante pruebas y revisión.

## Coste

El núcleo almacena valores concretos y no introduce almacenamiento dinámico. La asignación y la copia dependen de la implementación de los componentes. Las llamadas genéricas pueden permanecer en el código compilado; compare la implementación elegida con código directo equivalente antes de afirmar un coste.

Conserve los tipos concretos de los componentes en los bucles críticos mediante `Parts` y `Both`. La sección de [ejecución concreta](docs/semantics.md#concrete-execution) explica el escape de eventos con punteros e incluye comparaciones de productos anidados, ciclos de vida y eventos con punteros.

Mida en su entorno las cargas de trabajo compuesta y directa incluidas:

```sh
GOWORK=off GOMAXPROCS=1 go test -timeout 120s -run '^$' -bench '^Benchmark(Direct|Product)$' -benchmem -benchtime=100ms -count=10 -cpu=1 .
```

El [flujo de benchmarks](.github/workflows/benchmarks.yml) ejecuta ambas cargas, exige diez muestras completas de cada una y publica el entorno y la salida sin procesar en el resumen del trabajo y en un artefacto. Su insignia indica que la recopilación se completó correctamente, no que se haya alcanzado un umbral de rendimiento. Compare mediciones de entornos y configuraciones de compilador equivalentes.

## Licencia

MIT. Copyright ©2026 Robin He and Hayabusa Cloud Co., Ltd. Consulte [LICENSE](LICENSE).
