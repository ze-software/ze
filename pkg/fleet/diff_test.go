package fleet

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"slices"
	"strings"
	"testing"
)

func TestDiffRoundTripByteExactAcrossFormats(t *testing.T) {
	// VALIDATES: ComputeDiff is deterministic and ApplyDiff(base, delta)
	// reproduces byte-identical target (matching VersionHash) across
	// hierarchical configs, FormatSet, FormatSetMeta, comments, CRLF/LF,
	// missing trailing newlines, $9$ secrets, repeated lines, and empty files.
	// PREVENTS: Subtle byte or line-ending drift breaking fleet.VersionHash
	// equality or fleet-6/fleet-7 hold detection after diff apply.
	const maxOut = 16 << 20

	tests := []struct {
		name   string
		base   []byte
		target []byte
	}{
		{
			name: "hierarchical with comments and $9$ secret",
			base: []byte("# edge-01 baseline\n" +
				"system {\n" +
				"    host-name edge-01;\n" +
				"    login {\n" +
				"        user admin {\n" +
				"            password \"$9$abc123def456\"; # secret-scan: allow\n" +
				"        }\n" +
				"    }\n" +
				"}\n" +
				"bgp {\n" +
				"    local-as 65001;\n" +
				"    neighbor 10.0.0.1 {\n" +
				"        peer-as 65002;\n" +
				"    }\n" +
				"}\n"),
			target: []byte("# edge-01 baseline\n" +
				"system {\n" +
				"    host-name edge-01;\n" +
				"    login {\n" +
				"        user admin {\n" +
				"            password \"$9$abc123def456\"; # secret-scan: allow\n" +
				"        }\n" +
				"    }\n" +
				"}\n" +
				"bgp {\n" +
				"    local-as 65001;\n" +
				"    neighbor 10.0.0.1 {\n" +
				"        peer-as 65002;\n" +
				"        description \"transit-a\";\n" +
				"    }\n" +
				"    neighbor 10.0.0.2 {\n" +
				"        peer-as 65003;\n" +
				"    }\n" +
				"}\n"),
		},
		{
			name: "FormatSet lines",
			base: []byte("set system host-name edge-01\n" +
				"set bgp local-as 65001\n" +
				"set bgp neighbor 10.0.0.1 peer-as 65002\n"),
			target: []byte("set system host-name edge-01\n" +
				"set bgp local-as 65001\n" +
				"set bgp neighbor 10.0.0.1 peer-as 65009\n" +
				"set bgp neighbor 10.0.0.2 peer-as 65003\n"),
		},
		{
			name: "FormatSetMeta with annotations and comments",
			base: []byte("# format: set-meta\n" +
				"annotate bgp \"core routing\"\n" +
				"set bgp local-as 65001\n" +
				"deactivate bgp neighbor 10.0.0.9\n"),
			target: []byte("# format: set-meta\n" +
				"annotate bgp \"core routing v2\"\n" +
				"set bgp local-as 65001\n" +
				"activate bgp neighbor 10.0.0.9\n"),
		},
		{
			name:   "CRLF and mixed line endings",
			base:   []byte("line1\r\nline2\nline3\r\nline4\r\n"),
			target: []byte("line1\r\nline2-edited\r\nline3\r\nline4\n"),
		},
		{
			name:   "base has trailing newline target has none",
			base:   []byte("a\nb\nc\n"),
			target: []byte("a\nb\nc"),
		},
		{
			name:   "base has no trailing newline target adds one",
			base:   []byte("a\nb\nc"),
			target: []byte("a\nb\nc\n"),
		},
		{
			name:   "neither has trailing newline",
			base:   []byte("alpha\nbeta\ngamma"),
			target: []byte("alpha\ndelta\ngamma"),
		},
		{
			name:   "identical content",
			base:   []byte("same1\nsame2\n"),
			target: []byte("same1\nsame2\n"),
		},
		{
			name:   "empty base to non-empty target",
			base:   []byte{},
			target: []byte("first\nsecond\n"),
		},
		{
			name:   "non-empty base to empty target",
			base:   []byte("first\nsecond\n"),
			target: []byte{},
		},
		{
			name:   "empty base to empty target",
			base:   []byte{},
			target: []byte{},
		},
		{
			name:   "repeated non-unique lines exercising Myers gap solver",
			base:   []byte("{\n}\n{\n}\n{\n    a 1;\n}\n{\n}\n"),
			target: []byte("{\n}\n{\n    b 2;\n}\n{\n    a 1;\n}\n{\n    c 3;\n}\n"),
		},
		{
			name:   "reordered unique blocks exercising patience LIS anchors",
			base:   []byte("header\nblock-A\nblock-B\nblock-C\nblock-D\nfooter\n"),
			target: []byte("header\nblock-C\nblock-A\nblock-B\nblock-E\nfooter\n"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			baseCopy := append([]byte(nil), tt.base...)

			delta1, _, err := ComputeDiff(tt.base, tt.target)
			if err != nil {
				t.Fatalf("ComputeDiff(1): %v", err)
			}
			delta2, _, err := ComputeDiff(tt.base, tt.target)
			if err != nil {
				t.Fatalf("ComputeDiff(2): %v", err)
			}
			if !bytes.Equal(delta1, delta2) {
				t.Fatalf("ComputeDiff not deterministic:\n1: %s\n2: %s", delta1, delta2)
			}

			got, err := ApplyDiff(tt.base, delta1, maxOut)
			if err != nil {
				t.Fatalf("ApplyDiff failed (delta=%s): %v", delta1, err)
			}
			if !bytes.Equal(got, tt.target) {
				t.Fatalf("reconstructed mismatch:\ngot:  %q\nwant: %q\ndelta: %s", got, tt.target, delta1)
			}
			if VersionHash(got) != VersionHash(tt.target) {
				t.Fatalf("VersionHash mismatch: got %s, want %s", VersionHash(got), VersionHash(tt.target))
			}
			if !bytes.Equal(tt.base, baseCopy) {
				t.Fatalf("ApplyDiff mutated base slice")
			}
		})
	}
}

