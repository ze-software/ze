package interoplab

import (
	"net/netip"
	"slices"
	"strings"
	"testing"
)

// VALIDATES: ZeCLICommandAs seeds and reads the login's own store, passes
// its user, password, host and port as single shell words, and ZeCLICommand is the
// fixture account through the same builder.
// Method: compare the built argv's shell text.
// PREVENTS: a second account logging in with the first's seeded store, and
// a password with a quote splitting the shell command.
func TestZeCLICommandAsNamesItsLogin(t *testing.T) {
	login := ZeCLILogin{User: "admin", Password: "it's", Host: netip.MustParseAddr("192.168.50.1"), Port: "22", Store: "/tmp/ze-cli-store-admin"}
	argv := ZeCLICommandAs(login, "show interface pppoe-client | json")
	if len(argv) != 3 || argv[0] != "sh" || argv[1] != "-c" {
		t.Fatalf("argv = %q, want sh -c <shell>", argv)
	}
	shell := argv[2]
	for _, want := range []string{
		"[ -d /tmp/ze-cli-store-admin ] || ",
		"'admin' 'it'\\''s' '192.168.50.1' '22' | ZE_CONFIG_DIR=/tmp/ze-cli-store-admin ze init",
		"ZE_CONFIG_DIR=/tmp/ze-cli-store-admin ZE_SSH_PASSWORD='it'\\''s' ze cli -c 'show interface pppoe-client | json'",
	} {
		if !strings.Contains(shell, want) {
			t.Errorf("shell %q lacks %q", shell, want)
		}
	}
	if strings.Contains(shell, ZeCLIStore+" ") {
		t.Errorf("shell %q names the fixture account's store", shell)
	}
	fixture := ZeCLILogin{User: ZeCLIUser, Password: zeCLIPassword, Host: ZeCLILoopback(), Port: ZeCLIPort, Store: ZeCLIStore}
	if !slices.Equal(ZeCLICommand("show version"), ZeCLICommandAs(fixture, "show version")) {
		t.Error("ZeCLICommand is not the fixture account through ZeCLICommandAs")
	}
}
