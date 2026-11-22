// Design: docs/research/vpp-deployment-reference.md -- physical process prerequisites
// Related: process_peer_linux.go -- actual socketpair/pidfs capability probe
package vpp

import (
	"fmt"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
)

const (
	processReportSource  = "vpp-process"
	processReportFix     = "ze show diagnostic doctor-vpp-process"
	reportDetailErrorKey = "error"
)

func init() {
	if err := diagnostic.RegisterDoctorCheck(diagnostic.DoctorCheck{
		Name: "vpp-process-peer", Phase: diagnostic.DoctorPhasePostConfig,
		Order: 752, Component: componentVPP,
		Platforms:    []string{diagnostic.DoctorPlatformAny},
		Dependencies: []string{dependencyKernel},
		Codes:        []string{diagnostic.CodeDoctorVPPProcess},
		Check:        checkProcessPeer,
	}); err != nil {
		panic("BUG: VPP process doctor registration failed: " + err.Error())
	}
}

func processFailureKey(peer *processWitness, subject string) string {
	return fmt.Sprintf("%d/%d/%d/%d/%s", peer.identity.device, peer.identity.inode,
		peer.connector, peer.generation, subject)
}

func reportProcessFailure(peer *processWitness, subject string, cause error) {
	key := processFailureKey(peer, subject)
	report.RaiseWarningFix(processReportSource, diagnostic.CodeDoctorVPPProcess, key,
		"VPP process ownership could not be confirmed", processReportFix,
		map[string]any{reportDetailErrorKey: cause.Error(), "operation": subject})
	logger().Warn("vpp: process ownership could not be confirmed",
		"code", diagnostic.CodeDoctorVPPProcess, "fix", processReportFix,
		"operation", subject, "error", cause)
}

func clearProcessObservationFailure(peer *processWitness) {
	report.ClearWarning(processReportSource, diagnostic.CodeDoctorVPPProcess,
		processFailureKey(peer, "observation"))
}

func checkProcessPeer(input diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	return checkProcessPeerWithProbe(input, probePhysicalProcessCapability)
}

// Retained runtime failures remain visible even without delivered configuration.
// The host probe creates only its own socketpair and pidfd, before VPP starts.
func checkProcessPeerWithProbe(input diagnostic.DoctorCheckContext, probe func() error) []diagnostic.Diagnostic {
	var findings []diagnostic.Diagnostic
	for _, issue := range report.Warnings() {
		if issue.Source != processReportSource {
			continue
		}
		if issue.Code != diagnostic.CodeDoctorVPPProcess {
			continue
		}
		message := issue.Message
		if cause, ok := issue.Detail[reportDetailErrorKey].(string); ok {
			message += ": " + cause
		}
		findings = append(findings, diagnostic.Diagnostic{Code: diagnostic.CodeDoctorVPPProcess,
			Severity: diagnostic.SeverityWarning, Message: message, Fix: processReportFix})
	}
	if len(findings) != 0 {
		return findings
	}
	tree, ok := input.Tree.(*config.Tree)
	if !ok {
		return nil
	}
	if tree == nil {
		return nil
	}
	block := tree.GetContainer(componentVPP)
	if block == nil {
		return nil
	}
	enabled, _ := block.Get("enabled")
	if enabled != yangTrue {
		return nil
	}
	if err := probe(); err != nil {
		return []diagnostic.Diagnostic{{Code: diagnostic.CodeDoctorVPPProcess,
			Severity: diagnostic.SeverityError,
			Message:  "VPP cannot retain its actual peer process: " + err.Error(), Fix: processReportFix}}
	}
	return nil
}
