// Design: docs/architecture/core-design.md — tech-support archive generator

package support

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"slices"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/config/storage"
	"github.com/ze-software/ze/internal/component/config/system"
	hostinv "github.com/ze-software/ze/internal/component/host"
	"github.com/ze-software/ze/internal/component/iface"
	"github.com/ze-software/ze/internal/core/cliio"
	"github.com/ze-software/ze/internal/core/crashlog"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/internal/core/resolve"
	"github.com/ze-software/ze/internal/core/statestore"
	"github.com/ze-software/ze/internal/core/textbuf"
	zeversion "github.com/ze-software/ze/internal/core/version"
	"github.com/ze-software/ze/pkg/zefs"
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
// A local archive names the file it was written to (ArchivePath). An archive
// the daemon collected for a caller in another process names the store key it
// was kept under (ArchiveKey) and, in the answer, its size (ArchiveOctets),
// so the caller reads it back with `show data cat <key>`.
type SupportManifest struct {
	SchemaVersion int                     `json:"schema-version"`
	Hostname      string                  `json:"hostname"`
	Timestamp     string                  `json:"timestamp"`
	Reason        string                  `json:"reason,omitempty"`
	ArchivePath   string                  `json:"archive-path,omitempty"`
	ArchiveKey    string                  `json:"archive-key,omitempty"`
	ArchiveOctets int                     `json:"archive-octets,omitempty"`
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

// archivePath is the command path the handler is registered under.
const archivePath = "request support archive"

// The leaves of the command's YANG node (ze-support-cmd.yang). Each makes the
// choice its local flag makes, so a remote caller, whose `--flag` the daemon
// refuses, can make it too.
const (
	leafModule  = "module"
	leafExclude = "exclude"
	leafSince   = "since"
	leafReason  = "reason"
)

// archiveRequest is what one call asks for, from its leaves and, on the local
// route, its flags.
type archiveRequest struct {
	jsonOutput  bool
	sensitive   bool
	listModules bool
	help        bool
	configPath  string
	outputDir   string
	reason      string
	since       string
	includeStr  string
	excludeStr  string
}

// Run is the `ze request support archive` spelling: the handler on the local
// route, its answer printed as that route prints it.
func Run(validated command.ValidatedArgs) int {
	return command.PrintLocalAnswerWith(archivePath, command.AnswerLocal(Answer, validated), Render)
}

// Answer is the handler of `request support archive`. It writes nothing
// itself (command.LocalDataHandler).
//
// On the local route it collects into a file under --output (default the
// working directory) and answers its manifest (ArchiveAnswer), which Render
// prints as the archive's path, or under --json as the manifest, exactly as
// the command always printed. On the daemon route the
// caller's host is not the daemon's, so no directory of the caller's can be
// named: the daemon keeps the archive in its store under a registered key
// (zefs.KeySupportArchive), answers the manifest naming that key, and the
// caller reads the bytes with `show data cat <key>`. --output is refused
// there by name.
func Answer(validated command.ValidatedArgs) command.LocalAnswer {
	route := validated.Route()
	req, refusal, refused := parseArchiveRequest(validated)
	if refused {
		return refusal
	}
	if req.help {
		return command.LocalAnswer{Diagnostic: usageText()}
	}
	if req.listModules {
		return command.LocalAnswer{Payload: command.Blob(textbuf.Join(ModuleNames(), "\n") + "\n")}
	}
	if req.outputDir != "" && !route.CallerLocal() {
		return command.LocalRefusal(1, "error: --output names a directory on the caller's host, which the daemon cannot write; "+
			"the daemon keeps the archive in its store and answers the key 'show data cat <key>' reads, "+
			"or run 'ze request support archive --output <dir>' on the host that should hold the file\n")
	}
	if req.includeStr != "" && req.excludeStr != "" {
		return command.LocalRefusal(1, "error: --module and --exclude are mutually exclusive\n")
	}

	var include, exclude []string
	if req.includeStr != "" {
		include = strings.Split(req.includeStr, ",")
	}
	if req.excludeStr != "" {
		exclude = strings.Split(req.excludeStr, ",")
	}
	modules, errMsg := filterModules(include, exclude)
	if errMsg != "" {
		return command.LocalRefusal(1, "error: "+errMsg+"\n")
	}

	reason := req.reason
	if len(reason) > 256 {
		reason = reason[:256]
	}

	var sinceTime time.Time
	if req.since != "" {
		var parseErr error
		sinceTime, parseErr = parseSince(req.since)
		if parseErr != nil {
			return command.LocalRefusal(1, "error: --since: "+parseErr.Error()+"\n")
		}
	}

	configPath := req.configPath
	if configPath != "" {
		confined, err := command.CallerPath(route, archivePath, validated.Tokens(), configPath)
		if err != nil {
			return command.LocalRefusal(1, command.LocalErrorLine(err))
		}
		configPath = confined
	}
	opts := &collectOptions{
		ConfigPath: configPath,
		Since:      req.since,
		SinceTime:  sinceTime,
		Sensitive:  req.sensitive,
	}

	if !route.CallerLocal() {
		return storeAnswer(validated.Context(), modules, opts, reason)
	}
	manifest, err := collect(modules, opts, reason, req.outputDir)
	if err != nil {
		return command.LocalRefusal(1, "error: "+err.Error()+"\n")
	}
	return command.LocalAnswer{Payload: ArchiveAnswer{SupportManifest: *manifest, pathOnly: !req.jsonOutput}}
}

// ArchiveAnswer is the local route's answer: the manifest of the archive it
// wrote, the same document the daemon route answers. A pipe (`| json`, `ze
// cli -c`) reads the manifest's fields, because the embedded struct marshals
// flat and pathOnly is not exported. pathOnly records that --json was absent,
// so Render prints the archive's path, the line the command always printed.
type ArchiveAnswer struct {
	SupportManifest
	pathOnly bool
}

// Render is the `ze request support archive` drawing of an answer: the
// archive's path on one line, or under --json the manifest indented by two
// spaces, each ended by a newline, as the command always printed. Any other
// payload (the module list, the daemon's manifest) renders through
// command.RenderLocalAnswer. It answers the exit code.
func Render(path string, payload any) int {
	answer, isArchive := payload.(ArchiveAnswer)
	if !isArchive {
		return command.RenderLocalAnswer(path, payload)
	}
	if answer.pathOnly {
		os.Stdout.WriteString(answer.ArchivePath + "\n") //nolint:errcheck // CLI output, as command.WriteAnswer
		return 0
	}
	encoded, err := json.MarshalIndent(answer.SupportManifest, "", "  ")
	if err != nil {
		os.Stderr.WriteString("error: " + err.Error() + "\n") //nolint:errcheck // CLI output
		return 1
	}
	os.Stdout.Write(append(encoded, '\n')) //nolint:errcheck // CLI output, as command.WriteAnswer
	return 0
}

// storeAnswer collects for a caller in another process and keeps the archive
// in the daemon's store. The archive is bounded by what one remote answer
// carries (command.BlobRemoteOctetsMax), because `show data cat` must hand it
// back whole: a larger one is refused before it is stored, with its size and
// the local command that writes it to a file instead.
func storeAnswer(ctx context.Context, modules map[string]moduleCollector, opts *collectOptions, reason string) command.LocalAnswer {
	manifest, archive, err := collectToMemory(ctx, modules, opts, reason)
	if err != nil {
		return command.LocalRefusal(1, "error: "+err.Error()+"\n")
	}
	if len(archive) > command.BlobRemoteOctetsMax {
		var tb textbuf.Buffer
		tb.Str("error: the support archive is ").Int(int64(len(archive))).Str(" bytes, more than the ").
			Int(command.BlobRemoteOctetsMax).Str(" a remote answer carries; ").
			Str("narrow it with module or exclude, or run 'ze request support archive --output <dir>' on the router\n")
		return command.LocalRefusal(1, tb.String())
	}
	if err := statestore.Write(manifest.ArchiveKey, archive); err != nil {
		var tb textbuf.Buffer
		tb.Str("error: the support archive could not be stored under ").Str(manifest.ArchiveKey).Str(": ").Err(err).
			Str("; run 'ze request support archive --output <dir>' on the router to write it to a file\n")
		return command.LocalRefusal(1, tb.String())
	}
	manifest.ArchiveOctets = len(archive)
	return command.LocalAnswer{Payload: manifest}
}

// parseArchiveRequest reads the leaves, then the flags among the tokens no
// leaf keyword took. A flag making the choice a leaf already made is refused,
// naming both, so the call cannot mean two things. It answers the request, or
// the refusal and true.
func parseArchiveRequest(validated command.ValidatedArgs) (archiveRequest, command.LocalAnswer, bool) {
	var req archiveRequest
	fromLeaf := map[string]*string{
		leafModule:  &req.includeStr,
		leafExclude: &req.excludeStr,
		leafSince:   &req.since,
		leafReason:  &req.reason,
	}
	for leaf, target := range fromLeaf {
		if value, found := validated.Keyed(leaf); found {
			*target = value
		}
	}
	fromFlag := map[string]string{
		"--module":  leafModule,
		"--exclude": leafExclude,
		"--since":   leafSince,
		"--reason":  leafReason,
		"--output":  "",
	}

	args := validated.PositionalTokens()
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--json":
			req.jsonOutput = true
			continue
		case "--sensitive":
			req.sensitive = true
			continue
		case "--list-modules":
			req.listModules = true
			continue
		case "help", "-h", "--help":
			req.help = true
			return req, command.LocalAnswer{}, false
		}
		leaf, valued := fromFlag[args[i]]
		if !valued {
			if refusal, refused := positionalArgument(&req, args[i]); refused {
				return req, refusal, true
			}
			continue
		}
		if i+1 >= len(args) {
			return req, command.LocalRefusal(1, "error: "+args[i]+" requires a value\n"), true
		}
		i++
		if leaf == "" {
			req.outputDir = args[i]
			continue
		}
		if _, found := validated.Keyed(leaf); found {
			return req, command.LocalRefusal(1, "error: "+args[i-1]+" and the "+leaf+" leaf make the same choice; give one of them\n"), true
		}
		*fromLeaf[leaf] = args[i]
	}
	return req, command.LocalAnswer{}, false
}

