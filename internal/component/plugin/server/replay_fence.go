// Design: docs/architecture/api/process-protocol.md -- public replay-fence declaration.
// Related: events.go -- session readiness is a separate declaration.
package server

import (
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// declaresReplayFence projects fencing from the same current Process
// declaration used by readiness. The installed predicate MUST refuse after Stop.
func (s *Server) declaresReplayFence(name string) bool {
	_, fence := s.sessionReplayDeclarations(name)
	return fence
}

// sessionReplayDeclarations reads one current Process/configuration for both
// declaration predicates. Foreign external programs MUST declare their flags
// through an accepted Stage 1 and be running. Compiled-in plugins (in-daemon
// or separate-process) keep compiled readiness defaults; fencing additionally
// needs their accepted running lifetime. No declaration or process is cached here.
func (s *Server) sessionReplayDeclarations(name string) (ready, fence bool) {
	if s.stopEntered.Load() {
		return false, false
	}
	manager := s.procManager.Load()
	if manager == nil {
		return false, false
	}
	proc := manager.GetProcess(name)
	if proc == nil {
		return false, false
	}
	declaration := proc.Registration()
	if declaration != nil {
		ready = declaration.SignalsSessionReady
		if ready {
			fence = declaration.FencesLiveForwards
		}
	}
	configuration := proc.Config()
	if configuration.RunsExternalProgram() {
		if !proc.Running() {
			return false, false
		}
		if declaration == nil {
			return false, false
		}
		if !declaration.Done {
			return false, false
		}
		return ready, fence
	}
	// These are the bounded registry spellings of this one Internal runner.
	// Ready-only metadata remains a legacy reporting default, never a live hold.
	for _, implementation := range plugin.RegistryNames(configuration) {
		if signals, registered := registry.RegisteredSessionReady(implementation); registered {
			if signals {
				ready = true
			}
		}
		if registry.RegisteredReplayFence(implementation) {
			fence = true
		}
	}
	if !proc.Running() {
		return ready, false
	}
	if declaration == nil {
		return ready, false
	}
	if !declaration.Done {
		return ready, false
	}
	return ready, fence
}

func validateReplayFenceDeclaration(name string, input *rpc.DeclareRegistrationInput) error {
	if !input.FencesLiveForwards {
		return nil
	}
	if input.SignalsSessionReady {
		return nil
	}
	return opererr.New("Plugin "+name+" declares fences-live-forwards without signals-session-ready",
		"The replay fence can be released only by the plugin's session-ready report",
		"Declare signals-session-ready true beside fences-live-forwards in the plugin's Stage 1 registration")
}
