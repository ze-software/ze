package sysctl

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"sync"
	"unicode/utf8"

	"github.com/ze-software/ze/internal/component/plugin/cli"
	"github.com/ze-software/ze/internal/component/plugin/issuequeue"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	sysctlevents "github.com/ze-software/ze/internal/component/sysctl/events"
	sysctlyang "github.com/ze-software/ze/internal/component/sysctl/yang"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/events"
	"github.com/ze-software/ze/internal/core/slogutil"
	sysctlreg "github.com/ze-software/ze/internal/core/sysctl"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
	"github.com/ze-software/ze/pkg/ze"
)

func sysctlSends() []rpc.MessageSendDecl {
	return []rpc.MessageSendDecl{
		{Message: sysctlevents.Namespace + ":" + sysctlevents.EventApplied},
		{Message: sysctlevents.Namespace + ":" + sysctlevents.EventShowResult},
		{Message: sysctlevents.Namespace + ":" + sysctlevents.EventListResult},
		{Message: sysctlevents.Namespace + ":" + sysctlevents.EventDescribeResult},
	}
}

var (
	errSysctlDescribeRequiresKeyArgument        = errors.New("sysctl describe: requires key argument")
	errSysctlDescribeProfileRequiresProfileName = errors.New("sysctl describe-profile: requires profile name argument")
	errSysctlSetRequiresKeyAndValue             = errors.New("sysctl set: requires key and value arguments")
)

// eventBusMu guards eventBusRef. An interface cannot be stored in
// atomic.Pointer directly, so a mutex is used (same pattern as iface).
var (
	eventBusMu  sync.Mutex
	eventBusRef ze.EventBus
)

func setEventBusRef(eb ze.EventBus) {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	eventBusRef = eb
}

func getEventBusRef() ze.EventBus {
	eventBusMu.Lock()
	defer eventBusMu.Unlock()
	return eventBusRef
}

func init() {
	_ = events.RegisterNamespace(sysctlevents.Namespace,
		sysctlevents.EventDefault, sysctlevents.EventSet, sysctlevents.EventApplied,
		sysctlevents.EventShowRequest, sysctlevents.EventShowResult,
		sysctlevents.EventListRequest, sysctlevents.EventListResult,
		sysctlevents.EventDescribeRequest, sysctlevents.EventDescribeResult,
		sysctlevents.EventClearProfileDefaults,
		sysctlevents.EventClearSourceDefaults,
	)

	reg := registry.Registration{
		Name:        "sysctl",
		Description: "Kernel tunable management: three-layer precedence, restore on stop",
		Features:    "yang",
		YANG:        sysctlyang.ZeSysctlConfYANG,
		ConfigRoots: []string{configRoot},
		// sysctl acts on the Linux host itself, beside the selected data plane:
		// its keys apply to the Linux kernel on any plane (forwarding keys such
		// as rp_filter act only on packets that reach Linux, not packets VPP
		// forwards).
		DataPlaneFeatures: []registry.DataPlaneFeature{{
			ConfigPath: configRoot,
			Why:        "sysctl sets the Linux kernel's own tunables",
			Planes:     []string{"kernel"},
			InUseKind:  registry.DataPlaneInUseConfigRoot,
		}},
		HostSideDataPlane:       true,
		InProcessConfigVerifier: verifySysctlConfig,
		Sends:                   sysctlSends(),
		RunEngine:               runSysctlPlugin,
		Commands:                commandDecls(),
		ConfigureEngineLogger: func(loggerName string) {
			setLogger(slogutil.Logger(loggerName))
		},
		ConfigureEventBus: func(eb ze.EventBus) {
			setEventBusRef(eb)
		},
	}
	reg.CLIHandler = func(args []string) int {
		cfg := cli.BaseConfig(&reg)
		cfg.ConfigLogger = func(level string) {
			setLogger(slogutil.PluginLogger(reg.Name, level))
		}
		return cli.RunPlugin(cfg, args)
	}
	if err := registry.Register(reg); err != nil {
		panic("BUG: sysctl: registration failed: " + err.Error())
	}

	// Every sysctl is applied through /proc/sys, so this component owns the
	// readiness check that asks whether the daemon will be able to write it
	// (doctor.go). A refusal is a programmer error in the table beside it.
	if err := diagnostic.RegisterDoctorCheck(sysctlProcfsDoctorCheck()); err != nil {
		panic("BUG: sysctl: doctor check registration failed: " + err.Error())
	}
}

