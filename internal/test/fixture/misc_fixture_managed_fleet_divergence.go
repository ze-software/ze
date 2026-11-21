// Design: docs/architecture/fleet-config.md -- the hub's copy of the configuration a managed client holds
// Related: register_managed_fleet_divergence.go -- registers the driver
// Related: misc_fixture_managed_fleet_hold.go -- fleetHoldRun, fleetHoldScenario, the hub and the holding device this driver reuses

package fixture

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/ze-software/ze/internal/component/config"
)

// The two hub commands the verdicts read, and the keys of their answers
// (internal/component/managed, cmd_divergence.go and the column order in
// register.go).
const (
	fleetDivergenceListCommand = "show fleet divergence list | json compact"
	fleetDivergenceShowCommand = "show fleet divergence name " + caTrustClientName + " | json compact"
)

// fleetDivergenceToken is a secret leaf the device's edit adds outside its
// hub block: the hub's secret must stay, or the device would not authenticate.
// An api-server token with no transport enabled starts nothing.
const fleetDivergenceToken = "fleet-divergence-device-api-token-0123" // #nosec G101 -- a fixture token for an api-server that enables no transport, so nothing ever accepts it.

// fleetDivergenceTokenPath is the diff path of that leaf.
const fleetDivergenceTokenPath = "environment/api-server/token"

// fleetDivergenceRow is one row of `show fleet divergence list`.
type fleetDivergenceRow struct {
	Client        string `json:"client"`
	Declared      bool   `json:"declared"`
	DeviceVersion string `json:"device-version"`
	HubVersion    string `json:"hub-version"`
	EditedAt      string `json:"edited-at"`
	ReceivedAt    string `json:"received-at"`
}

// fleetDivergenceList is the answer of `show fleet divergence list`. The
// command is registered as a table (command.ShapeTab), and the CLI's JSON form
// of a table is the array of its rows, so that is the one form decoded here:
// any other shape is refused, so a change of the CLI's form fails the test.
type fleetDivergenceList struct {
	Clients []fleetDivergenceRow
}

// UnmarshalJSON reads the rows array the CLI renders for the table, and
// refuses any other document.
func (l *fleetDivergenceList) UnmarshalJSON(data []byte) error {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '[' {
		return fmt.Errorf("the divergence list is not the array of rows the CLI renders for a table: %s", trimmed)
	}
	return json.Unmarshal(trimmed, &l.Clients)
}

// fleetDivergenceDetail is the part of `show fleet divergence name <client>`
// the verdicts read.
type fleetDivergenceDetail struct {
	Client        string                  `json:"client"`
	DeviceVersion string                  `json:"device-version"`
	HubVersion    string                  `json:"hub-version"`
	ChangeCount   int                     `json:"change-count"`
	Changes       []fleetDivergenceChange `json:"changes"`
}

// fleetDivergenceChange is one change of the show's diff.
type fleetDivergenceChange struct {
	Path   string `json:"path"`
	Change string `json:"change"`
	Old    any    `json:"old"`
	New    any    `json:"new"`
}

// divergedConfig is the configuration the device is given with its daemon
// stopped: router-id routerID and a secret outside its hub block.
func (s *fleetHoldScenario) divergedConfig(routerID string) string {
	return s.config(routerID) + "environment {\n    api-server {\n        token \"" + fleetDivergenceToken + "\";\n    }\n}\n"
}

