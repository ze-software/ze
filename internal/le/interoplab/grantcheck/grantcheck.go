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
// sorted. Parse reads every flag of the argv: a flag that can widen what the
// container may do fills one of these lists, a flag classified as no grant
// (nonGrantFlags) is skipped, and any other flag lands in Unclassified, which
// Problems always reports. An argv flag nobody classified therefore fails the
// exact check rather than passing it unread.
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
	// OtherGrants are the --volumes-from, --gpus, --runtime, --cgroup-parent,
	// --user, --group-add, --cap-drop and --ulimit values, each written as
	// flag=value: each changes who the container runs as, what it shares or
	// which limits bind it, and no lab container sets one today.
	OtherGrants []string
	// Unclassified are the flags Parse found in neither grantFlags nor
	// nonGrantFlags, as written. Problems reports every one whatever the
	// want, because an unread flag can be a grant.
	Unclassified []string
	// AppArmorProfile is the registered Ze profile a lab peer runs under
	// (interoplab.PeerConfig.AppArmorProfile); empty is docker-default.
	AppArmorProfile string
	// Privileged is set by --privileged in any spelling Docker reads as true.
	Privileged bool
}

// grantFlags maps each flag that carries a privilege value to the list it
// fills. Every one takes a value.
var grantFlags = map[string]func(*Grants, string){
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
	"--volume":        func(g *Grants, v string) { g.Mounts = append(g.Mounts, v) },
	"--mount":         func(g *Grants, v string) { g.Mounts = append(g.Mounts, v) },
	"--volumes-from":  otherGrant("--volumes-from"),
	"--gpus":          otherGrant("--gpus"),
	"--runtime":       otherGrant("--runtime"),
	"--cgroup-parent": otherGrant("--cgroup-parent"),
	"--user":          otherGrant("--user"),
	"--group-add":     otherGrant("--group-add"),
	"--cap-drop":      otherGrant("--cap-drop"),
	"--ulimit":        otherGrant("--ulimit"),
}

// otherGrant answers the grantFlags entry that records flag=value in
// OtherGrants.
func otherGrant(flag string) func(*Grants, string) {
	return func(g *Grants, v string) { g.OtherGrants = append(g.OtherGrants, flag+"="+v) }
}

// nonGrantFlags lists every flag the lab argvs use that widens nothing, and
// whether it takes a value. A flag missing from here and from grantFlags is
// Unclassified, so classifying a new flag is a decision written here.
var nonGrantFlags = map[string]bool{
	"--detach":      false,
	"--entrypoint":  true,
	"--env":         true,
	"--interactive": false,
	"--ip":          true,
	"--ip6":         true,
	"--name":        true,
	"--platform":    true,
	"--rm":          false,
	"--tty":         false,
	"--workdir":     true,
}

// shortFlags maps each one-letter docker flag to its long spelling.
var shortFlags = map[byte]string{
	'd': "--detach",
	'e': "--env",
	'i': "--interactive",
	't': "--tty",
	'u': "--user",
	'v': "--volume",
	'w': "--workdir",
}

// dockerSubcommands are the positional words before a `docker run` argv's
// flags. Any other positional is the image, and what follows it is the
// container's own command, which Parse does not read as docker flags.
var dockerSubcommands = map[string]bool{"container": true, "create": true, "run": true}

// ParsePeer answers the grants of one lab peer: its argv and declared
// capabilities (Parse), its AppArmor profile, and each of its mounts that
// is a grant. A read-only mount of a file or directory whose source,
// symlinks resolved, lies strictly inside one of stagedRoots is the lab
// handing the daemon its own config file and is not a grant; a writable
// mount, a unix socket wherever it lies (a socket hands the container the
// service behind it), or a mount from anywhere else (/, the Docker socket,
// /lib/modules) is recorded as source:target[:ro].
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