const configRoot = "sysctl"

func verifySysctlConfig(sections []sdk.ConfigSection) error {
	for _, sec := range sections {
		if sec.Root != configRoot {
			continue
		}
		settings := parseSysctlConfig(sec.Data)
		for key, value := range settings {
			if err := validateKey(key); err != nil {
				return err
			}
			if err := sysctlreg.CheckManaged(key); err != nil {
				return err
			}
			if err := sysctlreg.Validate(key, value); err != nil {
				return err
			}
		}
		for _, prof := range parseSysctlProfileConfig(sec.Data) {
			for _, setting := range prof.Settings {
				if err := validateKey(setting.Key); err != nil {
					return fmt.Errorf("profile %s: %w", prof.Name, err)
				}
				if err := sysctlreg.CheckManaged(setting.Key); err != nil {
					return fmt.Errorf("profile %s: %w", prof.Name, err)
				}
				if err := sysctlreg.Validate(setting.Key, setting.Value); err != nil {
					return fmt.Errorf("profile %s: %w", prof.Name, err)
				}
			}
		}
	}
	return nil
}

// newSysctlBackend constructs the OS sysctl backend for runSysctlPlugin. Tests
// override this to avoid touching /proc/sys.
var newSysctlBackend = newBackend

func runSysctlPlugin(conn net.Conn) int {
	log := logger()
	log.Debug("sysctl plugin starting")

	p := sdk.NewWithConn("sysctl", conn)
	defer func() { _ = p.Close() }()

	// sysctl is forkable (CLIHandler + sdk.NewWithConn), so problems reach the
	// daemon's report bus through SDK RaiseIssue via issuequeue rather than
	// daemon-local internal/core/report. Stop MUST run after event handlers
	// unsubscribe and plugin work finishes, and before p.Close().
	issues := issuequeue.Start(p)
	defer issues.Stop()

	be := newSysctlBackend()
	s := newStore(be, log)
	s.issues = issues

	eb := getEventBusRef()

	// Subscribe to EventBus events.
	var unsubscribers []func()
	unsubscribeAll := func() {
		for _, unsub := range unsubscribers {
			unsub()
		}
		unsubscribers = nil
	}
	defer unsubscribeAll()
	if eb != nil {
		unsubscribers = append(unsubscribers,
			// Default events from other plugins.
			eb.Subscribe(sysctlevents.Namespace, sysctlevents.EventDefault, events.AsString(func(payload string) {
				var ev struct {
					Key    string `json:"key"`
					Value  string `json:"value"`
					Source string `json:"source"`
				}
				if err := json.Unmarshal([]byte(payload), &ev); err != nil {
					log.Warn("sysctl: bad default event", "err", err)
					return
				}
				applied, err := s.setDefault(ev.Key, ev.Value, ev.Source)
				if err != nil {
					log.Warn("sysctl: default write failed", "key", ev.Key, "err", err)
					return
				}
				if applied != "" {
					if _, emitErr := eb.Emit(sysctlevents.Namespace, sysctlevents.EventApplied, applied); emitErr != nil {
						reportSysctlEventEmitFailed(log, issues, sysctlevents.EventApplied, ev.Key, emitErr)
					}
				}
			})),
			// Transient set events (from CLI).
			eb.Subscribe(sysctlevents.Namespace, sysctlevents.EventSet, events.AsString(func(payload string) {
				var ev struct {
					Key   string `json:"key"`
					Value string `json:"value"`
				}
				if err := json.Unmarshal([]byte(payload), &ev); err != nil {
					log.Warn("sysctl: bad set event", "err", err)
					return
				}
				applied, err := s.setTransient(ev.Key, ev.Value)
				if err != nil {
					log.Warn("sysctl: set failed", "key", ev.Key, "err", err)
					return
				}
				if applied != "" {
					if _, emitErr := eb.Emit(sysctlevents.Namespace, sysctlevents.EventApplied, applied); emitErr != nil {
						reportSysctlEventEmitFailed(log, issues, sysctlevents.EventApplied, ev.Key, emitErr)
					}
				}
			})),
			// Query: show active keys.
			eb.Subscribe(sysctlevents.Namespace, sysctlevents.EventShowRequest, events.AsString(func(payload string) {
				var req struct {
					RequestID string `json:"request-id"`
				}
				_ = json.Unmarshal([]byte(payload), &req)
				entriesJSON, _ := json.Marshal(s.showEntries())
				resp, _ := json.Marshal(struct {
					RequestID string `json:"request-id"`
					Entries   string `json:"entries"`
				}{RequestID: req.RequestID, Entries: string(entriesJSON)})
				if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventShowResult, string(resp)); err != nil {
					reportSysctlEventEmitFailed(log, issues, sysctlevents.EventShowResult, req.RequestID, err)
				}
			})),
			// Query: list known keys.
			eb.Subscribe(sysctlevents.Namespace, sysctlevents.EventListRequest, events.AsString(func(payload string) {
				var req struct {
					RequestID string `json:"request-id"`
				}
				_ = json.Unmarshal([]byte(payload), &req)
				keysJSON, _ := json.Marshal(listKnownKeys())
				resp, _ := json.Marshal(struct {
					RequestID string `json:"request-id"`
					Entries   string `json:"entries"`
				}{RequestID: req.RequestID, Entries: string(keysJSON)})
				if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventListResult, string(resp)); err != nil {
					reportSysctlEventEmitFailed(log, issues, sysctlevents.EventListResult, req.RequestID, err)
				}
			})),
			// Clear profile defaults for an interface (before re-emission on reload).
			eb.Subscribe(sysctlevents.Namespace, sysctlevents.EventClearProfileDefaults, events.AsString(func(payload string) {
				var ev struct {
					Interface string `json:"interface"`
				}
				if err := json.Unmarshal([]byte(payload), &ev); err != nil {
					log.Warn("sysctl: bad clear-profile-defaults event", "err", err)
					return
				}
				s.clearProfileDefaults(ev.Interface)
			})),
			// Clear all defaults from a named source (before re-emission on reload).
			eb.Subscribe(sysctlevents.Namespace, sysctlevents.EventClearSourceDefaults, events.AsString(func(payload string) {
				var ev struct {
					Source string `json:"source"`
				}
				if err := json.Unmarshal([]byte(payload), &ev); err != nil {
					log.Warn("sysctl: bad clear-source-defaults event", "err", err)
					return
				}
				s.clearSourceDefaults(ev.Source)
			})),
			// Query: describe one key.
			eb.Subscribe(sysctlevents.Namespace, sysctlevents.EventDescribeRequest, events.AsString(func(payload string) {
				var req struct {
					RequestID string `json:"request-id"`
					Key       string `json:"key"`
				}
				_ = json.Unmarshal([]byte(payload), &req)
				detailJSON, _ := json.Marshal(s.describeKey(req.Key))
				resp, _ := json.Marshal(struct {
					RequestID string `json:"request-id"`
					Detail    string `json:"detail"`
				}{RequestID: req.RequestID, Detail: string(detailJSON)})
				if _, err := eb.Emit(sysctlevents.Namespace, sysctlevents.EventDescribeResult, string(resp)); err != nil {
					reportSysctlEventEmitFailed(log, issues, sysctlevents.EventDescribeResult, req.Key, err)
				}
			})),
		)
	}

	// applyFromJSON parses sysctl config JSON and applies settings.
	applyFromJSON := func(data string) error {
		settings := parseSysctlConfig(data)
		if len(settings) == 0 {
			return nil
		}
		applied, errs := s.applyConfig(settings)
		if len(errs) > 0 {
			return fmt.Errorf("sysctl config: %w", errs[0])
		}
		if eb != nil {
			for _, payload := range applied {
				if _, emitErr := eb.Emit(sysctlevents.Namespace, sysctlevents.EventApplied, payload); emitErr != nil {
					reportSysctlEventEmitFailed(log, issues, sysctlevents.EventApplied, appliedPayloadKey(payload), emitErr)
				}
			}
		}
		log.Info("sysctl config applied", "keys", len(settings))
		return nil
	}

	p.OnConfigure(func(sections []sdk.ConfigSection) error {
		for _, sec := range sections {
			if sec.Root != configRoot {
				continue
			}
			// Register user-defined profiles before applying settings.
			for _, prof := range parseSysctlProfileConfig(sec.Data) {
				sysctlreg.RegisterProfile(prof)
				log.Info("sysctl: registered user profile", "name", prof.Name, "keys", len(prof.Settings))
			}
			return applyFromJSON(sec.Data)
		}
		return nil
	})

	p.OnConfigVerify(verifySysctlConfig)

	var activeJournal *sdk.Journal

	p.OnConfigApply(func(sections []sdk.ConfigDiffSection) error {
		for _, sec := range sections {
			if sec.Root != configRoot {
				continue
			}
			// Deregister profiles that were removed from config.
			for _, prof := range parseSysctlProfileConfig(sec.Removed) {
				sysctlreg.DeregisterProfile(prof.Name)
				log.Info("sysctl: deregistered user profile", "name", prof.Name)
			}
			// Re-register user-defined profiles from added/changed config.
			for _, data := range []string{sec.Added, sec.Changed} {
				for _, prof := range parseSysctlProfileConfig(data) {
					sysctlreg.RegisterProfile(prof)
					log.Info("sysctl: registered user profile", "name", prof.Name, "keys", len(prof.Settings))
				}
			}

			// Merge Added and Changed into one settings map. Keys absent
			// from the result cause applyConfig to clear the config layer
			// (handling Removed implicitly).
			settings := make(map[string]string)
			for _, data := range []string{sec.Added, sec.Changed} {
				maps.Copy(settings, parseSysctlConfig(data))
			}

			// Snapshot config state before apply for rollback.
			snap := s.snapshotConfig()
			j := sdk.NewJournal()
			err := j.Record(
				func() error {
					applied, errs := s.applyConfig(settings)
					if len(errs) > 0 {
						return fmt.Errorf("sysctl config: %w", errs[0])
					}
					if eb != nil {
						for _, payload := range applied {
							if _, emitErr := eb.Emit(sysctlevents.Namespace, sysctlevents.EventApplied, payload); emitErr != nil {
								reportSysctlEventEmitFailed(log, issues, sysctlevents.EventApplied, appliedPayloadKey(payload), emitErr)
							}
						}
					}
					return nil
				},
				func() error {
					s.rollbackConfig(snap)
					return nil
				},
			)
			if err != nil {
				j.Rollback()
				return err
			}
			activeJournal = j
			log.Info("sysctl config reloaded", "keys", len(settings))
			return nil
		}
		return nil
	})

	p.OnConfigRollback(func(_ string) error {
		j := activeJournal
		activeJournal = nil
		if j == nil {
			return nil
		}
		if errs := j.Rollback(); len(errs) > 0 {
			return fmt.Errorf("sysctl rollback: %d errors", len(errs))
		}
		log.Info("sysctl config rolled back")
		return nil
	})

	p.OnStarted(func(_ context.Context) error {
		log.Info("sysctl plugin started")
		return nil
	})

	const (
		statusDone  = "done"
		statusError = "error"
	)

	p.OnExecuteCommand(func(_, command string, args []string, _ string, _ []sdk.PipeOperation) (string, any, error) {
		switch command {
		case "show sysctl":
			return statusDone, s.showEntries(), nil
		case "show sysctl keys":
			return statusDone, listKnownKeys(), nil
		case "show sysctl key":
			if len(args) < 1 {
				return statusError, "", errSysctlDescribeRequiresKeyArgument
			}
			return statusDone, s.describeKey(args[0]), nil
		case "show sysctl profiles":
			return statusDone, listProfiles(), nil
		case "show sysctl profile":
			if len(args) < 1 {
				return statusError, "", errSysctlDescribeProfileRequiresProfileName
			}
			return statusDone, describeProfile(args[0]), nil
		case "set sysctl":
			applied, err := answerSetSysctl(s, args)
			if err != nil {
				return statusError, "", err
			}
			if eb != nil {
				if _, emitErr := eb.Emit(sysctlevents.Namespace, sysctlevents.EventApplied, string(applied)); emitErr != nil {
					reportSysctlEventEmitFailed(log, issues, sysctlevents.EventApplied, args[0], emitErr)
				}
			}
			return statusDone, applied, nil
		}
		return statusError, "", fmt.Errorf("unknown command: %s", command)
	})

	ctx, cancel := sdk.SignalContext()
	defer cancel()
	err := p.Run(ctx, &sdk.Registration{
		WantsConfig:  []string{configRoot},
		VerifyBudget: 1,
		ApplyBudget:  1,
		Commands:     commandDecls(),
		Sends:        sysctlSends(),
	})
	if err != nil {
		log.Error("sysctl plugin failed", "error", err)
		return 1
	}

	// Unsubscribe event handlers before restoring originals and stopping the
	// report queue so no late callback queues onto a stopped queue.
	unsubscribeAll()

	// Restore original values on clean stop.
	s.restoreAll()

	return 0
}