// managedFleetDivergenceUploadDriver answers one question: does a managed
// device that holds send the configuration it runs to its hub, once, so that
// the hub lists the client with both hashes, shows what differs without the
// configuration text, and stops reporting the hold as a refused configuration
// (spec-fleet-7 AC-1, AC-2, AC-10)?
//
// Method: the hub and the device of fleetHoldRun. The device takes the hub's
// configuration, is given another one with its daemon stopped, and is started
// again: it holds. `show fleet divergence list` on the hub MUST list the
// client with the hash of the device's configuration and the hash of the
// hub's, `show fleet divergence name <client>` MUST answer the same hashes and
// at least one change, carry no configuration text (the client's secret
// appears in neither answer), report the secret the device's edit added outside
// its hub block masked, and the hub's `show doctor` MUST NOT list the
// hold under doctor-managed-client-rejected. A further write on the hub MUST
// be held again with no second upload: the record keeps the device hash and
// the time it was received, and names the hub's new hash.
//
// Args: <hub-ssh-port> <hub-plugin-port> <hub-managed-port>.
func managedFleetDivergenceUploadDriver(ctx context.Context, args []string) error {
	return fleetHoldRun(ctx, "managed/fleet-divergence-upload", args, false, func(ctx context.Context, s *fleetHoldScenario) error {
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			return s.requireFollows(ctx, daemon, restoreClientServedRouterID, "the first connection")
		}); err != nil {
			return err
		}
		if err := s.editStoppedTo(ctx, s.divergedConfig(fleetHoldLocalRouterID)); err != nil {
			return err
		}
		if err := s.withDevice(ctx, func(daemon *cachedBootDaemon) error {
			return s.requireUploaded(ctx, daemon)
		}); err != nil {
			return err
		}

		fmt.Fprintln(os.Stderr, "OK: a holding managed device sent its configuration to its hub once, "+ //nolint:errcheck // fixture progress line
			"and the hub listed it with both hashes and showed what differs, device:", managedVersionHash([]byte(s.divergedConfig(fleetHoldLocalRouterID))),
			"hub:", s.version(fleetHoldFurtherRouterID))
		return nil
	})
}

// requireUploaded is the device started on its own configuration while the
// hub holds another: the hub MUST hold its copy, show it, and keep it across a
// further hub write that the device holds against.
func (s *fleetHoldScenario) requireUploaded(ctx context.Context, daemon *cachedBootDaemon) error {
	device := managedVersionHash([]byte(s.divergedConfig(fleetHoldLocalRouterID)))
	first, err := s.requireDivergenceRow(ctx, daemon, device, s.version(restoreClientServedRouterID), "on connecting")
	if err != nil {
		return err
	}
	if err := s.requireDivergenceDetail(ctx, device, s.version(restoreClientServedRouterID)); err != nil {
		return err
	}
	// The copy is on the hub when the row is listed, and the device
	// acknowledges its hold after the upload, so the report is read after it.
	doctor, _ := storageLiveRequest(ctx, s.hubOperator, "show doctor") //nolint:errcheck // a report that carries a warning may exit non-zero; the answer is what is judged
	if strings.Contains(doctor, fleetHoldRefusalText) {
		return fmt.Errorf("show doctor on the hub lists the hold of %s as a refused configuration although it holds the copy:\n%s",
			caTrustClientName, doctor)
	}

	if err := s.hubImport(ctx, fleetHoldFurtherRouterID); err != nil {
		return err
	}
	held := cachedBootWait(ctx, daemon, fleetHoldLogAttempts, func() bool {
		return fleetHoldLogLines(daemon.output.String(), fleetHoldWarnMessage, s.version(fleetHoldFurtherRouterID)) > 0
	})
	if !held {
		return fmt.Errorf("the device did not hold against the hub's further change (offered %s), or its daemon stopped",
			s.version(fleetHoldFurtherRouterID))
	}
	again, err := s.requireDivergenceRow(ctx, daemon, device, s.version(fleetHoldFurtherRouterID), "after the hub's further change")
	if err != nil {
		return err
	}
	if again.ReceivedAt != first.ReceivedAt {
		return fmt.Errorf("the hub received the device's configuration again after its further change (received-at %q, then %q): "+
			"a hub that holds the device's hash MUST get no second upload", first.ReceivedAt, again.ReceivedAt)
	}
	return nil
}