// stagedInside answers whether source, symlinks resolved, lies strictly
// below one of roots, also resolved, and is not a unix socket. The root
// itself is not inside it, so mounting the whole checkout is a grant, and a
// symlink inside a root that points out of it is judged where it points.
func stagedInside(source string, roots []string) bool {
	resolved := resolvePath(source)
	if info, err := os.Stat(resolved); err == nil && info.Mode()&os.ModeSocket != 0 {
		return false
	}
	for _, root := range roots {
		relative, err := filepath.Rel(resolvePath(root), resolved)
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

// resolvePath answers path cleaned, with every symlink in its longest
// existing prefix resolved. A lab may name a file it has not rendered yet,
// so the part that does not exist is kept as written below the resolved
// prefix; it holds no symlink because it holds nothing.
func resolvePath(path string) string {
	clean := filepath.Clean(path)
	missing := ""
	for prefix := clean; ; prefix = filepath.Dir(prefix) {
		resolved, err := filepath.EvalSymlinks(prefix)
		if err == nil {
			return filepath.Join(resolved, missing)
		}
		if prefix == filepath.Dir(prefix) {
			return clean
		}
		missing = filepath.Join(filepath.Base(prefix), missing)
	}
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
// outside its argv (interoplab.PeerConfig.Capabilities). argv is a whole
// `docker run` argv or a lab peer's extra arguments. Every spelling Docker
// reads is read: `--flag value`, `--flag=value`, a short flag with its value
// joined (`-v/:/host`, `-v=/:/host`) or separate, and combined short
// booleans (`-it`). Reading stops at `--` or at the image, the first
// positional that is not a docker subcommand.
func Parse(capabilities, argv []string) Grants {
	g := Grants{Capabilities: slices.Clone(capabilities)}
	for i := 0; i < len(argv); i++ {
		argument := argv[i]
		if argument == "--" {
			break
		}
		if !strings.HasPrefix(argument, "-") || argument == "-" {
			if dockerSubcommands[argument] {
				continue
			}
			break
		}
		if strings.HasPrefix(argument, "--") {
			flag, value, joined := strings.Cut(argument, "=")
			i = g.readFlag(flag, value, joined, argument, argv, i)
			continue
		}
		i = g.readShortFlags(argument, argv, i)
	}
	for _, list := range []*[]string{
		&g.Capabilities, &g.Devices, &g.SecurityOptions, &g.Sysctls, &g.HostNamespaces,
		&g.Networks, &g.DeviceCgroupRules, &g.Mounts, &g.OtherGrants, &g.Unclassified,
	} {
		slices.Sort(*list)
	}
	return g
}

// readShortFlags reads one single-dash token as Docker does: each letter is a
// flag, and the first letter whose flag takes a value takes the rest of the
// token (an `=` after the letter is dropped) or, when nothing is left, the
// next argument. It answers the index of the last argument consumed.
func (g *Grants) readShortFlags(argument string, argv []string, i int) int {
	for at := 1; at < len(argument); at++ {
		flag, known := shortFlags[argument[at]]
		if !known {
			g.Unclassified = append(g.Unclassified, argument)
			return i
		}
		rest := argument[at+1:]
		if !takesValue(flag) {
			g.readFlag(flag, "", false, argument, argv, i)
			continue
		}
		if rest == "" {
			return g.readFlag(flag, "", false, argument, argv, i)
		}
		g.readFlag(flag, strings.TrimPrefix(rest, "="), true, argument, argv, i)
		return i
	}
	return i
}

// takesValue answers whether a classified flag takes a value. An
// unclassified flag is answered false, so it never swallows the next
// argument; it is reported either way.
func takesValue(flag string) bool {
	if _, grant := grantFlags[flag]; grant {
		return true
	}
	return nonGrantFlags[flag]
}

// readFlag records one flag. written is the argument as the argv spelled it,
// for Unclassified. When the flag takes a value and none was joined, the next
// argument is the value. It answers the index of the last argument consumed.
func (g *Grants) readFlag(flag, value string, joined bool, written string, argv []string, i int) int {
	if flag == "--privileged" {
		if !joined || privilegedValue(value) {
			g.Privileged = true
		}
		return i
	}
	add, grant := grantFlags[flag]
	needsValue, nonGrant := nonGrantFlags[flag]
	if !grant && !nonGrant {
		g.Unclassified = append(g.Unclassified, written)
		return i
	}
	if !grant && !needsValue {
		return i
	}
	if !joined {
		if i+1 >= len(argv) {
			g.Unclassified = append(g.Unclassified, written+" (no value)")
			return i
		}
		i++
		value = argv[i]
	}
	if grant {
		add(g, value)
	}
	return i
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
	compare("other grants", got.OtherGrants, want.OtherGrants)
	for _, flag := range got.Unclassified {
		problems = append(problems, "carries "+flag+", a flag grantcheck has not classified as a grant or as no grant; "+
			"add it to grantFlags or nonGrantFlags in internal/le/interoplab/grantcheck")
	}
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
