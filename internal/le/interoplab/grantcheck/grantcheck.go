// Design: docs/architecture/testing/interop.md -- lab containers hold only what their daemons need
//
// Package grantcheck reads the privileges a `docker run` argv hands a
// container, so a test can pin a lab container's grant set EXACTLY
// (spec-lab-containers-least-privilege). A test that checks only that the
// wanted grants are present passes a container that also runs with
// --cap-add ALL, seccomp=unconfined or --privileged; comparing the whole set
// fails on any extra.
package grantcheck

import (
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/interoplab"
	lepath "github.com/ze-software/ze/internal/le/le/path"
)

// Grants is every privilege-bearing option of one container, each list
// sorted. Only options that widen what the container may do are read.
type Grants struct {
	// Capabilities are the --cap-add values, and the capability list a lab
	// peer declares beside its arguments.
	Capabilities []string
	// Devices are the --device values.
	Devices []string
	// SecurityOptions are the --security-opt values (seccomp=, apparmor=,
	// systempaths=, no-new-privileges...).
	SecurityOptions []string
	// Sysctls are the --sysctl values.
	Sysctls []string
	// HostNamespaces are the --pid, --ipc, --uts, --userns and --cgroupns
	// values, each written as flag=value.
	HostNamespaces []string
	// Networks are the --network and --net values. A lab peer's own network
	// is chosen by the lab outside its arguments, so any value here (host
	// above all) is a grant.
	Networks []string
	// DeviceCgroupRules are the --device-cgroup-rule values: each lets the
	// container open device nodes no --device names.
	DeviceCgroupRules []string
	// Mounts are the -v, --volume and --mount values, plus every mount of a
	// lab peer that is writable or whose source lies outside the directories
	// the lab stages its files in (ParsePeer).
	Mounts []string
	// AppArmorProfile is the registered Ze profile a lab peer runs under
	// (interoplab.PeerConfig.AppArmorProfile); empty is docker-default.
	AppArmorProfile string
	// Privileged is set by --privileged in any spelling Docker reads as true.
	Privileged bool
}

// valueFlags maps each flag that carries a privilege value to the list it
// fills.
var valueFlags = map[string]func(*Grants, string){
	"--cap-add":      func(g *Grants, v string) { g.Capabilities = append(g.Capabilities, v) },
	"--device":       func(g *Grants, v string) { g.Devices = append(g.Devices, v) },
	"--security-opt": func(g *Grants, v string) { g.SecurityOptions = append(g.SecurityOptions, v) },
	"--sysctl":       func(g *Grants, v string) { g.Sysctls = append(g.Sysctls, v) },
	"--pid":          func(g *Grants, v string) { g.HostNamespaces = append(g.HostNamespaces, "--pid="+v) },
	"--ipc":          func(g *Grants, v string) { g.HostNamespaces = append(g.HostNamespaces, "--ipc="+v) },
	"--uts":          func(g *Grants, v string) { g.HostNamespaces = append(g.HostNamespaces, "--uts="+v) },
	"--userns":       func(g *Grants, v string) { g.HostNamespaces = append(g.HostNamespaces, "--userns="+v) },
	"--cgroupns":     func(g *Grants, v string) { g.HostNamespaces = append(g.HostNamespaces, "--cgroupns="+v) },
	"--network":      func(g *Grants, v string) { g.Networks = append(g.Networks, v) },
	"--net":          func(g *Grants, v string) { g.Networks = append(g.Networks, v) },
	"--device-cgroup-rule": func(g *Grants, v string) {
		g.DeviceCgroupRules = append(g.DeviceCgroupRules, v)
	},
	"-v":       func(g *Grants, v string) { g.Mounts = append(g.Mounts, v) },
	"--volume": func(g *Grants, v string) { g.Mounts = append(g.Mounts, v) },
	"--mount":  func(g *Grants, v string) { g.Mounts = append(g.Mounts, v) },
}

