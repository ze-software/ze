// Design: docs/guide/route-policy.md -- the system RIB has two execution placements.
// Related: register_sysrib_execution.go -- registers the paired daemon observer.
// Related: ../../component/sysrib/candidates.go -- authoritative candidate rendering.

package fixture

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

const (
	sysribExecutionOwner  = "sysrib-owner"
	sysribExecutionTagged = "198.18.211.0/24"
	sysribExecutionPlain  = "198.18.212.0/24"

	// sysribPlacementInternal and sysribPlacementForked are the two
	// placement arguments a .ci passes: the system RIB runs as a goroutine in
	// the daemon, or as a forked plugin process.
	sysribPlacementInternal = "internal"
	sysribPlacementForked   = "forked"

	// sysribPluginName is the system RIB's plugin name, as in ze plugin rib.
	sysribPluginName = "rib"
)

// sysribExecutionCandidate is the stored static path read over the command API.
// Tag deliberately retains presence, since zero is a valid configured tag.
type sysribExecutionCandidate struct {
	Source   string  `json:"source"`
	Instance uint32  `json:"instance"`
	Distance uint8   `json:"distance"`
	Metric   uint32  `json:"metric"`
	Tag      *uint32 `json:"tag"`
	Selected bool    `json:"selected"`
	Hidden   bool    `json:"hidden"`
}

type sysribExecutionRow struct {
	Prefix string                     `json:"prefix"`
	Paths  []sysribExecutionCandidate `json:"paths"`
}

func sysribExecutionDriver(ctx context.Context, args []string) error {
	if len(args) != 2 {
		return errors.New("sysrib execution fixture takes placement internal|forked and proof kernel")
	}
	placement := args[0]
	if placement != sysribPlacementInternal {
		if placement != sysribPlacementForked {
			return fmt.Errorf("unknown system RIB placement %q", placement)
		}
	}
	if args[1] != "kernel" {
		return fmt.Errorf("unknown system RIB proof %q: use kernel", args[1])
	}
	return Observe(ctx, "sysrib-execution-test", &sdk.Registration{}, func(ctx context.Context, plugin *sdk.Plugin) error {
		ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
		defer cancel()
		if err := sysribExecutionPlacement(ctx, plugin, placement); err != nil {
			return err
		}
		// Candidates are queried before selected output: the old external
		// command refuses here even though its child completed TLS startup.
		for _, command := range []string{
			"show rib candidates",
			"show rib candidates prefix " + sysribExecutionTagged,
			"show rib candidates prefix " + sysribExecutionPlain,
		} {
			if err := sysribExecutionCandidates(ctx, plugin, command); err != nil {
				return err
			}
		}
		for _, prefix := range []string{sysribExecutionTagged, sysribExecutionPlain} {
			if err := waitRIBWinner(ctx, plugin, prefix, protocolStatic); err != nil {
				return err
			}
		}
		var output textbuf.Buffer
		if err := output.Str("OK: static controls reached candidates and system output\n").StdErr(); err != nil {
			return err
		}
		return sysribExecutionKernel(ctx)
	})
}

// sysribExecutionPlacement checks the actual daemon's process manager and OS
// children. The daemon PID comes from its authenticated session RPC, rather
// than assuming the observer's immediate parent survived a shell exec.
func sysribExecutionPlacement(ctx context.Context, plugin *sdk.Plugin, placement string) error {
	return sysribExecutionPluginPlacement(ctx, plugin, sysribExecutionOwner, sysribPluginName, placement)
}

