//go:build ze_l2tp

package ppp

import (
	"context"
	"fmt"
	"strings"
	"testing"

	clitesting "github.com/ze-software/ze/internal/component/cli/testing"
	_ "github.com/ze-software/ze/internal/component/l2tp/pppoe/yang"
	_ "github.com/ze-software/ze/internal/component/l2tp/yang"
)

// TestAuthRefusalRepairCommands exercises the actual refusal producer, then
// executes every printed command through the editor's normal command parser.
// The editor must preserve the permitted methods and append the peer's method.
func TestAuthRefusalRepairCommands(t *testing.T) {
	for _, pppoe := range []bool{false, true} {
		name, path, leaf, seed := "l2tp", "l2tp authentication allow", "allow", `l2tp {
    auth-method chap-md5;
    authentication { allow [ chap-md5 ms-chap-v2 ]; }
}`
		if pppoe {
			name, path, leaf, seed = "pppoe", "pppoe authentication-allow", "authentication-allow", `pppoe {
    auth-method chap-md5;
    authentication-allow [ chap-md5 ms-chap-v2 ];
}`
		}
		t.Run(name, func(t *testing.T) {
			s, _, events := newRFC1661Session(LCPStateAckSent)
			t.Cleanup(func() {
				if s.restartTimer != nil {
					s.restartTimer.Stop()
				}
			})
			s.pppoe = pppoe
			s.authRequired = true
			s.authFallbackOrder = []AuthMethod{AuthMethodCHAPMD5, AuthMethodMSCHAPv2}
			if s.refuseAuthMethod(LCPStateAckSent, LCPPacket{
				Code: LCPConfigureNak, Data: optStream(authProtoOpt(authProtoPAP)),
			}) {
				t.Fatal("refusal ended the session before termination grace")
			}
			if s.currentState() != LCPStateClosing || len(events) != 0 || len(s.authEventsOut) != 0 {
				t.Fatal("repair text changed the required-auth refusal lifecycle")
			}
			if !strings.Contains(s.authRefusalReason, "Only if this method is intentionally permitted") {
				t.Fatal("repair omitted the explicit operator policy decision")
			}
			commands := refusalRepairCommands(s.authRefusalReason)
			methods := []string{"chap-md5", "ms-chap-v2", "pap"}
			if len(commands) != len(methods) {
				t.Fatalf("printed commands %q, want one per retained or added method", commands)
			}
			var script strings.Builder
			fmt.Fprintf(&script, "tmpfs=test.conf:terminator=EOF_CONF\n%s\nEOF_CONF\noption=file:path=test.conf\noption=timeout:value=20s\n", seed)
			for i, command := range commands {
				if want := "set " + path + " " + methods[i]; command != want {
					t.Fatalf("command %d = %q, want %q", i, command, want)
				}
				fmt.Fprintf(&script, "input=type:text=%s\ninput=enter\nexpect=error:none\n", command)
			}
			fmt.Fprintf(&script, "expect=content:pattern=%s\\s+\\[\\s*chap-md5\\s+ms-chap-v2\\s+pap\\s*\\]\n", leaf)
			result := clitesting.RunETContentIn(context.Background(), script.String(), t.TempDir())
			if !result.Passed {
				t.Fatalf("printed repair failed in editor: %s", result.Error)
			}
		})
	}
}

// TestAuthRefusalUnknownMethodRepair preserves the fail-closed explanation
// without offering an invented editor value for an unknown protocol or Reject.
func TestAuthRefusalUnknownMethodRepair(t *testing.T) {
	for _, packet := range []LCPPacket{
		{Code: LCPConfigureNak, Data: optStream(authProtoOpt(0xc227))},
		{Code: LCPConfigureReject},
	} {
		s, _, _ := newRFC1661Session(LCPStateReqSent)
		t.Cleanup(func() {
			if s.restartTimer != nil {
				s.restartTimer.Stop()
			}
		})
		s.authFallbackOrder = []AuthMethod{AuthMethodCHAPMD5}
		s.refuseAuthMethod(LCPStateReqSent, packet)
		if !strings.Contains(s.authRefusalReason, "configure the peer to accept an allowed method") {
			t.Fatal("refusal lacks an actionable peer-policy explanation")
		}
		if commands := refusalRepairCommands(s.authRefusalReason); len(commands) != 0 {
			t.Fatalf("unknown/rejected method offered policy commands: %q", commands)
		}
	}
}

func refusalRepairCommands(reason string) []string {
	parts := strings.Split(reason, "`")
	var commands []string
	for i := 1; i < len(parts); i += 2 {
		commands = append(commands, parts[i])
	}
	return commands
}
