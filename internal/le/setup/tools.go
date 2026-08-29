// Design: docs/architecture/core-design.md -- what a Ze dev or test workflow needs
//
// The table contains the authoritative setup data. Code for probing,
// installation, reporting, and appliance drift checks reads this table. To add
// a tool, add one row.

package setup

import "runtime"

// StaticcheckVersion is the one release of staticcheck this repository runs.
// A different one on PATH runs and disagrees, which is worse than one that is
// absent, so probes.go checks the version rather than the presence.
//
// Staticcheck type-checks with its own copy of the unified IR reader, so a
// release built against an x/tools older than the ambient Go toolchain cannot
// decode that toolchain's export data and reports every package as an import
// failure. 2026.1 reads to export data version 2 and Go 1.27 writes version 4,
// so raising the Go directive in go.mod raises this constant with it.
const StaticcheckVersion = "2026.2.1"

// GolangCIVersion is the one release of golangci-lint this repository runs.
// The linter type-checks with its own copy of go/types, so a release older
// than the Go directive in go.mod cannot read the export data the ambient
// toolchain writes and reports every package as a typecheck failure. Raising
// that directive therefore raises this constant with it.
const GolangCIVersion = "v2.13.1"

// PackageManager is the one package manager this host installs system packages
// with.
type PackageManager string

const (
	// ManagerBrew is Homebrew, on macOS.
	ManagerBrew PackageManager = "brew"
	// ManagerApt is apt, on Linux.
	ManagerApt PackageManager = "apt"
	// ManagerNone is neither, which is a platform this tool cannot install
	// for rather than a failure of the tool.
	ManagerNone PackageManager = ""
)

// grubByMachine names the GRUB module-set package for each host architecture.
//
// GRUB supplies one module set for each EFI target. Debian packages each set
// only for its own host architecture. grub-efi-amd64-bin is Architecture: amd64.
// On an arm64 machine, apt reports "has no installation candidate" and installs
// nothing, including grub-mkstandalone. Tests in debian:stable-slim measured
// this behavior on both architectures.
//
// arm64 does not install the amd64 package and installs grub-efi-arm64-bin
// instead. amd64 installs the amd64 package.
//
// Both packages install grub-common as a dependency. grub-mkstandalone is in
// grub-common.
//
// Therefore, the code requests the set that the host can build with.
// `ze appliance iso` selects its target from the architecture of the image that
// it packs (isoGRUBTarget, internal/appliance/cmd_iso.go). To build an ISO for
// the OTHER architecture, also install that architecture's set through
// `dpkg --add-architecture`.
var grubByMachine = map[string]string{
	"aarch64": "grub-efi-arm64-bin",
	"arm64":   "grub-efi-arm64-bin",
	"i386":    "grub-efi-ia32-bin",
	"i686":    "grub-efi-ia32-bin",
}

// grubDefault is what a host outside the table gets. Ze's appliance targets are
// amd64 and arm64 (isoGRUBTarget, internal/appliance/cmd_iso.go), and a host
// outside the table cannot build for itself whatever this answers.
const grubDefault = "grub-efi-amd64-bin"

// grubAptPackage answers the GRUB module-set package this host architecture can
// install. Debian names one package per EFI target and builds each only for its
// own architecture, so the answer is the host's, not a preference.
// machine is spelled the way uname reports it, which is what Python's
// platform.machine() answers and what hostMachine derives from runtime.GOARCH.
func grubAptPackage(machine string) string {
	if named, ok := grubByMachine[machine]; ok {
		return named
	}
	return grubDefault
}

// unameByArch converts runtime.GOARCH to the spelling used by uname -m. The GRUB
// table uses the uname vocabulary. The Go vocabulary differs for each
// architecture that has a package here. An absent value has the same spelling
// in both vocabularies, such as riscv64, ppc64le, or s390x. It passes through to
// the default in the same way as the script.
var unameByArch = map[string]string{
	"amd64": "x86_64",
	"arm64": "aarch64",
	"386":   "i686",
	"arm":   "armv7l",
}

