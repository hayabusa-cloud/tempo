#!/bin/sh
set -eu

repo=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
cd "$repo"
export GOWORK=off GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off

check_go() {
    unformatted=$(gofmt -l ./*.go ./tools/check/*.go)
    if [ -n "$unformatted" ]; then
        printf 'Unformatted Go sources:\n%s\n' "$unformatted" >&2
        return 1
    fi
    go test -buildvcs=false -tags=tempo_verify -timeout 120s -race -cover ./...
    go vet -buildvcs=false -tags=tempo_verify ./...
    (
        cd tools/check
        go test -buildvcs=false -timeout 120s -race ./...
        go vet -buildvcs=false ./...
    )
}

check_lean() {
    ./proofs/run.sh
}

check_tla() (
    scratch=$(mktemp -d "${TMPDIR:-/tmp}/tempo-check.XXXXXX")
    trap 'rm -rf "$scratch"' EXIT HUP INT TERM
    (cd tools/check && go build -buildvcs=false -o "$scratch/check" .)
    "$scratch/check" -config spec/Safety.cfg spec/Lifetime.tla
    "$scratch/check" -config spec/Progress.cfg spec/Lifetime.tla
    "$scratch/check" -config spec/Service.cfg spec/Service.tla

    expect_violation() {
        status=0
        "$scratch/check" -config "spec/$1.cfg" "spec/$3.tla" > "$scratch/$1.json" || status=$?
        if [ "$status" -ne 1 ] || ! grep -Fq "\"property\": \"$2\"" "$scratch/$1.json"; then
            cat "$scratch/$1.json" >&2
            printf 'Expected a counterexample for %s in %s.\n' "$2" "$1" >&2
            exit 1
        fi
        printf '%s: replayed counterexample for %s.\n' "$1" "$2"
    }
    expect_violation Unsafe ReturnedAfterSettlement Lifetime
    expect_violation Unfair EventuallyReturned Lifetime
    expect_violation Owner Response Service
    expect_violation Starved Recurring Service
)

check_certificates() {
    (
        cd tools/check
        TEMPO_CHECK_LEAN=1 go test -buildvcs=false -timeout 180s -run '^TestKernelCertificates$' -v .
    )
}

case ${1:-all} in
    go) check_go ;;
    lean) check_lean ;;
    tla) check_tla ;;
    certificates) check_lean; check_certificates ;;
    all) check_go; check_lean; check_tla; check_certificates ;;
    *) printf 'Usage: %s [go|lean|tla|certificates|all]\n' "$0" >&2; exit 2 ;;
esac
