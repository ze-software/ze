package fleet

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"
)

const (
	testReportTimestamp = "2026-11-26T12:00:00Z"
	testSoftwareVersion = "0.9.1"
	testNICDriver       = "ixgbe"
	testComponentBGP    = "bgp"
	testMethodA         = "a-report"
)

func TestInventoryReportPayload(t *testing.T) {
	// VALIDATES: InventoryReport round-trips all populated and unavailable
	// sections with lowercase kebab-case JSON keys, enforces ReportBodyMax on
	// encode and decode, refuses unknown fields and unavailable-section values,
	// and isolates Clone mutations.
	// PREVENTS: Zero/stale hardware ambiguity, raw SMART/drive-serial leaks,
	// unbounded payloads, and shared-slice aliasing across registry snapshots.
	t.Run("full round-trip with zero-error ECC", func(t *testing.T) {
		in := InventoryReport{
			CollectedAt:     testReportTimestamp,
			SoftwareVersion: testSoftwareVersion,
			CPU: InventoryCPU{
				Available:   true,
				LogicalCPUs: 16,
				Model:       "AMD EPYC 7313P",
			},
			NIC: InventoryNIC{
				Available: true,
				Count:     4,
				Drivers:   []string{"mlx5_core", testNICDriver},
			},
			Memory: InventoryMemory{
				Available:  true,
				TotalBytes: 64 << 30,
				ECC: &InventoryECC{
					CorrectableErrors:   0,
					UncorrectableErrors: 0,
				},
			},
			Storage: InventoryStorage{
				Available:   true,
				DeviceCount: 2,
				TotalBytes:  1 << 40,
			},
			DMI: InventoryDMI{
				Available: true,
				Model:     "PowerEdge R6515",
				Serial:    "SN-EDGE-42",
			},
		}

		data, err := EncodeInventoryReport(in)
		if err != nil {
			t.Fatalf("encode full inventory: %v", err)
		}
		raw := string(data)
		for _, wantKey := range []string{
			`"collected-at"`,
			`"software-version"`,
			`"logical-cpus"`,
			`"total-bytes"`,
			`"ecc":{"correctable-errors":0,"uncorrectable-errors":0}`,
			`"device-count"`,
			`"serial":"SN-EDGE-42"`,
		} {
			if !strings.Contains(raw, wantKey) {
				t.Errorf("encoded JSON missing %s: %s", wantKey, raw)
			}
		}

		got, err := DecodeInventoryReport(data)
		if err != nil {
			t.Fatalf("decode full inventory: %v", err)
		}
		if !reflect.DeepEqual(got, in) {
			t.Fatalf("round-trip mismatch:\ngot:  %+v\nwant: %+v", got, in)
		}
	})

	t.Run("partial unavailable sections", func(t *testing.T) {
		in := InventoryReport{
			CollectedAt:     testReportTimestamp,
			SoftwareVersion: testSoftwareVersion,
			CPU:             InventoryCPU{Available: true, LogicalCPUs: 4},
			NIC:             InventoryNIC{Available: false},
			Memory:          InventoryMemory{Available: true, TotalBytes: 8 << 30},
			Storage:         InventoryStorage{Available: false},
			DMI:             InventoryDMI{Available: false},
		}
		data, err := EncodeInventoryReport(in)
		if err != nil {
			t.Fatalf("encode partial inventory: %v", err)
		}
		got, err := DecodeInventoryReport(data)
		if err != nil {
			t.Fatalf("decode partial inventory: %v", err)
		}
		if !reflect.DeepEqual(got, in) {
			t.Fatalf("partial round-trip mismatch:\ngot:  %+v\nwant: %+v", got, in)
		}
		if got.Memory.ECC != nil {
			t.Fatalf("expected nil ECC when absent, got %+v", got.Memory.ECC)
		}
	})

	t.Run("unknown field and strict decode refusals", func(t *testing.T) {
		base, err := EncodeInventoryReport(InventoryReport{
			CollectedAt:     testReportTimestamp,
			SoftwareVersion: testSoftwareVersion,
		})
		if err != nil {
			t.Fatalf("encode base: %v", err)
		}
		badInputs := []struct {
			name string
			body []byte
		}{
			{
				name: "top-level unknown smart field",
				body: []byte(`{"collected-at":"2026-11-26T12:00:00Z","software-version":"0.9.1","smart":{"temp":40}}`),
			},
			{
				name: "nested unknown drive serial field",
				body: []byte(`{"collected-at":"2026-11-26T12:00:00Z","software-version":"0.9.1","storage":{"available":true,"device-count":1,"serial":"WD-123"}}`),
			},
			{
				name: "trailing JSON object",
				body: append(append([]byte{}, base...), []byte(`{}`)...),
			},
			{
				name: "invalid UTF-8",
				body: []byte("{\"collected-at\":\"2026-11-26T12:00:00Z\",\"software-version\":\"0.9.\xff\"}"),
			},
			{
				name: "empty body",
				body: nil,
			},
		}
		for _, tc := range badInputs {
			if _, err := DecodeInventoryReport(tc.body); err == nil {
				t.Errorf("%s: expected decode error, got nil", tc.name)
			}
		}
	})

	t.Run("oversize refusal on encode and decode", func(t *testing.T) {
		drivers := make([]string, InventoryDriversMax)
		// HTML escaping in encoding/json expands '<' to \u003c (6 bytes), so
		// 128 valid 128-byte driver strings exceed ReportBodyMax on encode.
		escapedDriver := strings.Repeat("<", ReportNameMax)
		for i := range drivers {
			drivers[i] = escapedDriver
		}
		oversize := InventoryReport{
			CollectedAt:     testReportTimestamp,
			SoftwareVersion: testSoftwareVersion,
			NIC: InventoryNIC{
				Available: true,
				Count:     InventoryDriversMax,
				Drivers:   drivers,
			},
		}
		if _, err := EncodeInventoryReport(oversize); err == nil {
			t.Fatal("expected encode error for report exceeding ReportBodyMax")
		}
		bigBody := make([]byte, ReportBodyMax+1)
		for i := range bigBody {
			bigBody[i] = ' '
		}
		if _, err := DecodeInventoryReport(bigBody); err == nil {
			t.Fatal("expected decode error for body exceeding ReportBodyMax")
		}
	})

	t.Run("unavailable section with data and invalid field refusals", func(t *testing.T) {
		base := InventoryReport{
			CollectedAt:     testReportTimestamp,
			SoftwareVersion: testSoftwareVersion,
		}
		cases := []struct {
			name   string
			mutate func(*InventoryReport)
		}{
			{
				name:   "unavailable cpu with logical-cpus",
				mutate: func(r *InventoryReport) { r.CPU = InventoryCPU{Available: false, LogicalCPUs: 2} },
			},
			{
				name:   "unavailable cpu with model",
				mutate: func(r *InventoryReport) { r.CPU = InventoryCPU{Available: false, Model: "Xeon"} },
			},
			{
				name:   "unavailable nic with count",
				mutate: func(r *InventoryReport) { r.NIC = InventoryNIC{Available: false, Count: 1} },
			},
			{
				name:   "unavailable nic with drivers",
				mutate: func(r *InventoryReport) { r.NIC = InventoryNIC{Available: false, Drivers: []string{"igb"}} },
			},
			{
				name:   "unavailable memory with total-bytes",
				mutate: func(r *InventoryReport) { r.Memory = InventoryMemory{Available: false, TotalBytes: 1024} },
			},
			{
				name:   "unavailable memory with ecc",
				mutate: func(r *InventoryReport) { r.Memory = InventoryMemory{Available: false, ECC: &InventoryECC{}} },
			},
			{
				name:   "unavailable storage with device-count",
				mutate: func(r *InventoryReport) { r.Storage = InventoryStorage{Available: false, DeviceCount: 1} },
			},
			{
				name:   "unavailable storage with total-bytes",
				mutate: func(r *InventoryReport) { r.Storage = InventoryStorage{Available: false, TotalBytes: 4096} },
			},
			{
				name:   "unavailable dmi with model",
				mutate: func(r *InventoryReport) { r.DMI = InventoryDMI{Available: false, Model: "Box"} },
			},
			{
				name:   "unavailable dmi with serial",
				mutate: func(r *InventoryReport) { r.DMI = InventoryDMI{Available: false, Serial: "S1"} },
			},
			{
				name:   "negative cpu logical-cpus",
				mutate: func(r *InventoryReport) { r.CPU = InventoryCPU{Available: true, LogicalCPUs: -1} },
			},
			{
				name:   "negative nic count",
				mutate: func(r *InventoryReport) { r.NIC = InventoryNIC{Available: true, Count: -1} },
			},
			{
				name:   "negative storage device-count",
				mutate: func(r *InventoryReport) { r.Storage = InventoryStorage{Available: true, DeviceCount: -1} },
			},
			{
				name: "too many drivers",
				mutate: func(r *InventoryReport) {
					d := make([]string, InventoryDriversMax+1)
					for i := range d {
						d[i] = testNICDriver
					}
					r.NIC = InventoryNIC{Available: true, Count: 1, Drivers: d}
				},
			},
			{
				name:   "empty driver entry",
				mutate: func(r *InventoryReport) { r.NIC = InventoryNIC{Available: true, Count: 1, Drivers: []string{""}} },
			},
			{
				name: "oversize cpu model",
				mutate: func(r *InventoryReport) {
					r.CPU = InventoryCPU{Available: true, Model: strings.Repeat("x", ReportTextMax+1)}
				},
			},
			{
				name:   "control char in software version",
				mutate: func(r *InventoryReport) { r.SoftwareVersion = "0.9.1\n" },
			},
			{
				name:   "missing software version",
				mutate: func(r *InventoryReport) { r.SoftwareVersion = "" },
			},
			{
				name:   "invalid collected-at timestamp",
				mutate: func(r *InventoryReport) { r.CollectedAt = "not-a-timestamp" },
			},
			{
				name:   "control char in dmi serial",
				mutate: func(r *InventoryReport) { r.DMI = InventoryDMI{Available: true, Serial: "SN\t1"} },
			},
		}
		for _, tc := range cases {
			r := base.Clone()
			tc.mutate(&r)
			if err := r.Validate(); err == nil {
				t.Errorf("%s: expected validation error, got nil", tc.name)
			}
		}
	})

	t.Run("Clone isolation", func(t *testing.T) {
		orig := InventoryReport{
			CollectedAt:     testReportTimestamp,
			SoftwareVersion: testSoftwareVersion,
			NIC:             InventoryNIC{Available: true, Count: 1, Drivers: []string{testNICDriver}},
			Memory:          InventoryMemory{Available: true, TotalBytes: 1024, ECC: &InventoryECC{CorrectableErrors: 1}},
		}
		cloned := orig.Clone()
		cloned.NIC.Drivers[0] = "mutated"
		cloned.Memory.ECC.CorrectableErrors = 99
		if orig.NIC.Drivers[0] != testNICDriver {
			t.Fatalf("Clone aliased NIC.Drivers: orig=%q", orig.NIC.Drivers[0])
		}
		if orig.Memory.ECC.CorrectableErrors != 1 {
			t.Fatalf("Clone aliased Memory.ECC: orig=%d", orig.Memory.ECC.CorrectableErrors)
		}
	})
}