// positionalArgument takes a token no flag or leaf claimed: the config file,
// once. A second one, or an unknown flag, is refused with the usage.
func positionalArgument(req *archiveRequest, token string) (command.LocalAnswer, bool) {
	if strings.HasPrefix(token, "-") {
		return command.LocalRefusal(1, "error: unknown flag: "+token+"\n"+usageText()), true
	}
	if req.configPath != "" {
		return command.LocalRefusal(1, "error: unexpected argument: "+token+"\n"+usageText()), true
	}
	req.configPath = token
	return command.LocalAnswer{}, false
}

// archiveName names an archive by host and time, so two archives of one host
// never share a file name or a store key.
func archiveName(hostname string, now time.Time) string {
	var tb textbuf.Buffer
	return tb.Str("ze-support-").Str(hostname).Byte('-').Str(now.Format("20060102-150405")).Str(".tar.gz").String()
}

// collect writes an archive into outputDir (the working directory when empty)
// and answers its manifest. It is the local route's collection, which lives as
// long as the operator's process.
func collect(modules map[string]moduleCollector, opts *collectOptions, reason, outputDir string) (*SupportManifest, error) {
	if outputDir == "" {
		outputDir = "."
	}
	if info, err := os.Stat(outputDir); err != nil || !info.IsDir() {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("output directory does not exist: ").Str(outputDir).String())
	}
	now := time.Now().UTC().Truncate(time.Second)
	manifest, collected, err := gather(context.Background(), modules, opts, reason, now)
	if err != nil {
		return nil, err
	}
	manifest.ArchivePath = filepath.Join(outputDir, archiveName(manifest.Hostname, now))
	if err := writeArchiveFile(manifest, collected); err != nil {
		return nil, err
	}
	return manifest, nil
}

