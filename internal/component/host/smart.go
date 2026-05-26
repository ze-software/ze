// Design: plan/learned/695-host-3-smart.md — SMART health monitoring
// Overview: inventory.go — SmartInfo type
// Detail: smart_linux.go — detectSMART via direct ioctl (linux-only)

package host

import (
	"encoding/json"
	"fmt"
	"strings"
)

// parseSMARTJSON extracts SmartInfo from raw smartctl JSON output.
// Uses map-based extraction because smartctl emits snake_case keys
// which do not match ze's kebab-case JSON convention.
func parseSMARTJSON(data []byte) (*SmartInfo, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("smart: parse smartctl json: %w", err)
	}

	exitStatus, messages := parseSmartctlSection(raw["smartctl"])
	if exitStatus&0x02 != 0 {
		note := "SMART not available"
		for _, msg := range messages {
			if strings.Contains(msg, "Unavailable") || strings.Contains(msg, "not supported") {
				note = msg
				break
			}
		}
		return &SmartInfo{
			Unavailable:     true,
			UnavailableNote: note,
		}, nil
	}

	return &SmartInfo{
		Healthy:      parseSmartStatus(raw["smart_status"]),
		TempCelsius:  parseTemperature(raw["temperature"]),
		PowerOnHours: parsePowerOnTime(raw["power_on_time"]),
		ErrorCount:   parseErrorCount(raw["ata_smart_error_log"]),
	}, nil
}

func parseSmartctlSection(data json.RawMessage) (int, []string) {
	if data == nil {
		return 0, nil
	}
	var sc map[string]json.RawMessage
	if json.Unmarshal(data, &sc) != nil {
		return 0, nil
	}
	var exitStatus int
	json.Unmarshal(sc["exit_status"], &exitStatus) //nolint:errcheck // best-effort

	var msgs []struct {
		S string `json:"string"`
	}
	json.Unmarshal(sc["messages"], &msgs) //nolint:errcheck // best-effort

	strs := make([]string, len(msgs))
	for i, m := range msgs {
		strs[i] = m.S
	}
	return exitStatus, strs
}

func parseSmartStatus(data json.RawMessage) bool {
	if data == nil {
		return false
	}
	var ss map[string]json.RawMessage
	if json.Unmarshal(data, &ss) != nil {
		return false
	}
	var passed bool
	json.Unmarshal(ss["passed"], &passed) //nolint:errcheck // best-effort
	return passed
}

func parseTemperature(data json.RawMessage) int {
	if data == nil {
		return 0
	}
	var t map[string]json.RawMessage
	if json.Unmarshal(data, &t) != nil {
		return 0
	}
	var current int
	json.Unmarshal(t["current"], &current) //nolint:errcheck // best-effort
	return current
}

func parsePowerOnTime(data json.RawMessage) uint64 {
	if data == nil {
		return 0
	}
	var pot map[string]json.RawMessage
	if json.Unmarshal(data, &pot) != nil {
		return 0
	}
	var hours uint64
	json.Unmarshal(pot["hours"], &hours) //nolint:errcheck // best-effort
	return hours
}

func parseErrorCount(data json.RawMessage) uint64 {
	if data == nil {
		return 0
	}
	var el map[string]json.RawMessage
	if json.Unmarshal(data, &el) != nil {
		return 0
	}
	var summary map[string]json.RawMessage
	if json.Unmarshal(el["summary"], &summary) != nil {
		return 0
	}
	var count uint64
	json.Unmarshal(summary["count"], &count) //nolint:errcheck // best-effort
	return count
}
