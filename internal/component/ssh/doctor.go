// Design: docs/architecture/system-architecture.md -- the readiness check this component owns
// Overview: register.go -- the init() that installs the registration below
// Related: ssh.go -- NewServer, which resolves the host key path this check probes
//
// The check lived in internal/component/doctor and the runner reached it by
// writing its name out. A check over environment/ssh/host-key and
// host-certificate belongs to the component that loads them
// (ai/patterns/registration.md, "Doctor Check Registry"), so it is owned here
// now and dropping this component drops its check with it.

package ssh

import (
	"os"
	"path/filepath"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// codeSSHHostKeyMissing names a host key or host certificate file the config
// points at and the filesystem does not hold. internal/core/diagnostic/codes.go
// declares it, so `ze explain doctor-ssh-hostkey-missing` answers.
const codeSSHHostKeyMissing = "doctor-ssh-hostkey-missing"

// configLeafTrue is the spelling of a true boolean leaf in the config tree.
const configLeafTrue = "true"

// doctorComponentSSH names this component as the owner of the check below.
const doctorComponentSSH = "ssh"

// sshHostKeyDoctorCheck describes the check. register.go registers it.
//
// Order 2010 reproduces the sequence the doctor runner printed before the
// check moved onto the registry: it was the first check the runner called
// after its own phase dispatch, so it sorts first in the 2000 band that
// internal/component/doctor/doctor_checks.go reserves for those, before the
// disk-space check at 2020.
var sshHostKeyDoctorCheck = diagnostic.DoctorCheck{
	Name:         "ssh-host-key",
	Phase:        diagnostic.DoctorPhasePostConfig,
	Order:        2010,
	Component:    doctorComponentSSH,
	Dependencies: []string{"filesystem"},
	Platforms:    []string{diagnostic.DoctorPlatformAny},
	Codes:        []string{codeSSHHostKeyMissing},
	Check:        checkSSHHostKey,
}

// checkSSHHostKey reports the host key file an enabled ssh block will load
// when it is absent, as a warning because NewServer generates one, and the
// host certificate file when it is absent, as an error because nothing
// generates that.
//
// A run that knows no config directory cannot resolve a relative path the way
// the daemon will, so it reports nothing rather than probing the wrong file.
//
// A nil tree is the missing-config phase, which this check does not run in, and
// a context carrying anything else is a runner defect the runner's own type
// assertion reports (doctorTree, internal/component/doctor/registry.go).
func checkSSHHostKey(ctx diagnostic.DoctorCheckContext) []diagnostic.Diagnostic {
	tree, ok := ctx.Tree.(*config.Tree)
	if !ok || tree == nil {
		return nil
	}
	if ctx.ConfigDir == "" {
		return nil
	}

	envBlock := tree.GetContainer("environment")
	if envBlock == nil {
		return nil
	}
	sshBlock := envBlock.GetContainer("ssh")
	if sshBlock == nil {
		return nil
	}
	enabled, _ := sshBlock.Get("enabled")
	if enabled != configLeafTrue {
		return nil
	}

	var diags []diagnostic.Diagnostic

	keyPath := ""
	if v, ok := sshBlock.Get("host-key"); ok && v != "" {
		keyPath = diagnostic.DoctorConfigPath(v, ctx.ConfigDir)
	}
	if keyPath == "" {
		keyPath = filepath.Join(ctx.ConfigDir, defaultHostKeyFile)
	}
	var tb textbuf.Buffer
	if _, err := os.Stat(keyPath); err != nil {
		diags = append(diags, diagnostic.Diagnostic{
			Code:     codeSSHHostKeyMissing,
			Severity: diagnostic.SeverityWarning,
			Message:  tb.Str("SSH host key not found: ").Str(keyPath).Str(" (will be auto-generated on first start)").String(),
			Path:     keyPath,
		})
	}

	if certPath, ok := sshBlock.Get("host-certificate"); ok && certPath != "" {
		resolved := diagnostic.DoctorConfigPath(certPath, ctx.ConfigDir)
		if _, err := os.Stat(resolved); err != nil {
			diags = append(diags, diagnostic.Diagnostic{
				Code:     codeSSHHostKeyMissing,
				Severity: diagnostic.SeverityError,
				Message:  tb.Reset().Str("SSH host certificate not found: ").Str(resolved).String(),
				Path:     resolved,
			})
		}
	}

	return diags
}