// ParsePeer answers the grants of one lab peer: its argv and declared
// capabilities (Parse), its AppArmor profile, and each of its mounts that
// is a grant. A read-only mount whose source lies strictly inside one of
// stagedRoots is the lab handing the daemon its own config file and is not
// a grant; a writable mount, or one from anywhere else (/, the Docker
// socket, /lib/modules), is recorded as source:target[:ro].
func ParsePeer(peer interoplab.PeerConfig, stagedRoots ...string) Grants {
	g := Parse(peer.Capabilities, peer.Arguments)
	g.AppArmorProfile = peer.AppArmorProfile
	for _, mount := range peer.Mounts {
		if mount.ReadOnly && stagedInside(mount.Source, stagedRoots) {
			continue
		}
		value := mount.Source + ":" + mount.Target
		if mount.ReadOnly {
			value += ":ro"
		}
		g.Mounts = append(g.Mounts, value)
	}
	slices.Sort(g.Mounts)
	return g
}

// StagedRoots answers the directories a lab stages its peers' files in: the
// checkout (test/interop-*, tmp/ rendered configs) and the system temporary
// directory. It is the stagedRoots a privilege test passes to ParsePeer.
func StagedRoots(tb testing.TB) []string {
	tb.Helper()
	root, err := lepath.Root()
	if err != nil {
		tb.Fatalf("resolve the checkout the lab stages files in: %v", err)
	}
	return []string{root, os.TempDir()}
}

// stagedInside answers whether source lies strictly below one of roots. The
// root itself is not inside it, so mounting the whole checkout is a grant.
func stagedInside(source string, roots []string) bool {
	clean := filepath.Clean(source)
	for _, root := range roots {
		relative, err := filepath.Rel(filepath.Clean(root), clean)
		if err != nil {
			continue
		}
		if relative == "." {
			continue
		}
		if relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			continue
		}
		return true
	}
	return false
}

// privilegedValue answers whether a --privileged=<value> spelling grants
// privilege. Docker reads the value as a boolean; a value it would refuse is
// reported as privileged, so a typo never hides the flag.
func privilegedValue(value string) bool {
	granted, err := strconv.ParseBool(value)
	if err != nil {
		return true
	}
	return granted
}

// Parse answers the grants in argv, plus capabilities a lab peer declares
// outside its argv (interoplab.PeerConfig.Capabilities). Both the
// `--flag value` and the `--flag=value` spellings are read.
func Parse(capabilities, argv []string) Grants {
	g := Grants{Capabilities: slices.Clone(capabilities)}
	for i := 0; i < len(argv); i++ {
		argument := argv[i]
		flag, value, joined := strings.Cut(argument, "=")
		if flag == "--privileged" {
			if !joined || privilegedValue(value) {
				g.Privileged = true
			}
			continue
		}
		add, ok := valueFlags[flag]
		if !ok {
			continue
		}
		if joined {
			add(&g, value)
			continue
		}
		if i+1 < len(argv) {
			i++
			add(&g, argv[i])
		}
	}
	for _, list := range []*[]string{&g.Capabilities, &g.Devices, &g.SecurityOptions, &g.Sysctls, &g.HostNamespaces, &g.Networks, &g.DeviceCgroupRules, &g.Mounts} {
		slices.Sort(*list)
	}
	return g
}

// Problems answers one line for each way got differs from want, empty when
// they match exactly. want's lists need not be sorted.
func Problems(got, want Grants) []string {
	var problems []string
	compare := func(what string, gotList, wantList []string) {
		wantSorted := slices.Clone(wantList)
		slices.Sort(wantSorted)
		if slices.Equal(gotList, wantSorted) {
			return
		}
		var tb textbuf.Buffer
		problems = append(problems, tb.Str(what).Str(" = [").Str(strings.Join(gotList, " ")).
			Str("], want exactly [").Str(strings.Join(wantSorted, " ")).Byte(']').String())
	}
	compare("capabilities", got.Capabilities, want.Capabilities)
	compare("devices", got.Devices, want.Devices)
	compare("security options", got.SecurityOptions, want.SecurityOptions)
	compare("sysctls", got.Sysctls, want.Sysctls)
	compare("host namespaces", got.HostNamespaces, want.HostNamespaces)
	compare("networks", got.Networks, want.Networks)
	compare("device cgroup rules", got.DeviceCgroupRules, want.DeviceCgroupRules)
	compare("mounts", got.Mounts, want.Mounts)
	if got.AppArmorProfile != want.AppArmorProfile {
		problems = append(problems, "AppArmor profile = \""+got.AppArmorProfile+"\", want \""+want.AppArmorProfile+"\"")
	}
	if got.Privileged != want.Privileged {
		if got.Privileged {
			problems = append(problems, "runs --privileged")
		} else {
			problems = append(problems, "lacks --privileged")
		}
	}
	return problems
}
