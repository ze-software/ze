// Design: docs/architecture/core-design.md — Linux collector tests

//go:build linux

package support

import (
	"testing"

	"github.com/google/nftables"
)

func TestParseDmesgLine(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantNil bool
		level   int
		msg     string
	}{
		{
			name:  "valid entry",
			input: "6,1234,567890,-;kernel: test message\n",
			level: 6,
			msg:   "kernel: test message",
		},
		{
			name:  "level 0 emergency",
			input: "0,100,999,-;panic: system halted",
			level: 0,
			msg:   "panic: system halted",
		},
		{
			name:  "message with semicolons",
			input: "4,500,12345,-;key=value;extra;data",
			level: 4,
			msg:   "key=value;extra;data",
		},
		{
			name:    "no semicolon",
			input:   "malformed line without semicolon",
			wantNil: true,
		},
		{
			name:    "empty string",
			input:   "",
			wantNil: true,
		},
		{
			name:    "too few header fields",
			input:   "6;message",
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDmesgLine(tt.input)
			if tt.wantNil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}
			if result == nil {
				t.Fatal("expected non-nil result")
			}
			if got, _ := result["level"].(int); got != tt.level {
				t.Errorf("level = %d, want %d", got, tt.level)
			}
			if got, _ := result["message"].(string); got != tt.msg {
				t.Errorf("message = %q, want %q", got, tt.msg)
			}
		})
	}
}

func TestCategorizeFD(t *testing.T) {
	tests := []struct {
		target string
		want   string
	}{
		{"socket:[12345]", "socket"},
		{"pipe:[67890]", "pipe"},
		{"anon_inode:[eventpoll]", "anon-inode"},
		{"/dev/null", "device"},
		{"/dev/pts/0", "device"},
		{"/proc/self/fd", "file"},
		{"/home/user/data.txt", "file"},
		{"(unknown)", "unknown"},
	}
	for _, tt := range tests {
		got := categorizeFD(tt.target)
		if got != tt.want {
			t.Errorf("categorizeFD(%q) = %q, want %q", tt.target, got, tt.want)
		}
	}
}

func TestTableFamilyName(t *testing.T) {
	tests := []struct {
		input nftables.TableFamily
		want  string
	}{
		{nftables.TableFamilyIPv4, "ip"},
		{nftables.TableFamilyIPv6, "ip6"},
		{nftables.TableFamilyINet, "inet"},
		{nftables.TableFamilyARP, "arp"},
		{nftables.TableFamilyNetdev, "netdev"},
		{nftables.TableFamilyBridge, "bridge"},
		{99, "unknown"},
	}
	for _, tt := range tests {
		got := tableFamilyName(tt.input)
		if got != tt.want {
			t.Errorf("tableFamilyName(%d) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
