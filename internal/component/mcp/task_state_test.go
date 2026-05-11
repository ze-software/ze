package mcp

import "testing"

func TestTaskState_String(t *testing.T) {
	tests := []struct {
		state TaskState
		want  string
	}{
		{TaskUnspecified, ""},
		{TaskWorking, "working"},
		{TaskInputRequired, "input_required"},
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

func TestTaskState_IsTerminal(t *testing.T) {
	terminal := []TaskState{TaskCompleted, TaskFailed, TaskCancelled}
	nonTerminal := []TaskState{TaskUnspecified, TaskWorking, TaskInputRequired}

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
		{TaskInputRequired, "input_required", false},
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
		{"input_required", TaskInputRequired, false},
		{"completed", TaskCompleted, false},
		{"failed", TaskFailed, false},
		{taskStateWireCancelled, TaskCancelled, false},
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
	states := []TaskState{TaskWorking, TaskInputRequired, TaskCompleted, TaskFailed, TaskCancelled}
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