func TestComputeDiffSkipsWhenLargerThanTarget(t *testing.T) {
	// VALIDATES: ComputeDiff reports worthSending == true when a diff is
	// smaller than target, and worthSending == false when delta >= target,
	// when target is empty, when target contains invalid UTF-8, or when the
	// diff requires more than MaxDiffOps operations.
	// PREVENTS: Hub sending diffs that waste wire bytes or exceed MaxDiffOps.
	t.Run("small edit on larger config is worth sending", func(t *testing.T) {
		var baseBuf, targetBuf strings.Builder
		for i := range 200 {
			line := fmt.Sprintf("set bgp neighbor 10.0.%d.%d peer-as %d\n", i/256, i%256, 65000+i)
			baseBuf.WriteString(line)
			if i == 100 {
				targetBuf.WriteString("set bgp neighbor 10.0.0.100 peer-as 65999\n")
			} else {
				targetBuf.WriteString(line)
			}
		}
		base := []byte(baseBuf.String())
		target := []byte(targetBuf.String())

		delta, worth, err := ComputeDiff(base, target)
		if err != nil {
			t.Fatalf("ComputeDiff: %v", err)
		}
		if !worth {
			t.Fatalf("expected worthSending=true, got false (delta=%d target=%d)", len(delta), len(target))
		}
		if len(delta) >= len(target) {
			t.Fatalf("expected len(delta) < len(target), got %d >= %d", len(delta), len(target))
		}
		got, err := ApplyDiff(base, delta, len(target))
		if err != nil {
			t.Fatalf("ApplyDiff: %v", err)
		}
		if !bytes.Equal(got, target) {
			t.Fatalf("reconstructed config differs from target")
		}
	})

	t.Run("tiny replacement diff larger than target is skipped", func(t *testing.T) {
		delta, worth, err := ComputeDiff([]byte("a\n"), []byte("b\n"))
		if err != nil {
			t.Fatalf("ComputeDiff: %v", err)
		}
		if worth {
			t.Fatalf("expected worthSending=false for delta=%d target=2", len(delta))
		}
	})

	t.Run("empty target is skipped", func(t *testing.T) {
		_, worth, err := ComputeDiff([]byte("a\nb\n"), nil)
		if err != nil {
			t.Fatalf("ComputeDiff: %v", err)
		}
		if worth {
			t.Fatalf("expected worthSending=false for empty target")
		}
	})

	t.Run("invalid UTF-8 target is skipped", func(t *testing.T) {
		_, worth, err := ComputeDiff([]byte("ok\n"), []byte{0xff, 0xfe, '\n'})
		if err != nil {
			t.Fatalf("ComputeDiff: %v", err)
		}
		if worth {
			t.Fatalf("expected worthSending=false for non-UTF-8 target")
		}
	})

	t.Run("ops exceeding MaxDiffOps short-circuits with worthSending false", func(t *testing.T) {
		// Create (MaxDiffOps/2 + 2) unique kept lines with an inserted unique
		// line between each pair -> > MaxDiffOps alternating copy/ins ops.
		const pairs = MaxDiffOps/2 + 2
		var baseBuf, targetBuf strings.Builder
		for i := range pairs {
			kept := fmt.Sprintf("keep-%06d\n", i)
			baseBuf.WriteString(kept)
			targetBuf.WriteString(kept)
			fmt.Fprintf(&targetBuf, "ins-%06d\n", i)
		}
		delta, worth, err := ComputeDiff([]byte(baseBuf.String()), []byte(targetBuf.String()))
		if err != nil {
			t.Fatalf("ComputeDiff: %v", err)
		}
		if worth {
			t.Fatalf("expected worthSending=false when ops exceed MaxDiffOps")
		}
		if delta != nil {
			t.Fatalf("expected nil delta on MaxDiffOps short-circuit, got %d bytes", len(delta))
		}
	})
}

