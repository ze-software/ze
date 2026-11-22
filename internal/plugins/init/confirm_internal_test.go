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
	"strconv"
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
	if got := strings.Count(asked, ": write it into the config? yes/no: "); got != 4 {
		t.Errorf("asked %d questions, want 4 (every device but the tunnel):\n%s", got, asked)
	}
	if strings.Contains(asked, "gre1") {
		t.Errorf("asked about the tunnel, which the config does not hold:\n%s", asked)
	}
	if want := `veth veth1a2b (mac 0e:00:00:00:00:02, member of docker0, alias "container web"): write it`; !strings.Contains(asked, want) {
		t.Errorf("the veth question does not show its owner hints, want %q in:\n%s", want, asked)
	}
	if want := "yes/no: loopback lo: write it"; !strings.Contains(asked, want) {
		t.Errorf("the loopback question is not %q:\n%s", want, asked)
	}
	if want := "discovered 5 interface(s), wrote the 2 confirmed into the initial config with the SSH listener on 127.0.0.1:2222\n"; !strings.Contains(out, want) {
		t.Errorf("status line is not %q:\n%s", want, out)
	}
}

// VALIDATES: every device answered no or n writes no interface block at all.
func TestZeInitConfirmEveryNoWritesNoInterface(t *testing.T) {
	fixedDiscovery(t, hostDevices(), nil)
	dir := t.TempDir()
	code, out := runConfirm(t, strings.NewReader(credentials+"no\nn\nN\nNO\n"), nil, dir)
	if code != 0 {
		t.Fatalf("ze init --confirm exit = %d, want 0", code)
	}
	if interfaces := writtenInterfaces(t, dir); interfaces != "" {
		t.Fatalf("no answers wrote interfaces:\n%s", interfaces)
	}
	if want := "discovered 5 interface(s), wrote the 0 confirmed"; !strings.Contains(out, want) {
		t.Errorf("status line is not %q:\n%s", want, out)
	}
}

// eth0Question is how ze init --confirm names hostDevices' ethernet.
const eth0Question = "ethernet eth0 (mac 52:54:00:00:00:01)"

