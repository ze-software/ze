package doctor

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// TestBuildRegistrationStopRowNamesTheCodeTheCauseAndTheFix proves the
// last-stop check reports a start refused because a module compiled into ze
// names a registration this ze does not hold under
// doctor-build-registration-unheld, with the time, the refusal's own text and
// the fix the refusal named; an entry that kept no fix points at ze's log.
//
// VALIDATES: `ze doctor`, run with no daemon after the refused start, names
// the build defect, the module and the name.
// PREVENTS: the entry falling to the default arm and reading as a fatal plugin
// stop, under a code that blames a plugin for the build.
func TestBuildRegistrationStopRowNamesTheCodeTheCauseAndTheFix(t *testing.T) {
	at := time.Date(2026, 11, 17, 9, 30, 0, 0, time.UTC)
	detail := `Module "ze-bgp-conf", compiled into ze, names ze:reference "bfd-profile" at /ze-bgp-conf/bgp/bfd, ` +
		"which no package linked into this ze registers. this ze was built wrong"
	record := server.LastStop{Entries: []server.LastStopEntry{
		{
			Reason: server.LastStopBuildRegistrationUnheld, Time: at, Detail: detail,
			Fix: `rebuild ze with the package that registers "bfd-profile" linked wherever module "ze-bgp-conf" is`,
		},
		{Reason: server.LastStopBuildRegistrationUnheld, Time: at, Detail: "no fix kept"},
	}}

	rows := fatalStopRows(&record)
	require.Len(t, rows, 2)

	row := rows[0]
	assert.Equal(t, diagnostic.CodeDoctorBuildRegistrationUnheld, row.Code)
	assert.Equal(t, diagnostic.SeverityError, row.Severity)
	assert.Equal(t, "ze refused to start at 2026-11-17T09:30:00Z because this ze was built without a registration its own YANG names: "+
		detail, row.Message)
	assert.Equal(t, `rebuild ze with the package that registers "bfd-profile" linked wherever module "ze-bgp-conf" is`, row.Fix)

	assert.Equal(t, diagnostic.CodeDoctorBuildRegistrationUnheld, rows[1].Code)
	assert.Equal(t, "journalctl -u ze --until 2026-11-17T09:30:00Z", rows[1].Fix)
}

// TestBuildRegistrationReasonRoundTrips proves the reason is stored and read
// back under its own spelling, and answers its own code.
//
// VALIDATES: the record a refused start writes is read by the next doctor run.
// PREVENTS: an entry the decoder refuses, which would hide every row.
func TestBuildRegistrationReasonRoundTrips(t *testing.T) {
	text, err := server.LastStopBuildRegistrationUnheld.MarshalText()
	require.NoError(t, err)
	assert.Equal(t, "build-registration-unheld", string(text))
	var reason server.LastStopReason
	require.NoError(t, reason.UnmarshalText(text))
	assert.Equal(t, server.LastStopBuildRegistrationUnheld, reason)
	assert.Equal(t, diagnostic.CodeDoctorBuildRegistrationUnheld, reason.Code())
}