func sysribExecutionPluginPlacement(ctx context.Context, plugin *sdk.Plugin, configuredOwner, pluginName, placement string) error {
	var session struct {
		PID int `json:"pid"`
	}
	data, err := requireDone02(ctx, plugin, "plugin session ping")
	if err != nil {
		return err
	}
	if err := decode02(data, &session); err != nil {
		return err
	}
	if session.PID < 1 {
		return fmt.Errorf("plugin session ping returned no daemon PID: %s", data)
	}
	data, err = requireDone02(ctx, plugin, "system subsystem list")
	if err != nil {
		return err
	}
	var state struct {
		Subsystems []struct {
			Name     string `json:"name"`
			Stage    string `json:"stage"`
			Running  bool   `json:"running"`
			Commands int    `json:"command-count"`
		} `json:"subsystems"`
	}
	if err := decode02(data, &state); err != nil {
		return err
	}
	found := false
	for _, owner := range state.Subsystems {
		if owner.Name != configuredOwner {
			continue
		}
		if !owner.Running {
			return fmt.Errorf("configured system RIB owner is not running: %s", data)
		}
		if owner.Stage != "Running" {
			return fmt.Errorf("configured system RIB owner has not completed startup: %s", data)
		}
		if owner.Commands < 1 {
			return fmt.Errorf("configured system RIB owner registered no commands: %s", data)
		}
		found = true
	}
	if !found {
		return fmt.Errorf("process manager has no configured system RIB owner: %s", data)
	}
	children, observed, err := sysribExecutionPluginChildren(ctx, session.PID, pluginName)
	if err != nil {
		return fmt.Errorf("%w; daemon descendants: %s", err, observed)
	}
	want := 0
	if placement == sysribPlacementForked {
		want = 1
	}
	if len(children) != want {
		return fmt.Errorf("%s %s: daemon %d has ze plugin %s child PIDs %v, want %d; daemon descendants: %s", placement, configuredOwner, session.PID, pluginName, children, want, observed)
	}
	var proof textbuf.Buffer
	subject := pluginName
	if pluginName == sysribPluginName {
		subject = "sysrib"
	}
	proof.Str("OK: ").Str(subject).Str(" placement ").Str(placement).Str(", running owner and child count ").Uint(uint64(len(children)))
	proof.Str("; daemon PID ").Uint(uint64(session.PID))
	for _, pid := range children {
		proof.Str("; ze plugin ").Str(pluginName).Str(" PID ").Uint(uint64(pid))
	}
	return proof.Byte('\n').StdErr()
}

// sysribExecutionChildren follows PPIDs so both an exec-optimized shell and a
// retained shell count the real ze child once. A Running external owner can
// only have passed process.startExternal's authenticated TLS waiter.
func sysribExecutionChildren(ctx context.Context, daemon int) ([]int, string, error) {
	return sysribExecutionPluginChildren(ctx, daemon, sysribPluginName)
}

