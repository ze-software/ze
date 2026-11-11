package topology

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/interoplab"
	"github.com/ze-software/ze/internal/test/labconfig/homecpe"
)

// TestPassThroughAdditionsOnTheConversion proves access-up's CPE is the
// conversion plus the pass-through subnet and nothing else. Method: render
// the committed ze-cpe.conf, apply passThroughAdditions, then check that
// removing the inserted text gives the rendering back, that the subnet and its
// br0 address each appear once, and that 192.168.50.0/24 stays first in both
// lists, so the subnet a new client is allocated from is unchanged.
func TestPassThroughAdditionsOnTheConversion(t *testing.T) {
	rendered, err := homecpe.Render(checkoutRoot(t), time.Now(), homecpe.KeyP256)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	added, err := addToCPEConfig(rendered.Config, passThroughAdditions)
	if err != nil {
		t.Fatalf("the pass-through additions do not apply to the conversion: %v", err)
	}
	stripped := added
	for _, addition := range passThroughAdditions {
		stripped = strings.Replace(stripped, addition.text, "", 1)
	}
	if stripped != rendered.Config {
		t.Fatal("removing the additions does not give the rendered conversion back: they changed more than they insert")
	}
	subnet := "subnet " + passThroughSubnetIP + " {"
	if count := strings.Count(added, subnet); count != 1 {
		t.Fatalf("the CPE holds %q %d times, want once", subnet, count)
	}
	address := passThroughGateway + "/24 "
	if count := strings.Count(added, address); count != 1 {
		t.Fatalf("the CPE holds br0 address %q %d times, want once", address, count)
	}
	if strings.Contains(added, "dns-server "+passThroughGateway) {
		t.Fatal("the pass-through subnet names a dns-server, so the default never reaches received")
	}
	first := strings.Index(added, "subnet 192.168.50.0/24 {")
	ours := strings.Index(added, subnet)
	if first < 0 || first > ours {
		t.Fatalf("192.168.50.0/24 is not the first subnet (at %d, pass-through at %d)", first, ours)
	}
	if !strings.Contains(added, "[ "+cpeLANGateway+"/24 "+passThroughGateway+"/24 ") {
		t.Fatal("br0's address list does not start with the LAN gateway followed by the pass-through gateway")
	}
}

// TestAddToCPEConfigRefusesAnAnchorNotHeldOnce proves an addition never lands
// nowhere or in a place nobody chose. Method: an anchor the text lacks, and
// one it holds twice; then one it holds once, inserted before it.
func TestAddToCPEConfigRefusesAnAnchorNotHeldOnce(t *testing.T) {
	config := "a\nb\nb\n"
	for _, anchor := range []string{"c\n", "b\n"} {
		if _, err := addToCPEConfig(config, []cpeAddition{{anchor: anchor, text: "x\n"}}); err == nil {
			t.Errorf("anchor %q accepted, want a refusal", anchor)
		}
	}
	added, err := addToCPEConfig(config, []cpeAddition{{anchor: "a\n", text: "x\n"}})
	if err != nil {
		t.Fatalf("anchor held once refused: %v", err)
	}
	if added != "x\na\nb\nb\n" {
		t.Fatalf("got %q, want the text inserted before the anchor", added)
	}
}