const fixSysctlEmitFailed = "ze show sysctl"

// reportSysctlEventEmitFailed logs a WARN carrying what, why, fix, and the
// registered doctor code, and queues an error event onto the daemon's report
// bus through issuequeue -> SDK RaiseIssue so both internal and forked runs
// report the lost EventBus notification.
func reportSysctlEventEmitFailed(log *slog.Logger, issues *issuequeue.Queue, eventType, subject string, emitErr error) {
	if subject == "" {
		subject = eventType
	}
	if len(subject) > rpc.ReportSubjectMax {
		subject = fitUTF8Bound(subject, rpc.ReportSubjectMax)
	}
	msg := fitUTF8Bound(
		"Sysctl event "+eventType+" for "+subject+" could not be emitted on the EventBus; subscribers or query callers missed the update: "+emitErr.Error(),
		rpc.ReportMessageMax,
	)
	reportErr := issues.RaiseError(codeDoctorSysctlEventEmit, subject, msg, fixSysctlEmitFailed, func(err error) {
		log.Warn("sysctl: report-issue RPC for event emit failure was refused by the daemon",
			"event", eventType, "key", subject, "err", err, "fix", fixSysctlEmitFailed, "code", codeDoctorSysctlEventEmit)
	})
	log.Warn("sysctl: failed to emit event on the EventBus; subscribers or query callers did not receive the update",
		"event", eventType, "key", subject, "err", emitErr, "fix", fixSysctlEmitFailed,
		"code", codeDoctorSysctlEventEmit, issuequeue.Outcome(reportErr))
}

