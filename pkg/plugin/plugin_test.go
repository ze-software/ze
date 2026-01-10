package plugin

import (
	"testing"
	"time"
)

// TestParseRegisterCommand verifies register command parsing.
//
// VALIDATES: All register command formats are parsed correctly.
// PREVENTS: Registration failures from malformed commands.
func TestParseRegisterCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *CommandDef
		wantErr bool
	}{
		{
			name:  "basic",
			input: `register command "myapp status" description "Show status"`,
			want: &CommandDef{
				Name:        "myapp status",
				Description: "Show status",
				Timeout:     DefaultCommandTimeout,
			},
		},
		{
			name:  "with args",
			input: `register command "myapp check" description "Check component" args "<component>"`,
			want: &CommandDef{
				Name:        "myapp check",
				Description: "Check component",
				Args:        "<component>",
				Timeout:     DefaultCommandTimeout,
			},
		},
		{
			name:  "with completable",
			input: `register command "myapp status" description "Show status" args "<component>" completable`,
			want: &CommandDef{
				Name:        "myapp status",
				Description: "Show status",
				Args:        "<component>",
				Completable: true,
				Timeout:     DefaultCommandTimeout,
			},
		},
		{
			name:  "with timeout seconds",
			input: `register command "myapp dump" description "Dump data" timeout 60s`,
			want: &CommandDef{
				Name:        "myapp dump",
				Description: "Dump data",
				Timeout:     60 * time.Second,
			},
		},
		{
			name:  "with timeout ms",
			input: `register command "myapp quick" description "Quick check" timeout 500ms`,
			want: &CommandDef{
				Name:        "myapp quick",
				Description: "Quick check",
				Timeout:     500 * time.Millisecond,
			},
		},
		{
			name:  "all options",
			input: `register command "myapp full" description "Full command" args "<arg>" completable timeout 120s`,
			want: &CommandDef{
				Name:        "myapp full",
				Description: "Full command",
				Args:        "<arg>",
				Completable: true,
				Timeout:     120 * time.Second,
			},
		},
		{
			name:    "missing command keyword",
			input:   `register "myapp status" description "Show status"`,
			wantErr: true,
		},
		{
			name:    "missing description keyword",
			input:   `register command "myapp status" "Show status"`,
			wantErr: true,
		},
		{
			name:    "empty name",
			input:   `register command "" description "Show status"`,
			wantErr: true,
		},
		{
			name:    "uppercase name rejected",
			input:   `register command "MyApp Status" description "Show status"`,
			wantErr: true,
		},
		{
			name:    "name with quotes rejected",
			input:   `register command "myapp \"test\"" description "Show status"`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tokenize(tt.input)
			// Skip "register" token
			got, err := parseRegisterCommand(tokens[1:])

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.Name != tt.want.Name {
				t.Errorf("Name = %q, want %q", got.Name, tt.want.Name)
			}
			if got.Description != tt.want.Description {
				t.Errorf("Description = %q, want %q", got.Description, tt.want.Description)
			}
			if got.Args != tt.want.Args {
				t.Errorf("Args = %q, want %q", got.Args, tt.want.Args)
			}
			if got.Completable != tt.want.Completable {
				t.Errorf("Completable = %v, want %v", got.Completable, tt.want.Completable)
			}
			if got.Timeout != tt.want.Timeout {
				t.Errorf("Timeout = %v, want %v", got.Timeout, tt.want.Timeout)
			}
		})
	}
}

// TestParseUnregisterCommand verifies unregister command parsing.
//
// VALIDATES: Unregister command format is parsed correctly.
// PREVENTS: Unregistration failures from malformed commands.
func TestParseUnregisterCommand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "basic",
			input: `unregister command "myapp status"`,
			want:  "myapp status",
		},
		{
			name:    "missing command keyword",
			input:   `unregister "myapp status"`,
			wantErr: true,
		},
		{
			name:    "missing name",
			input:   `unregister command`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := tokenize(tt.input)
			// Skip "unregister" token
			got, err := parseUnregisterCommand(tokens[1:])

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// TestParseResponseLine verifies response line parsing.
//
// VALIDATES: @serial done/error and streaming responses are parsed.
// PREVENTS: Response routing failures.
func TestParseResponseLine(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantSerial string
		wantType   string
		wantData   string
		wantOK     bool
	}{
		{
			name:       "done no data",
			input:      "@a done",
			wantSerial: "a",
			wantType:   "done",
			wantData:   "",
			wantOK:     true,
		},
		{
			name:       "done with data",
			input:      `@a done {"status": "ok"}`,
			wantSerial: "a",
			wantType:   "done",
			wantData:   `{"status": "ok"}`,
			wantOK:     true,
		},
		{
			name:       "error",
			input:      `@b error "something went wrong"`,
			wantSerial: "b",
			wantType:   "error",
			wantData:   "something went wrong",
			wantOK:     true,
		},
		{
			name:       "partial streaming",
			input:      `@a+ {"chunk": 1}`,
			wantSerial: "a",
			wantType:   "partial",
			wantData:   `{"chunk": 1}`,
			wantOK:     true,
		},
		{
			name:       "multi-char serial",
			input:      "@bcd done",
			wantSerial: "bcd",
			wantType:   "done",
			wantData:   "",
			wantOK:     true,
		},
		{
			name:   "not a response",
			input:  "announce route 10.0.0.0/24",
			wantOK: false,
		},
		{
			name:   "just @",
			input:  "@",
			wantOK: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			serial, respType, data, ok := parsePluginResponse(tt.input)

			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}

			if serial != tt.wantSerial {
				t.Errorf("serial = %q, want %q", serial, tt.wantSerial)
			}
			if respType != tt.wantType {
				t.Errorf("respType = %q, want %q", respType, tt.wantType)
			}
			if data != tt.wantData {
				t.Errorf("data = %q, want %q", data, tt.wantData)
			}
		})
	}
}
