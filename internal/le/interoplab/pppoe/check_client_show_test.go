package pppoe

import (
	"context"
	"encoding/json"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
)

// zeShowPPPoEClientLearned is the REST reply of `show interface pppoe-client`
// once 01-pppoe-chap-ipv4's client learned accel-ppp's two IPCP servers. Its
// shape follows iface.PPPoEClientStatus.
const zeShowPPPoEClientLearned = `{"status":"done","data":[{"name":"pppoe0","state":"up",` +
	`"source-interface":"eth0","ppp-interface":"pppoe0","session-id":1,` +
	`"local-ip":"10.11.0.2","peer-ip":"10.11.0.1","uptime":"5s",` +
	`"dns-servers":["192.0.2.53","192.0.2.54"]}]}`

// isShowPPPoEClientQuery reports whether argv is the checker's REST request
// for `show interface pppoe-client`, run in the Ze container.
func isShowPPPoEClientQuery(peer string, argv []string) bool {
	if peer != zeImageName || len(argv) == 0 || argv[0] != "curl" {
		return false
	}
	return strings.Contains(argv[len(argv)-1], `"show interface pppoe-client"`)
}

// showPPPoEClientReply wraps one client's dns-servers member (the raw JSON,
// or no key at all when member is empty) in a REST reply for pppoe0.
func showPPPoEClientReply(member string) string {
	client := `{"name":"pppoe0","state":"up"`
	if member != "" {
		client += `,"dns-servers":` + member
	}
	return `{"status":"done","data":[` + client + `}]}`
}

