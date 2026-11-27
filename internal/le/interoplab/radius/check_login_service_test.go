// Design: docs/architecture/testing/interop.md -- polarity tests for the FreeRADIUS login-service refusal checker.
// Related: check_login_service.go -- the checker under test.

package radius

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

type loginServicePolicy struct {
	base           labPolicy
	webRefuse      bool
	logRefusal     bool
	exportMetric   bool
	sendNASPortSSH bool
	sendNASPortWeb bool
}

func conformingLoginServicePolicy() loginServicePolicy {
	return loginServicePolicy{
		base:           conformingPAP(),
		webRefuse:      true,
		logRefusal:     true,
		exportMetric:   true,
		sendNASPortSSH: true,
		sendNASPortWeb: true,
	}
}

type loginServiceStubLab struct {
	inner     *stubLab
	policy    loginServicePolicy
	serverLog []string
	refused   int
}

func newLoginServiceStubLab(policy loginServicePolicy) *scenarioLab {
	return &scenarioLab{
		lab: &loginServiceStubLab{
			inner:  &stubLab{policy: policy.base},
			policy: policy,
		},
		timeout: 1500 * time.Millisecond,
	}
}

func (s *loginServiceStubLab) Exec(ctx context.Context, peer string, argv []string, env []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	command := strings.Join(argv, " ")
	if peer == zePeer && strings.Contains(command, "curl ") {
		return s.handleWebProbe(command), nil
	}
	if peer == zePeer && strings.Contains(command, metricsURL) {
		if !s.policy.exportMetric {
			return interoplab.CommandResult{Stdout: "# HELP ze_up fixture\nze_up 1\n"}, nil
		}
		return interoplab.CommandResult{
			Stdout: loginServiceMetricName + `{service="web"} ` + strconv.Itoa(s.refused) + "\n",
		}, nil
	}
	if peer == zePeer && strings.Contains(command, "ze cli --user ") {
		if s.policy.sendNASPortSSH {
			s.serverLog = append(s.serverLog, "\t"+nasPortIDSSH)
		}
	}
	return s.inner.Exec(ctx, peer, argv, env)
}

func (s *loginServiceStubLab) handleWebProbe(command string) interoplab.CommandResult {
	if strings.Contains(command, localUser+":"+localPassword) {
		return interoplab.CommandResult{Stdout: webStatusMarker + "200\n"}
	}
	if s.policy.sendNASPortWeb {
		s.serverLog = append(s.serverLog, "\t"+nasPortIDWeb)
	}
	if !s.policy.webRefuse {
		return interoplab.CommandResult{Stdout: webStatusMarker + "200\n"}
	}
	if s.policy.logRefusal {
		s.inner.zeLog = append(s.inner.zeLog,
			`INFO msg="`+loginServiceGateMessage+`" account=`+radiusUser+` login_service_set=ssh-only service=web reason="service not in login-service set"`)
	}
	s.refused++
	return interoplab.CommandResult{Stdout: webStatusMarker + "401\n"}
}

func (s *loginServiceStubLab) Query(ctx context.Context, peer string, argv []string, env []interoplab.EnvironmentVariable) (string, error) {
	result, err := s.Exec(ctx, peer, argv, env)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(result.Stdout) == "" {
		return "", errors.New("stub query returned no output")
	}
	return result.Stdout, nil
}

func (s *loginServiceStubLab) Logs(ctx context.Context, peer string, lines int) (interoplab.LogResult, error) {
	if peer == serverPeer {
		return interoplab.LogResult{Text: strings.Join(s.serverLog, "\n") + "\n", Available: true}, nil
	}
	return s.inner.Logs(ctx, peer, lines)
}

func (s *loginServiceStubLab) ExecDetached(ctx context.Context, peer string, argv []string, env []interoplab.EnvironmentVariable) error {
	return s.inner.ExecDetached(ctx, peer, argv, env)
}

func (s *loginServiceStubLab) PeerPID(ctx context.Context, peer string) (int, error) {
	return s.inner.PeerPID(ctx, peer)
}

func (s *loginServiceStubLab) PeerImage(ctx context.Context, peer string) (string, error) {
	return s.inner.PeerImage(ctx, peer)
}

func (s *loginServiceStubLab) Signal(ctx context.Context, peer, sig string) error {
	return s.inner.Signal(ctx, peer, sig)
}

func (s *loginServiceStubLab) Pause(ctx context.Context, peer string) error {
	return s.inner.Pause(ctx, peer)
}

func (s *loginServiceStubLab) Unpause(ctx context.Context, peer string) error {
	return s.inner.Unpause(ctx, peer)
}

func (s *loginServiceStubLab) Start(ctx context.Context, peer string) error {
	return s.inner.Start(ctx, peer)
}

func (s *loginServiceStubLab) Stop(ctx context.Context, peer string, sec int) error {
	return s.inner.Stop(ctx, peer, sec)
}

// VALIDATES: checkLoginServiceRefused passes on a conforming OD-1 option (d)
// exchange and fails when web is wrongly admitted, when NAS-Port-Id is missing
// on SSH or Web, when the gate refusal log is missing, or when the refusal
// Prometheus counter does not step.
// PREVENTS: a vacuous FreeRADIUS interop checker that ignores the web refusal,
// NAS-Port-Id preservation, or refusal observability.
func TestLoginServiceRefusedCheckerPolarities(t *testing.T) {
	cases := []struct {
		name    string
		policy  func() loginServicePolicy
		wantErr string
	}{
		{
			name:   "conforming OD-1 option d exchange passes",
			policy: conformingLoginServicePolicy,
		},
		{
			name: "admitting radiusop on excluded web door fails",
			policy: func() loginServicePolicy {
				p := conformingLoginServicePolicy()
				p.webRefuse = false
				return p
			},
			wantErr: "returned HTTP 200, want 401",
		},
		{
			name: "missing NAS-Port-Id ssh on FreeRADIUS fails",
			policy: func() loginServicePolicy {
				p := conformingLoginServicePolicy()
				p.sendNASPortSSH = false
				return p
			},
			wantErr: nasPortIDSSH,
		},
		{
			name: "missing NAS-Port-Id web on FreeRADIUS fails",
			policy: func() loginServicePolicy {
				p := conformingLoginServicePolicy()
				p.sendNASPortWeb = false
				return p
			},
			wantErr: nasPortIDWeb,
		},
		{
			name: "missing login-service refusal log fails",
			policy: func() loginServicePolicy {
				p := conformingLoginServicePolicy()
				p.logRefusal = false
				return p
			},
			wantErr: loginServiceGateMessage,
		},
		{
			name: "missing login-service refusal metric fails",
			policy: func() loginServicePolicy {
				p := conformingLoginServicePolicy()
				p.exportMetric = false
				return p
			},
			wantErr: loginServiceMetricName,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := checkLoginServiceRefused(context.Background(), newLoginServiceStubLab(tc.policy()))
			assertCheckerVerdict(t, err, tc.wantErr)
		})
	}
}
