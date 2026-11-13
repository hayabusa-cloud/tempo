# tempo

[![Go Reference](https://pkg.go.dev/badge/code.hybscloud.com/tempo.svg)](https://pkg.go.dev/code.hybscloud.com/tempo)
[![Go Report Card](https://goreportcard.com/badge/github.com/hayabusa-cloud/tempo)](https://goreportcard.com/report/github.com/hayabusa-cloud/tempo)
[![Codecov](https://codecov.io/gh/hayabusa-cloud/tempo/graph/badge.svg)](https://codecov.io/gh/hayabusa-cloud/tempo)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Benchmarks](https://github.com/hayabusa-cloud/tempo/actions/workflows/benchmarks.yml/badge.svg)](https://github.com/hayabusa-cloud/tempo/actions/workflows/benchmarks.yml)

[English](README.md) | [简体中文](README.zh-CN.md) | [Español](README.es.md) | [日本語](README.ja.md) | **Français**

`tempo` décrit ce qui reste d’un contrat temporel après chaque événement. Il fournit des transitions d’état pures et leur composition dans un seul paquet Go sans dépendances.

Le chemin d’importation est `code.hybscloud.com/tempo`.

```go
type Residual[R Residual[R, E, O], E, O any] interface {
	Observe() O
	Step(E) R
}
```

Utilisez des types concrets pour les états, les événements et les observations. Composez des contrats indépendants avec `Both` :

```go
state := tempo.Both(requestContract, loanContract)
state = state.Step(event)
observation := state.Observe()
```

`observation.Left` décrit la requête ; `observation.Right` décrit le prêt de ressource physique associé. L’achèvement de la requête n’implique pas la restitution de la ressource. L’[exemple exécutable](example_test.go) distingue ces deux états d’avancement.

Les quatre déclarations exportées de premier niveau sont `Residual`, `Pair`, `Product` et `Both`. `Product.Parts` renvoie les états des composants pour une reconstruction explicite ou une transformation d’état vérifiée.

Les événements apportent le temps et les autres faits externes. Chaque implémentation définit l’admission, le rejet, les échéances et la terminaison. `tempo` n’effectue ni E/S, ni ordonnancement, ni annulation, ni libération de ressources.

## Intégration

Réutilisez les types existants de la pile dans les types d’événements et d’observations du paquet consommateur. Préservez les résultats d’`iox`, la propriété des continuations de reprise de `kont`, le contexte de `cove`, les états d’avancement du protocole de `sess`, ainsi que la corrélation et les durées de vie physiques de `takt`/`urex`. Le noyau n’a besoin ni de paquets adaptateurs ni de dépendances envers les paquets responsables de ces fonctions.

Les états purs indépendants se composent point par point. Les interactions sur un état partagé nécessitent leur propre transition conjointe. Les types Go ne garantissent pas la pureté, et copier un état résiduel ne transfère jamais les obligations liées à une extrémité de communication, une continuation ou un tampon.

## Vérification

Go 1.26 ou une version ultérieure est requis. Lancez les tests du paquet avec :

```sh
GOWORK=off go test -timeout 120s -race ./...
```

Le [workflow CI](.github/workflows/ci.yml) vérifie Go, TLA+, les preuves Lean et les certificats. Le badge de couverture porte sur le paquet d’exécution à la racine.

La vérification complète nécessite une copie du dépôt incluant `tools/check` et Elan avec une [chaîne d’outils Lean 4.35.x](proofs/lean-version) déjà installée. Lancez `./verify.sh` pour les quatre phases, ou sélectionnez-en une :

| Commande | Vérifications |
|----------|---------------|
| `./verify.sh go` | Détection des courses de données, vet et comparaison avec TLA+ de chaque transition accessible des composants du cycle de vie via `tempo_verify` |
| `./verify.sh lean` | Lois des états résiduels, réponse conditionnelle, service sous équité faible, correction des certificats et dépendances transitives aux axiomes |
| `./verify.sh tla` | Modèles bornés du cycle de vie des requêtes et du service répété, avec contrôles négatifs reproduisant les contre-exemples |
| `./verify.sh certificates` | Vérification par Lean des assertions valides sur les graphes et rejet des certificats corrompus |

Les outils de vérification n’installent rien. La [bibliothèque Lean](proofs/README.md) est autonome et n’a aucune dépendance externe. Le [vérificateur](tools/check/README.md) est un module de commande Go indépendant qui utilise uniquement la bibliothèque standard et prend en charge un fragment fini documenté de TLA+. Les constructions non prises en charge et l’épuisement des limites renvoient `not_checked`.

Les [modèles](spec/README.md) déclarent explicitement les hypothèses d’équité ; la [sémantique](docs/semantics.md) définit leur correspondance avec Go. Les certificats vérifiés par le noyau prouvent des assertions sur les graphes encodés ; l’interprétation du code source reste une frontière validée par les tests et la revue.

## Coût

Le noyau stocke des valeurs concrètes et n’introduit aucun stockage dynamique. L’allocation et la copie dépendent de l’implémentation des composants. Des appels génériques peuvent subsister dans le code compilé ; comparez l’implémentation choisie à du code direct équivalent avant d’avancer une conclusion sur le coût.

Conservez les types concrets des composants dans les boucles critiques avec `Parts` et `Both`. La section sur l’[exécution concrète](docs/semantics.md#concrete-execution) explique l’échappement des événements contenant des pointeurs et propose des comparaisons de produits imbriqués, de cycles de vie et d’événements contenant des pointeurs.

Mesurez dans votre environnement les charges de travail composée et directe fournies :

```sh
GOWORK=off GOMAXPROCS=1 go test -timeout 120s -run '^$' -bench '^Benchmark(Direct|Product)$' -benchmem -benchtime=100ms -count=10 -cpu=1 .
```

Le [workflow de benchmarks](.github/workflows/benchmarks.yml) exécute ces deux charges, exige dix échantillons complets par charge et publie l’environnement ainsi que la sortie brute dans le résumé du job et un artefact. Son badge indique que la collecte a réussi, pas qu’un seuil de performance a été atteint. Comparez des mesures obtenues avec des environnements et des réglages du compilateur équivalents.

## Licence

MIT. Copyright ©2026 Robin He and Hayabusa Cloud Co., Ltd. Voir [LICENSE](LICENSE).
