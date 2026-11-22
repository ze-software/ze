// Design: docs/features/interfaces.md -- ze init claims every discovered device, and asks per device with --confirm

package init

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/iface"
)

// fixedDiscovery makes init discover set in place of this host's devices,
// until the test ends.
func fixedDiscovery(t *testing.T, set []iface.DiscoveredInterface, err error) {
	t.Helper()
	saved := discoverHostInterfaces
	discoverHostInterfaces = func() ([]iface.DiscoveredInterface, error) {
		if err != nil {
			return nil, err
		}
		return slices.Clone(set), nil
	}
	t.Cleanup(func() { discoverHostInterfaces = saved })
}

// hostDevices is a discovered set in discovery order (kind, then name): a
// bridge and a container veth another program owns, the NIC and loopback an
// operator gives ze, and a tunnel, a kind the interface config does not hold.
func hostDevices() []iface.DiscoveredInterface {
	return []iface.DiscoveredInterface{
		{Name: "docker0", Type: iface.TypeBridge, MAC: "02:42:ac:11:00:01"},
		{Name: "eth0", Type: "ethernet", MAC: "52:54:00:00:00:01", PermanentMAC: "52:54:00:00:00:01"},
		{Name: "lo", Type: "loopback"},
		{Name: "gre1", Type: "tunnel"},
		{Name: "veth1a2b", Type: iface.TypeVeth, MAC: "0e:00:00:00:00:02", Master: "docker0", Alias: "container web"},
	}
}

// credentials are the five answers before the interface questions: the
// listener stays on its defaults.
const credentials = "admin\nsecret123\n\n\nedge\n"

// runConfirm runs ze init --confirm on r, writing the prompts to w (nil for a
// piped stdin), and answers the exit code and what init printed on stdout.
func runConfirm(t *testing.T, r io.Reader, w io.Writer, dir string) (int, string) {
	t.Helper()
	var code int
	out := captureFile(t, &os.Stdout, func() {
		code = runInitStore(r, w, dir, managedInit{}, "", "", false, false, initDiscoverConfirmed)
	})
	return code, out
}

// captureFile answers what run wrote to *target, which it replaces with a
// pipe while run runs.
func captureFile(t *testing.T, target **os.File, run func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := *target
	*target = writer
	run()
	*target = saved
	writer.Close() //nolint:errcheck // pipe writer
	out, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	return string(out)
}

// writtenInterfaces answers what the initial config holds after the default
// SSH listener.
func writtenInterfaces(t *testing.T, dir string) string {
	t.Helper()
	store := openInitStore(t, dir)
	data, err := store.ReadKey(initialConfigKey(t, store))
	if err != nil {
		t.Fatalf("ze init wrote no initial config: %v", err)
	}
	interfaces, found := strings.CutPrefix(string(data), sshListenerBlock(defaultHost, defaultPort))
	if !found {
		t.Fatalf("the initial config does not open with the default SSH listener:\n%s", data)
	}
	return interfaces
}

// assertNoStore fails when a refused init left a store behind.
func assertNoStore(t *testing.T, dir string) {
	t.Helper()
	if _, err := os.Stat(filepath.Join(dir, "database")); !os.IsNotExist(err) {
		t.Fatalf("a refused ze init left a store behind: %v", err)
	}
}

// VALIDATES: without --confirm, ze init writes every device discovery finds,
// the ones another program owns included, and asks nothing.
// PREVENTS: the --confirm work changing the default, which the owner kept.
func TestZeInitDefaultWritesEveryDiscoveredDevice(t *testing.T) {
	fixedDiscovery(t, hostDevices(), nil)
	dir := t.TempDir()
	if code := runWithReader(strings.NewReader(credentials), dir, false); code != 0 {
		t.Fatalf("ze init exit = %d, want 0", code)
	}
	interfaces := writtenInterfaces(t, dir)
	if interfaces != iface.EmitConfig(hostDevices()) {
		t.Fatalf("the default config is not every discovered device:\n%s", interfaces)
	}
	for _, entry := range []string{"    bridge docker0 {\n", "    ethernet eth0 {\n", "    loopback {\n", "    veth veth1a2b {\n"} {
		if !strings.Contains(interfaces, entry) {
			t.Errorf("the default config lacks %q:\n%s", entry, interfaces)
		}
	}
}

