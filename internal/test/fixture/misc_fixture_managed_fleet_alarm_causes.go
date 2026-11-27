// Design: docs/architecture/fleet-alarms.md -- hub-derived alarm cause hiding, hidden pipe filter, and reconnect visibility restoration fixture
// Related: register_managed_fleet_alarm_causes.go -- registers managed/alarm-hidden-behind-offline
// Related: misc_fixture_managed_fleet_alarms.go -- shared fleetAlarmClient, waitAlarmList, queryAlarmList, and health helpers
// Related: misc_fixture_managed_fleet_hold.go -- fleetHoldRunWithHubConfig provisions the hub and device stores

package fixture

import (
	"context"
	"fmt"
	"strings"
	"time"

	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/fleet"
)

const (
	fleetAlarmCausesFixtureName = "managed/alarm-hidden-behind-offline"
	fleetAlarmPipeHiddenTrue    = " | hidden true"
	fleetAlarmPipeHiddenFalse   = " | hidden false"
	fleetAlarmPipeHiddenInvalid = " | hidden invalid"
	fleetAlarmPipeKindCompDown  = " | kind " + pluginserver.AlarmKindComponentDown
)

// alarmCauseHistoryEvent decodes the CLI JSON contract of one event row from
// `show fleet alarm history device <name> | json compact`, including the
// derived cause-overlap fields.
type alarmCauseHistoryEvent struct {
	ID            string    `json:"id"`
	Kind          string    `json:"kind"`
	Device        string    `json:"device"`
	Subject       string    `json:"subject"`
	Opened        time.Time `json:"opened"`
	Cleared       bool      `json:"cleared"`
	ClearedAt     time.Time `json:"cleared-at"`
	Hidden        bool      `json:"hidden"`
	HiddenBehind  string    `json:"hidden-behind"`
	HiddenUnknown bool      `json:"hidden-unknown"`
}

// alarmCauseHistoryPage decodes the CLI JSON envelope returned by
// `show fleet alarm history device <name> | json compact`.
type alarmCauseHistoryPage struct {
	Device string                   `json:"device"`
	Events []alarmCauseHistoryEvent `json:"events"`
	Total  int                      `json:"total"`
}

func fleetAlarmCausesHubConfig(sshPort, pluginPort, managedPort string) string {
	var b textbuf.Buffer
	b.Str(restoreClientSSHListener(sshPort))
	b.Str("bus {\n    hub {\n")
	b.Str("        server local { ip 127.0.0.1; port ").Str(pluginPort).
		Str("; secret \"").Str(caTrustPluginSecret).Str("\"; }\n")
	b.Str("        server remote-fleet { ip 127.0.0.1; port ").Str(managedPort).
		Str("; secret \"").Str(caTrustHubSecret).Str("\";\n")
	b.Str("            client ").Str(caTrustClientName).Str(" { secret \"").Str(caTrustClientSecret).Str("\"; }\n")
	b.Str("        }\n")
	b.Str("        alarms {\n")
	b.Str("            offline-delay ").Int(fleetAlarmOfflineDelaySec).Str(";\n")
	b.Str("        }\n")
	b.Str("    }\n}\n")
	b.Str("bgp {\n    session { asn { local 65000; } }\n    router-id 10.0.0.1\n}\n")
	return b.String()
}

