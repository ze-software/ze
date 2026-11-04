#!/usr/bin/env bash
# Measure assumption A-6 of spec-filter-wire-0-umbrella on the owner's Mac:
# the forked-filter slow path of a route the filter modifies (LOCAL_PREF 100
# to 200), from the received UPDATE to the rebuilt one, at the text-era
# revision and at this checkout's HEAD, under one Go toolchain.
#
#   HEAD:     BenchmarkFilterSlowPathModify
#             (internal/component/bgp/reactor/filter_slow_path_bench_test.go)
#   text era: BenchmarkFilterSlowPathModifyTextEra at ec9f715546, the last
#             revision whose live import chain sent the text subject. Its
#             source is textera_bench_test.go.txt beside this script; the
#             script exports that revision into a temporary directory, adds
#             the file, measures, and removes the directory.
#
# Both run a real SDK plugin over net.Pipe through the engine's real
# PluginConn RPC framing; neither includes the TLS socket.
#
# Usage, from the checkout root on the Mac:  bash test/perf/filter-a6/measure.sh
# It writes provenance.txt, head.txt and textera.txt into one directory and
# prints its path. Paste the three files back, or the directory's path.
#
# Time figures come only from the Mac, so the script refuses another OS.
# A6_CHECK=1 runs one short pass anywhere, to prove the script works; its
# times are not figures.
set -euo pipefail

textera=ec9f715546
root=$(git rev-parse --show-toplevel)
cd "$root"
here=test/perf/filter-a6

if [ "${A6_CHECK:-}" = 1 ]; then
	bench=(-benchtime 20x -count 1)
elif [ "$(uname -s)" != Darwin ]; then
	echo "measure.sh: time figures come only from the owner's Mac; this host is $(uname -s)." >&2
	echo "  Run it on the Mac, or set A6_CHECK=1 to check the script works (its times are not figures)." >&2
	exit 1
else
	bench=(-count 10)
fi

goroot=$(./le go stdpatch goroot)
export GOROOT="$goroot" GOTOOLCHAIN=local PATH="$goroot/bin:$PATH" CGO_ENABLED=0

out=$(mktemp -d "${TMPDIR:-/tmp}/ze-a6-XXXXXX")
src=$(mktemp -d "${TMPDIR:-/tmp}/ze-a6-textera-XXXXXX")
trap 'rm -rf "$src"' EXIT

{
	echo "date: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
	echo "head: $(git rev-parse HEAD)"
	echo "head-dirty-files-under-internal-pkg: $(git status --porcelain -- internal pkg | wc -l | tr -d ' ')"
	echo "textera: $(git rev-parse "$textera")"
	echo "textera-bench-sha256: $(shasum -a 256 "$here/textera_bench_test.go.txt" | cut -d' ' -f1)"
	echo "go: $(go version)"
	echo "goroot: $goroot"
	echo "uname: $(uname -a)"
	if [ "$(uname -s)" = Darwin ]; then
		echo "cpu: $(sysctl -n machdep.cpu.brand_string)"
		echo "power: $(pmset -g batt | head -1)"
	fi
	echo "bench-flags: ${bench[*]} -benchmem"
} >"$out/provenance.txt"

go test -tags ze_bgp -run '^$' -bench '^BenchmarkFilterSlowPathModify$' -benchmem "${bench[@]}" \
	./internal/component/bgp/reactor/ >"$out/head.txt" 2>&1

git archive "$textera" | tar -x -C "$src"
cp "$here/textera_bench_test.go.txt" "$src/internal/component/bgp/reactor/filter_slowpath_textera_bench_test.go"
(cd "$src" && go test -run '^$' -bench '^BenchmarkFilterSlowPathModifyTextEra$' -benchmem "${bench[@]}" \
	./internal/component/bgp/reactor/) >"$out/textera.txt" 2>&1

echo "A-6 measurement written to $out"