// collectToMemory builds an archive for the daemon's store and answers its
// manifest, naming the key it is to be kept under, and its bytes. It stops
// when ctx ends: the caller that asked for it has gone.
func collectToMemory(ctx context.Context, modules map[string]moduleCollector, opts *collectOptions, reason string) (*SupportManifest, []byte, error) {
	now := time.Now().UTC().Truncate(time.Second)
	manifest, collected, err := gather(ctx, modules, opts, reason, now)
	if err != nil {
		return nil, nil, err
	}
	manifest.ArchiveKey = zefs.KeySupportArchive.Key(archiveName(manifest.Hostname, now))
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		var tb textbuf.Buffer
		return nil, nil, errors.New(tb.Str("marshal manifest: ").Err(err).String())
	}
	var archive bytes.Buffer
	if err := writeArchive(&archive, manifestData, collected); err != nil {
		return nil, nil, err
	}
	return manifest, archive.Bytes(), nil
}

// gather runs each selected module, in name order, and answers the manifest
// and each module's data. It checks ctx before each module, so a request that
// ended stops the collection between modules rather than after all of them.
// now is the archive's time, taken once by the caller, which also names the
// archive with it, so the manifest's timestamp and the name never disagree.
func gather(ctx context.Context, modules map[string]moduleCollector, opts *collectOptions, reason string, now time.Time) (*SupportManifest, []moduleData, error) {
	hostname, _ := os.Hostname()
	var tb textbuf.Buffer

	manifest := &SupportManifest{
		SchemaVersion: diagnostic.SchemaVersion,
		Hostname:      hostname,
		Timestamp:     now.Format(time.RFC3339),
		Reason:        reason,
		Modules:       make(map[string]ModuleResult, len(modules)),
	}

	collected := make([]moduleData, 0, len(modules))

	names := make([]string, 0, len(modules))
	for k := range modules {
		names = append(names, k)
	}
	slices.Sort(names)

	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, nil, errors.New(tb.Reset().Str("the request ended before module ").Str(name).
				Str(" was collected: ").Err(err).String())
		}
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
	return manifest, collected, nil
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