func TestHealthReportPayload(t *testing.T) {
	// VALIDATES: HealthReport round-trips healthy, degraded, down, and empty
	// component sets matching /health JSON structure, enforces ReportBodyMax,
	// rejects duplicate components, invalid statuses, contradictory overall
	// statuses, and unknown fields, and isolates Clone mutations.
	// PREVENTS: Health status drift from /health, null components arrays,
	// contradictory device summaries, and shared-slice aliasing.
	t.Run("round-trip with healthy degraded and down components", func(t *testing.T) {
		in := HealthReport{
			Status: HealthStatusDown,
			Components: []HealthComponent{
				{Name: testComponentBGP, Status: HealthStatusHealthy, CheckedAt: testReportTimestamp},
				{Name: "ospf", Status: HealthStatusDegraded, Reason: "1 neighbor down", CheckedAt: testReportTimestamp},
				{Name: "storage", Status: HealthStatusDown, Reason: "read-only fs", CheckedAt: testReportTimestamp},
			},
			CheckedAt: testReportTimestamp,
		}
		data, err := EncodeHealthReport(in)
		if err != nil {
			t.Fatalf("encode health report: %v", err)
		}
		raw := string(data)
		want := `{"status":"down","components":[` +
			`{"name":"bgp","status":"healthy","checked-at":"2026-11-26T12:00:00Z"},` +
			`{"name":"ospf","status":"degraded","reason":"1 neighbor down","checked-at":"2026-11-26T12:00:00Z"},` +
			`{"name":"storage","status":"down","reason":"read-only fs","checked-at":"2026-11-26T12:00:00Z"}` +
			`],"checked-at":"2026-11-26T12:00:00Z"}`
		if raw != want {
			t.Fatalf("encoded JSON mismatch:\ngot:  %s\nwant: %s", raw, want)
		}
		got, err := DecodeHealthReport(data)
		if err != nil {
			t.Fatalf("decode health report: %v", err)
		}
		if !reflect.DeepEqual(got, in) {
			t.Fatalf("round-trip mismatch:\ngot:  %+v\nwant: %+v", got, in)
		}
	})

	t.Run("empty components marshals as empty array", func(t *testing.T) {
		in := HealthReport{
			Status:    HealthStatusHealthy,
			CheckedAt: testReportTimestamp,
		}
		data, err := EncodeHealthReport(in)
		if err != nil {
			t.Fatalf("encode empty health report: %v", err)
		}
		want := `{"status":"healthy","components":[],"checked-at":"2026-11-26T12:00:00Z"}`
		if string(data) != want {
			t.Fatalf("got %s, want %s", data, want)
		}
		got, err := DecodeHealthReport(data)
		if err != nil {
			t.Fatalf("decode empty health report: %v", err)
		}
		if got.Status != HealthStatusHealthy {
			t.Fatalf("status: got %v, want healthy", got.Status)
		}
		if len(got.Components) != 0 {
			t.Fatalf("components: got %d, want 0", len(got.Components))
		}
	})

	t.Run("validation refusals", func(t *testing.T) {
		tooManyComps := make([]HealthComponent, HealthComponentsMax+1)
		for i := range tooManyComps {
			tooManyComps[i] = HealthComponent{
				Name:      "comp-" + strconv.Itoa(i),
				Status:    HealthStatusHealthy,
				CheckedAt: testReportTimestamp,
			}
		}
		cases := []struct {
			name   string
			report HealthReport
		}{
			{
				name: "duplicate component name",
				report: HealthReport{
					Status: HealthStatusHealthy,
					Components: []HealthComponent{
						{Name: testComponentBGP, Status: HealthStatusHealthy, CheckedAt: testReportTimestamp},
						{Name: testComponentBGP, Status: HealthStatusHealthy, CheckedAt: testReportTimestamp},
					},
					CheckedAt: testReportTimestamp,
				},
			},
			{
				name: "too many components",
				report: HealthReport{
					Status:     HealthStatusHealthy,
					Components: tooManyComps,
					CheckedAt:  testReportTimestamp,
				},
			},
			{
				name: "unspecified overall status",
				report: HealthReport{
					Status:    HealthStatusUnspecified,
					CheckedAt: testReportTimestamp,
				},
			},
			{
				name: "unknown numeric component status",
				report: HealthReport{
					Status: HealthStatusHealthy,
					Components: []HealthComponent{
						{Name: testComponentBGP, Status: HealthStatus(99), CheckedAt: testReportTimestamp},
					},
					CheckedAt: testReportTimestamp,
				},
			},
			{
				name: "contradictory overall healthy when component degraded",
				report: HealthReport{
					Status: HealthStatusHealthy,
					Components: []HealthComponent{
						{Name: testComponentBGP, Status: HealthStatusDegraded, CheckedAt: testReportTimestamp},
					},
					CheckedAt: testReportTimestamp,
				},
			},
			{
				name: "contradictory overall degraded when component down",
				report: HealthReport{
					Status: HealthStatusDegraded,
					Components: []HealthComponent{
						{Name: testComponentBGP, Status: HealthStatusDown, CheckedAt: testReportTimestamp},
					},
					CheckedAt: testReportTimestamp,
				},
			},
			{
				name: "contradictory overall degraded with no components",
				report: HealthReport{
					Status:    HealthStatusDegraded,
					CheckedAt: testReportTimestamp,
				},
			},
			{
				name: "control char in reason",
				report: HealthReport{
					Status: HealthStatusDegraded,
					Components: []HealthComponent{
						{Name: testComponentBGP, Status: HealthStatusDegraded, Reason: "bad\nline", CheckedAt: testReportTimestamp},
					},
					CheckedAt: testReportTimestamp,
				},
			},
			{
				name: "invalid checked-at",
				report: HealthReport{
					Status:    HealthStatusHealthy,
					CheckedAt: "2026-11-26",
				},
			},
		}
		for _, tc := range cases {
			if err := tc.report.Validate(); err == nil {
				t.Errorf("%s: expected validation error, got nil", tc.name)
			}
		}
	})

	t.Run("invalid status and unknown field decode refusals", func(t *testing.T) {
		badBodies := []struct {
			name string
			body []byte
		}{
			{
				name: "invalid status string",
				body: []byte(`{"status":"ok","components":[],"checked-at":"2026-11-26T12:00:00Z"}`),
			},
			{
				name: "null overall status",
				body: []byte(`{"status":null,"components":[],"checked-at":"2026-11-26T12:00:00Z"}`),
			},
			{
				name: "unknown top-level field",
				body: []byte(`{"status":"healthy","components":[],"checked-at":"2026-11-26T12:00:00Z","extra":1}`),
			},
			{
				name: "unknown component field",
				body: []byte(`{"status":"healthy","components":[{"name":"bgp","status":"healthy","checked-at":"2026-11-26T12:00:00Z","extra":1}],"checked-at":"2026-11-26T12:00:00Z"}`),
			},
			{
				name: "trailing JSON value",
				body: []byte(`{"status":"healthy","components":[],"checked-at":"2026-11-26T12:00:00Z"} true`),
			},
		}
		for _, tc := range badBodies {
			if _, err := DecodeHealthReport(tc.body); err == nil {
				t.Errorf("%s: expected decode error, got nil", tc.name)
			}
		}

		// Verify HealthStatus.UnmarshalJSON leaves receiver unchanged on error.
		st := HealthStatusDegraded
		if err := json.Unmarshal([]byte(`"bogus"`), &st); err == nil {
			t.Fatal("expected error unmarshaling bogus HealthStatus")
		}
		if st != HealthStatusDegraded {
			t.Fatalf("status mutated on failed unmarshal: got %v", st)
		}
		if _, err := json.Marshal(HealthStatusUnspecified); err == nil {
			t.Fatal("expected error marshaling HealthStatusUnspecified")
		}
	})

	t.Run("oversize refusal on encode and decode", func(t *testing.T) {
		comps := make([]HealthComponent, HealthComponentsMax)
		longReason := strings.Repeat("r", ReportTextMax)
		for i := range comps {
			comps[i] = HealthComponent{
				Name:      "comp-" + strings.Repeat("a", 40) + strconv.Itoa(i),
				Status:    HealthStatusDegraded,
				Reason:    longReason,
				CheckedAt: testReportTimestamp,
			}
		}
		oversize := HealthReport{
			Status:     HealthStatusDegraded,
			Components: comps,
			CheckedAt:  testReportTimestamp,
		}
		if _, err := EncodeHealthReport(oversize); err == nil {
			t.Fatal("expected encode error for health report exceeding ReportBodyMax")
		}
		bigBody := make([]byte, ReportBodyMax+1)
		for i := range bigBody {
			bigBody[i] = ' '
		}
		if _, err := DecodeHealthReport(bigBody); err == nil {
			t.Fatal("expected decode error for body exceeding ReportBodyMax")
		}
	})

	t.Run("Clone isolation", func(t *testing.T) {
		orig := HealthReport{
			Status: HealthStatusHealthy,
			Components: []HealthComponent{
				{Name: testComponentBGP, Status: HealthStatusHealthy, CheckedAt: testReportTimestamp},
			},
			CheckedAt: testReportTimestamp,
		}
		cloned := orig.Clone()
		cloned.Components[0].Name = "mutated"
		if orig.Components[0].Name != testComponentBGP {
			t.Fatalf("Clone aliased Components: orig=%q", orig.Components[0].Name)
		}
	})
}

