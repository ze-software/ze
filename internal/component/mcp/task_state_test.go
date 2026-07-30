package mcp

import "testing"

// test-relax: D-4 removes TaskInputRequired, so the rows naming it cannot
// reference the constant. Every one of them is replaced by a NEGATIVE assertion
// that the state is unreachable and its wire name is rejected, plus
// TestTaskStateWireVocabulary below which pins the whole enumeration (AC-15).
// The assertion count goes up, not down.

func TestTaskState_String(t *testing.T) {
	tests := []struct {
		state TaskState
		want  string
	}{
		{TaskUnspecified, ""},
		{TaskWorking, "working"},
		{TaskCompleted, "completed"},
		{TaskFailed, "failed"},
		{TaskCancelled, taskStateWireCancelled},
		{TaskState(99), ""},
	}
	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("TaskState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

// TestTaskStateWireVocabulary pins the exact set of wire values this server can
// produce.
//
// VALIDATES: AC-15 -- "Any TaskState rendered to the wire: the vocabulary is
// exactly working, completed, failed and the terminal cancel state.
// input_required is not a value Ze can produce (D-4)."
// PREVENTS: reintroducing input_required as a producible state without also
// implementing the inputRequests payload and inputResponses matching that make
// it meaningful.
//
// This asserts an ENUMERATION rather than an absence: it walks every value the
// type can hold, so a newly added state fails here whether or not anyone
// remembers this test exists.
func TestTaskStateWireVocabulary(t *testing.T) {
	want := []string{"working", "completed", "failed", taskStateWireCancelled}

	var got []string
	for v := range 256 {
		if name := TaskState(v).String(); name != "" {
			got = append(got, name)
		}
	}

	if len(got) != len(want) {
		t.Fatalf("producible wire states = %v, want exactly %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("producible wire state[%d] = %q, want %q (full set %v)", i, got[i], want[i], got)
		}
	}

	// The decoder must not accept the state either: a round-trip through a
	// stored value is the other way the state could re-enter the registry.
	var s TaskState
	if err := s.UnmarshalText([]byte("input_required")); err == nil {
		t.Errorf(`UnmarshalText("input_required") = %v, want an error: D-4 removed the state`, s)
	}
}

func TestTaskState_IsTerminal(t *testing.T) {
	terminal := []TaskState{TaskCompleted, TaskFailed, TaskCancelled}
	nonTerminal := []TaskState{TaskUnspecified, TaskWorking}

	for _, s := range terminal {
		if !s.IsTerminal() {
			t.Errorf("%v.IsTerminal() = false, want true", s)
		}
	}
	for _, s := range nonTerminal {
		if s.IsTerminal() {
			t.Errorf("%v.IsTerminal() = true, want false", s)
		}
	}
}

func TestTaskState_MarshalText(t *testing.T) {
	tests := []struct {
		state   TaskState
		want    string
		wantErr bool
	}{
		{TaskWorking, "working", false},
		{TaskCompleted, "completed", false},
		{TaskFailed, "failed", false},
		{TaskCancelled, taskStateWireCancelled, false},
		{TaskUnspecified, "", true},
		{TaskState(99), "", true},
	}
	for _, tt := range tests {
		got, err := tt.state.MarshalText()
		if (err != nil) != tt.wantErr {
			t.Errorf("TaskState(%d).MarshalText() error = %v, wantErr %v", tt.state, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && string(got) != tt.want {
			t.Errorf("TaskState(%d).MarshalText() = %q, want %q", tt.state, got, tt.want)
		}
	}
}

func TestTaskState_UnmarshalText(t *testing.T) {
	tests := []struct {
		input   string
		want    TaskState
		wantErr bool
	}{
		{"working", TaskWorking, false},
		{"completed", TaskCompleted, false},
		{"failed", TaskFailed, false},
		{taskStateWireCancelled, TaskCancelled, false},
		// D-4: the extension permits this state, Ze cannot enter it, so the
		// decoder refuses it rather than minting an unreachable value.
		{"input_required", TaskUnspecified, true},
		{"", TaskUnspecified, true},
		{"bogus", TaskUnspecified, true},
	}
	for _, tt := range tests {
		var s TaskState
		err := s.UnmarshalText([]byte(tt.input))
		if (err != nil) != tt.wantErr {
			t.Errorf("UnmarshalText(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && s != tt.want {
			t.Errorf("UnmarshalText(%q) = %v, want %v", tt.input, s, tt.want)
		}
	}
}

func TestTaskState_MarshalUnmarshalRoundtrip(t *testing.T) {
	states := []TaskState{TaskWorking, TaskCompleted, TaskFailed, TaskCancelled}
	for _, orig := range states {
		b, err := orig.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText(%v) = %v", orig, err)
		}
		var got TaskState
		if err := got.UnmarshalText(b); err != nil {
			t.Fatalf("UnmarshalText(%q) = %v", b, err)
		}
		if got != orig {
			t.Errorf("roundtrip: %v -> %q -> %v", orig, b, got)
		}
	}
}
