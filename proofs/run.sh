#!/bin/sh
# ©Robin He and Hayabusa Cloud Co., Ltd. 2026. All rights reserved.
# Use of this source code is governed by a MIT-style
# license that can be found in the LICENSE file.
set -eu

cd -- "$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
series=$(cat lean-version)
installed=$(elan toolchain list)
toolchain=$(printf '%s\n' "$installed" | awk -v prefix="leanprover/lean4:v${series%.x}." '
    index($1, prefix) == 1 {
        patch = substr($1, length(prefix) + 1)
        if (patch ~ /^[0-9]+$/ && (selected == "" || patch + 0 > latest)) {
            selected = $1
            latest = patch + 0
        }
    }
    END { print selected }
')
if [ -z "$toolchain" ]; then
    printf 'Lean %s is required; no matching installed toolchain found.\n' "$series" >&2
    exit 2
fi

printf 'Lean toolchain: %s\n' "$toolchain" >&2
if [ "$#" -eq 0 ]; then
    set -- lake build
fi
exec elan run "$toolchain" "$@"