func TestReportProducerRegistry(t *testing.T) {
	// VALIDATES: Report producer registration enforces required fields and
	// unique Name/Method pairs, returns a sorted defensive copy, panics with
	// BUG prefix on MustRegisterReportProducer errors, and restores cleanly via
	// SnapshotReportProducers.
	// PREVENTS: Duplicate report RPC registrations, silent startup failures,
	// and cross-test registry pollution.
	restore := SnapshotReportProducers()
	t.Cleanup(restore)

	noop := func(context.Context) ([]byte, error) { return []byte(`{}`), nil }

	t.Run("register and sorted list", func(t *testing.T) {
		subRestore := SnapshotReportProducers()
		defer subRestore()

		if err := RegisterReportProducer(ReportProducer{
			Name:     "test-z",
			Method:   "z-report",
			Interval: time.Minute,
			Collect:  noop,
		}); err != nil {
			t.Fatalf("register z-report: %v", err)
		}
		if err := RegisterReportProducer(ReportProducer{
			Name:     "test-a",
			Method:   testMethodA,
			Interval: 5 * time.Minute,
			Collect:  noop,
		}); err != nil {
			t.Fatalf("register a-report: %v", err)
		}

		list := ReportProducers()
		idxA, idxZ := -1, -1
		for i, p := range list {
			if p.Method == testMethodA {
				idxA = i
			}
			if p.Method == "z-report" {
				idxZ = i
			}
		}
		if idxA == -1 {
			t.Fatalf("registered producer %q missing from list: %+v", testMethodA, list)
		}
		if idxZ == -1 {
			t.Fatalf("registered producer z-report missing from list: %+v", list)
		}
		if idxA >= idxZ {
			t.Fatalf("expected sorted by Method (a-report before z-report), got idxA=%d idxZ=%d", idxA, idxZ)
		}

		// Mutating returned slice must not affect registry.
		list[idxA].Name = "mutated"
		again := ReportProducers()
		for _, p := range again {
			if p.Method != testMethodA {
				continue
			}
			if p.Name != "test-a" {
				t.Fatalf("ReportProducers exposed live slice: got %q", p.Name)
			}
		}
	})

	t.Run("duplicate name and duplicate method refusals", func(t *testing.T) {
		subRestore := SnapshotReportProducers()
		defer subRestore()

		base := ReportProducer{
			Name:     "dup-name-test",
			Method:   "dup-method-test",
			Interval: time.Minute,
			Collect:  noop,
		}
		if err := RegisterReportProducer(base); err != nil {
			t.Fatalf("register base: %v", err)
		}
		if err := RegisterReportProducer(ReportProducer{
			Name:     base.Name,
			Method:   "other-method-test",
			Interval: time.Minute,
			Collect:  noop,
		}); err == nil {
			t.Error("expected error on duplicate producer name")
		}
		if err := RegisterReportProducer(ReportProducer{
			Name:     "other-name-test",
			Method:   base.Method,
			Interval: time.Minute,
			Collect:  noop,
		}); err == nil {
			t.Error("expected error on duplicate producer method")
		}
	})

	t.Run("missing or invalid fields refusals", func(t *testing.T) {
		bad := []struct {
			name string
			p    ReportProducer
		}{
			{name: "empty name", p: ReportProducer{Method: "m1", Interval: time.Second, Collect: noop}},
			{name: "empty method", p: ReportProducer{Name: "n1", Interval: time.Second, Collect: noop}},
			{name: "zero interval", p: ReportProducer{Name: "n1", Method: "m1", Interval: 0, Collect: noop}},
			{name: "negative interval", p: ReportProducer{Name: "n1", Method: "m1", Interval: -time.Second, Collect: noop}},
			{name: "nil collect", p: ReportProducer{Name: "n1", Method: "m1", Interval: time.Second}},
			{name: "control char in method", p: ReportProducer{Name: "n1", Method: "m1\n", Interval: time.Second, Collect: noop}},
		}
		for _, tc := range bad {
			if err := RegisterReportProducer(tc.p); err == nil {
				t.Errorf("%s: expected registration error, got nil", tc.name)
			}
		}
	})

	t.Run("MustRegisterReportProducer panics with BUG prefix", func(t *testing.T) {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("expected panic from MustRegisterReportProducer")
			}
			msg, ok := r.(string)
			if !ok {
				t.Fatalf("expected string panic, got %T (%v)", r, r)
			}
			if !strings.HasPrefix(msg, "BUG: fleet:") {
				t.Fatalf("expected panic starting with 'BUG: fleet:', got %q", msg)
			}
		}()
		MustRegisterReportProducer(ReportProducer{})
	})

	t.Run("snapshot and restore isolation", func(t *testing.T) {
		before := ReportProducers()
		snap := SnapshotReportProducers()
		if err := RegisterReportProducer(ReportProducer{
			Name:     "ephemeral-producer",
			Method:   "ephemeral-method",
			Interval: time.Minute,
			Collect:  noop,
		}); err != nil {
			t.Fatalf("register ephemeral: %v", err)
		}
		snap()
		after := ReportProducers()
		if len(after) != len(before) {
			t.Fatalf("restore count mismatch: got %d, want %d", len(after), len(before))
		}
		for _, p := range after {
			if p.Method == "ephemeral-method" {
				t.Fatal("ephemeral producer survived SnapshotReportProducers restore")
			}
		}
	})
}
