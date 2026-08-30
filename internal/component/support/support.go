// Design: docs/architecture/core-design.md — tech-support archive generator

package support

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"sort"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/config"
	hostinv "github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/textbuf"
	zeversion "github.com/ze-software/ze/internal/core/version"
)

// The collector payload keys shared by the modules in this package.
const (
	keyAvailable = "available"
	keyCount     = "count"
	keyName      = "name"
	keyPath      = "path"
	keyReason    = "reason"
)

// SupportManifest is the top-level metadata written to manifest.json.
type SupportManifest struct {
	SchemaVersion int                     `json:"schema-version"`
	Hostname      string                  `json:"hostname"`
	Timestamp     string                  `json:"timestamp"`
	Reason        string                  `json:"reason,omitempty"`
	ArchivePath   string                  `json:"archive-path"`
	Modules       map[string]ModuleResult `json:"modules"`
}

// ModuleResult records the outcome of a single module collection.
type ModuleResult struct {
	Collected  bool   `json:"collected"`
	DurationMs int64  `json:"duration-ms"`
	Error      string `json:"error,omitempty"`
}

type moduleData struct {
	name string
	data []byte
}

// Run executes the support command.
func Run(args []string) int {
	var (
		jsonOutput  bool
		sensitive   bool
		listModules bool
		configPath  string
		outputDir   string
		reason      string
		since       string
		includeStr  string
		excludeStr  string
	)

	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			jsonOutput = true
		case "--sensitive":
			sensitive = true
		case "--list-modules":
			listModules = true
		case "help", "-h", "--help":
			usage()
			return 0
		case "--module":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --module requires a value")
				return 1
			}
			i++
			includeStr = args[i]
		case "--exclude":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --exclude requires a value")
				return 1
			}
			i++
			excludeStr = args[i]
		case "--since":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --since requires a value")
				return 1
			}
			i++
			since = args[i]
		case "--reason":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --reason requires a value")
				return 1
			}
			i++
			reason = args[i]
		case "--output":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --output requires a value")
				return 1
			}
			i++
			outputDir = args[i]
		default:
			if strings.HasPrefix(args[i], "-") {
				fmt.Fprintf(os.Stderr, "error: unknown flag: %s\n", args[i])
				usage()
				return 1
			}
			if configPath != "" {
				fmt.Fprintf(os.Stderr, "error: unexpected argument: %s\n", args[i])
				usage()
				return 1
			}
			configPath = args[i]
		}
	}

	if listModules {
		names := ModuleNames()
		for _, name := range names {
			os.Stdout.WriteString(name + "\n") //nolint:errcheck // CLI output
		}
		return 0
	}

	if includeStr != "" && excludeStr != "" {
		fmt.Fprintln(os.Stderr, "error: --module and --exclude are mutually exclusive")
		return 1
	}

	var include, exclude []string
	if includeStr != "" {
		include = strings.Split(includeStr, ",")
	}
	if excludeStr != "" {
		exclude = strings.Split(excludeStr, ",")
	}

	modules, errMsg := filterModules(include, exclude)
	if errMsg != "" {
		fmt.Fprintln(os.Stderr, "error: "+errMsg)
		return 1
	}

	if reason != "" && len(reason) > 256 {
		reason = reason[:256]
	}

	var sinceTime time.Time
	if since != "" {
		var parseErr error
		sinceTime, parseErr = parseSince(since)
		if parseErr != nil {
			fmt.Fprintln(os.Stderr, "error: --since: "+parseErr.Error())
			return 1
		}
	}

	manifest, err := collect(modules, &collectOptions{
		ConfigPath: configPath,
		Since:      since,
		SinceTime:  sinceTime,
		Sensitive:  sensitive,
	}, reason, outputDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error: "+err.Error())
		return 1
	}

	if jsonOutput {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(manifest); err != nil {
			fmt.Fprintln(os.Stderr, "error: "+err.Error())
			return 1
		}
		return 0
	}

	os.Stdout.WriteString(manifest.ArchivePath + "\n") //nolint:errcheck // CLI output
	return 0
}