// writeArchiveFile writes the archive to manifest.ArchivePath, which it
// creates and never replaces; a partial file is removed on failure.
func writeArchiveFile(manifest *SupportManifest, modules []moduleData) (retErr error) {
	var tb textbuf.Buffer
	manifestData, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return errors.New(tb.Str("marshal manifest: ").Err(err).String())
	}
	path := manifest.ArchivePath
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600) //nolint:gosec // path is constructed from hostname+timestamp, not user input
	if err != nil {
		return errors.New(tb.Reset().Str("create archive: ").Err(err).String())
	}
	defer func() {
		if retErr != nil {
			f.Close()       //nolint:errcheck // best-effort on error path
			os.Remove(path) //nolint:errcheck // best-effort cleanup of partial archive
		}
	}()

	if err := writeArchive(f, manifestData, modules); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return errors.New(tb.Reset().Str("close file: ").Err(err).String())
	}
	return nil
}

// writeArchive writes the gzip-compressed tar of the manifest and each
// module's data to w.
func writeArchive(w io.Writer, manifestData []byte, modules []moduleData) error {
	var tb textbuf.Buffer
	gw := gzip.NewWriter(w)
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

	var configData []byte
	var configName string
	var err error

	if opts.ConfigPath == "" {
		store, openErr := storage.OpenReadOnly(resolve.StoreDir(""), resolve.StoreDir(""))
		if openErr != nil {
			return map[string]any{keyAvailable: false, keyReason: tb.Str("storage: ").Err(openErr).String()}, nil
		}
		defer func() { _ = store.Close() }()
		configName, err = resolve.DefaultConfig(store)
		if err != nil {
			return map[string]any{keyAvailable: false, keyReason: tb.Reset().Err(err).String()}, nil
		}
		configData, err = store.ReadFile(configName)
		if err != nil {
			return map[string]any{keyAvailable: false, keyReason: tb.Reset().Str("no config found: ").Err(err).String()}, nil
		}
	} else {
		configData, err = cliio.ReadFile(opts.ConfigPath) // "-" reads stdin
		if err != nil {
			return map[string]any{keyAvailable: false, keyReason: tb.Reset().Str("config file: ").Err(err).String()}, nil
		}
		configName = opts.ConfigPath
	}

	result, parseErr := config.ReadConfig(string(configData), configName, nil)
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

// collectCrashes gathers crash reports of both kinds, plus the readiness block.
//
// Readiness travels with the reports because a bundle with no kernel report is
// ambiguous on its own: it means either that the box never faulted, or that the
// reservation was never armed and no fault could ever have been recorded. The
// engineer reading the archive cannot tell those apart from the file list.
func collectCrashes(opts *collectOptions) (any, error) {
	crashlog.Init()

	reports := crashlog.CrashListFields()
	for _, entry := range reports {
		name, ok := entry[keyName].(string)
		if !ok {
			continue
		}
		entry["content"] = crashlog.ReadCrash(name)
	}

	payload := map[string]any{
		keyCount:  len(reports),
		"dir":     crashlog.CrashDir(),
		"crashes": reports,
	}

	intent, err := system.LoadCrashDumpIntent(opts.ConfigPath)
	if err != nil {
		readiness := crashlog.CrashReadiness(crashlog.Intent{}).Fields()
		readiness[keyReason] = err.Error()
		payload["readiness"] = readiness
		return payload, nil //nolint:nilerr // a module error is non-fatal: the reason travels in the payload so the archive still carries the reports
	}
	payload["readiness"] = crashlog.CrashReadiness(intent).Fields()
	return payload, nil
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

// usageText is the command's usage, printed on stderr by the local route.
func usageText() string {
	var tb textbuf.Buffer
	return tb.Str("Usage: ze request support archive [options] [<config-file>]\n" +
		"\nGenerate a tech-support archive for troubleshooting.\n" +
		"\nOptions:\n" +
		"  --module M       Collect only named modules (comma-separated)\n" +
		"  --exclude M      Exclude named modules (comma-separated)\n" +
		"  --since T        Time scope for log collection (e.g., \"2h\", \"30m\")\n" +
		"  --reason R       Reason string included in manifest\n" +
		"  --sensitive      Include passwords and secrets (default: redacted)\n" +
		"  --output DIR     Output directory (default: current directory)\n" +
		"  --json           Output manifest JSON to stdout\n" +
		"  --list-modules   List available modules and exit\n").
		Str("\nModules: ").Str(ModuleList()).Byte('\n').String()
}