// VALIDATES: ze init --confirm asks about each device the config can hold,
// showing its kind, name and owner hints, and writes only the ones answered
// yes; the status line counts what was discovered and what was written.
// PREVENTS: a refused device reaching the config, or a question about a kind
// init would never write.
func TestZeInitConfirmWritesOnlyConfirmed(t *testing.T) {
	fixedDiscovery(t, hostDevices(), nil)
	dir := t.TempDir()
	var prompts bytes.Buffer
	// docker0 no, eth0 yes, lo Y, veth1a2b NO; gre1 is not asked.
	code, out := runConfirm(t, strings.NewReader(credentials+"no\nyes\nY\nNO\n"), &prompts, dir)
	if code != 0 {
		t.Fatalf("ze init --confirm exit = %d, want 0", code)
	}
	interfaces := writtenInterfaces(t, dir)
	for _, entry := range []string{"    ethernet eth0 {\n", "    loopback {\n"} {
		if !strings.Contains(interfaces, entry) {
			t.Errorf("the confirmed %q is missing:\n%s", entry, interfaces)
		}
	}
	for _, name := range []string{"docker0", "veth1a2b", "gre1"} {
		if strings.Contains(interfaces, name) {
			t.Errorf("%s was not confirmed but is in the config:\n%s", name, interfaces)
		}
	}
	asked := prompts.String()
	if got := strings.Count(asked, ": write it into the config? yes/no [no]: "); got != 4 {
		t.Errorf("asked %d questions, want 4 (every device but the tunnel):\n%s", got, asked)
	}
	if strings.Contains(asked, "gre1") {
		t.Errorf("asked about the tunnel, which the config does not hold:\n%s", asked)
	}
	if want := `veth veth1a2b (mac 0e:00:00:00:00:02, member of docker0, alias "container web"): write it`; !strings.Contains(asked, want) {
		t.Errorf("the veth question does not show its owner hints, want %q in:\n%s", want, asked)
	}
	if want := "[no]: loopback lo: write it"; !strings.Contains(asked, want) {
		t.Errorf("the loopback question is not %q:\n%s", want, asked)
	}
	if want := "discovered 5 interface(s), wrote the 2 confirmed into the initial config with the SSH listener on 127.0.0.1:2222\n"; !strings.Contains(out, want) {
		t.Errorf("status line is not %q:\n%s", want, out)
	}
}

// VALIDATES: an empty answer leaves the device out, as the prompt's [no]
// says, so every device refused writes no interface block at all.
func TestZeInitConfirmEmptyAnswerIsNo(t *testing.T) {
	fixedDiscovery(t, hostDevices(), nil)
	dir := t.TempDir()
	code, out := runConfirm(t, strings.NewReader(credentials+"\n\n\n\n"), nil, dir)
	if code != 0 {
		t.Fatalf("ze init --confirm exit = %d, want 0", code)
	}
	if interfaces := writtenInterfaces(t, dir); interfaces != "" {
		t.Fatalf("empty answers wrote interfaces:\n%s", interfaces)
	}
	if want := "discovered 5 interface(s), wrote the 0 confirmed"; !strings.Contains(out, want) {
		t.Errorf("status line is not %q:\n%s", want, out)
	}
}

// VALIDATES: a kept NIC whose factory address another NIC also reports binds
// by os-name, because uniqueness is judged over every discovered device.
// PREVENTS: filtering before emission, which made the shared address look
// unique and wrote a `mac match` naming two NICs: the commit refuses it, and
// the first start loses the whole config.
func TestZeInitConfirmKeepsSharedAddressNotUnique(t *testing.T) {
	shared := "52:54:00:00:00:09"
	fixedDiscovery(t, []iface.DiscoveredInterface{
		{Name: "eth0", Type: "ethernet", MAC: shared, PermanentMAC: shared},
		{Name: "eth1", Type: "ethernet", MAC: shared, PermanentMAC: shared},
	}, nil)
	dir := t.TempDir()
	if code, _ := runConfirm(t, strings.NewReader(credentials+"y\nn\n"), nil, dir); code != 0 {
		t.Fatalf("ze init --confirm exit = %d, want 0", code)
	}
	interfaces := writtenInterfaces(t, dir)
	if !strings.Contains(interfaces, "        os-name eth0;\n") {
		t.Errorf("the kept eth0 does not bind by os-name:\n%s", interfaces)
	}
	if strings.Contains(interfaces, "match") {
		t.Errorf("the kept eth0 binds by an address eth1 shares:\n%s", interfaces)
	}
}