// hostMachine answers this host's architecture in uname's spelling.
func hostMachine() string {
	if machine, ok := unameByArch[runtime.GOARCH]; ok {
		return machine
	}
	return runtime.GOARCH
}

// Each tool name occurs once. The name is the report label and the executable
// that the probe searches for. It is frequently also the package name. Multiple
// spellings of one name increase the risk of a typing error.
const (
	toolGo          = "go"
	toolGit         = "git"
	toolStaticcheck = "staticcheck"
	toolGopls       = "gopls"
	toolE2fsprogs   = "e2fsprogs"
	toolXorriso     = "xorriso"
	toolSshpass     = "sshpass"
	toolDocker      = "docker"
	toolColima      = "colima"
	toolXl2tpd      = "xl2tpd"
)

// The two platforms this tool installs for, spelled the way runtime.GOOS does.
const (
	osDarwin = "darwin"
	osLinux  = "linux"
)

// Tool is one thing that must be on the machine, and every route to putting it
// there.
type Tool struct {
	// Name is what the report calls it.
	Name string
	// Probe is the executable names to look for.
	Probe []string
	// ProbeAny says one of those names is enough. Without it every name must
	// be found, which is what a package shipping two required binaries needs.
	ProbeAny bool
	// Brew is the Homebrew formula, when there is one.
	Brew string
	// Apt is the Debian package, when there is one.
	Apt string
	// GoInstall is a `go install` target. It works on both platforms, so it
	// wins over the system package manager.
	GoInstall string
	// Required says the run fails without it.
	Required bool
	// Note is a sentence for a reader who has to decide what to do about it.
	Note string
	// DoctorCheck is the appliance doctor check that requires this tool.
	// An empty value means the tool has no appliance doctor check.
	DoctorCheck string
}

// packageFor answers the package name this manager would install, if it has
// one.
func (t Tool) packageFor(manager PackageManager) string {
	if manager == ManagerBrew {
		return t.Brew
	}
	if manager == ManagerApt {
		return t.Apt
	}
	return ""
}

// installableBy reports whether a method can install this tool on this host.
//
// A tool without an installation method is neither present nor missing. It is
// skipped. This result distinguishes an accurate report from a failure on a
// platform where installation is not possible.
func (t Tool) installableBy(manager PackageManager) bool {
	if t.GoInstall != "" {
		return true
	}
	return t.packageFor(manager) != ""
}

// goplsNote explains the effect of an absent gopls. The session-start rule makes
// the LSP tool BLOCKING, but the query text disables the rule and does not check
// the server. Therefore, no other repository check can detect this condition.
const goplsNote = "language server behind the agent LSP tool; without it every LSP call" +
	" returns ENOENT and the session reads whole files instead" +
	" (ai/rules/context-economy.md)"

// e2fsNote records why macOS finds e2fsprogs without a PATH entry.
const e2fsNote = "keg-only on macOS; Go code resolves via Cellar glob"

// grubNote records why macOS skips grub.
const grubNote = "no first-party Homebrew formula; macOS skips (ISO builds are Linux/container-only)"

// The pinned `go install` targets.
const golangciTarget = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@" + GolangCIVersion

// staticcheckTarget names the same release the probe checks the running binary
// against, so a bump moves both at once.
const staticcheckTarget = "honnef.co/go/tools/cmd/staticcheck@" + StaticcheckVersion

const goimportsTarget = "golang.org/x/tools/cmd/goimports@latest"
const goplsTarget = "golang.org/x/tools/gopls"

