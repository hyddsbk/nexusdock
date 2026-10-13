#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
RACE_LOG=$(mktemp "${TMPDIR:-/tmp}/nexusdock-race.XXXXXX")
trap 'rm -f "$RACE_LOG"' EXIT HUP INT TERM

set +e
go test -race ./... >"$RACE_LOG" 2>&1
RACE_STATUS=$?
set -e
cat "$RACE_LOG"

if [ "$RACE_STATUS" -eq 0 ]; then
    exit 0
fi

if [ "$(go env GOOS)" != "linux" ]; then
    exit "$RACE_STATUS"
fi

IS_MUSL=0
if ls /lib/ld-musl-*.so.1 >/dev/null 2>&1; then
    IS_MUSL=1
elif command -v ldd >/dev/null 2>&1 && ldd --version 2>&1 | grep -qi musl; then
    IS_MUSL=1
fi

if [ "$IS_MUSL" -ne 1 ]; then
    exit "$RACE_STATUS"
fi

SQLITE_VERSION=$(go list -m -f '{{.Version}}' modernc.org/sqlite 2>/dev/null || true)
LIBC_VERSION=$(go list -m -f '{{.Version}}' modernc.org/libc 2>/dev/null || true)

if [ "$SQLITE_VERSION" != "v1.39.1" ] || [ "$LIBC_VERSION" != "v1.66.10" ]; then
    echo "race-gate: musl race failure is not on the verified modernc dependency pair; keeping it blocking" >&2
    exit "$RACE_STATUS"
fi

if ! awk -f "$SCRIPT_DIR/classify-race-noise.awk" "$RACE_LOG"; then
    echo "race-gate: race output contains an unknown or mixed report; keeping it blocking" >&2
    exit "$RACE_STATUS"
fi

echo "race-gate: downgraded verified modernc/libc musl allocator false-positive (same-goroutine Xmalloc mem_musl.go:32)" >&2
echo "race-gate: rerunning the full suite without -race to prove there is no independent functional failure" >&2
go test ./...