// VALIDATES: an answer that is neither yes nor no, and input that ends before
// every device is answered, are refused before a store exists, naming the
// device and the accepted answers.
// PREVENTS: a device decided for the operator by a guess.
func TestZeInitConfirmRefusesUnclearAnswers(t *testing.T) {
	tests := []struct {
		name    string
		answers string
		want    string
	}{
		{"not yes or no", "no\nmaybe\n", `the answer "maybe" for ethernet eth0 (mac 52:54:00:00:00:01) is not yes or no`},
		{"input ends", "no\nyes\n", "got no answer for loopback lo"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixedDiscovery(t, hostDevices(), nil)
			dir := t.TempDir()
			var code int
			errOut := captureFile(t, &os.Stderr, func() {
				code, _ = runConfirm(t, strings.NewReader(credentials+tt.answers), nil, dir)
			})
			if code == 0 {
				t.Fatalf("ze init --confirm accepted the answers %q", tt.answers)
			}
			if !strings.Contains(errOut, tt.want) {
				t.Errorf("stderr does not say %q:\n%s", tt.want, errOut)
			}
			if !strings.Contains(errOut, "\nfix: ze init --confirm") {
				t.Errorf("stderr does not end with the command that asks again:\n%s", errOut)
			}
			assertNoStore(t, dir)
		})
	}
}

// VALIDATES: readAnswer accepts yes, y, no, n and an empty line in any case,
// and names the accepted answers when it refuses another.
func TestReadAnswer(t *testing.T) {
	for _, tt := range []struct {
		line string
		yes  bool
	}{{"yes", true}, {"Y", true}, {" YES ", true}, {"no", false}, {"N", false}, {"", false}} {
		yes, err := readAnswer(bufio.NewScanner(strings.NewReader(tt.line+"\n")), "loopback lo")
		if err != nil {
			t.Fatalf("readAnswer(%q) refused: %v", tt.line, err)
		}
		if yes != tt.yes {
			t.Errorf("readAnswer(%q) = %v, want %v", tt.line, yes, tt.yes)
		}
	}
	_, err := readAnswer(bufio.NewScanner(strings.NewReader("ok\n")), "loopback lo")
	if err == nil {
		t.Fatal(`readAnswer("ok") accepted it`)
	}
	if !strings.Contains(err.Error(), "yes, y, no, n, or an empty line for no") {
		t.Errorf("the refusal does not name the accepted answers: %v", err)
	}
}

// VALIDATES: when discovery fails, --confirm asks nothing, warns as the
// default does, and still writes the listener.
func TestZeInitConfirmDiscoveryFailure(t *testing.T) {
	fixedDiscovery(t, nil, errors.New("netlink: permission denied"))
	dir := t.TempDir()
	var prompts bytes.Buffer
	if code, _ := runConfirm(t, strings.NewReader(credentials), &prompts, dir); code != 0 {
		t.Fatalf("ze init --confirm exit = %d, want 0", code)
	}
	if strings.Contains(prompts.String(), "write it into the config") {
		t.Errorf("asked about a device although discovery failed:\n%s", prompts.String())
	}
	if interfaces := writtenInterfaces(t, dir); interfaces != "" {
		t.Errorf("a failed discovery wrote interfaces:\n%s", interfaces)
	}
}

// VALIDATES: --confirm with --seed, --managed or --from, which write no
// discovered interface, is refused by name with the command to run instead,
// before any input is read.
// PREVENTS: the flag silently ignored, so the operator believes each device
// was confirmed.
func TestZeInitConfirmRefusesProfilesWithoutDiscovery(t *testing.T) {
	tests := []struct {
		args []string
		want string
		fix  string
	}{
		{[]string{"--confirm", "--seed"}, "--confirm was given with --seed", "\nfix: ze init --seed"},
		{[]string{"--confirm", "--managed"}, "--confirm was given with --managed", "\nfix: ze init --managed"},
		{[]string{"--confirm", "--from", "/nonexistent/database.zefs"}, "--confirm was given with --from", "\nfix: ze init --from /nonexistent/database.zefs"},
	}
	for _, tt := range tests {
		t.Run(strings.Join(tt.args, " "), func(t *testing.T) {
			var code int
			errOut := captureFile(t, &os.Stderr, func() { code = Run(tt.args) })
			if code != 1 {
				t.Fatalf("Run(%v) = %d, want 1", tt.args, code)
			}
			if !strings.Contains(errOut, tt.want) {
				t.Errorf("stderr does not refuse by name, want %q:\n%s", tt.want, errOut)
			}
			if !strings.Contains(errOut, tt.fix) {
				t.Errorf("stderr does not name the command to run instead, want %q:\n%s", tt.fix, errOut)
			}
		})
	}
}