func collect(modules map[string]moduleCollector, opts *collectOptions, reason, outputDir string) (*SupportManifest, error) {
	hostname, _ := os.Hostname()
	now := time.Now().UTC()
	ts := now.Format("20060102-150405")

	var tb textbuf.Buffer
	archiveName := tb.Str("ze-support-").Str(hostname).Byte('-').Str(ts).Str(".tar.gz").String()
	if outputDir == "" {
		outputDir = "."
	}
	archivePath := filepath.Join(outputDir, archiveName)

	if info, err := os.Stat(outputDir); err != nil || !info.IsDir() {
		return nil, errors.New(tb.Reset().Str("output directory does not exist: ").Str(outputDir).String())
	}

	manifest := &SupportManifest{
		SchemaVersion: diagnostic.SchemaVersion,
		Hostname:      hostname,
		Timestamp:     now.Format(time.RFC3339),
		Reason:        reason,
		ArchivePath:   archivePath,
		Modules:       make(map[string]ModuleResult, len(modules)),
	}

	collected := make([]moduleData, 0, len(modules))

	names := make([]string, 0, len(modules))
	for k := range modules {
		names = append(names, k)
	}
	sort.Strings(names)

	for _, name := range names {
		fn := modules[name]
		start := time.Now()
		result, err := runCollector(fn, opts)
		duration := time.Since(start).Milliseconds()

		if err != nil {
			manifest.Modules[name] = ModuleResult{
				Collected:  false,
				DurationMs: duration,
				Error:      err.Error(),
			}
			errData, _ := json.MarshalIndent(map[string]string{"error": err.Error()}, "", "  ")
			collected = append(collected, moduleData{name: name, data: errData})
			continue
		}

		data, marshalErr := json.MarshalIndent(result, "", "  ")
		if marshalErr != nil {
			manifest.Modules[name] = ModuleResult{
				Collected:  false,
				DurationMs: duration,
				Error:      tb.Reset().Str("json marshal: ").Err(marshalErr).String(),
			}
			continue
		}

		manifest.Modules[name] = ModuleResult{
			Collected:  true,
			DurationMs: duration,
		}
		collected = append(collected, moduleData{name: name, data: data})
	}

	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return nil, errors.New(tb.Reset().Str("marshal manifest: ").Err(err).String())
	}

	if err := writeArchive(archivePath, manifestData, collected); err != nil {
		return nil, err
	}

	return manifest, nil
}