// VALIDATES: an empty line, and any line that is not yes, y, no or n, decides
// nothing: ze init writes one notice per such line naming the accepted
// answers, asks about the same device again, and takes the next explicit
// answer, on a pipe and on a terminal alike. The notice goes to stderr, never
// to the stdout a script reads.
// PREVENTS: an empty line read as no, or an unclear answer decided for the
// operator; and a re-asked question that moves on to the next device.
func TestZeInitConfirmAsksAgainUntilExplicit(t *testing.T) {
	tests := []struct {
		name       string
		unaccepted []string
		final      string
		written    bool
	}{
		{"empty line then yes", []string{""}, "yes", true},
		{"empty line then n", []string{""}, "n", false},
		{"unclear lines then yes", []string{"", "maybe", "yess"}, "yes", true},
		{"unclear lines then n", []string{"", "maybe", "yess"}, "n", false},
	}
	for _, tt := range tests {
		for _, terminal := range []bool{false, true} {
			name := tt.name + " piped"
			if terminal {
				name = tt.name + " terminal"
			}
			t.Run(name, func(t *testing.T) {
				fixedDiscovery(t, hostDevices(), nil)
				dir := t.TempDir()
				// docker0 no, then eth0 the unaccepted lines and the final
				// answer, then lo and veth1a2b no.
				answers := "no\n" + strings.Join(tt.unaccepted, "\n") + "\n" + tt.final + "\nno\nno\n"
				var prompts bytes.Buffer
				var promptW io.Writer
				if terminal {
					promptW = &prompts
				}
				var code int
				var out string
				errOut := captureFile(t, &os.Stderr, func() {
					code, out = runConfirm(t, strings.NewReader(credentials+answers), promptW, dir)
				})
				if code != 0 {
					t.Fatalf("ze init --confirm exit = %d, want 0; stderr:\n%s", code, errOut)
				}
				interfaces := writtenInterfaces(t, dir)
				if got := strings.Contains(interfaces, "    ethernet eth0 {\n"); got != tt.written {
					t.Errorf("eth0 written = %v after the final answer %q, want %v:\n%s", got, tt.final, tt.written, interfaces)
				}
				for _, device := range []string{"docker0", "loopback", "veth1a2b"} {
					if strings.Contains(interfaces, device) {
						t.Errorf("%s was answered no but is in the config:\n%s", device, interfaces)
					}
				}
				if got := strings.Count(errOut, " is not an answer for "+eth0Question+": type yes or y to write it, or no or n to leave it out\n"); got != len(tt.unaccepted) {
					t.Errorf("wrote %d notices for eth0, want one per unaccepted line (%d):\n%s", got, len(tt.unaccepted), errOut)
				}
				if got := strings.Count(errOut, "an empty line is not an answer for "+eth0Question); got != 1 {
					t.Errorf("the empty line is noted %d times, want 1:\n%s", got, errOut)
				}
				for _, line := range tt.unaccepted[1:] {
					if want := strconv.Quote(line) + " is not an answer for " + eth0Question; !strings.Contains(errOut, want) {
						t.Errorf("stderr does not name the unaccepted %q, want %q:\n%s", line, want, errOut)
					}
				}
				if got := strings.Count(errOut, " is not an answer for "); got != len(tt.unaccepted) {
					t.Errorf("wrote %d notices in all, want %d, none for another device:\n%s", got, len(tt.unaccepted), errOut)
				}
				if strings.Contains(out, "is not an answer") {
					t.Errorf("a notice reached stdout, which a script reads:\n%s", out)
				}
				if !terminal {
					return
				}
				asked := prompts.String()
				if got := strings.Count(asked, eth0Question+": write it into the config? yes/no: "); got != len(tt.unaccepted)+1 {
					t.Errorf("asked about eth0 %d times, want once plus once per unaccepted line (%d):\n%s", got, len(tt.unaccepted)+1, asked)
				}
				if got := strings.Count(asked, ": write it into the config? yes/no: "); got != len(tt.unaccepted)+4 {
					t.Errorf("asked %d questions in all, want %d:\n%s", got, len(tt.unaccepted)+4, asked)
				}
			})
		}
	}
}

// VALIDATES: input that ends after only unaccepted lines refuses the whole
// init before a store exists, naming the device still unanswered, after one
// notice per unaccepted line.
// PREVENTS: the loop spinning on the end of input, or reading the end of
// input as an empty line and deciding the device.
func TestZeInitConfirmRefusesInputEndingUnanswered(t *testing.T) {
	fixedDiscovery(t, hostDevices(), nil)
	dir := t.TempDir()
	var code int
	errOut := captureFile(t, &os.Stderr, func() {
		code, _ = runConfirm(t, strings.NewReader(credentials+"no\n\nmaybe\nyess\n"), nil, dir)
	})
	if code == 0 {
		t.Fatal("ze init --confirm accepted input that ended with eth0 unanswered")
	}
	if want := "got no answer for " + eth0Question; !strings.Contains(errOut, want) {
		t.Errorf("stderr does not say %q:\n%s", want, errOut)
	}
	if !strings.Contains(errOut, "\nfix: ze init --confirm") {
		t.Errorf("stderr does not end with the command that asks again:\n%s", errOut)
	}
	if got := strings.Count(errOut, " is not an answer for "+eth0Question); got != 3 {
		t.Errorf("wrote %d notices, want 3, one per unaccepted line:\n%s", got, errOut)
	}
	assertNoStore(t, dir)
}