func queryAlarmCauseHistory(ctx context.Context, s *fleetHoldScenario, device string) (alarmCauseHistoryPage, string, error) {
	cmd := fleetAlarmHistoryPrefix + device + fleetJSONCompactSuffix
	raw, err := storageLiveRequest(ctx, s.hubOperator, cmd)
	if err != nil {
		return alarmCauseHistoryPage{}, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	var page alarmCauseHistoryPage
	if err := decodeAlarmJSON(raw, &page); err != nil {
		return alarmCauseHistoryPage{}, raw, fmt.Errorf("%s: %w: %s", cmd, err, raw)
	}
	return page, raw, nil
}

func findAlarmCauseHistoryEvent(events []alarmCauseHistoryEvent, id string) (int, alarmCauseHistoryEvent) {
	count := 0
	var matched alarmCauseHistoryEvent
	for i := range events {
		if events[i].ID == id {
			count++
			matched = events[i]
		}
	}
	return count, matched
}

func verifyAlarmCausesOnline(
	ctx context.Context,
	s *fleetHoldScenario,
	openPage *pluginserver.AlarmListPage,
	rawOpen string,
) (pluginserver.AlarmOccurrence, error) {
	if openPage.Counts.Visible != 1 {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("online counts.visible=%d, want 1: %s", openPage.Counts.Visible, rawOpen)
	}
	if openPage.Counts.Hidden != 0 {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("online counts.hidden=%d, want 0: %s", openPage.Counts.Hidden, rawOpen)
	}
	orig, okBGP := findOpenAlarm(openPage.Alarms, pluginserver.AlarmKindComponentDown, caTrustClientName, fleetAlarmCompBGP)
	if !okBGP {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("online component-down alarm for %s missing: %s", fleetAlarmCompBGP, rawOpen)
	}
	if err := requireAlarmIDAndSince("online bgp component-down", &orig); err != nil {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("%w: %s", err, rawOpen)
	}
	if orig.HiddenBehind != "" {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("online component-down hidden-behind=%q, want empty: %s", orig.HiddenBehind, rawOpen)
	}

	shownPage, rawShown, err := s.queryAlarmList(ctx, fleetAlarmPipeHiddenFalse)
	if err != nil {
		return pluginserver.AlarmOccurrence{}, err
	}
	if len(shownPage.Alarms) != 1 {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("online hidden false returned %d alarms, want 1: %s", len(shownPage.Alarms), rawShown)
	}
	if shownPage.Alarms[0].ID != orig.ID {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("online hidden false returned id=%q, want %q: %s", shownPage.Alarms[0].ID, orig.ID, rawShown)
	}
	if shownPage.Alarms[0].HiddenBehind != "" {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("online hidden false returned hidden-behind=%q, want empty: %s", shownPage.Alarms[0].HiddenBehind, rawShown)
	}

	hiddenPage, rawHidden, err := s.queryAlarmList(ctx, fleetAlarmPipeHiddenTrue)
	if err != nil {
		return pluginserver.AlarmOccurrence{}, err
	}
	if len(hiddenPage.Alarms) != 0 {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("online hidden true returned %d alarms, want 0: %s", len(hiddenPage.Alarms), rawHidden)
	}
	return orig, nil
}

func verifyAlarmCausesOffline(
	ctx context.Context,
	s *fleetHoldScenario,
	orig *pluginserver.AlarmOccurrence,
) (pluginserver.AlarmOccurrence, error) {
	offlinePage, rawOffline, err := s.waitAlarmList(ctx, "", func(p *pluginserver.AlarmListPage) bool {
		if p.Total != 2 {
			return false
		}
		if len(p.Alarms) != 2 {
			return false
		}
		_, found := findOpenAlarm(p.Alarms, pluginserver.AlarmKindDeviceOffline, caTrustClientName, "")
		return found
	}, "device-offline opened after edge-01 disconnect")
	if err != nil {
		return pluginserver.AlarmOccurrence{}, err
	}
	if offlinePage.Counts.Visible != 1 {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("offline counts.visible=%d, want 1: %s", offlinePage.Counts.Visible, rawOffline)
	}
	if offlinePage.Counts.Hidden != 1 {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("offline counts.hidden=%d, want 1: %s", offlinePage.Counts.Hidden, rawOffline)
	}

	offlineAlarm, okOffline := findOpenAlarm(offlinePage.Alarms, pluginserver.AlarmKindDeviceOffline, caTrustClientName, "")
	if !okOffline {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("offline device-offline alarm missing: %s", rawOffline)
	}
	if err := requireAlarmIDAndSince("device-offline", &offlineAlarm); err != nil {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("%w: %s", err, rawOffline)
	}
	if offlineAlarm.HiddenBehind != "" {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("device-offline hidden-behind=%q, want empty: %s", offlineAlarm.HiddenBehind, rawOffline)
	}

	hiddenBGP, okBGP := findOpenAlarm(offlinePage.Alarms, pluginserver.AlarmKindComponentDown, caTrustClientName, fleetAlarmCompBGP)
	if !okBGP {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("offline component-down alarm missing: %s", rawOffline)
	}
	if err := requireSameAlarmIdentity("offline bgp component-down", orig, &hiddenBGP); err != nil {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("%w: %s", err, rawOffline)
	}
	if hiddenBGP.HiddenBehind != offlineAlarm.ID {
		return pluginserver.AlarmOccurrence{}, fmt.Errorf("offline component-down hidden-behind=%q, want %q: %s", hiddenBGP.HiddenBehind, offlineAlarm.ID, rawOffline)
	}

	if err := verifyOfflineHiddenPipes(ctx, s, orig, &offlineAlarm); err != nil {
		return pluginserver.AlarmOccurrence{}, err
	}
	return offlineAlarm, nil
}

func verifyOfflineHiddenPipes(
	ctx context.Context,
	s *fleetHoldScenario,
	orig, offlineAlarm *pluginserver.AlarmOccurrence,
) error {
	hiddenPage, rawHidden, err := s.queryAlarmList(ctx, fleetAlarmPipeHiddenTrue)
	if err != nil {
		return err
	}
	if len(hiddenPage.Alarms) != 1 {
		return fmt.Errorf("offline hidden true returned %d alarms, want 1: %s", len(hiddenPage.Alarms), rawHidden)
	}
	if hiddenPage.Alarms[0].Kind != pluginserver.AlarmKindComponentDown {
		return fmt.Errorf("offline hidden true returned kind=%q, want %q: %s", hiddenPage.Alarms[0].Kind, pluginserver.AlarmKindComponentDown, rawHidden)
	}
	if hiddenPage.Alarms[0].ID != orig.ID {
		return fmt.Errorf("offline hidden true returned id=%q, want %q: %s", hiddenPage.Alarms[0].ID, orig.ID, rawHidden)
	}
	if hiddenPage.Alarms[0].HiddenBehind != offlineAlarm.ID {
		return fmt.Errorf("offline hidden true returned hidden-behind=%q, want %q: %s", hiddenPage.Alarms[0].HiddenBehind, offlineAlarm.ID, rawHidden)
	}

	shownPage, rawShown, err := s.queryAlarmList(ctx, fleetAlarmPipeHiddenFalse)
	if err != nil {
		return err
	}
	if len(shownPage.Alarms) != 1 {
		return fmt.Errorf("offline hidden false returned %d alarms, want 1: %s", len(shownPage.Alarms), rawShown)
	}
	if shownPage.Alarms[0].Kind != pluginserver.AlarmKindDeviceOffline {
		return fmt.Errorf("offline hidden false returned kind=%q, want %q: %s", shownPage.Alarms[0].Kind, pluginserver.AlarmKindDeviceOffline, rawShown)
	}
	if shownPage.Alarms[0].ID != offlineAlarm.ID {
		return fmt.Errorf("offline hidden false returned id=%q, want %q: %s", shownPage.Alarms[0].ID, offlineAlarm.ID, rawShown)
	}
	if shownPage.Alarms[0].HiddenBehind != "" {
		return fmt.Errorf("offline hidden false returned hidden-behind=%q, want empty: %s", shownPage.Alarms[0].HiddenBehind, rawShown)
	}

	kindPage, rawKind, err := s.queryAlarmList(ctx, fleetAlarmPipeKindCompDown)
	if err != nil {
		return err
	}
	if len(kindPage.Alarms) != 1 {
		return fmt.Errorf("offline kind component-down returned %d alarms, want 1: %s", len(kindPage.Alarms), rawKind)
	}
	if kindPage.Alarms[0].ID != orig.ID {
		return fmt.Errorf("offline kind component-down returned id=%q, want %q: %s", kindPage.Alarms[0].ID, orig.ID, rawKind)
	}
	if kindPage.Alarms[0].HiddenBehind != offlineAlarm.ID {
		return fmt.Errorf("offline kind component-down returned hidden-behind=%q, want %q: %s", kindPage.Alarms[0].HiddenBehind, offlineAlarm.ID, rawKind)
	}

	return verifyInvalidHiddenPipeRefused(ctx, s)
}

func verifyInvalidHiddenPipeRefused(ctx context.Context, s *fleetHoldScenario) error {
	cmd := fleetAlarmShowPrefix + fleetAlarmPipeHiddenInvalid
	output, err := storageLiveRequest(ctx, s.hubOperator, cmd)
	if err == nil {
		if !strings.Contains(strings.ToLower(output), "error") {
			return fmt.Errorf("%q was not refused:\n%s", cmd, output)
		}
	}
	lower := strings.ToLower(output)
	if !strings.Contains(lower, "true") {
		return fmt.Errorf("%q refusal did not name \"true\":\n%s", cmd, output)
	}
	if !strings.Contains(lower, "false") {
		return fmt.Errorf("%q refusal did not name \"false\":\n%s", cmd, output)
	}
	return nil
}

func verifyAlarmCausesReconnected(
	ctx context.Context,
	s *fleetHoldScenario,
	orig, offlineAlarm *pluginserver.AlarmOccurrence,
) error {
	reconnPage, rawReconn, err := s.waitAlarmList(ctx, "", func(p *pluginserver.AlarmListPage) bool {
		if p.Total != 1 {
			return false
		}
		if len(p.Alarms) != 1 {
			return false
		}
		_, offlineOpen := findOpenAlarm(p.Alarms, pluginserver.AlarmKindDeviceOffline, caTrustClientName, "")
		return !offlineOpen
	}, "device-offline cleared after edge-01 reconnect")
	if err != nil {
		return err
	}
	if reconnPage.Counts.Visible != 1 {
		return fmt.Errorf("reconnected counts.visible=%d, want 1: %s", reconnPage.Counts.Visible, rawReconn)
	}
	if reconnPage.Counts.Hidden != 0 {
		return fmt.Errorf("reconnected counts.hidden=%d, want 0: %s", reconnPage.Counts.Hidden, rawReconn)
	}

	reconnBGP, okBGP := findOpenAlarm(reconnPage.Alarms, pluginserver.AlarmKindComponentDown, caTrustClientName, fleetAlarmCompBGP)
	if !okBGP {
		return fmt.Errorf("reconnected component-down alarm missing: %s", rawReconn)
	}
	if err := requireSameAlarmIdentity("reconnected bgp component-down", orig, &reconnBGP); err != nil {
		return fmt.Errorf("%w: %s", err, rawReconn)
	}
	if reconnBGP.HiddenBehind != "" {
		return fmt.Errorf("reconnected component-down hidden-behind=%q, want empty: %s", reconnBGP.HiddenBehind, rawReconn)
	}

	histPage, rawHist, err := queryAlarmCauseHistory(ctx, s, caTrustClientName)
	if err != nil {
		return err
	}
	nOffline, offlineEv := findAlarmCauseHistoryEvent(histPage.Events, offlineAlarm.ID)
	if nOffline != 1 {
		return fmt.Errorf("history has %d rows for device-offline (%s), want 1: %s", nOffline, offlineAlarm.ID, rawHist)
	}
	if offlineEv.Kind != pluginserver.AlarmKindDeviceOffline {
		return fmt.Errorf("history row for %s has kind=%q, want %q: %s", offlineAlarm.ID, offlineEv.Kind, pluginserver.AlarmKindDeviceOffline, rawHist)
	}
	if !offlineEv.Cleared {
		return fmt.Errorf("history row for device-offline (%s) has cleared=false, want true: %s", offlineAlarm.ID, rawHist)
	}
	if offlineEv.ClearedAt.IsZero() {
		return fmt.Errorf("history row for device-offline (%s) missing cleared-at: %s", offlineAlarm.ID, rawHist)
	}

	nBGP, bgpEv := findAlarmCauseHistoryEvent(histPage.Events, orig.ID)
	if nBGP != 1 {
		return fmt.Errorf("history has %d rows for component-down (%s), want 1: %s", nBGP, orig.ID, rawHist)
	}
	if bgpEv.Kind != pluginserver.AlarmKindComponentDown {
		return fmt.Errorf("history row for %s has kind=%q, want %q: %s", orig.ID, bgpEv.Kind, pluginserver.AlarmKindComponentDown, rawHist)
	}
	if bgpEv.Subject != fleetAlarmCompBGP {
		return fmt.Errorf("history row for %s has subject=%q, want %q: %s", orig.ID, bgpEv.Subject, fleetAlarmCompBGP, rawHist)
	}
	if bgpEv.Cleared {
		return fmt.Errorf("history row for still-open component-down (%s) has cleared=true, want false: %s", orig.ID, rawHist)
	}
	if !bgpEv.Hidden {
		return fmt.Errorf("history row for component-down (%s) has hidden=false, want true: %s", orig.ID, rawHist)
	}
	if bgpEv.HiddenBehind != offlineAlarm.ID {
		return fmt.Errorf("history row for component-down (%s) has hidden-behind=%q, want %q: %s", orig.ID, bgpEv.HiddenBehind, offlineAlarm.ID, rawHist)
	}
	return nil
}

// managedFleetAlarmCausesDriver validates fleet-ui-8 AC-1, AC-2, and AC-11
// against a real ze hub daemon: an open component-down alarm on edge-01 gains
// hidden-behind naming device-offline while disconnected, is selectable via
// | hidden true and excluded by | hidden false while invalid | hidden values
// are refused, and loses hidden-behind with its original ID and Since intact
// after reconnect while history records the overlap behind device-offline.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetAlarmCausesDriver(ctx context.Context, args []string) error {
	return fleetHoldRunWithHubConfig(ctx, fleetAlarmCausesFixtureName, args, false, fleetAlarmCausesHubConfig,
		func(ctx context.Context, s *fleetHoldScenario) error {
			// Step 1: Connect declared device edge-01 and report component-down (bgp).
			client, err := dialFleetAlarmClient(ctx, s.hubAddress, caTrustClientSecret)
			if err != nil {
				return err
			}
			defer func() {
				if client != nil {
					_ = client.close() //nolint:errcheck // best-effort cleanup on return
				}
			}()

			bgpDown := makeAlarmHealthReport([]fleet.HealthComponent{
				{Name: fleetAlarmCompBGP, Status: fleet.HealthStatusDown, Reason: "bgp engine down"},
			})
			if err := client.sendHealth(ctx, bgpDown); err != nil {
				return err
			}

			openPage, rawOpen, err := s.waitAlarmList(ctx, "", func(p *pluginserver.AlarmListPage) bool {
				return p.Total == 1 && len(p.Alarms) == 1
			}, "initial component-down alarm for bgp")
			if err != nil {
				return err
			}

			// Step 2: Verify online alarm visibility and hidden pipes.
			origAlarm, err := verifyAlarmCausesOnline(ctx, s, &openPage, rawOpen)
			if err != nil {
				return err
			}

			// Step 3: Stop edge-01 and verify device-offline hides component-down.
			if err := client.close(); err != nil {
				return fmt.Errorf("disconnect edge-01: %w", err)
			}
			client = nil

			offlineAlarm, err := verifyAlarmCausesOffline(ctx, s, &origAlarm)
			if err != nil {
				return err
			}

			// Step 4: Reconnect edge-01 still reporting bgp down and verify visibility
			// restoration, preserved Since, and cause-overlap history marking.
			client, err = dialFleetAlarmClient(ctx, s.hubAddress, caTrustClientSecret)
			if err != nil {
				return fmt.Errorf("reconnect edge-01: %w", err)
			}
			bgpStillDown := makeAlarmHealthReport([]fleet.HealthComponent{
				{Name: fleetAlarmCompBGP, Status: fleet.HealthStatusDown, Reason: "bgp engine still down"},
			})
			if err := client.sendHealth(ctx, bgpStillDown); err != nil {
				return fmt.Errorf("send post-reconnect health report: %w", err)
			}

			if err := verifyAlarmCausesReconnected(ctx, s, &origAlarm, &offlineAlarm); err != nil {
				return err
			}

			return storageLiveOK("alarm hidden-behind-offline hid component-down behind device-offline while disconnected, supported hidden true/false pipes, and restored original Since on reconnect")
		})
}