func runCollector(fn moduleCollector, opts *collectOptions) (result any, err error) {
	defer func() {
		if r := recover(); r != nil {
			result = nil
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	return fn(opts)
}

func writeArchive(path string, manifestData []byte, modules []moduleData) (retErr error) {
	var tb textbuf.Buffer
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600) //nolint:gosec // path is constructed from hostname+timestamp, not user input
	if err != nil {
		return errors.New(tb.Str("create archive: ").Err(err).String())
	}
	defer func() {
		if retErr != nil {
			f.Close()       //nolint:errcheck // best-effort on error path
			os.Remove(path) //nolint:errcheck // best-effort cleanup of partial archive
		}
	}()

	gw := gzip.NewWriter(f)
	tw := tar.NewWriter(gw)

	if err := writeTarEntry(tw, "manifest.json", manifestData); err != nil {
		return err
	}

	for _, m := range modules {
		if err := writeTarEntry(tw, tb.Reset().Str(m.name).Str(".json").String(), m.data); err != nil {
			return err
		}
	}

	if err := tw.Close(); err != nil {
		return errors.New(tb.Reset().Str("close tar: ").Err(err).String())
	}
	if err := gw.Close(); err != nil {
		return errors.New(tb.Reset().Str("close gzip: ").Err(err).String())
	}
	if err := f.Close(); err != nil {
		return errors.New(tb.Reset().Str("close file: ").Err(err).String())
	}
	return nil
}

func writeTarEntry(tw *tar.Writer, name string, data []byte) error {
	var tb textbuf.Buffer
	hdr := &tar.Header{
		Name:    name,
		Size:    int64(len(data)),
		Mode:    0o644,
		ModTime: time.Now().UTC(),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return errors.New(tb.Str("tar header ").Str(name).Str(": ").Err(err).String())
	}
	if _, err := tw.Write(data); err != nil {
		return errors.New(tb.Reset().Str("tar write ").Str(name).Str(": ").Err(err).String())
	}
	return nil
}

// collectVersion gathers Ze version, build info, and Go runtime details.
func collectVersion(opts *collectOptions) (any, error) {
	_ = opts
	result := map[string]any{
		"ze-version": zeversion.Release(),
		"build-date": zeversion.BuildDate(),
		"go-version": runtime.Version(),
		"go-os":      runtime.GOOS,
		"go-arch":    runtime.GOARCH,
		"cpus":       runtime.NumCPU(),
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		deps := make([]map[string]string, 0, len(info.Deps))
		for _, dep := range info.Deps {
			deps = append(deps, map[string]string{
				keyPath:   dep.Path,
				"version": dep.Version,
			})
		}
		result["module"] = info.Main.Path
		result["dependencies"] = deps
	}
	return result, nil
}

// collectDoctor runs the registered doctor checks.
func collectDoctor(opts *collectOptions) (any, error) {
	diags := diagnostic.RunDoctorChecks(opts.ConfigPath)
	if diags == nil {
		return map[string]any{keyAvailable: false, keyReason: "no doctor provider registered"}, nil
	}
	ready := true
	for i := range diags {
		if diags[i].Severity == diagnostic.SeverityError {
			ready = false
			break
		}
	}
	return diagnostic.NewDoctorResult(ready, diags), nil
}

// collectHost gathers hardware inventory.
func collectHost(opts *collectOptions) (any, error) {
	_ = opts
	return hostinv.Detect()
}

// collectPlatform gathers runtime platform type and capabilities.
func collectPlatform(opts *collectOptions) (any, error) {
	_ = opts
	return hostinv.DetectPlatform()
}

// collectConfig loads and optionally sanitizes the configuration.
func collectConfig(opts *collectOptions) (any, error) {
	var tb textbuf.Buffer
	store, err := resolve.Storage()
	if err != nil {
		return map[string]any{keyAvailable: false, keyReason: tb.Str("storage: ").Err(err).String()}, nil
	}
	defer func() { _ = store.Close() }()

	var configData []byte
	var configName string

	if opts.ConfigPath != "" {
		configData, err = cliio.ReadFile(opts.ConfigPath) // "-" reads stdin
		if err != nil {
			return map[string]any{keyAvailable: false, keyReason: tb.Reset().Str("config file: ").Err(err).String()}, nil
		}
		configName = opts.ConfigPath
	} else {
		configName = resolve.DefaultConfig(store)
		configData, err = store.ReadFile(configName)
		if err != nil {
			return map[string]any{keyAvailable: false, keyReason: tb.Reset().Str("no config found: ").Err(err).String()}, nil
		}
	}

	result, parseErr := config.LoadConfig(string(configData), configName, nil)
	if parseErr != nil {
		return map[string]any{
			keyAvailable:  true,
			"parse-error": parseErr.Error(),
			keyPath:       configName,
		}, nil
	}

	tree := result.Tree.ToMap()
	if !opts.Sensitive {
		tree = sanitizeConfig(tree)
	}

	return map[string]any{
		keyPath: configName,
		"tree":  tree,
	}, nil
}

// collectCrashes gathers crash log files.
func collectCrashes(opts *collectOptions) (any, error) {
	_ = opts
	crashlog.Init()
	summaries := crashlog.ListCrashes()
	if len(summaries) == 0 {
		return map[string]any{
			keyCount: 0,
			"dir":    crashlog.CrashDir(),
		}, nil
	}

	crashes := make([]map[string]any, 0, len(summaries))
	for _, s := range summaries {
		entry := map[string]any{
			keyName:   s.Name,
			"size":    s.Size,
			"content": crashlog.ReadCrash(s.Name),
		}
		crashes = append(crashes, entry)
	}
	return map[string]any{
		keyCount:  len(crashes),
		"dir":     crashlog.CrashDir(),
		"crashes": crashes,
	}, nil
}

// collectDisk gathers filesystem usage information.
func collectDisk(opts *collectOptions) (any, error) {
	_ = opts
	return collectDiskInfo()
}

// collectInterfaces gathers kernel network interface state.
func collectInterfaces(opts *collectOptions) (any, error) {
	_ = opts
	ifs, err := iface.ListInterfaces()
	if err != nil {
		return nil, err
	}
	return map[string]any{"interfaces": ifs, keyCount: len(ifs)}, nil
}

// collectRoutes gathers the kernel routing table.
func collectRoutes(opts *collectOptions) (any, error) {
	_ = opts
	const routeLimit = 50000
	routes, err := iface.ListKernelRoutes("", routeLimit)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"routes": routes, keyCount: len(routes)}
	if len(routes) >= routeLimit {
		result["truncated"] = true
		result["limit"] = routeLimit
	}
	return result, nil
}

// collectNeighbors gathers the kernel ARP/neighbor table.
func collectNeighbors(opts *collectOptions) (any, error) {
	_ = opts
	neighbors, err := iface.ListNeighbors(iface.NeighborFamilyAny)
	if err != nil {
		return nil, err
	}
	return map[string]any{"neighbors": neighbors, keyCount: len(neighbors)}, nil
}

