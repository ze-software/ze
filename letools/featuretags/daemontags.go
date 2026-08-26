// Design: ai/rules/plugins.md -- feature-gates.txt is the ONE record of the gates
// Related: featuretags.go -- the four derived files the same parser feeds
//
// daemontags.go answers a second question from the same manifest: what `-tags`
// string does a build of the ze DAEMON carry. The four files featuretags.go
// rewrites are one consumer; every evidence tool that cross-compiles a daemon
// to drive against a real peer is another, and it needs the list at run time
// rather than written into a file.
//
// It lives beside the parser rather than in the tools, because a per-tool
// literal is a second record of one fact and the record nothing compares
// drifts. That is measured rather than feared: ze_bgp became a gate and
// scripts/evidence/effective-vpp.py went on building a ze with no BGP, and
// ze_l2tp became a gate on 2026-07-24 with no evidence script updated, so the
// proof that L2TP works against real kernel modules was silently unavailable
// for a month.

package featuretags

import (
	"errors"
	"path/filepath"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// gatePrefix is what a feature-gate tag starts with. The Makefile selects on it
// (`$$1 ~ /^ze_/` at Makefile:110) and so does every consumer that reads the
// manifest, so the selection is stated here once rather than at each of them.
const gatePrefix = "ze_"

// DaemonBase is the tag pair a daemon build carries before its gates: the core,
// and the word for which product this build is.
//
// A HOST driver is built without it. The `ze-host` that runs `ze appliance ...`
// on the build machine is `ze_core ze_setup` with no feature tags at all
// (mk/build-gokrazy.mk), because feature gates select daemon features and have
// nothing to say about the setup surface.
const DaemonBase = "ze_core ze_distro"

// ErrNoGateTags says the manifest declares no gate tag.
//
// It is an ERROR rather than an empty list, and that is the whole reason to
// read the manifest at run time. An empty answer builds a daemon with every
// feature compiled out, which then dies on "unknown top-level keyword" for the
// protocol the caller was about to prove, and the caller cannot tell that
// failure from a defect in the protocol.
var ErrNoGateTags = errors.New("feature-gates.txt declares no ze_ gate tag")

// DaemonTags answers every gate tag feature-gates.txt declares, sorted and
// deduplicated.
//
// Sorted rather than in manifest order: a build tag list is a set, and one
// spelling across the tree is one fewer thing to check when a build comes out
// missing a feature.
func DaemonTags(root string) ([]string, error) {
	_, sorted, err := readTags(root)
	if err != nil {
		return nil, err
	}

	gates := make([]string, 0, len(sorted))
	for _, tag := range sorted {
		if strings.HasPrefix(tag, gatePrefix) {
			gates = append(gates, tag)
		}
	}
	if len(gates) == 0 {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str(filepath.Join(root, manifestFile)).Str(": ").
			Err(ErrNoGateTags).String())
	}

	return gates, nil
}

// DaemonBuildTags answers the `-tags` value for a daemon build: base, then
// every gate the manifest declares.
//
// Space-separated, which is what the Makefile passes and what `go build`
// documents. Commas work too, and using both spellings is how two builds of one
// tree come to carry different features.
func DaemonBuildTags(root, base string) (string, error) {
	gates, err := DaemonTags(root)
	if err != nil {
		return "", err
	}

	var tb textbuf.Buffer
	tb.Str(base)
	for _, tag := range gates {
		tb.Byte(' ').Str(tag)
	}
	return tb.String(), nil
}
