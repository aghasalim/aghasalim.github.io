#!/usr/bin/env bash
# Recompute every published value on the site using eight independent
# implementations and fail if any of them disagrees.
#
# Each check is skipped with a clear message if its toolchain is missing.
set -uo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"

pass=0 fail=0 skip=0

run () {
    local name="$1" tool="$2"; shift 2
    printf '\n=== %s ===\n' "$name"
    if ! command -v "$tool" >/dev/null 2>&1; then
        printf 'skipped: %s is not installed\n' "$tool"
        skip=$((skip + 1)); return
    fi
    if "$@"; then pass=$((pass + 1)); else fail=$((fail + 1)); fi
}

check_sql () {
    local out
    out=$(sqlite3 :memory: ".read verify/claims.sql" < /dev/null 2>&1 | tr -d '\r')
    local rows bad
    rows=$(echo "$out" | grep '^ROWS,' | cut -d, -f2)
    bad=$(echo "$out" | grep '^BADROWS,' | cut -d, -f2)
    if [ "$bad" != "0" ]; then
        echo "$out"
        return 1
    fi
    echo "SQL: $rows values reproduced, 0 disagreements"
}

check_c () {
    cc -std=c99 -O2 -Wall -o /tmp/claimc verify/claims.c -lm &&
    /tmp/claimc "$root"
}

check_go () { ( cd verify/gocheck && go run . -root "$root" ); }
check_js () { node verify/claims.js "$root"; }
check_py () { python3 verify/claims.py "$root"; }
check_r  () { Rscript verify/claims.R "$root"; }
check_rb () { ruby verify/claims.rb "$root"; }

run "SQL, recompute values"          sqlite3 check_sql
run "C, recompute values"            cc      check_c
run "Go, structure and values"       go      check_go
run "JavaScript, values and phrases" node    check_js
run "Python, recompute values"       python3 check_py
run "R, recompute values"            Rscript check_r
run "Ruby, recompute values"         ruby    check_rb

printf '\n%s\n' "----------------------------------------"
printf '%d passed, %d failed, %d skipped\n' "$pass" "$fail" "$skip"
[ "$fail" -eq 0 ] || exit 1
[ "$pass" -gt 0 ] || { echo "nothing ran"; exit 1; }