// requiredTools contains every tool that a Go development or test workflow
// requires.
func requiredTools() []Tool {
	return []Tool{
		{Name: toolGo, Probe: []string{toolGo}, Brew: toolGo, Apt: "golang-go", Required: true},
		{Name: toolGit, Probe: []string{toolGit}, Brew: toolGit, Apt: toolGit, Required: true},
		{Name: "protobuf", Probe: []string{"protoc"}, Brew: "protobuf", Apt: "protobuf-compiler", Required: true},
		{Name: "jq", Probe: []string{"jq"}, Brew: "jq", Apt: "jq", Required: true},
		{Name: toolGolangCI, Probe: []string{toolGolangCI}, GoInstall: golangciTarget, Required: true},
		{Name: toolStaticcheck, Probe: []string{toolStaticcheck}, GoInstall: staticcheckTarget, Required: true},
		{Name: toolGoimports, Probe: []string{toolGoimports}, GoInstall: goimportsTarget, Required: true},
		{Name: toolGopls, Probe: []string{toolGopls}, GoInstall: goplsTarget, Required: true, Note: goplsNote},

		{
			Name:     "qemu",
			Probe:    []string{"qemu-system-x86_64", "qemu-system-aarch64"},
			ProbeAny: true,
			Brew:     "qemu",
			Apt:      "qemu-system-x86",
			Required: true,
		},
		{
			Name:        toolE2fsprogs,
			Probe:       []string{"mkfs.ext4", "debugfs"},
			ProbeAny:    true,
			Brew:        toolE2fsprogs,
			Apt:         toolE2fsprogs,
			Required:    true,
			Note:        e2fsNote,
			DoctorCheck: "appliance-e2fsprogs",
		},
		{
			Name:        toolXorriso,
			Probe:       []string{toolXorriso},
			Brew:        toolXorriso,
			Apt:         toolXorriso,
			Required:    true,
			DoctorCheck: "appliance-xorriso",
		},
		{
			Name:        "grub",
			Probe:       []string{"grub-mkstandalone", "grub2-mkstandalone"},
			ProbeAny:    true,
			Apt:         grubAptPackage(hostMachine()),
			Required:    true,
			Note:        grubNote,
			DoctorCheck: "appliance-grub",
		},
	}
}

// optionalTools is every tool whose absence is reported and forgiven.
func optionalTools() []Tool {
	return []Tool{
		{
			Name:  toolSshpass,
			Probe: []string{toolSshpass},
			Brew:  toolSshpass,
			Apt:   toolSshpass,
			Note:  "optional SSH probe fallback",
		},
		{
			Name:  toolDocker,
			Probe: []string{toolDocker},
			Brew:  toolDocker,
			Apt:   "docker.io",
			Note:  "container appliance/kernel builds",
		},
		{Name: toolColima, Probe: []string{toolColima}, Brew: toolColima, Note: "macOS Docker runtime"},
		{
			Name:  toolXl2tpd,
			Probe: []string{toolXl2tpd},
			Apt:   toolXl2tpd,
			Note: "Linux root-only L2TP LAC peer for L2TP PPP evidence tests" +
				" (ze-deployment-l2tp-ppp-test, ze-deployment-gokrazy-l2tp-ppp-test)",
		},
		{
			Name:  "ppp",
			Probe: []string{"pppd"},
			Apt:   "ppp",
			Note:  "Linux root-only pppd for the same L2TP PPP/NCP evidence tests",
		},
	}
}

// allTools is the whole table, required first, in the order the report visits
// them.
func allTools() []Tool {
	return append(requiredTools(), optionalTools()...)
}

// The external binaries this setup runs, and the subcommands and flags it
// repeats across them. aptBin and toolGit are declared with the install table.
const (
	versionFlag       = "--version"
	versionArgument   = "-version"
	versionSubcommand = "version"
	firewallBin       = "ufw"
	aptQuietFlag      = "-qq"
	toolGoimports     = "goimports"
	toolGolangCI      = "golangci-lint"
	gitApply          = "apply"
	toolClaude        = "claude"
	envBin            = "env"
	goModSubcommand   = "mod"
	toolNode          = "node"
	toolNpm           = "npm"
	statusSubcommand  = "status"
	updateSubcommand  = "update"
)