// VALIDATES: readAnswer takes yes, y, Y, no, n and N each as an explicit
// answer with no notice; reads past every unaccepted line, an empty one
// included, writing one notice each and asking again; and refuses when the
// input ends, rather than reading the end as an empty line.
func TestReadAnswerAsksAgain(t *testing.T) {
	for _, tt := range []struct {
		line string
		yes  bool
	}{{"yes", true}, {"y", true}, {"Y", true}, {"no", false}, {"n", false}, {"N", false}} {
		var notices bytes.Buffer
		yes, err := readAnswer(bufio.NewScanner(strings.NewReader(tt.line+"\n")), nil, &notices, "loopback lo")
		if err != nil {
			t.Fatalf("readAnswer(%q) refused: %v", tt.line, err)
		}
		if yes != tt.yes {
			t.Errorf("readAnswer(%q) = %v, want %v", tt.line, yes, tt.yes)
		}
		if notices.Len() != 0 {
			t.Errorf("readAnswer(%q) wrote a notice for an accepted answer: %q", tt.line, notices.String())
		}
	}

	var prompts, notices bytes.Buffer
	yes, err := readAnswer(bufio.NewScanner(strings.NewReader("\nok\nY\nno\n")), &prompts, &notices, "loopback lo")
	if err != nil {
		t.Fatalf("readAnswer refused an explicit answer after unaccepted lines: %v", err)
	}
	if !yes {
		t.Error("readAnswer did not take the Y after the unaccepted lines")
	}
	if want := "an empty line is not an answer for loopback lo: type yes or y to write it, or no or n to leave it out\n" +
		`"ok" is not an answer for loopback lo: type yes or y to write it, or no or n to leave it out` + "\n"; notices.String() != want {
		t.Errorf("notices are\n%q\nwant\n%q", notices.String(), want)
	}
	if got := strings.Count(prompts.String(), "loopback lo: write it into the config? yes/no: "); got != 3 {
		t.Errorf("asked %d times, want 3 (once, then once per unaccepted line):\n%s", got, prompts.String())
	}

	for _, input := range []string{"", "\n", "ok\n", "\n\n\n"} {
		notices.Reset()
		_, err := readAnswer(bufio.NewScanner(strings.NewReader(input)), nil, &notices, "loopback lo")
		if err == nil {
			t.Fatalf("readAnswer(%q) decided an answer from input that ended unanswered", input)
		}
		if !strings.Contains(err.Error(), "got no answer for loopback lo") {
			t.Errorf("readAnswer(%q) refused with %v, want the end of input named", input, err)
		}
		if got, want := strings.Count(notices.String(), " is not an answer for "), strings.Count(input, "\n"); got != want {
			t.Errorf("readAnswer(%q) wrote %d notices, want %d", input, got, want)
		}
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

// VALIDATES: input that ends before every device is answered, after an
// explicit answer or after a line that is not accepted, is refused before
// a store exists, naming the device still unanswered.
// PREVENTS: a device decided for the operator by a guess.
func TestZeInitConfirmRefusesUnclearAnswers(t *testing.T) {
	tests := []struct {
		name    string
		answers string
		want    string
	}{
		{"input ends after an unaccepted line", "no\nmaybe\n", "got no answer for ethernet eth0 (mac 52:54:00:00:00:01)"},
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

// VALIDATES: readAnswer accepts yes, y, no and n in any case, and an
// answer that is not one of them is not accepted: it writes a notice
// naming the accepted answers and reads on, so input that ends there is
// refused.
func TestReadAnswer(t *testing.T) {
	for _, tt := range []struct {
		line string
		yes  bool
	}{{"yes", true}, {"Y", true}, {" YES ", true}, {"y", true}, {"no", false}, {"N", false}, {"n", false}} {
		yes, err := readAnswer(bufio.NewScanner(strings.NewReader(tt.line+"\n")), nil, io.Discard, "loopback lo")
		if err != nil {
			t.Fatalf("readAnswer(%q) refused: %v", tt.line, err)
		}
		if yes != tt.yes {
			t.Errorf("readAnswer(%q) = %v, want %v", tt.line, yes, tt.yes)
		}
	}
	var notices bytes.Buffer
	_, err := readAnswer(bufio.NewScanner(strings.NewReader("ok\n")), nil, &notices, "loopback lo")
	if err == nil {
		t.Fatal(`readAnswer("ok") accepted it`)
	}
	if !strings.Contains(notices.String(), "type yes or y to write it, or no or n to leave it out") {
		t.Errorf("the notice does not name the accepted answers: %q", notices.String())
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