func sysribExecutionPluginChildren(ctx context.Context, daemon int, pluginName string) ([]int, string, error) {
	var observed textbuf.Buffer
	observed.Str("self pid=").Uint(uint64(os.Getpid())).Str(" ppid=").Uint(uint64(os.Getppid())).Str(";")
	parents, err := sysribExecutionParentCensus(ctx, "/proc")
	if err != nil {
		observed.Str(" PID/PPID census unavailable; daemon-present=unknown; self-present=unknown;")
		return nil, observed.String(), err
	}
	pids := make([]int, 0, len(parents))
	for pid := range parents {
		pids = append(pids, pid)
	}
	slices.Sort(pids)
	observed.Str(" PID/PPID census=[")
	for _, pid := range pids {
		observed.Uint(uint64(pid)).Byte(':').Uint(uint64(parents[pid])).Byte(' ')
	}
	observed.Str("]; daemon-present=")
	if _, present := parents[daemon]; !present {
		observed.Str("false;")
		return nil, observed.String(), errors.New("authenticated daemon PID is absent from procfs; run observer and daemon in the same PID namespace")
	}
	observed.Str("true; self-present=")
	if _, present := parents[os.Getpid()]; !present {
		observed.Str("false;")
		return nil, observed.String(), errors.New("observer PID is absent from procfs; mount procfs for the observer's PID namespace")
	}
	observed.Str("true;")
	daemonExecutable, err := sysribProcessExecutable(ctx, daemon)
	if err != nil {
		return nil, observed.String(), fmt.Errorf("inspect daemon executable identity: %w", err)
	}
	observed.Str(" expected executable=").Quoted(daemonExecutable).Str(";")
	var children []int
	for _, pid := range pids {
		parent := parents[pid]
		descendant := false
		for range 16 {
			if parent == daemon {
				descendant = true
				break
			}
			if parent < 2 {
				break
			}
			parent = parents[parent]
		}
		if !descendant {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, observed.String(), err
		}
		observed.Str(" pid=").Uint(uint64(pid)).Str(" ppid=").Uint(uint64(parents[pid]))
		// Read exact NUL-delimited arguments only for this daemon's descendants.
		raw, err := os.ReadFile("/proc/" + strconv.Itoa(pid) + "/cmdline")
		if errors.Is(err, os.ErrNotExist) {
			observed.Str(" exited during observation;")
			continue
		}
		if err != nil {
			return nil, observed.String(), fmt.Errorf("read descendant %d argv: %w", pid, err)
		}
		args := strings.Split(strings.TrimSuffix(string(raw), "\x00"), "\x00")
		observed.Str(" argv=[")
		for _, arg := range args {
			observed.Quoted(arg).Byte(' ')
		}
		observed.Byte(']')
		if !sysribExecutionPluginArgs(args, pluginName) {
			observed.Str(" different command;")
			continue
		}
		executable, err := sysribProcessExecutable(ctx, pid)
		if err != nil {
			return nil, observed.String(), fmt.Errorf("inspect system RIB child %d executable identity: %w", pid, err)
		}
		observed.Str(" executable=").Quoted(executable)
		if executable != daemonExecutable {
			observed.Str(" executable differs;")
			continue
		}
		observed.Str(" matched;")
		children = append(children, pid)
	}
	return children, observed.String(), nil
}

// sysribExecutionParentCensus reads only kernel PID/PPID metadata. Processes
// that disappear during enumeration are absent; other read/parse errors refuse
// the observation. root is /proc in the real fixture and a private test tree in
// the unit controls.
func sysribExecutionParentCensus(ctx context.Context, root string) (map[int]int, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("enumerate kernel PID directories: %w", err)
	}
	parents := make(map[int]int)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			continue
		}
		if pid < 1 {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		raw, err := os.ReadFile(filepath.Join(root, entry.Name(), "status")) //nolint:gosec // <root>/<pid>/status of a PID directory this census just enumerated
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if errors.Is(err, syscall.ESRCH) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("read kernel parent for PID %d: %w", pid, err)
		}
		parent, err := sysribExecutionParentStatus(raw)
		if err != nil {
			return nil, fmt.Errorf("read kernel parent for PID %d: %w", pid, err)
		}
		parents[pid] = parent
	}
	return parents, nil
}

func sysribExecutionParentStatus(raw []byte) (int, error) {
	parent := 0
	found := false
	for line := range strings.SplitSeq(string(raw), "\n") {
		key, value, colon := strings.Cut(line, ":")
		if !colon {
			continue
		}
		if key != "PPid" {
			continue
		}
		if found {
			return 0, errors.New("duplicate PPid field in process status")
		}
		fields := strings.Fields(value)
		if len(fields) != 1 {
			return 0, errors.New("process status PPid must hold one numeric parent")
		}
		parsed, err := strconv.Atoi(fields[0])
		if err != nil {
			return 0, fmt.Errorf("invalid process status PPid: %w", err)
		}
		if parsed < 0 {
			return 0, errors.New("process status PPid must not be negative")
		}
		parent = parsed
		found = true
	}
	if !found {
		return 0, errors.New("process status has no PPid field")
	}
	return parent, nil
}

// sysribExecutionRIBArgs identifies the selected CLI plugin before its flags.
// The matching process must separately pass ancestry and executable identity.
func sysribExecutionRIBArgs(args []string) bool {
	return sysribExecutionPluginArgs(args, sysribPluginName)
}

