#!/bin/sh
set -eu

LAST_ARG=""
for ARG in "$@"; do
    LAST_ARG="$ARG"
done

case "${1:-}" in
    env)
        if [ "${2:-}" = "GOOS" ]; then
            echo "linux"
            exit 0
        fi
        ;;
    list)
        case "$LAST_ARG" in
            modernc.org/sqlite)
                echo "${FAKE_SQLITE_VERSION:-v1.39.1}"
                exit 0
                ;;
            modernc.org/libc)
                echo "${FAKE_LIBC_VERSION:-v1.66.10}"
                exit 0
                ;;
        esac
        ;;
    test)
        if [ "${2:-}" = "-race" ]; then
            cat "${RACE_GATE_FIXTURE:?RACE_GATE_FIXTURE is required}"
            exit "${FAKE_RACE_STATUS:-1}"
        fi
        echo "ok fake non-race suite"
        exit "${FAKE_TEST_STATUS:-0}"
        ;;
esac

echo "unexpected fake go invocation: $*" >&2
exit 2
