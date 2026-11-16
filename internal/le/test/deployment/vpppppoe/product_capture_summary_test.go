// Design: docs/labs/pppoe-interop.md -- capture loss invalidates product evidence

package testvpppppoe

import "testing"

// TestProductCaptureNoDropsRequiresUniqueZero prevents incomplete or lossy
// capture observations from proving native discovery, refusal or family gates.
// Method: accept the existing native zero-summary grammar, then vary the count,
// completeness and uniqueness independently of captured packet count.
// MUTATION: accepting a zero substring admits ten dropped packets and duplicates.
func TestProductCaptureNoDropsRequiresUniqueZero(t *testing.T) {
	const summary = "0 packets dropped by kernel\n"
	for _, test := range []struct {
		name      string
		output    string
		wantError bool
	}{
		{name: "zero", output: "listening on eth0, link-type EN10MB\n12 packets captured\n12 packets received by filter\n" + summary},
		{name: "whitespace", output: " \t0\tpackets  dropped by\tkernel \n"},
		{name: "ten", output: "10 packets dropped by kernel\n", wantError: true},
		{name: "missing", output: "12 packets captured\n12 packets received by filter\n", wantError: true},
		{name: "empty", wantError: true},
		{name: "duplicate", output: summary + summary, wantError: true},
		{name: "zero then ten", output: summary + "10 packets dropped by kernel\n", wantError: true},
		{name: "ten then zero", output: "10 packets dropped by kernel\n" + summary, wantError: true},
		{name: "prefixed", output: "prefix " + summary, wantError: true},
		{name: "suffixed", output: "0 packets dropped by kernel extra\n", wantError: true},
		{name: "incomplete alongside zero", output: summary + "0 packets dropped\n", wantError: true},
		{name: "nonnumeric", output: "unknown packets dropped by kernel\n", wantError: true},
		{name: "signed zero", output: "+0 packets dropped by kernel\n", wantError: true},
		{name: "padded zero", output: "00 packets dropped by kernel\n", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := productCaptureNoDrops(test.output)
			if test.wantError {
				if err == nil {
					t.Fatalf("accepted unknown or nonzero capture loss: %q", test.output)
				}
				return
			}
			if err != nil {
				t.Fatalf("rejected complete zero-loss capture: %v", err)
			}
		})
	}
}

// Goal: one dropped interface packet invalidates the capture evidence.
// Method: retain the completed zero-kernel summary and vary only interface loss.
// MUTATION: recognizing only plural drop summaries ignores a single lost packet.
func TestProductCaptureNoDropsRejectsOneInterfaceDrop(t *testing.T) {
	const kernel = "0 packets dropped by kernel\n"
	if err := productCaptureNoDrops(kernel + "1 packet dropped by interface\n"); err == nil {
		t.Fatal("one interface drop was accepted as zero capture loss")
	}
	if err := productCaptureNoDrops(kernel + "0 packets dropped by interface\n"); err != nil {
		t.Fatalf("complete measured zero was refused: %v", err)
	}
}

// Goal: a singular kernel loss cannot hide beside a zero completion summary.
// Method: put a one-packet kernel report before and after the valid zero report.
func TestProductCaptureNoDropsRejectsOneKernelDrop(t *testing.T) {
	const zero = "0 packets dropped by kernel\n"
	const lost = "1 packet dropped by kernel\n"
	for _, output := range []string{lost, zero + lost, lost + zero} {
		if err := productCaptureNoDrops(output); err == nil {
			t.Fatalf("one kernel drop was accepted as zero capture loss: %q", output)
		}
	}
	if err := productCaptureNoDrops(zero); err != nil {
		t.Fatalf("native zero-kernel summary was refused: %v", err)
	}
}

// Goal: an optional interface summary must have one exact measured-zero value.
// Method: retain the required kernel zero and vary the interface report's grammar,
// value and uniqueness. Native omission and an explicit zero remain permitted.
func TestProductCaptureNoDropsValidatesOptionalInterfaceSummary(t *testing.T) {
	const kernel = "0 packets dropped by kernel\n"
	const iface = "0 packets dropped by interface\n"
	for _, test := range []struct {
		name      string
		output    string
		wantError bool
	}{
		{name: "native omission", output: kernel},
		{name: "explicit zero", output: kernel + iface},
		{name: "zero before kernel", output: iface + kernel},
		{name: "whitespace", output: kernel + "\t0  packets\tdropped by interface \n"},
		{name: "two drops", output: kernel + "2 packets dropped by interface\n", wantError: true},
		{name: "unknown count", output: kernel + "unknown packets dropped by interface\n", wantError: true},
		{name: "unknown source", output: kernel + "0 packets dropped by driver\n", wantError: true},
		{name: "duplicate", output: kernel + iface + iface, wantError: true},
		{name: "conflicting", output: kernel + iface + "1 packet dropped by interface\n", wantError: true},
		{name: "incomplete", output: kernel + "0 packets dropped by\n", wantError: true},
		{name: "malformed singular zero", output: kernel + "0 packet dropped by interface\n", wantError: true},
		{name: "malformed prefix", output: kernel + "prefix " + iface, wantError: true},
		{name: "malformed suffix", output: kernel + "0 packets dropped by interface extra\n", wantError: true},
		{name: "signed zero", output: kernel + "+0 packets dropped by interface\n", wantError: true},
		{name: "padded zero", output: kernel + "00 packets dropped by interface\n", wantError: true},
		{name: "interface alone", output: iface, wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := productCaptureNoDrops(test.output)
			if test.wantError {
				if err == nil {
					t.Fatalf("accepted unknown or nonzero interface loss: %q", test.output)
				}
				return
			}
			if err != nil {
				t.Fatalf("refused complete measured-zero capture: %v", err)
			}
		})
	}
}
