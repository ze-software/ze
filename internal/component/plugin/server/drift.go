// Design: docs/architecture/diagnostics/production-diagnostics.md -- daemon drift-check RPC fan-out across internal and forked plugins
// Related: stage1_checks.go -- Stage 1 drift-subsystems validation and recording
// Related: server.go -- StartWithContext registers remote collector/lister; Stop unregisters

package server

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/core/drift"
	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

const (
	maxDriftSubsystemsPerPlugin = 16
	maxDriftSubsystemNameLen    = 64
	driftCheckTimeout           = 5 * time.Second
)

// validateDriftSubsystemDecls validates Stage 1 drift-subsystems declarations:
// at most 16 subsystems per plugin, each 1..64 ASCII [a-z0-9-]+ characters,
// with no duplicates within one declaration.
func validateDriftSubsystemDecls(subsystems []string) error {
	if len(subsystems) > maxDriftSubsystemsPerPlugin {
		var tb textbuf.Buffer
		tb.Str("too many drift subsystems: ").Int(int64(len(subsystems))).
			Str(" (max ").Int(maxDriftSubsystemsPerPlugin).Byte(')')
		return errors.New(tb.String())
	}
	seen := make(map[string]struct{}, len(subsystems))
	for _, sub := range subsystems {
		if sub == "" {
			return errors.New("drift subsystem name must not be empty")
		}
		if len(sub) > maxDriftSubsystemNameLen {
			var tb textbuf.Buffer
			tb.Str("drift subsystem name ").Str(strconv.Quote(sub)).
				Str(" exceeds max length ").Int(maxDriftSubsystemNameLen)
			return errors.New(tb.String())
		}
		if !isValidDriftSubsystemName(sub) {
			var tb textbuf.Buffer
			tb.Str("invalid drift subsystem name ").Str(strconv.Quote(sub)).
				Str(" (must match [a-z0-9-]+)")
			return errors.New(tb.String())
		}
		if _, dup := seen[sub]; dup {
			return errors.New("duplicate drift subsystem " + strconv.Quote(sub))
		}
		seen[sub] = struct{}{}
	}
	return nil
}

// isValidDriftSubsystemName reports whether s consists solely of ASCII [a-z0-9-]+.
func isValidDriftSubsystemName(s string) bool {
	if s == "" {
		return false
	}
	for i := range len(s) {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			continue
		}
		if c >= '0' && c <= '9' {
			continue
		}
		if c == '-' {
			continue
		}
		return false
	}
	return true
}

// registerPluginDriftSubsystems atomically claims the Stage 1 validated
// drift-subsystems for proc, pruning retired or stopped processes first and
// refusing any subsystem already owned by another live plugin. Keying by
// *process.Process ensures stopped or restarted generations never shadow live
// plugins. Safe for concurrent use.
func (s *Server) registerPluginDriftSubsystems(proc *process.Process, subsystems []string) error {
	if s == nil {
		return nil
	}
	if proc == nil {
		return nil
	}
	var liveSet map[*process.Process]struct{}
	if pm := s.procManager.Load(); pm != nil {
		procs := pm.AllProcesses()
		liveSet = make(map[*process.Process]struct{}, len(procs))
		for _, p := range procs {
			if p.Running() {
				liveSet[p] = struct{}{}
			}
		}
	}

	s.driftMu.Lock()
	defer s.driftMu.Unlock()
	if len(subsystems) == 0 {
		if s.driftSubsystems != nil {
			delete(s.driftSubsystems, proc)
		}
		return nil
	}
	for p := range s.driftSubsystems {
		if p == proc {
			continue
		}
		if liveSet != nil {
			if _, alive := liveSet[p]; !alive {
				delete(s.driftSubsystems, p)
			}
			continue
		}
		if !p.Running() {
			delete(s.driftSubsystems, p)
		}
	}
	for _, want := range subsystems {
		for owner, declared := range s.driftSubsystems {
			if owner == proc {
				continue
			}
			if slices.Contains(declared, want) {
				var tb textbuf.Buffer
				return errors.New(tb.Str("drift subsystem ").Quoted(want).Str(" is already registered by plugin ").Quoted(owner.Name()).String())
			}
		}
	}
	if s.driftSubsystems == nil {
		s.driftSubsystems = make(map[*process.Process][]string)
	}
	s.driftSubsystems[proc] = slices.Clone(subsystems)
	return nil
}

// unregisterPluginDriftSubsystems removes proc's drift-subsystem claims on
// startup rollback, plugin removal, or restart. Safe for concurrent use.
func (s *Server) unregisterPluginDriftSubsystems(proc *process.Process) {
	if s == nil {
		return
	}
	if proc == nil {
		return
	}
	s.driftMu.Lock()
	if s.driftSubsystems != nil {
		delete(s.driftSubsystems, proc)
	}
	s.driftMu.Unlock()
}

func (s *Server) clearPluginDriftSubsystems() {
	if s == nil {
		return
	}
	s.driftMu.Lock()
	s.driftSubsystems = nil
	s.driftMu.Unlock()
}

