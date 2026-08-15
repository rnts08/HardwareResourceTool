#!/usr/bin/env bash

set -u

project_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
duration=${HRT_DURATION:-5s}

usage() {
    printf 'Usage: %s [--duration DURATION]\n' "$0"
    printf '\nRuns the CLI build and root-only live collection, writing results under /tmp.\n'
}

while [ "$#" -gt 0 ]; do
    case "$1" in
        --duration)
            [ "$#" -ge 2 ] || { printf '%s\n' '--duration requires a value' >&2; exit 2; }
            duration=$2
            shift 2
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        --run-root)
            [ "$#" -ge 3 ] || { printf '%s\n' '--run-root requires binary and output directory' >&2; exit 2; }
            binary=$2
            output_dir=$3
            shift 3
            ;;
        *)
            printf 'Unknown argument: %s\n' "$1" >&2
            usage >&2
            exit 2
            ;;
    esac
done

if [ "${EUID:-$(id -u)}" -ne 0 ]; then
    output_dir=$(mktemp -d /tmp/hardware-resources-live.XXXXXX)
    binary="$output_dir/hardware-resources"

    printf 'Building Linux binary...\n'
    if ! (cd "$project_dir" && go build -o "$binary" ./cmd/hardware-resources); then
        printf 'Build failed; no live collection was run.\n' >&2
        exit 1
    fi

    printf 'Re-running collection as root. Results will be written to %s\n' "$output_dir"
    exec sudo "$0" --duration "$duration" --run-root "$binary" "$output_dir"
fi

if [ -z "${binary:-}" ]; then
    output_dir=$(mktemp -d /tmp/hardware-resources-live.XXXXXX)
    binary="$output_dir/hardware-resources"
    printf 'Building Linux binary as root...\n'
    if ! (cd "$project_dir" && go build -o "$binary" ./cmd/hardware-resources); then
        printf 'Build failed; no live collection was run.\n' >&2
        exit 1
    fi
fi

if [ -z "${binary:-}" ] || [ -z "${output_dir:-}" ]; then
    printf '%s\n' 'Internal error: root invocation arguments are missing.' >&2
    exit 2
fi

umask 022
mkdir -p "$output_dir"
# mktemp creates a private directory; make the captured diagnostics readable by
# the invoking user after sudo has finished.
chmod 755 "$output_dir"

{
    printf 'started_at=%s\n' "$(date -Is)"
    printf 'hostname=%s\n' "$(hostname)"
    printf 'kernel=%s\n' "$(uname -a)"
    printf 'uid=%s\n' "$(id -u)"
    printf 'duration=%s\n' "$duration"
    printf 'binary=%s\n' "$binary"
} > "$output_dir/metadata.txt"

run_and_record() {
    name=$1
    shift
    if "$@" > "$output_dir/$name" 2>&1; then
        printf '%s=passed\n' "$name"
    else
        status=$?
        printf '%s=failed(exit %s)\n' "$name" "$status"
    fi
}

printf 'Running root-only live collection...\n'
{
    printf 'check.txt=%s\n' "$output_dir/check.txt"
    run_and_record check.txt "$binary" check
    printf 'report.txt=%s\n' "$output_dir/report.txt"
    run_and_record report.txt "$binary" report --duration "$duration"
    printf 'report.json=%s\n' "$output_dir/report.json"
    run_and_record report.json "$binary" report --json --duration "$duration"
} | tee "$output_dir/summary.txt"

printf 'finished_at=%s\n' "$(date -Is)" >> "$output_dir/metadata.txt"
printf 'Results are in %s\n' "$output_dir"
