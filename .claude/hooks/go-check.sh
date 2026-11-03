#!/usr/bin/env bash
# Claude Code PostToolUse hook (Edit|Write): after a Go file changes, run what
# CI runs (gofmt, go vet, go test; about 2 s here). Exit 2 sends the output
# back to Claude.
set -uo pipefail

cd "$(dirname "$0")/../.." || exit 0

file=$(jq -r '.tool_input.file_path // empty')
[[ "${file}" == *.go || "${file}" == */go.mod || "${file}" == */testdata/* ]] || exit 0

# go test runs its binaries from GOTMPDIR (default /tmp), which may be noexec
export GOTMPDIR=${GOTMPDIR:-${XDG_CACHE_HOME:-${HOME}/.cache}/go-tmp}
mkdir -p "${GOTMPDIR}"

fail() {
    printf '%s\n%s\n' "$1" "$2" >&2
    exit 2
}

unformatted=$(gofmt -l .)
[[ -z "${unformatted}" ]] || fail "gofmt: run gofmt -w on:" "${unformatted}"
out=$(go vet ./... 2>&1) || fail "go vet failed:" "${out}"
out=$(go test ./... 2>&1) || fail "go test failed:" "${out}"
exit 0