func TestPPPoEClientDNSServersReadsTheNamedClient(t *testing.T) {
	// VALIDATES: The parser answers the dns-servers list of the named client
	// in order, an empty list as empty and non-nil, and refuses an absent
	// key, a null, a non-list, a missing client, a nameless client and an
	// answer that is not a list.
	// PREVENTS: An absent key or a null decoding as the empty list a client
	// that learned no server owes, so AC-11's empty half could never go red.
	tests := []struct {
		name string
		data string
		want []string
		err  string
	}{
		{"two servers in order", `[{"name":"pppoe0","dns-servers":["192.0.2.53","192.0.2.54"]}]`, []string{"192.0.2.53", "192.0.2.54"}, ""},
		{"empty list", `[{"name":"pppoe0","dns-servers":[]}]`, []string{}, ""},
		{"named among others", `[{"name":"a","dns-servers":["198.51.100.1"]},{"name":"pppoe0","dns-servers":["192.0.2.53"]}]`, []string{"192.0.2.53"}, ""},
		{"key absent", `[{"name":"pppoe0"}]`, nil, "has no dns-servers key"},
		{"null", `[{"name":"pppoe0","dns-servers":null}]`, nil, "is null"},
		{"not a list", `[{"name":"pppoe0","dns-servers":"192.0.2.53"}]`, nil, "not a list"},
		{"client absent", `[{"name":"pppoe1","dns-servers":[]}]`, nil, "answered no client pppoe0"},
		{"no clients", `[]`, nil, "answered no client pppoe0"},
		{"nameless client", `[{"dns-servers":[]}]`, nil, "client with no name"},
		{"object not list", `{"name":"pppoe0","dns-servers":[]}`, nil, "no list of clients"},
		{"null data", `null`, nil, "answered no client pppoe0"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := pppoeClientDNSServers(json.RawMessage(test.data), zeClientDevice)
			if test.err != "" {
				if err == nil || !strings.Contains(err.Error(), test.err) {
					t.Fatalf("pppoeClientDNSServers(%s) = %v, %v; want error containing %q", test.data, got, err, test.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("pppoeClientDNSServers(%s): %v", test.data, err)
			}
			if got == nil || !slices.Equal(got, test.want) {
				t.Fatalf("pppoeClientDNSServers(%s) = %#v, want %#v", test.data, got, test.want)
			}
		})
	}
}

func TestZeClientShowDNSJudgesTheDaemonAnswer(t *testing.T) {
	// VALIDATES: The checker passes only when Ze's REST answer for
	// `show interface pppoe-client` lists exactly the wanted servers, in
	// order: the two learned ones for 01-pppoe-chap-ipv4, an empty list for
	// pppoe-ipcp-dns-rejected-ze-client.
	// PREVENTS: AC-11's populated half passing on a show answer that drops,
	// reorders or nulls the learned servers, or the empty half passing on an
	// absent key.
	learned := []string{"192.0.2.53", "192.0.2.54"}
	tests := []struct {
		name  string
		reply string
		want  []string
		pass  bool
	}{
		{"learned pass", zeShowPPPoEClientLearned, learned, true},
		{"learned swapped", showPPPoEClientReply(`["192.0.2.54","192.0.2.53"]`), learned, false},
		{"learned secondary missing", showPPPoEClientReply(`["192.0.2.53"]`), learned, false},
		{"learned empty", showPPPoEClientReply(`[]`), learned, false},
		{"learned key absent", showPPPoEClientReply(""), learned, false},
		{"rejected empty pass", showPPPoEClientReply(`[]`), []string{}, true},
		{"rejected key absent", showPPPoEClientReply(""), []string{}, false},
		{"rejected null", showPPPoEClientReply(`null`), []string{}, false},
		{"rejected server listed", showPPPoEClientReply(`["192.0.2.53"]`), []string{}, false},
		{"error reply", `{"status":"error","error":"PPPoE client status unavailable"}`, learned, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lab := &fakeLab{}
			lab.queryFn = func(peer string, argv []string) (string, error) {
				if !isShowPPPoEClientQuery(peer, argv) {
					return "", errors.New("unexpected query: " + peer + " " + strings.Join(argv, " "))
				}
				return test.reply, nil
			}
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			err := checkZeClientShowDNS(ctx, &interoplab.CheckContext{Lab: lab}, test.want)
			if test.pass {
				if err != nil {
					t.Fatalf("reply %s refused, want accepted: %v", test.reply, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("reply %s accepted, want dns-servers %v refused", test.reply, test.want)
			}
			if !strings.Contains(err.Error(), "show interface pppoe-client: pppoe0 dns-servers") {
				t.Fatalf("error = %v, want the dns-servers refusal", err)
			}
		})
	}
}

func TestZeClientDNSRejectedStopsOnAPopulatedShowAnswer(t *testing.T) {
	// VALIDATES: checkZeClientDNSRejected reads `show interface pppoe-client`
	// after resolv.conf (which lists exactly the host's server) and refuses a
	// listed server there before the session proof stops Ze.
	// PREVENTS: The DNS-rejected scenario never asking the daemon, so its
	// empty dns-servers list stayed unmeasured.
	lab := &fakeLab{}
	lab.execFn = func(peer string, argv []string) (interoplab.CommandResult, error) {
		command := strings.Join(argv, " ")
		switch command {
		case "ip -o link show type ppp":
			return interoplab.CommandResult{Stdout: "12: ppp0: <POINTOPOINT> mtu 1492\n"}, nil
		case "ip -o addr show dev ppp0":
			return interoplab.CommandResult{Stdout: "12: ppp0 inet 10.11.0.2 peer 10.11.0.1/32\n"}, nil
		case strings.Join(zeHostResolvConfArgv, " "):
			return interoplab.CommandResult{Stdout: zeDockerHostResolvConf}, nil
		case strings.Join(zeResolvConfArgv, " "):
			return interoplab.CommandResult{Stdout: zeResolvConfPresent + zeHostOnlyResolvConf}, nil
		default:
			return interoplab.CommandResult{}, errors.New("unexpected command: " + peer + " " + command)
		}
	}
	showAsked := false
	lab.queryFn = func(peer string, argv []string) (string, error) {
		if !isShowPPPoEClientQuery(peer, argv) {
			return "", errors.New("unexpected query: " + peer + " " + strings.Join(argv, " "))
		}
		showAsked = true
		return zeShowPPPoEClientLearned, nil
	}
	lab.logsFn = func(string, int) (interoplab.LogResult, error) {
		return interoplab.LogResult{Text: accelCHAPTrace, Available: true}, nil
	}
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
	defer cancel()

	err := checkZeClientDNSRejected(ctx, &interoplab.CheckContext{Lab: lab})
	if err == nil {
		t.Fatal("populated dns-servers accepted for the DNS-rejected scenario")
	}
	if !showAsked {
		t.Fatal("checker never asked show interface pppoe-client")
	}
	if lab.stopPeer != "" {
		t.Fatalf("checker stopped %q after refusing the show answer, want no session proof", lab.stopPeer)
	}
}

// zeDockerHostResolvConf is the Ze container's /etc/resolv.conf as Docker
// writes it on a user-defined network: its embedded resolver, then options.
const zeDockerHostResolvConf = "# Generated by Docker Engine.\n" +
	"nameserver 127.0.0.11\n" +
	"options ndots:0\n"

// zeHostOnlyResolvConf is Ze's resolv.conf when no lease lists a server and
// the host fallback lists zeDockerHostResolvConf's one server.
const zeHostOnlyResolvConf = "# Generated by ze\nnameserver 127.0.0.11\n"

func TestZeClientDNSRejectedWantsExactlyTheHostServers(t *testing.T) {
	// VALIDATES: checkZeClientDNSRejected passes resolv.conf only when it is
	// the generated header plus exactly the Ze container's /etc/resolv.conf
	// servers, in order, and refuses a host list holding the concentrator's
	// address before reading Ze's file.
	// PREVENTS: The DNS-rejected scenario accepting a learned server beside
	// the host's, losing one of the host's servers, or passing on a host list
	// whose server collides with what the concentrator could have offered.
	// Method: a fake lab answers the host file and Ze's file per case; an
	// accepted resolv.conf reaches the show query, a refused one does not.
	tests := []struct {
		name      string
		host      string
		ze        interoplab.CommandResult
		wantError string
		wantShow  bool
	}{
		{
			name:     "host server only",
			host:     zeDockerHostResolvConf,
			ze:       interoplab.CommandResult{Stdout: zeResolvConfPresent + zeHostOnlyResolvConf},
			wantShow: true,
		},
		{
			name:     "two host servers in order",
			host:     "nameserver 192.0.2.1\n# comment\nnameserver 192.0.2.2\n",
			ze:       interoplab.CommandResult{Stdout: zeResolvConfPresent + "# Generated by ze\nnameserver 192.0.2.1\nnameserver 192.0.2.2\n"},
			wantShow: true,
		},
		{
			name:     "no host server and no file",
			host:     "options ndots:0\n",
			ze:       interoplab.CommandResult{Stdout: zeResolvConfAbsent},
			wantShow: true,
		},
		{
			name:      "extra server beyond the host's",
			host:      zeDockerHostResolvConf,
			ze:        interoplab.CommandResult{Stdout: zeResolvConfPresent + zeHostOnlyResolvConf + "nameserver 192.0.2.53\n"},
			wantError: "ze resolv.conf " + zeResolvConf,
		},
		{
			name:      "host list holds the concentrator",
			host:      "nameserver 127.0.0.11\nnameserver " + accelGateway + "\n",
			ze:        interoplab.CommandResult{Stdout: zeResolvConfPresent + "# Generated by ze\nnameserver 127.0.0.11\nnameserver " + accelGateway + "\n"},
			wantError: accelGateway + ", the concentrator's address",
		},
		{
			name:      "host server missing",
			host:      "nameserver 127.0.0.11\nnameserver 192.0.2.1\n",
			ze:        interoplab.CommandResult{Stdout: zeResolvConfPresent + zeHostOnlyResolvConf},
			wantError: "ze resolv.conf " + zeResolvConf,
		},
		{
			name:      "host servers swapped",
			host:      "nameserver 192.0.2.1\nnameserver 192.0.2.2\n",
			ze:        interoplab.CommandResult{Stdout: zeResolvConfPresent + "# Generated by ze\nnameserver 192.0.2.2\nnameserver 192.0.2.1\n"},
			wantError: "ze resolv.conf " + zeResolvConf,
		},
		{
			name:      "file absent while the host lists a server",
			host:      zeDockerHostResolvConf,
			ze:        interoplab.CommandResult{Stdout: zeResolvConfAbsent},
			wantError: "ze resolv.conf " + zeResolvConf,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			lab := &fakeLab{}
			zeAsked := false
			lab.execFn = func(peer string, argv []string) (interoplab.CommandResult, error) {
				command := strings.Join(argv, " ")
				switch command {
				case "ip -o link show type ppp":
					return interoplab.CommandResult{Stdout: "12: ppp0: <POINTOPOINT> mtu 1492\n"}, nil
				case "ip -o addr show dev ppp0":
					return interoplab.CommandResult{Stdout: "12: ppp0 inet 10.11.0.2 peer 10.11.0.1/32\n"}, nil
				case strings.Join(zeHostResolvConfArgv, " "):
					if peer != zeImageName {
						return interoplab.CommandResult{}, errors.New("host resolv.conf read in " + peer + ", want " + zeImageName)
					}
					return interoplab.CommandResult{Stdout: test.host}, nil
				case strings.Join(zeResolvConfArgv, " "):
					zeAsked = true
					return test.ze, nil
				default:
					return interoplab.CommandResult{}, errors.New("unexpected command: " + peer + " " + command)
				}
			}
			showAsked := false
			lab.queryFn = func(peer string, argv []string) (string, error) {
				if !isShowPPPoEClientQuery(peer, argv) {
					return "", errors.New("unexpected query: " + peer + " " + strings.Join(argv, " "))
				}
				showAsked = true
				// A populated answer stops the checker right after the show
				// read, so an accepted resolv.conf is seen without the
				// session proof.
				return zeShowPPPoEClientLearned, nil
			}
			lab.logsFn = func(string, int) (interoplab.LogResult, error) {
				return interoplab.LogResult{Text: accelCHAPTrace, Available: true}, nil
			}
			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)
			defer cancel()

			err := checkZeClientDNSRejected(ctx, &interoplab.CheckContext{Lab: lab})
			if err == nil {
				t.Fatal("checker passed on a populated show answer")
			}
			if showAsked != test.wantShow {
				t.Fatalf("show asked = %v, want %v (err = %v)", showAsked, test.wantShow, err)
			}
			if test.wantError != "" && !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("error = %v, want it to contain %q", err, test.wantError)
			}
			if test.wantError == accelGateway+", the concentrator's address" && zeAsked {
				t.Fatal("checker read Ze's resolv.conf after the host list held the concentrator")
			}
			if lab.stopPeer != "" {
				t.Fatalf("checker stopped %q, want no session proof", lab.stopPeer)
			}
		})
	}
}