// collectEnv gathers Ze's registered environment variables.
func collectEnv(opts *collectOptions) (any, error) {
	_ = opts
	entries := env.Entries()
	result := make([]map[string]any, 0, len(entries))
	for _, e := range entries {
		entry := map[string]any{
			"key":         e.Key,
			"type":        e.Type,
			"description": e.Description,
		}
		if e.Secret {
			entry["value"] = redactedValue
		} else {
			entry["value"] = env.Get(e.Key)
		}
		result = append(result, entry)
	}
	return map[string]any{"variables": result, keyCount: len(result)}, nil
}

// collectSysctl gathers current values of Ze's registered sysctl keys.
func collectSysctl(opts *collectOptions) (any, error) {
	_ = opts
	return collectSysctlInfo()
}

// collectDmesg gathers recent kernel log entries from /dev/kmsg.
func collectDmesg(opts *collectOptions) (any, error) {
	return collectDmesgInfo(opts.SinceTime)
}

// collectSockets gathers open TCP/UDP sockets from /proc/net.
func collectSockets(opts *collectOptions) (any, error) {
	_ = opts
	return collectSocketsInfo()
}

// collectModules gathers loaded kernel modules from /proc/modules.
func collectModules(opts *collectOptions) (any, error) {
	_ = opts
	return collectKernelModulesInfo()
}

// collectConntrack gathers netfilter connection tracking state.
func collectConntrack(opts *collectOptions) (any, error) {
	_ = opts
	return collectConntrackInfo()
}

// collectFDs gathers open file descriptors for the current process.
func collectFDs(opts *collectOptions) (any, error) {
	_ = opts
	return collectFDsInfo()
}

// collectDNS gathers DNS resolver configuration.
func collectDNS(opts *collectOptions) (any, error) {
	_ = opts
	return collectDNSInfo()
}

// collectFirewall gathers nftables tables and chains via netlink.
func collectFirewall(opts *collectOptions) (any, error) {
	_ = opts
	return collectFirewallInfo()
}

// collectRuntime gathers Go runtime memory and goroutine stats.
func collectRuntime(opts *collectOptions) (any, error) {
	_ = opts
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	return map[string]any{
		"goroutines":   runtime.NumGoroutine(),
		"gomaxprocs":   runtime.GOMAXPROCS(0),
		"alloc-bytes":  mem.Alloc,
		"sys-bytes":    mem.Sys,
		"heap-alloc":   mem.HeapAlloc,
		"heap-inuse":   mem.HeapInuse,
		"heap-objects": mem.HeapObjects,
		"stack-inuse":  mem.StackInuse,
		"gc-cycles":    mem.NumGC,
		"gc-pause-ns":  mem.PauseTotalNs,
	}, nil
}

// parseSince parses a time specification into an absolute time.Time.
// Accepts Go duration strings ("2h", "30m", "1h30m", "24h") as relative
// offsets from now, and ISO 8601 date/datetime strings.
func parseSince(s string) (time.Time, error) {
	// Strip "time=" prefix if someone pastes from Ze's slog output.
	s = strings.TrimPrefix(s, "time=")

	if d, err := time.ParseDuration(s); err == nil {
		if d < 0 {
			return time.Time{}, errors.New("duration must be positive (e.g. 2h, not -2h)")
		}
		return time.Now().Add(-d), nil
	}
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05.000",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, errors.New("expected duration (e.g. 2h, 30m) or date (e.g. 2026-05-25, 2026-05-25T14:00:00Z)")
}

func usage() {
	os.Stderr.WriteString("Usage: ze support [options] [<config-file>]\n" + //nolint:errcheck // CLI help
		"\nGenerate a tech-support archive for troubleshooting.\n" +
		"\nOptions:\n" +
		"  --module M       Collect only named modules (comma-separated)\n" +
		"  --exclude M      Exclude named modules (comma-separated)\n" +
		"  --since T        Time scope for log collection (e.g., \"2h\", \"30m\")\n" +
		"  --reason R       Reason string included in manifest\n" +
		"  --sensitive      Include passwords and secrets (default: redacted)\n" +
		"  --output DIR     Output directory (default: current directory)\n" +
		"  --json           Output manifest JSON to stdout\n" +
		"  --list-modules   List available modules and exit\n")
	var tb2 textbuf.Buffer
	os.Stderr.WriteString(tb2.Str("\nModules: ").Str(ModuleList()).Byte('\n').Slice()) //nolint:errcheck // CLI help
}