// appliedPayloadKey extracts the sysctl key from an appliedJSON payload so
// event-emit diagnostics name the tunable that changed.
func appliedPayloadKey(payload string) string {
	var parsed struct {
		Key string `json:"key"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err == nil && parsed.Key != "" {
		return parsed.Key
	}
	return sysctlevents.EventApplied
}

// fitUTF8Bound cuts s to at most limit octets at a valid UTF-8 rune boundary
// so report-issue RPC fields never fail the wire length check.
func fitUTF8Bound(s string, limit int) string {
	if len(s) <= limit {
		return s
	}
	cut := limit
	for cut > 0 && !utf8.RuneStart(s[cut]) {
		cut--
	}
	return s[:cut]
}

// answerSetSysctl applies `set sysctl <key> <value>` as a transient value and
// answers the applied event object. It is a json.RawMessage because the
// payload is already encoded: handed over as a Go string it would marshal to a
// JSON string scalar, which no format renders and plugin.RawJSON refuses.
func answerSetSysctl(s *store, args []string) (json.RawMessage, error) {
	if len(args) < 2 {
		return nil, errSysctlSetRequiresKeyAndValue
	}
	applied, err := s.setTransient(args[0], args[1])
	if err != nil {
		return nil, fmt.Errorf("sysctl set %s: %w", args[0], err)
	}
	return json.RawMessage(applied), nil
}

// commandDecls names the commands this plugin serves and states what each
// answer holds (pkg/plugin/rpc/types.go, CommandDecl).
//
// It has two readers and MUST stay one function. init() puts it on the
// registry.Registration, which anything linking the composition root reads
// without starting an engine, and the runner sends it in the Stage 1
// registration message, which a running daemon reads.
//
// An argument is spelled in ANGLE BRACKETS, because both readers publish the
// tokens verbatim as the command's invocation form (pluginUsage,
// cmd/ze/help_command.go and internal/le/cli/catalog/catalog.go). A bare
// identifier reads as a keyword an operator types, so `show sysctl key` was
// published as `show sysctl key key`.
func commandDecls() []sdk.CommandDecl {
	return []sdk.CommandDecl{
		{Name: "show sysctl", ShortHelp: "Show all active sysctl keys with source and persistence"},
		{Name: "show sysctl keys", ShortHelp: "List all known sysctl keys with descriptions"},
		{Name: "show sysctl key", ShortHelp: "Show detail for one sysctl key", Args: []string{"<key>"}},
		{Name: "set sysctl", ShortHelp: "Set a transient sysctl value", Args: []string{"<key>", "<value>"}},
		{Name: "show sysctl profiles", ShortHelp: "List all registered sysctl profiles"},
		{Name: "show sysctl profile", ShortHelp: "Show detail for one sysctl profile", Args: []string{"<name>"}},
	}
}
