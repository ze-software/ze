// Design: docs/labs/l2tp-interop.md -- mounted kl2tpd and pppd inputs.
// Related: scale_inputs.go -- peer/session identities used by the checker.
package l2tp

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestScalePeerInputs(t *testing.T) {
	directory := filepath.Join("..", "..", "..", "..", "test", "interop-l2tp", "scenarios", scenarioScale)
	usersFile, err := os.ReadFile(filepath.Join(directory, "users"))
	if err != nil {
		t.Fatal(err)
	}
	users := make(map[string]bool)
	for line := range strings.SplitSeq(string(usersFile), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 || strings.HasPrefix(fields[0], "#") || fields[0] == "Framed-Protocol" {
			continue
		}
		if len(fields) != 4 || fields[1] != "Cleartext-Password" || fields[2] != ":=" || fields[3] != `"scale-fixture-password"` {
			t.Fatalf("invalid RADIUS user declaration %q", line)
		}
		if users[fields[0]] {
			t.Fatalf("duplicate RADIUS user %s", fields[0])
		}
		users[fields[0]] = true
	}
	interfaces := make(map[string]bool)
	for lac := 1; lac <= scaleLACCount; lac++ {
		raw, err := os.ReadFile(filepath.Join(directory, fmt.Sprintf("lac-%d.toml", lac)))
		if err != nil {
			t.Fatal(err)
		}
		var config struct {
			Tunnels map[string]struct {
				Local         string `toml:"local"`
				Peer          string `toml:"peer"`
				Version       string `toml:"version"`
				Encapsulation string `toml:"encap"`
				Window        int    `toml:"window_size"`
				Sessions      map[string]struct {
					ID         int    `toml:"sid"`
					Pseudowire string `toml:"pseudowire"`
					Arguments  string `toml:"pppd_args"`
				} `toml:"session"`
			} `toml:"tunnel"`
		}
		if err := toml.Unmarshal(raw, &config); err != nil {
			t.Fatal(err)
		}
		if len(config.Tunnels) != 1 {
			t.Fatalf("LAC%d has %d tunnels", lac, len(config.Tunnels))
		}
		for _, tunnel := range config.Tunnels {
			if tunnel.Local != fmt.Sprintf("172.29.0.%d:1702", 10+lac) || tunnel.Peer != fmt.Sprintf("172.29.0.%d:1701", 20+lac) {
				t.Fatalf("LAC%d wrong relay/socket addresses", lac)
			}
			if tunnel.Version != "l2tpv2" || tunnel.Encapsulation != "udp" || tunnel.Window != 4 {
				t.Fatalf("LAC%d wrong tunnel protocol/window", lac)
			}
			if len(tunnel.Sessions) != scaleSessionsPerLAC {
				t.Fatalf("LAC%d has %d sessions", lac, len(tunnel.Sessions))
			}
			ids := make(map[int]bool)
			for _, session := range tunnel.Sessions {
				if session.ID < 1 || session.ID > scaleSessionsPerLAC || ids[session.ID] || session.Pseudowire != "ppp" {
					t.Fatalf("LAC%d invalid/repeated session ID %d", lac, session.ID)
				}
				ids[session.ID] = true
				user := scaleUsername(lac, session.ID)
				if !users[user] || session.Arguments != "/etc/ppp/scale/"+user {
					t.Fatalf("LAC%d session%d lacks own authenticated input", lac, session.ID)
				}
				arguments, err := os.ReadFile(filepath.Join(directory, "ppp", user))
				if err != nil {
					t.Fatal(err)
				}
				options := make(map[string]string)
				for line := range strings.SplitSeq(strings.TrimSuffix(string(arguments), "\n"), "\n") {
					fields := strings.Split(line, " ") // kl2tpd's actual pppd args reader.
					if len(fields) < 1 || len(fields) > 2 || fields[0] == "" {
						t.Fatalf("unsafe kl2tpd argument line %q", line)
					}
					if _, exists := options[fields[0]]; exists {
						t.Fatalf("duplicate PPP option %s", fields[0])
					}
					value := ""
					if len(fields) == 2 {
						value = fields[1]
					}
					options[fields[0]] = value
				}
				if options["name"] != user || options["password"] != "scale-fixture-password" || options["linkname"] != user || options["logfile"] != "/run/"+user+".log" {
					t.Fatalf("session %s lacks own authentication/process/log identity", user)
				}
				name := scaleInterface(lac, session.ID)
				if options["ifname"] != name || interfaces[name] {
					t.Fatalf("session %s has wrong/shared interface", user)
				}
				interfaces[name] = true
				if _, exists := options["persist"]; exists {
					t.Fatalf("session %s redials after targeted teardown", user)
				}
				if _, exists := options["nopersist"]; !exists {
					t.Fatalf("session %s lacks explicit no-redial", user)
				}
			}
		}
	}
	if len(users) != scaleLACCount*scaleSessionsPerLAC || len(interfaces) != len(users) {
		t.Fatalf("have %d users/%d interfaces, want48", len(users), len(interfaces))
	}
}

func TestScaleDeferredLACLaunch(t *testing.T) {
	peers := scalePeerTemplates("", "fixture", "deferred")
	for lac := 1; lac <= scaleLACCount; lac++ {
		peer := peerByName(t, peers, "kl2tpd-"+strconv.Itoa(lac))
		if !slices.Equal(peer.Command, []string{"sleep", "infinity"}) {
			t.Fatalf("LAC%d starts before checker can observe real CWND4", lac)
		}
		if peer.Ready == nil || !slices.Equal(peer.Ready.Command, []string{"test", "-r", "/etc/kl2tpd/kl2tpd.toml"}) {
			t.Fatalf("LAC%d readiness requires a running protocol peer", lac)
		}
	}
}