// TestOption6Verdict judges story 1b's clause from dhcpcd lease dumps: the
// IPCP pair in order passes; no option 6 (a default without received), the
// host fallback alone, the pair reversed and another address are refused.
// Method: dump text in dhcpcd's name=value form, parsed then judged.
func TestOption6Verdict(t *testing.T) {
	dump := func(address, servers string) string {
		text := "ip_address='" + address + "'\nsubnet_cidr='24'\nrouters='10.77.0.1'\n"
		if servers != "" {
			text += "domain_name_servers='" + servers + "'\n"
		}
		return text
	}
	want := []string{dnsPrimary, dnsSecondary}
	cases := []struct {
		name    string
		dump    string
		wantErr string
	}{
		{"pair", dump(resCAddress, dnsPrimary+" "+dnsSecondary), ""},
		{"no option 6", dump(resCAddress, ""), "carried no option 6"},
		{"fallback alone", dump(resCAddress, dnsPrimary), "option 6 carried"},
		{"reversed", dump(resCAddress, dnsSecondary+" "+dnsPrimary), "option 6 carried"},
		{"other address", dump("10.77.0.100", dnsPrimary+" "+dnsSecondary), "want its reservation"},
	}
	for _, tc := range cases {
		lease, err := parseDhcpcdLease(tc.dump)
		if err != nil {
			t.Fatalf("%s: parse: %v", tc.name, err)
		}
		err = option6Verdict(lease, resCAddress, want)
		if tc.wantErr == "" {
			if err != nil {
				t.Errorf("%s: refused: %v", tc.name, err)
			}
			continue
		}
		if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
			t.Errorf("%s: verdict %v, want one containing %q", tc.name, err, tc.wantErr)
		}
	}
	if _, err := parseDhcpcdLease(""); !errors.Is(err, errNoLeaseAddress) {
		t.Errorf("an empty dump gave %v, want errNoLeaseAddress", err)
	}
}

// TestAccessUpDeclaresThePassThroughClient proves the declaration the option
// 6 clause needs: access-up runs lan-res-c on link lan with the pass-through
// additions in its prepare step, and the other access scenarios, which share
// accessChain, run neither. Method: read the registered scenarios.
func TestAccessUpDeclaresThePassThroughClient(t *testing.T) {
	holds := func(name string) bool {
		for _, link := range scenarios[name].links {
			if link.Name == homeLANLink && slices.Contains(link.Members, nodeLANResC) {
				return true
			}
		}
		return false
	}
	if !holds(accessUpScenario) {
		t.Fatalf("access-up's link %s holds no %s", homeLANLink, nodeLANResC)
	}
	subnet := "subnet " + passThroughSubnetIP + " {"
	if !strings.Contains(preparedCPEConfig(t, accessUpScenario), subnet) {
		t.Fatalf("access-up's prepared CPE configuration holds no %q", subnet)
	}
	for _, name := range []string{accessPoolScenario, accessAccountingScenario} {
		if holds(name) {
			t.Errorf("%s runs %s, which only access-up checks", name, nodeLANResC)
		}
		if strings.Contains(preparedCPEConfig(t, name), subnet) {
			t.Errorf("%s's prepared CPE configuration holds %q, which only access-up adds", name, subnet)
		}
	}
}

// preparedCPEConfig runs scenario name's prepare step into a fresh directory
// and answers the CPE configuration it wrote.
func preparedCPEConfig(t *testing.T, name string) string {
	t.Helper()
	prepare := scenarios[name].prepare
	if prepare == nil {
		t.Fatalf("%s has no prepare step", name)
	}
	directory := t.TempDir()
	if err := prepare(checkoutRoot(t), directory); err != nil {
		t.Fatalf("prepare %s: %v", name, err)
	}
	config, err := os.ReadFile(filepath.Join(directory, filepath.FromSlash(renderedCPEConfig)))
	if err != nil {
		t.Fatalf("read %s's prepared CPE configuration: %v", name, err)
	}
	return string(config)
}

