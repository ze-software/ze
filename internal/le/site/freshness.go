// Design: docs/contributing/feature-maturity.md -- recorded runs, and a held Supported level
// Related: actions.go -- runBuild, which judges freshness before Build
//
// Owner decision 2026-10-08: a feature once supported stays supported, and the
// site build warns before it publishes when the data behind that is getting
// old. A stale recorded run never lowers the level (internal/le/feature
// held.go), so the published page is unchanged by it; what the build adds is
// the warning, naming every supported feature carrying a stale run, and the
// offer to re-record them first.
//
// The offer is the `refresh` keyword, never a prompt: the build runs in
// scripts and in agents, and a build that waits on a terminal blocks them. A
// refresh runs each feature's tests through `record-run`, which can need
// Docker or a QEMU guest, so it is never the default.

package site

import (
	"io"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/feature"
)

// staleJudge answers the supported features carrying a stale run;
// staleRecorder re-records the features it is given and answers each.
type (
	staleJudge    func() ([]feature.StaleFeature, error)
	staleRecorder func([]feature.StaleFeature) []feature.StaleFeature
)

// freshness judges the stale supported features, re-records them first when
// refresh is set, and writes one warning line per stale run still standing,
// and one per refresh that recorded nothing, to warn. It answers the stale
// features left and the refresh results (nil without refresh).
func freshness(warn io.Writer, refresh bool, judge staleJudge,
	record staleRecorder,
) ([]feature.StaleFeature, []feature.StaleFeature, error) {
	stale, err := judge()
	if err != nil {
		return nil, nil, err
	}
	if len(stale) == 0 {
		return stale, nil, nil
	}
	var tb textbuf.Buffer
	var refreshed []feature.StaleFeature
	if refresh {
		refreshed = record(stale)
		writeRefusedRefresh(&tb, refreshed)
		stale, err = judge()
		if err != nil {
			return nil, nil, err
		}
	}
	writeStaleWarning(&tb, stale, refresh)
	if _, err := io.WriteString(warn, tb.String()); err != nil {
		return nil, nil, err
	}
	return stale, refreshed, nil
}

// writeRefusedRefresh names each feature a refresh recorded nothing for.
func writeRefusedRefresh(tb *textbuf.Buffer, refreshed []feature.StaleFeature) {
	for _, entry := range refreshed {
		if entry.Refreshed {
			continue
		}
		tb.Str("warning: refresh of feature ").Str(entry.Feature).Str(" recorded nothing: ").
			Str(entry.Refusal).Byte('\n')
	}
}

// writeStaleWarning names each stale run of each supported feature, then how
// to re-record them. After a refresh the hint names only the per-feature
// command: running the refresh again would repeat what just failed.
func writeStaleWarning(tb *textbuf.Buffer, stale []feature.StaleFeature, refreshed bool) {
	if len(stale) == 0 {
		return
	}
	for _, entry := range stale {
		for _, run := range entry.Stale {
			tb.Str("warning: supported feature ").Str(entry.Feature).Str(" publishes on a stale run: ").
				Str(run).Byte('\n')
		}
	}
	tb.Str("warning: the level stands; re-record with ")
	if !refreshed {
		tb.Str("`./le site build refresh` (runs the tests first; some need Docker or a QEMU guest) or ")
	}
	tb.Str("`./le feature record-run feature <id>`, then commit features/runs/\n")
}