type driftTarget struct {
	pluginName string
	subsystem  string
	proc       *process.Process
}

// snapshotDriftTargets collects live running plugin processes matching
// subsystem (or all when subsystem == "") while pruning retired process keys.
// Releases driftMu before returning so callers never hold locks across RPCs.
func (s *Server) snapshotDriftTargets(subsystem string) []driftTarget {
	if s == nil {
		return nil
	}
	pm := s.procManager.Load()
	if pm == nil {
		return nil
	}
	procs := pm.AllProcesses()
	liveSet := make(map[*process.Process]struct{}, len(procs))
	for _, proc := range procs {
		liveSet[proc] = struct{}{}
	}

	s.driftMu.Lock()
	for p := range s.driftSubsystems {
		if _, alive := liveSet[p]; !alive {
			delete(s.driftSubsystems, p)
		}
	}
	var targets []driftTarget
	for _, proc := range procs {
		if !proc.Running() {
			continue
		}
		declared := s.driftSubsystems[proc]
		if len(declared) == 0 {
			continue
		}
		for _, sub := range declared {
			if subsystem != "" {
				if sub != subsystem {
					continue
				}
			}
			targets = append(targets, driftTarget{
				pluginName: proc.Name(),
				subsystem:  sub,
				proc:       proc,
			})
		}
	}
	s.driftMu.Unlock()

	slices.SortFunc(targets, func(a, b driftTarget) int {
		if cmp := strings.Compare(a.subsystem, b.subsystem); cmp != 0 {
			return cmp
		}
		return strings.Compare(a.pluginName, b.pluginName)
	})
	return targets
}

// listPluginDriftSubsystems returns sorted unique drift subsystem names
// declared by live running plugins. Registered with drift.RegisterRemoteLister
// on Server.StartWithContext. Safe for concurrent use.
func (s *Server) listPluginDriftSubsystems() []string {
	targets := s.snapshotDriftTargets("")
	if len(targets) == 0 {
		return nil
	}
	names := make([]string, 0, len(targets))
	for _, t := range targets {
		names = append(names, t.subsystem)
	}
	slices.Sort(names)
	return slices.Compact(names)
}

// collectPluginDrift fans out ze-plugin-callback:drift-check to live running
// plugins whose declared drift subsystems match subsystem (or all when empty).
// Works identically across DirectBridge (internal) and MuxConn (forked).
// Registered with drift.RegisterRemoteCollector on Server.StartWithContext.
// Safe for concurrent use.
func (s *Server) collectPluginDrift(ctx context.Context, subsystem string) []drift.Report {
	if ctx == nil {
		ctx = context.Background()
	}
	targets := s.snapshotDriftTargets(subsystem)
	if len(targets) == 0 {
		return nil
	}
	reports := make([]drift.Report, 0, len(targets))
	for _, target := range targets {
		reports = append(reports, queryPluginDrift(ctx, target))
	}
	return reports
}

func queryPluginDrift(ctx context.Context, target driftTarget) drift.Report {
	conn := target.proc.Conn()
	if conn == nil {
		return driftErrorReport(target.subsystem, ErrPluginConnectionClosed.Error())
	}
	// context.WithTimeout respects any earlier deadline already on ctx,
	// bounding the call to min(caller deadline, driftCheckTimeout).
	subCtx, cancel := context.WithTimeout(ctx, driftCheckTimeout)
	defer cancel()

	out, err := conn.SendDriftCheck(subCtx, target.subsystem)
	if err != nil {
		return driftErrorReport(target.subsystem, err.Error())
	}
	if out == nil {
		return driftErrorReport(target.subsystem, "drift-check returned no output")
	}
	return driftReportFromRPC(target.subsystem, out)
}

func driftErrorReport(subsystem, errMsg string) drift.Report {
	return drift.Report{
		Subsystem: subsystem,
		Status:    drift.StatusError,
		Findings:  []drift.Finding{},
		Error:     errMsg,
	}
}

func driftReportFromRPC(subsystem string, out *rpc.DriftCheckOutput) drift.Report {
	sub := out.Subsystem
	if sub == "" {
		sub = subsystem
	}
	status := drift.Status(out.Status)
	errMsg := out.Error
	if status == "" {
		status = drift.StatusError
		if errMsg == "" {
			errMsg = "drift-check returned empty status"
		}
	}
	truncated := out.Truncated
	rawFindings := out.Findings
	if len(rawFindings) > drift.MaxFindingsPerSubsystem {
		rawFindings = rawFindings[:drift.MaxFindingsPerSubsystem]
		truncated = true
	}
	findings := make([]drift.Finding, len(rawFindings))
	for i, f := range rawFindings {
		findings[i] = drift.Finding{
			Subject:  f.Subject,
			Kind:     drift.Kind(f.Kind),
			Expected: f.Expected,
			Actual:   f.Actual,
			Detail:   f.Detail,
		}
	}
	return drift.Report{
		Subsystem: sub,
		Status:    status,
		Checked:   out.Checked,
		Drifted:   out.Drifted,
		Truncated: truncated,
		Findings:  findings,
		Error:     errMsg,
	}
}