func sysribExecutionPluginArgs(args []string, pluginName string) bool {
	if len(args) < 3 {
		return false
	}
	if args[1] != "plugin" {
		return false
	}
	return args[2] == pluginName
}

func sysribExecutionCandidates(ctx context.Context, plugin *sdk.Plugin, command string) error {
	data, err := requireDone02(ctx, plugin, command)
	if err != nil {
		return err
	}
	var rows []sysribExecutionRow
	if err := decode02(data, &rows); err != nil {
		return fmt.Errorf("%s: %w", command, err)
	}
	for _, prefix := range []string{sysribExecutionTagged, sysribExecutionPlain} {
		if strings.HasPrefix(command, "show rib candidates prefix ") {
			if !strings.HasSuffix(command, " "+prefix) {
				continue
			}
		}
		found := false
		for _, row := range rows {
			if row.Prefix != prefix {
				continue
			}
			if len(row.Paths) != 1 {
				return fmt.Errorf("%s: %s has %d paths, want one real static path", command, prefix, len(row.Paths))
			}
			path := row.Paths[0]
			if path.Source != protocolStatic {
				return fmt.Errorf("%s: %s source=%q, want static", command, prefix, path.Source)
			}
			if path.Distance != 7 {
				return fmt.Errorf("%s: %s distance=%d, want 7", command, prefix, path.Distance)
			}
			if !path.Selected {
				return fmt.Errorf("%s: %s static control is not selected", command, prefix)
			}
			if path.Hidden {
				return fmt.Errorf("%s: %s permitted static control is hidden", command, prefix)
			}
			if prefix == sysribExecutionTagged {
				if path.Tag == nil {
					return fmt.Errorf("%s: %s lost its explicit zero tag", command, prefix)
				}
				if *path.Tag != 0 {
					return fmt.Errorf("%s: %s tag=%d, want explicit zero", command, prefix, *path.Tag)
				}
			} else if path.Tag != nil {
				return fmt.Errorf("%s: %s acquired an unconfigured tag", command, prefix)
			}
			found = true
		}
		if !found {
			return fmt.Errorf("%s has no permitted static control %s: %s", command, prefix, data)
		}
	}
	return nil
}

func sysribExecutionKernel(ctx context.Context) error {
	for _, prefix := range []string{sysribExecutionTagged, sysribExecutionPlain} {
		var entries []string
		if !Poll(ctx, 40, 100*time.Millisecond, func() bool {
			entries = kernelEntries(ctx, prefix)
			if len(entries) != 1 {
				return false
			}
			if !strings.HasPrefix(entries[0], "6 "+prefix+" ") {
				return false
			}
			return strings.Contains(entries[0], "proto 250 ")
		}) {
			return fmt.Errorf("%s: want exactly one blackhole kernel entry proto 250, observed %q", prefix, entries)
		}
	}
	var proof textbuf.Buffer
	return proof.Str("OK: static controls reached the kernel as blackhole proto 250\n").StdErr()
}

// sysribExecutionKernelEmpty refuses existing routes before daemon startup and
// verifies its explicit flush-on-stop cleanup afterwards. It never deletes a
// route: a foreign owner or a previous failed run must not become test input.
func sysribExecutionKernelEmpty(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return errors.New("system RIB kernel-empty fixture takes no arguments")
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	for _, prefix := range []string{sysribExecutionTagged, sysribExecutionPlain} {
		output, err := netfilterCommandOutput(ctx, "ip", "-N", "route", "show", "table", "all", "exact", prefix)
		if err != nil {
			return fmt.Errorf("inspect initial/cleaned kernel route %s: %w", prefix, err)
		}
		if strings.TrimSpace(output) != "" {
			return fmt.Errorf("kernel must hold no route for fixture prefix %s; refusing to delete existing route: %s", prefix, output)
		}
	}
	var proof textbuf.Buffer
	return proof.Str("OK: system RIB fixture prefixes are absent from every kernel table\n").StdErr()
}
