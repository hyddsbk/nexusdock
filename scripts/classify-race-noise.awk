function reset_report() {
    xmalloc = 0
    mem_musl = 0
    accesses = 0
    has_read = 0
    first_goroutine = ""
    second_goroutine = ""
}

function finish_report() {
    if (!in_report) {
        return
    }

    if (xmalloc != 2 ||
        mem_musl != 2 ||
        accesses != 2 ||
        has_read ||
        first_goroutine == "" ||
        second_goroutine == "" ||
        first_goroutine != second_goroutine) {
        all_known = 0
    }

    in_report = 0
}

function record_access(line, id) {
    id = line
    sub(/^.* by goroutine /, "", id)
    sub(/:.*/, "", id)

    accesses++
    if (accesses == 1) {
        first_goroutine = id
    } else if (accesses == 2) {
        second_goroutine = id
    }
}

BEGIN {
    all_known = 1
    reports = 0
    in_report = 0
}

/^WARNING: DATA RACE$/ {
    finish_report()
    reports++
    in_report = 1
    reset_report()
    next
}

in_report && /modernc\.org\/libc\.Xmalloc\(\)/ {
    xmalloc++
}

in_report && /modernc\.org\/libc@v1\.66\.10\/mem_musl\.go:32/ {
    mem_musl++
}

in_report && /^Write at .* by goroutine [0-9]+:$/ {
    record_access($0)
}

in_report && /^Previous write at .* by goroutine [0-9]+:$/ {
    record_access($0)
}

in_report && /^(Read|Previous read) at .* by goroutine [0-9]+:$/ {
    has_read = 1
}

/^==================$/ {
    finish_report()
}

END {
    finish_report()
    if (reports < 1 || !all_known) {
        exit 1
    }
}