func TestApplyDiffRejectsMalformedAndOversized(t *testing.T) {
	// VALIDATES: ApplyDiff enforces every structural, index, op-count, and
	// output-size boundary (MaxDiffOps vs MaxDiffOps+1, Start=-1, Count=0,
	// Start+Count == len vs len+1, maxOutput vs maxOutput+1, unknown fields,
	// trailing JSON garbage, null, and diff-bomb output caps).
	// PREVENTS: Hostile or corrupted diffs causing OOB panics, silent data
	// corruption, or memory exhaustion on managed clients.
	base := []byte("line0\nline1\nline2\n") // 3 lines, 18 bytes

	t.Run("exact last valid copy boundary Start+Count == len(baseLines)", func(t *testing.T) {
		delta := []byte(`[{"op":"copy","start":0,"count":3}]`)
		got, err := ApplyDiff(base, delta, len(base))
		if err != nil {
			t.Fatalf("expected valid full copy at exact maxOutput, got err: %v", err)
		}
		if !bytes.Equal(got, base) {
			t.Fatalf("got %q, want %q", got, base)
		}
	})

	t.Run("exact MaxDiffOps (65536) succeeds and MaxDiffOps+1 (65537) fails", func(t *testing.T) {
		singleLineBase := []byte("x\n")
		ops := make([]DiffOp, MaxDiffOps)
		for i := range ops {
			ops[i] = DiffOp{Op: DiffOpCopy, Start: 0, Count: 1}
		}
		deltaExact, err := json.Marshal(ops)
		if err != nil {
			t.Fatalf("marshal MaxDiffOps: %v", err)
		}
		got, err := ApplyDiff(singleLineBase, deltaExact, MaxDiffOps*len(singleLineBase))
		if err != nil {
			t.Fatalf("ApplyDiff at exact MaxDiffOps failed: %v", err)
		}
		if len(got) != MaxDiffOps*len(singleLineBase) {
			t.Fatalf("got len %d, want %d", len(got), MaxDiffOps*len(singleLineBase))
		}

		opsOver := slices.Concat(ops, []DiffOp{{Op: DiffOpCopy, Start: 0, Count: 1}})
		deltaOver, err := json.Marshal(opsOver)
		if err != nil {
			t.Fatalf("marshal MaxDiffOps+1: %v", err)
		}
		if _, err := ApplyDiff(singleLineBase, deltaOver, (MaxDiffOps+1)*len(singleLineBase)); err == nil {
			t.Fatalf("expected error for MaxDiffOps+1 (%d) ops", len(opsOver))
		}
	})

	t.Run("maxOutput boundary at 16 MiB (exact succeeds, +1 byte rejected)", func(t *testing.T) {
		const maxWire = 16 << 20 // 16777216 bytes
		// 1 KiB line copied 16384 times = exact 16 MiB.
		line1K := strings.Repeat("a", 1023) + "\n"
		base1K := []byte(line1K)
		ops := make([]DiffOp, 16384)
		for i := range ops {
			ops[i] = DiffOp{Op: DiffOpCopy, Start: 0, Count: 1}
		}
		deltaExact, err := json.Marshal(ops)
		if err != nil {
			t.Fatalf("marshal 16MiB ops: %v", err)
		}
		got, err := ApplyDiff(base1K, deltaExact, maxWire)
		if err != nil {
			t.Fatalf("ApplyDiff exact 16MiB failed: %v", err)
		}
		if len(got) != maxWire {
			t.Fatalf("got %d bytes, want %d", len(got), maxWire)
		}

		// Adding 1 byte exceeds maxWire (16777217 bytes) -> rejected before alloc.
		opsPlusOne := slices.Concat(ops, []DiffOp{{Op: DiffOpInsert, Lines: []string{"z"}}})
		deltaPlusOne, err := json.Marshal(opsPlusOne)
		if err != nil {
			t.Fatalf("marshal 16MiB+1 ops: %v", err)
		}
		if _, err := ApplyDiff(base1K, deltaPlusOne, maxWire); err == nil {
			t.Fatalf("expected error for output of %d bytes with maxOutput=%d", maxWire+1, maxWire)
		}
	})

	t.Run("empty base and empty diff array", func(t *testing.T) {
		got, err := ApplyDiff(nil, []byte("[]"), 0)
		if err != nil {
			t.Fatalf("ApplyDiff empty array with maxOutput=0: %v", err)
		}
		if len(got) != 0 {
			t.Fatalf("expected empty output, got %q", got)
		}
	})

	rejectCases := []struct {
		name      string
		base      []byte
		delta     []byte
		maxOutput int
	}{
		{
			name:      "negative maxOutput",
			base:      base,
			delta:     []byte(`[]`),
			maxOutput: -1,
		},
		{
			name:      "empty delta",
			base:      base,
			delta:     nil,
			maxOutput: 1024,
		},
		{
			name:      "invalid UTF-8 delta",
			base:      base,
			delta:     []byte{0xff, 0xfe},
			maxOutput: 1024,
		},
		{
			name:      "JSON null instead of array",
			base:      base,
			delta:     []byte(`null`),
			maxOutput: 1024,
		},
		{
			name:      "trailing JSON value",
			base:      base,
			delta:     []byte(`[{"op":"copy","start":0,"count":1}] {"extra":true}`),
			maxOutput: 1024,
		},
		{
			name:      "trailing non-JSON garbage",
			base:      base,
			delta:     []byte(`[{"op":"copy","start":0,"count":1}] trailing`),
			maxOutput: 1024,
		},
		{
			name:      "unknown JSON field",
			base:      base,
			delta:     []byte(`[{"op":"copy","start":0,"count":1,"bogus":1}]`),
			maxOutput: 1024,
		},
		{
			name:      "unknown op kind",
			base:      base,
			delta:     []byte(`[{"op":"delete","start":0,"count":1}]`),
			maxOutput: 1024,
		},
		{
			name:      "copy Start = -1",
			base:      base,
			delta:     []byte(`[{"op":"copy","start":-1,"count":1}]`),
			maxOutput: 1024,
		},
		{
			name:      "copy Count = 0",
			base:      base,
			delta:     []byte(`[{"op":"copy","start":0,"count":0}]`),
			maxOutput: 1024,
		},
		{
			name:      "copy Start+Count == len(baseLines)+1",
			base:      base,
			delta:     []byte(`[{"op":"copy","start":1,"count":3}]`),
			maxOutput: 1024,
		},
		{
			name:      "copy Start == len(baseLines)",
			base:      base,
			delta:     []byte(`[{"op":"copy","start":3,"count":1}]`),
			maxOutput: 1024,
		},
		{
			name:      "copy over empty base",
			base:      nil,
			delta:     []byte(`[{"op":"copy","start":0,"count":1}]`),
			maxOutput: 1024,
		},
		{
			name:      "copy carrying lines",
			base:      base,
			delta:     []byte(`[{"op":"copy","start":0,"count":1,"lines":["extra\n"]}]`),
			maxOutput: 1024,
		},
		{
			name:      "ins with empty lines",
			base:      base,
			delta:     []byte(`[{"op":"ins","lines":[]}]`),
			maxOutput: 1024,
		},
		{
			name:      "ins with non-zero start",
			base:      base,
			delta:     []byte(`[{"op":"ins","start":1,"lines":["a\n"]}]`),
			maxOutput: 1024,
		},
		{
			name:      "ins with non-zero count",
			base:      base,
			delta:     []byte(`[{"op":"ins","count":1,"lines":["a\n"]}]`),
			maxOutput: 1024,
		},
		{
			name:      "output exceeds maxOutput by 1 byte on copy",
			base:      base,
			delta:     []byte(`[{"op":"copy","start":0,"count":3}]`),
			maxOutput: len(base) - 1,
		},
		{
			name:      "output exceeds maxOutput by 1 byte on ins",
			base:      base,
			delta:     []byte(`[{"op":"ins","lines":["12345"]}]`),
			maxOutput: 4,
		},
	}

	for _, tc := range rejectCases {
		t.Run(tc.name, func(t *testing.T) {
			if got, err := ApplyDiff(tc.base, tc.delta, tc.maxOutput); err == nil {
				t.Fatalf("expected error, got %d bytes (%q)", len(got), got)
			}
		})
	}

	t.Run("overflowing start+count reports start and count without negative wrap", func(t *testing.T) {
		delta := fmt.Appendf(nil, `[{"op":"copy","start":1,"count":%d}]`, math.MaxInt)
		_, err := ApplyDiff(base, delta, 1024)
		if err == nil {
			t.Fatalf("expected error for start=1 count=MaxInt")
		}
		want := fmt.Sprintf("start=1 count=%d", math.MaxInt)
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected error containing %q, got %q", want, err.Error())
		}
		if strings.Contains(err.Error(), "-") {
			t.Fatalf("error message must not contain negative overflowed number: %q", err.Error())
		}
	})
}
