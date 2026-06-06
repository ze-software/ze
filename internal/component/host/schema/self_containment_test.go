package schema

import (
	"strings"
	"testing"
)

func TestHostCmdSchemaOwnsShowHost(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:host-cpu"`,
		`ze:command "ze-show:host-nic"`,
		`ze:command "ze-show:host-dmi"`,
		`ze:command "ze-show:host-memory"`,
		`ze:command "ze-show:host-thermal"`,
		`ze:command "ze-show:host-storage"`,
		`ze:command "ze-show:host-kernel"`,
		`ze:command "ze-show:host-platform"`,
		`ze:command "ze-show:host-all"`,
		"container host",
		"container cpu",
		"container nic",
		"container dmi",
		"container thermal",
		"container storage",
		"container all",
	} {
		if !strings.Contains(ZeHostCmdYANG, want) {
			t.Errorf("ze-host-cmd.yang must declare %q so removing host removes the surface", want)
		}
	}
}

func TestHostCmdSchemaOwnsSystemKernelLog(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-show:system-kernel-log"`,
		"container kernel-log",
	} {
		if !strings.Contains(ZeHostCmdYANG, want) {
			t.Errorf("ze-host-cmd.yang must declare %q so removing host removes the surface", want)
		}
	}
}

func TestHostSetCmdSchemaOwnsSetFileDescriptors(t *testing.T) {
	for _, want := range []string{
		`ze:command "ze-set:system-file-descriptors"`,
		"container file-descriptors",
	} {
		if !strings.Contains(ZeHostSetCmdYANG, want) {
			t.Errorf("ze-host-set-cmd.yang must declare %q so removing host removes the surface", want)
		}
	}
}
