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
	"slices"
	"strings"

	"github.com/ze-software/ze/internal/core/textbuf"
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
	// Privileged is set by --privileged.
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
}

// Parse answers the grants in argv, plus capabilities a lab peer declares
// outside its argv (interoplab.PeerConfig.Capabilities). Both the
// `--flag value` and the `--flag=value` spellings are read.
func Parse(capabilities, argv []string) Grants {
	g := Grants{Capabilities: slices.Clone(capabilities)}
	for i := 0; i < len(argv); i++ {
		argument := argv[i]
		if argument == "--privileged" || argument == "--privileged=true" {
			g.Privileged = true
			continue
		}
		flag, value, joined := strings.Cut(argument, "=")
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
	for _, list := range []*[]string{&g.Capabilities, &g.Devices, &g.SecurityOptions, &g.Sysctls, &g.HostNamespaces} {
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
	if got.Privileged != want.Privileged {
		if got.Privileged {
			problems = append(problems, "runs --privileged")
		} else {
			problems = append(problems, "lacks --privileged")
		}
	}
	return problems
}
