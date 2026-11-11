package subdispatch

import "testing"

func TestRegisteredDefaultHandlesBareCommand(t *testing.T) {
	d := New("install", "test")
	calls := 0
	d.RegisterDefault(func(args []string) int {
		calls++
		if len(args) != 0 {
			t.Fatalf("bare command supplied arguments: %v", args)
		}
		return 17
	}, SubMeta{Desc: "Install this host"})
	if code := d.Dispatch(nil); code != 17 || calls != 1 {
		t.Fatalf("default code=%d calls=%d", code, calls)
	}
}

func TestDefaultDoesNotHandleHelpOrUnknownTargets(t *testing.T) {
	for _, args := range [][]string{{"help"}, {"--help"}, {"-h"}, {"unknown"}} {
		d := New("install", "test")
		d.RegisterDefault(func([]string) int {
			t.Fatalf("default ran for %v", args)
			return 0
		}, SubMeta{Desc: "Install this host"})
		code := d.Dispatch(args)
		if args[0] == "unknown" && code == 0 {
			t.Fatal("unknown target succeeded")
		}
	}
}

func TestDefaultReceivesOptionsAndExplicitTargetsKeepTheirOwner(t *testing.T) {
	d := New("install", "test")
	d.RegisterDefault(func(args []string) int {
		if len(args) != 1 || args[0] != "--dry-run" {
			t.Fatalf("default options=%v", args)
		}
		return 11
	}, SubMeta{Desc: "Install this host"})
	d.Register("local", func([]string) int { return 12 }, SubMeta{Desc: "Copy only"})
	if code := d.Dispatch([]string{"--dry-run"}); code != 11 {
		t.Fatalf("option route=%d", code)
	}
	if code := d.Dispatch([]string{"local"}); code != 12 {
		t.Fatalf("explicit route=%d", code)
	}
}
