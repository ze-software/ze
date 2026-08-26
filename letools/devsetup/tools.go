// Design: docs/architecture/core-design.md -- what a Ze dev or test workflow needs
//
// tools.go is a Go port of scripts/le/devtools/tools.py. The table contains the
// data. Code for probing, installation, and reporting reads this table from
// other locations. To add a tool, add one row.
//
// The script has one item that this file omits: `APPLIANCE_CHECKS` and
// `ApplianceCheck`. This omission is intentional. That list duplicates the
// probes and packages in the grub, xorriso, and e2fsprogs rows. The duplicate
// lets a Go test read the appliance doctor dependency names from a Python file.
// internal/appliance/dev_setup_drift_test.go parses tools.py with a regex.
// scripts/le/devtools_test.py verifies that the duplicate rows equal the source
// tool rows.
//
// In Go, both sides of that comparison are Go data, so the duplicate is no
// longer necessary. The three tool rows below contain the data.
// TestTheApplianceRowsCarryTheDoctorsDependencies verifies the relationship.
// When step 14 of spec-le-is-a-ze-binary deletes the script, the drift test
// compares applianceDoctorChecks() with these rows.

package devsetup

import "runtime"

// StaticcheckVersion is the one release of staticcheck this repository runs.
// A different one on PATH runs and disagrees, which is worse than one that is
// absent, so probes.go checks the version rather than the presence.
const StaticcheckVersion = "2026.1"

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

