package fixture

import (
	"context"
	"fmt"
	"os"
	"reflect"
	"strings"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

func plugin15SetFD(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin15WaitEOR(ctx, p, "*"); err != nil {
		return err
	}
	r := plugin15Dispatch(ctx, p, "set system file-descriptors max")
	if strings.Contains(r.text(), "only supported on Linux") {
		fmt.Fprintln(os.Stderr, "OK: set file-descriptors not available on this platform")
		return nil
	}
	if !plugin15Done(r) {
		return fmt.Errorf("fd-set: status=%s data=%s", r.status, r.text())
	}
	m, err := plugin15Map(r)
	if err != nil {
		return fmt.Errorf("fd-set: expected dict: %w", err)
	}
	for _, key := range []string{"previous", "current", "hard-limit"} {
		if _, ok := m[key]; !ok {
			return fmt.Errorf("fd-set: missing %q in %v", key, m)
		}
	}
	if !reflect.DeepEqual(m["current"], m["hard-limit"]) {
		return fmt.Errorf("fd-set: max should set current=%v to hard-limit=%v", m["current"], m["hard-limit"])
	}
	fmt.Fprintf(os.Stderr, "OK: fd-set previous=%v current=%v hard=%v\n", m["previous"], m["current"], m["hard-limit"])
	return nil
}

func plugin15ShutdownPrompt(context.Context, *sdk.Plugin) error {
	fmt.Fprintln(os.Stderr, "OK: daemon up, requesting shutdown")
	return nil
}

func plugin15SignalStop(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin15WaitEOR(ctx, p, "*"); err != nil {
		return err
	}
	r := plugin15Dispatch(ctx, p, "request shutdown")
	if !plugin15Done(r) {
		return fmt.Errorf("shutdown status=%s data=%s", r.status, r.text())
	}
	fmt.Fprintln(os.Stderr, "OK: daemon shutdown dispatched")
	return nil
}

func plugin15SubsystemList(ctx context.Context, p *sdk.Plugin) error {
	if err := plugin15WaitEOR(ctx, p, "*"); err != nil {
		return err
	}
	r := plugin15Dispatch(ctx, p, "system subsystem list")
	if !plugin15Done(r) {
		return fmt.Errorf("subsystem-list: status=%s data=%s", r.status, r.text())
	}
	data, err := plugin15Map(r)
	if err != nil {
		return err
	}
	subsystems, _ := data["subsystems"].([]any)
	count := plugin15Number(data["count"])
	if count < 1 {
		return fmt.Errorf("subsystem-list: expected at least 1 subsystem, got %v", count)
	}
	var own map[string]any
	names := make([]string, 0, len(subsystems))
	for _, value := range subsystems {
		subsystem, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("subsystem-list: entry is not a dict: %v", value)
		}
		for _, field := range []string{"name", "stage", "running", "command-count"} {
			if _, ok := subsystem[field]; !ok {
				return fmt.Errorf("subsystem-list: missing field %q in %v", field, subsystem)
			}
		}
		name, _ := subsystem["name"].(string)
		names = append(names, name)
		if name == "subsystem-list-test" {
			if own != nil {
				return fmt.Errorf("subsystem-list: expected exactly one own row")
			}
			own = subsystem
		}
	}
	if own == nil {
		return fmt.Errorf("subsystem-list: own row missing")
	}
	if plugin15Number(own["command-count"]) != 2 {
		return fmt.Errorf("subsystem-list: command-count=%v, want 2", own["command-count"])
	}
	fmt.Fprintf(os.Stderr, "OK: subsystem-list returned %v subsystem(s): %v\n", count, names)
	return nil
}

func plugin15SysctlDescribe(ctx context.Context, p *sdk.Plugin) error {
	show := plugin15DispatchUntilDone(ctx, p, "show sysctl")
	if !plugin15Done(show) {
		return fmt.Errorf("sysctl show: status=%s", show.status)
	}
	entries, err := plugin15Slice(show)
	if err != nil {
		return fmt.Errorf("sysctl show: expected list: %w", err)
	}
	if entries == nil {
		entries = []any{}
	}
	keysResult := plugin15Dispatch(ctx, p, "show sysctl keys")
	if !plugin15Done(keysResult) {
		return fmt.Errorf("sysctl list for describe: status=%s", keysResult.status)
	}
	keys, err := plugin15Slice(keysResult)
	if err != nil || len(keys) == 0 {
		return fmt.Errorf("sysctl list returned empty, cannot test describe")
	}
	first, _ := keys[0].(map[string]any)
	key, _ := first["key"].(string)
	detailResult := plugin15Dispatch(ctx, p, "show sysctl key "+key)
	if !plugin15Done(detailResult) {
		return fmt.Errorf("show sysctl key %s: status=%s", key, detailResult.status)
	}
	detail, err := plugin15Map(detailResult)
	if err != nil {
		return err
	}
	if detail["key"] != key {
		return fmt.Errorf("sysctl describe: key=%v want %q", detail["key"], key)
	}
	if detail["description"] == nil || detail["description"] == "" {
		return fmt.Errorf("show sysctl key %s: missing description", key)
	}
	if detail["type"] == nil || detail["type"] == "" {
		return fmt.Errorf("show sysctl key %s: missing type", key)
	}
	return nil
}

func plugin15SysctlList(ctx context.Context, p *sdk.Plugin) error {
	r := plugin15DispatchUntilDone(ctx, p, "show sysctl keys")
	if !plugin15Done(r) {
		return fmt.Errorf("sysctl list: status=%s", r.status)
	}
	keys, err := plugin15Slice(r)
	if err != nil || len(keys) == 0 {
		return fmt.Errorf("sysctl list: expected non-empty list")
	}
	foundForwarding := false
	for _, value := range keys {
		entry, ok := value.(map[string]any)
		if !ok {
			return fmt.Errorf("sysctl list: entry is not dict: %v", value)
		}
		for _, field := range []string{"key", "description", "type"} {
			if _, ok := entry[field]; !ok {
				return fmt.Errorf("sysctl list: missing field %s in %v", field, entry)
			}
		}
		key, _ := entry["key"].(string)
		foundForwarding = foundForwarding || strings.Contains(key, "forwarding")
	}
	if !foundForwarding {
		return fmt.Errorf("sysctl list: no forwarding key found")
	}
	return nil
}
