package observe

import "testing"

// VALIDATES: AC-8 -- the section arrives wrapped as {"anomaly":{"observe":{...}}}
// and every leaf arrives as a JSON string, so ParseConfig must unwrap two levels
// and accept the string form.
// PREVENTS: the failure the ddos mirror already hit -- a parser that reads only the
// native JSON number silently falls back to every default, so a configured ring
// size never reaches the store.
func TestParseObserveConfig(t *testing.T) {
	cases := []struct {
		name          string
		data          string
		ringSize      int
		timeoutSecond int
	}{
		{
			name:          "wrapped string leaves",
			data:          `{"anomaly":{"observe":{"incident-ring-size":"50","stale-incident-timeout":"120"}}}`,
			ringSize:      50,
			timeoutSecond: 120,
		},
		{
			name:          "wrapped numeric leaves",
			data:          `{"anomaly":{"observe":{"incident-ring-size":50,"stale-incident-timeout":120}}}`,
			ringSize:      50,
			timeoutSecond: 120,
		},
		{
			name:          "empty section keeps the defaults",
			data:          `{"anomaly":{"observe":{}}}`,
			ringSize:      1000,
			timeoutSecond: 3600,
		},
		{
			name:          "no data keeps the defaults",
			data:          ``,
			ringSize:      1000,
			timeoutSecond: 3600,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseConfig(tc.data)
			if err != nil {
				t.Fatal(err)
			}
			if cfg.IncidentRingSize != tc.ringSize {
				t.Errorf("incident-ring-size = %d, want %d", cfg.IncidentRingSize, tc.ringSize)
			}
			if cfg.StaleIncidentTimeout != tc.timeoutSecond {
				t.Errorf("stale-incident-timeout = %d, want %d", cfg.StaleIncidentTimeout, tc.timeoutSecond)
			}
		})
	}
}

// VALIDATES: malformed JSON is an error rather than a silent fallback to defaults.
// PREVENTS: a broken config section starting the plugin with an unintended ring
// size, which fails open on a memory bound.
func TestParseObserveConfigRejectsBadJSON(t *testing.T) {
	if _, err := ParseConfig(`{"anomaly":`); err == nil {
		t.Fatal("ParseConfig accepted truncated JSON, want an error")
	}
}

// VALIDATES: AC-8 boundary rows -- both leaves are rejected one below and one above
// their range, and accepted at each edge.
// PREVENTS: a zero ring size building a store that drops every incident, and an
// out-of-range timeout defeating the memory bound.
func TestValidateObserveConfigBoundaries(t *testing.T) {
	cases := []struct {
		name          string
		ringSize      int
		timeoutSecond int
		wantErr       bool
	}{
		{name: "defaults", ringSize: 1000, timeoutSecond: 3600},
		{name: "ring size first valid", ringSize: 1, timeoutSecond: 3600},
		{name: "ring size last valid", ringSize: 100000, timeoutSecond: 3600},
		{name: "ring size below range", ringSize: 0, timeoutSecond: 3600, wantErr: true},
		{name: "ring size above range", ringSize: 100001, timeoutSecond: 3600, wantErr: true},
		{name: "timeout first valid", ringSize: 1000, timeoutSecond: 1},
		{name: "timeout last valid", ringSize: 1000, timeoutSecond: 86400},
		{name: "timeout below range", ringSize: 1000, timeoutSecond: 0, wantErr: true},
		{name: "timeout above range", ringSize: 1000, timeoutSecond: 86401, wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{IncidentRingSize: tc.ringSize, StaleIncidentTimeout: tc.timeoutSecond}
			err := cfg.Validate()
			if tc.wantErr && err == nil {
				t.Fatalf("Validate(%+v) = nil, want an out-of-range error", cfg)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("Validate(%+v) = %v, want nil", cfg, err)
			}
		})
	}
}
