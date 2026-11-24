// Design: docs/architecture/plugin/plugin-system.md -- data-plane writer judgement
// Related: dataplane_writer.go -- JudgeDataPlaneWriter, DataPlaneWriter

package registry

import (
	"errors"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/opererr"
)

// TestWriterJudgement verifies the single data-plane writer judgement shared by
// boot, reload, and doctor across its four outcome classes.
//
// VALIDATES: spec-fib-dummy AC-5, AC-6, AC-11 -- JudgeDataPlaneWriter returns
// the data plane's writer when a NeedsDataPlane producer runs with no chosen
// ProgramsFIB plugin, returns no writer when any ProgramsFIB plugin is chosen
// or no producer needs a data plane, and refuses an unresolvable or gated data
// plane.
// PREVENTS: boot, reload, and doctor diverging on whether the engine adds a
// data-plane writer or refuses an unprogrammed plane.
func TestWriterJudgement(t *testing.T) {
	snap := Snapshot()
	t.Cleanup(func() { Restore(snap) })

	run := func(_ net.Conn) int { return 0 }
	cli := func([]string) int { return 0 }

	const (
		producerName     = "judge-producer-fixture"
		nonProducerName  = "judge-observer-fixture"
		planeName        = "judge-plane-a"
		writerName       = "judge-writer-a"
		chosenFIBName    = "judge-chosen-fib"
		gatedPlaneName   = "judge-gated-plane"
		gatedWriterName  = "judge-gated-writer"
		unprogrammedName = "judge-unprogrammed-plane"
	)

	for _, reg := range []Registration{
		{
			Name: producerName, Description: "test", RunEngine: run, CLIHandler: cli,
			NeedsDataPlane: true,
		},
		{
			Name: nonProducerName, Description: "test", RunEngine: run, CLIHandler: cli,
		},
		{
			Name: writerName, Description: "test", RunEngine: run, CLIHandler: cli,
			ProgramsFIB: true, DataPlane: planeName,
		},
		{
			Name: chosenFIBName, Description: "test", RunEngine: run, CLIHandler: cli,
			ProgramsFIB: true, NoDataPlaneChoice: true,
		},
	} {
		require.NoError(t, Register(reg))
	}
	gatedDataPlaneWriters[gatedPlaneName] = gatedWriterName
	gatedPluginTags[gatedWriterName] = []string{"ze_test_gated_plane"}
	t.Cleanup(func() {
		delete(gatedDataPlaneWriters, gatedPlaneName)
		delete(gatedPluginTags, gatedWriterName)
	})

	tests := []struct {
		name        string
		producers   []string
		chosenFIB   []string
		dataPlane   string
		want        string
		wantOperErr bool
		wantGated   bool
		errSubstr   string
	}{
		{
			name:      "producer with no chosen FIB plugin picks the plane writer",
			producers: []string{nonProducerName, producerName},
			chosenFIB: nil,
			dataPlane: planeName,
			want:      writerName,
		},
		{
			name:      "chosen ProgramsFIB plugin suppresses auto-added writer",
			producers: []string{producerName},
			chosenFIB: []string{chosenFIBName},
			dataPlane: planeName,
			want:      "",
		},
		{
			name:      "chosen ProgramsFIB plugin suppresses refusal on unprogrammed plane",
			producers: []string{producerName},
			chosenFIB: []string{chosenFIBName},
			dataPlane: unprogrammedName,
			want:      "",
		},
		{
			name:      "non-ProgramsFIB entry in chosenFIB does not count as chosen",
			producers: []string{producerName},
			chosenFIB: []string{nonProducerName},
			dataPlane: planeName,
			want:      writerName,
		},
		{
			name:      "no NeedsDataPlane producer needs no writer even on unprogrammed plane",
			producers: []string{nonProducerName},
			chosenFIB: nil,
			dataPlane: unprogrammedName,
			want:      "",
		},
		{
			name:      "empty producers list needs no writer",
			producers: nil,
			chosenFIB: nil,
			dataPlane: planeName,
			want:      "",
		},
		{
			name:        "unprogrammed data plane refuses with operator error",
			producers:   []string{producerName},
			chosenFIB:   nil,
			dataPlane:   unprogrammedName,
			wantOperErr: true,
			errSubstr:   unprogrammedName,
		},
		{
			name:      "gated data plane refuses with MissingDependencyError",
			producers: []string{producerName},
			chosenFIB: nil,
			dataPlane: gatedPlaneName,
			wantGated: true,
			errSubstr: "ze_test_gated_plane",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := JudgeDataPlaneWriter(tc.producers, tc.chosenFIB, tc.dataPlane)
			if tc.wantOperErr {
				require.Error(t, err)
				_, isOper := errors.AsType[*opererr.Error](err)
				assert.True(t, isOper, "got error %T (%v), want *opererr.Error", err, err)
				assert.Contains(t, err.Error(), tc.errSubstr)
				assert.Empty(t, got)
				return
			}
			if tc.wantGated {
				require.Error(t, err)
				_, isMissing := errors.AsType[*MissingDependencyError](err)
				assert.True(t, isMissing, "got error %T (%v), want *MissingDependencyError", err, err)
				assert.Contains(t, err.Error(), tc.errSubstr)
				assert.Empty(t, got)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}
