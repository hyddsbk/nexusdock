#!/bin/sh
set -eu

SCRIPT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
TMP_DIR=$(mktemp -d "${TMPDIR:-/tmp}/nexusdock-race-classifier.XXXXXX")
trap 'rm -rf "$TMP_DIR"' EXIT HUP INT TERM

KNOWN="$SCRIPT_DIR/testdata/race-known-musl.log"
REAL="$SCRIPT_DIR/testdata/race-real.log"
MIXED="$TMP_DIR/mixed.log"
cat "$KNOWN" "$REAL" >"$MIXED"

if ! awk -f "$SCRIPT_DIR/classify-race-noise.awk" "$KNOWN"; then
    echo "classifier rejected the verified modernc musl noise fixture" >&2
    exit 1
fi

if awk -f "$SCRIPT_DIR/classify-race-noise.awk" "$REAL"; then
    echo "classifier accepted a real project race fixture" >&2
    exit 1
fi

if awk -f "$SCRIPT_DIR/classify-race-noise.awk" "$MIXED"; then
    echo "classifier accepted mixed known-noise and real-race output" >&2
    exit 1
fi

echo "race noise classifier valid: exact known signature passes; real and mixed races remain blocking"