// TestOption6RebindsWhenNoDhcpcdRuns proves access-up's option 6 clause binds
// and judges a lease when lan-res-c runs no dhcpcd, so `dhcpcd -x` exits 1,
// and that a release that never ran still fails before any bind. Method:
// checkOption6PassThrough over a fixture Docker whose runner answers dhcpcd
// as the container does, so Exec builds its real error for the exit; then
// rebindLease with a lease holding the fallback alone, with a release that
// exits 127 (no dhcpcd in the image), and with one the docker client never ran.
func TestOption6RebindsWhenNoDhcpcdRuns(t *testing.T) {
	dump := func(servers string) string {
		return "ip_address='" + resCAddress + "'\ndomain_name_servers='" + servers + "'\n"
	}
	bound := []string{"-x", "-w", "--dumplease"}

	runner := &dhcpcdRunner{release: 1, dump: dump(dnsPrimary + " " + dnsSecondary)}
	if err := checkOption6PassThrough(t.Context(), option6Check(runner)); err != nil {
		t.Fatalf("a release with no daemon to stop failed the clause: %v", err)
	}
	if !slices.Equal(runner.flags, bound) {
		t.Fatalf("dhcpcd ran %v, want %v", runner.flags, bound)
	}

	runner = &dhcpcdRunner{release: 1, dump: dump(dnsPrimary)}
	lease, err := rebindLease(t.Context(), option6Check(runner), nodeLANResC, option6Device)
	if err != nil {
		t.Fatalf("a release with no daemon to stop failed the rebind: %v", err)
	}
	if err := option6Verdict(lease, resCAddress, []string{dnsPrimary, dnsSecondary}); err == nil {
		t.Fatal("a lease carrying the fallback alone passed the verdict")
	}

	for _, runner := range []*dhcpcdRunner{{release: execNotFound}, {refuse: true}} {
		if _, err := rebindLease(t.Context(), option6Check(runner), nodeLANResC, option6Device); err == nil {
			t.Errorf("a release that never ran (exit %d, refused %t) passed", runner.release, runner.refuse)
		}
		if !slices.Equal(runner.flags, []string{"-x"}) {
			t.Errorf("dhcpcd ran %v after a release that never ran, want only -x", runner.flags)
		}
	}
}

// option6Device is the device the fixture declares lan-res-c holds on the LAN.
const option6Device = "eth1"

// option6Check is the CheckContext checkOption6PassThrough reads: lan-res-c on
// the LAN link, and a lab whose Exec runs through runner.
func option6Check(runner *dhcpcdRunner) *interoplab.CheckContext {
	return &interoplab.CheckContext{
		Lab: dockerExecLab{docker: interoplab.NewFixtureDocker(runner)},
		Memberships: map[string][]interoplab.LinkMembership{
			nodeLANResC: {{Link: homeLANLink, Device: option6Device}},
		},
	}
}

// dockerExecLab serves Exec from a fixture Docker, so an exit reaches the
// checker as the error Exec builds for it; it serves no other call.
type dockerExecLab struct {
	interoplab.CheckerLab
	docker *interoplab.Docker
}

func (l dockerExecLab) Exec(ctx context.Context, peer string, argv []string, environ []interoplab.EnvironmentVariable) (interoplab.CommandResult, error) {
	return l.docker.Exec(ctx, peer, argv, environ)
}

// dhcpcdRunner answers lan-res-c's dhcpcd as its container does: the release
// exits release (1 is dhcpcd finding no daemon to stop) or, with refuse, the
// docker client fails before it runs; the bind succeeds; the dump prints dump.
// It records each dhcpcd action flag in order.
type dhcpcdRunner struct {
	release int
	refuse  bool
	dump    string
	flags   []string
}

func (r *dhcpcdRunner) Run(_ context.Context, command interoplab.ProcessCommand) (interoplab.ProcessResult, error) {
	argv := command.Arguments
	at := slices.Index(argv, commandDHCPCD)
	if at < 0 || at+2 >= len(argv) {
		return interoplab.ProcessResult{}, fmt.Errorf("the fixture runs only dhcpcd -4 <flag>, got %v", argv)
	}
	flag := argv[at+2]
	r.flags = append(r.flags, flag)
	switch flag {
	case "-x":
		if r.refuse {
			return interoplab.ProcessResult{}, errors.New("docker client stopped by the fixture")
		}
		return interoplab.ProcessResult{Stderr: "dhcpcd not running", ExitCode: r.release}, nil
	case "-w":
		return interoplab.ProcessResult{}, nil
	case "--dumplease":
		return interoplab.ProcessResult{Stdout: r.dump}, nil
	}
	return interoplab.ProcessResult{}, fmt.Errorf("the fixture has no answer for dhcpcd %s", flag)
}