// GrubAptPackage answers the GRUB module-set package this host architecture can
// install. Debian names one package per EFI target and builds each only for its
// own architecture, so the answer is the host's, not a preference.
//
// machine is spelled the way uname reports it, which is what Python's
// platform.machine() answers and what HostMachine derives from runtime.GOARCH.
func GrubAptPackage(machine string) string {
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

// HostMachine answers this host's architecture in uname's spelling.
func HostMachine() string {
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
	toolPython3     = "python3"
	toolE2fsprogs   = "e2fsprogs"
	toolXorriso     = "xorriso"
	toolPipx        = "pipx"
	toolRuff        = "ruff"
	toolMypy        = "mypy"
	toolPyright     = "pyright"
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
	// PipxInstall is a pipx package. It works on both platforms too.
	PipxInstall string
	// Required says the run fails without it.
	Required bool
	// Note is a sentence for a reader who has to decide what to do about it.
	Note string
}

// PackageFor answers the package name this manager would install, if it has
// one.
func (t Tool) PackageFor(manager PackageManager) string {
	if manager == ManagerBrew {
		return t.Brew
	}
	if manager == ManagerApt {
		return t.Apt
	}
	return ""
}

// InstallableBy reports whether a method can install this tool on this host.
//
// A tool without an installation method is neither present nor missing. It is
// skipped. This result distinguishes an accurate report from a failure on a
// platform where installation is not possible.
func (t Tool) InstallableBy(manager PackageManager) bool {
	if t.GoInstall != "" || t.PipxInstall != "" {
		return true
	}
	return t.PackageFor(manager) != ""
}

// goplsNote explains the effect of an absent gopls. The session-start rule makes
// the LSP tool BLOCKING, but the query text disables the rule and does not check
// the server. Therefore, no other repository check can detect this condition.
const goplsNote = "language server behind the agent LSP tool; without it every LSP call" +
	" returns ENOENT and the session reads whole files instead" +
	" (ai/rules/context-economy.md)"

// pyrightNote is the Python half of the same story.
const pyrightNote = "language server for the Python half of the tree; gopls answers a" +
	" symbol question about a .go file, pyright answers the same" +
	" question about a .py one (ai/rules/context-economy.md)"

// uvNote records why uv uses pipx on BOTH platforms. Linux requires this method
// because uv is not in the Debian or Ubuntu repositories. An empty Apt value
// gave a REQUIRED tool no package on Linux. InstallableBy returns false for that
// state, and the loop reports [skipped]. As a result, the evidence SSH probe did
// not have its required tool (`uv run --with paramiko`).
//
// Check mode reported "All required tools present" on a machine without uv.
// Thus, the guard did not detect the absent tool.
const uvNote = "not in apt repos, so pipx is the one route that works on both platforms"

// e2fsNote records why macOS finds e2fsprogs without a PATH entry.
const e2fsNote = "keg-only on macOS; Go code resolves via Cellar glob"

// grubNote records why macOS skips grub.
const grubNote = "no first-party Homebrew formula; macOS skips (ISO builds are Linux/container-only)"

// mypyNote names the gate that stops working when mypy is absent.
const mypyNote = "the type gate for scripts/le; strict mode, configured in" +
	" pyproject.toml and run by `le lint`"

// The pinned `go install` targets.
const golangciTarget = "github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.10.1"

// staticcheckTarget names the same release the probe checks the running binary
// against, so a bump moves both at once.
const staticcheckTarget = "honnef.co/go/tools/cmd/staticcheck@" + StaticcheckVersion

const goimportsTarget = "golang.org/x/tools/cmd/goimports@latest"

const goplsTarget = "golang.org/x/tools/gopls@latest"

// RequiredTools contains every tool that a run requires.
//
// The pipx row precedes each tool that pipx installs. A pipx installation is
// skipped if pipx is not yet present. The installation uses pipx instead of brew
// on macOS. It also uses pipx instead of the curl installer on all platforms.
// One installation method has one maintenance point. In addition, `curl | sh`
// would introduce a supply-chain risk on every dev machine.
func RequiredTools() []Tool {
	return []Tool{
		{Name: toolGo, Probe: []string{toolGo}, Brew: toolGo, Apt: "golang-go", Required: true},
		{Name: toolGit, Probe: []string{toolGit}, Brew: toolGit, Apt: toolGit, Required: true},
		{Name: "protobuf", Probe: []string{"protoc"}, Brew: "protobuf", Apt: "protobuf-compiler", Required: true},
		{Name: "jq", Probe: []string{"jq"}, Brew: "jq", Apt: "jq", Required: true},
		{Name: "golangci-lint", Probe: []string{"golangci-lint"}, GoInstall: golangciTarget, Required: true},
		{Name: toolStaticcheck, Probe: []string{toolStaticcheck}, GoInstall: staticcheckTarget, Required: true},
		{Name: "goimports", Probe: []string{"goimports"}, GoInstall: goimportsTarget, Required: true},
		{Name: toolGopls, Probe: []string{toolGopls}, GoInstall: goplsTarget, Required: true, Note: goplsNote},
		{Name: toolPython3, Probe: []string{toolPython3}, Brew: "python", Apt: toolPython3, Required: true},
		{
			Name:     "qemu",
			Probe:    []string{"qemu-system-x86_64", "qemu-system-aarch64"},
			ProbeAny: true,
			Brew:     "qemu",
			Apt:      "qemu-system-x86",
			Required: true,
		},
		{
			Name:     toolE2fsprogs,
			Probe:    []string{"mkfs.ext4", "debugfs"},
			ProbeAny: true,
			Brew:     toolE2fsprogs,
			Apt:      toolE2fsprogs,
			Required: true,
			Note:     e2fsNote,
		},
		{Name: toolXorriso, Probe: []string{toolXorriso}, Brew: toolXorriso, Apt: toolXorriso, Required: true},
		{
			Name:     "grub",
			Probe:    []string{"grub-mkstandalone", "grub2-mkstandalone"},
			ProbeAny: true,
			Apt:      GrubAptPackage(HostMachine()),
			Required: true,
			Note:     grubNote,
		},
		{Name: toolPipx, Probe: []string{toolPipx}, Brew: toolPipx, Apt: toolPipx, Required: true},
		{Name: "uv", Probe: []string{"uv"}, PipxInstall: "uv", Required: true, Note: uvNote},
		{Name: toolRuff, Probe: []string{toolRuff}, PipxInstall: toolRuff, Required: true},
		{Name: toolMypy, Probe: []string{toolMypy}, PipxInstall: toolMypy, Required: true, Note: mypyNote},
		{
			Name:        toolPyright,
			Probe:       []string{toolPyright, "pyright-langserver"},
			PipxInstall: toolPyright,
			Required:    true,
			Note:        pyrightNote,
		},
	}
}

// OptionalTools is every tool whose absence is reported and forgiven.
func OptionalTools() []Tool {
	return []Tool{
		{
			Name:  toolSshpass,
			Probe: []string{toolSshpass},
			Brew:  toolSshpass,
			Apt:   toolSshpass,
			Note:  "SSH-probe fallback only; uv+paramiko is primary",
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

// AllTools is the whole table, required first, in the order the report visits
// them.
func AllTools() []Tool {
	return append(RequiredTools(), OptionalTools()...)
}