// requireDivergenceRow waits until `show fleet divergence list` on the hub
// lists the client with deviceVersion and hubVersion, declared by a server
// block, and answers that row. The list carries no secret of the client.
func (s *fleetHoldScenario) requireDivergenceRow(ctx context.Context, daemon *cachedBootDaemon,
	deviceVersion, hubVersion, state string,
) (fleetDivergenceRow, error) {
	var answer string
	var row fleetDivergenceRow
	listed := cachedBootWait(ctx, daemon, fleetHoldDoctorAttempts, func() bool {
		answer, _ = storageLiveRequest(ctx, s.hubOperator, fleetDivergenceListCommand) //nolint:errcheck // an answer that is no list is "not yet"
		var list fleetDivergenceList
		if fleetDivergenceDecode(answer, &list) != nil {
			return false
		}
		for _, candidate := range list.Clients {
			if candidate.Client != caTrustClientName {
				continue
			}
			row = candidate
			return candidate.DeviceVersion == deviceVersion && candidate.HubVersion == hubVersion
		}
		return false
	})
	if !listed {
		return row, fmt.Errorf("%s: %q on the hub never listed %s with device-version %s and hub-version %s, or the device stopped; "+
			"the last answer:\n%s", state, fleetDivergenceListCommand, caTrustClientName, deviceVersion, hubVersion, answer)
	}
	if !row.Declared {
		return row, fmt.Errorf("%s: the hub lists %s as declared by no server block: %+v", state, caTrustClientName, row)
	}
	if row.ReceivedAt == "" {
		return row, fmt.Errorf("%s: the hub lists %s with no time it received the copy: %+v", state, caTrustClientName, row)
	}
	if strings.Contains(answer, caTrustClientSecret) {
		return row, fmt.Errorf("%s: %q answered the client's secret:\n%s", state, fleetDivergenceListCommand, answer)
	}
	return row, nil
}

// requireDivergenceDetail requires `show fleet divergence name <client>` on
// the hub to answer both hashes and at least one change, and no configuration
// text: neither the client's secret nor the device's router-id line.
func (s *fleetHoldScenario) requireDivergenceDetail(ctx context.Context, deviceVersion, hubVersion string) error {
	answer, err := storageLiveRequest(ctx, s.hubOperator, fleetDivergenceShowCommand)
	if err != nil {
		return fmt.Errorf("%q on the hub failed: %w\n%s", fleetDivergenceShowCommand, err, answer)
	}
	var detail fleetDivergenceDetail
	if err := fleetDivergenceDecode(answer, &detail); err != nil {
		return fmt.Errorf("%q on the hub: %w\n%s", fleetDivergenceShowCommand, err, answer)
	}
	if detail.Client != caTrustClientName || detail.DeviceVersion != deviceVersion || detail.HubVersion != hubVersion {
		return fmt.Errorf("%q answered client %q, device-version %q and hub-version %q, want %q, %q and %q",
			fleetDivergenceShowCommand, detail.Client, detail.DeviceVersion, detail.HubVersion,
			caTrustClientName, deviceVersion, hubVersion)
	}
	if detail.ChangeCount == 0 || detail.ChangeCount != len(detail.Changes) {
		return fmt.Errorf("%q answered change-count %d and %d changes, want the same number and at least one: the router-id differs",
			fleetDivergenceShowCommand, detail.ChangeCount, len(detail.Changes))
	}
	if strings.Contains(answer, caTrustClientSecret) {
		return fmt.Errorf("%q answered the client's secret:\n%s", fleetDivergenceShowCommand, answer)
	}
	if strings.Contains(answer, "router-id "+fleetHoldLocalRouterID) {
		return fmt.Errorf("%q answered configuration text:\n%s", fleetDivergenceShowCommand, answer)
	}
	if strings.Contains(answer, fleetDivergenceToken) {
		return fmt.Errorf("%q answered the secret the device's edit added:\n%s", fleetDivergenceShowCommand, answer)
	}
	for _, change := range detail.Changes {
		if strings.HasPrefix(change.Path, fleetDivergenceTokenPath) || strings.HasPrefix(fleetDivergenceTokenPath, change.Path+"/") {
			if !strings.Contains(fmt.Sprint(change.New), config.SecretDataPlaceholder) {
				return fmt.Errorf("%q answered the added secret unmasked at %s: %v", fleetDivergenceShowCommand, change.Path, change.New)
			}
			return nil
		}
	}
	return fmt.Errorf("%q answered no change at %s, which the device's edit added:\n%s",
		fleetDivergenceShowCommand, fleetDivergenceTokenPath, answer)
}

// fleetDivergenceDecode decodes the JSON document of a `ze cli` answer into
// into. The answer starts at its first brace: anything before it is no part
// of the document.
func fleetDivergenceDecode(answer string, into any) error {
	// The answer's JSON starts at its first array or object: a table is
	// rendered as an array of rows, a document as an object.
	start := strings.IndexAny(answer, "[{")
	if start < 0 {
		return errors.New("the answer carries no JSON document")
	}
	return json.NewDecoder(strings.NewReader(answer[start:])).Decode(into)
}
