// VALIDATES: registry.Register validates and deep-clones ParticipantNodes, Sends, and Receives, and CheckBusDeclarationsMatch detects separate-process drift.
// PREVENTS: malformed bus declarations entering the registry or a separate-process internal plugin diverging from its compiled registration.
package registry

import (
	"testing"

	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// VALIDATES: Register accepts well-formed ParticipantNodes, Sends, and Receives
// and stores defensive clones independent of caller mutation.
// PREVENTS: shared slice aliasing between caller and global plugin registry.
func TestRegisterBusDeclarationsValidAndCloned(t *testing.T) {
	snap := Snapshot()
	Reset()
	t.Cleanup(func() { Restore(snap) })

	nodes := []rpc.ParticipantNodeDecl{{
		Module:          "ze-bgp-conf",
		Prefix:          "bgp",
		SchemaPath:      "/bgp:bgp/bgp:peer",
		SecondaryIPPath: "/bgp/ip",
	}}
	sends := []rpc.MessageSendDecl{
		{
			Message: "bgp:update",
			Tags: []rpc.MessageTagDecl{{
				Name:   "direction",
				Values: []string{"received", "sent"},
			}},
			DefaultDestinations: []string{"/bgp"},
		},
		{
			Message: "cli:show bgp peer",
		},
	}
	receives := []rpc.MessageReceiveDecl{{
		Message: "bgp:state",
		From:    []string{"/bgp"},
		Tags:    []rpc.MessageTagDecl{{Name: "source", Values: []string{"bgp"}}},
	}}

	reg := validReg("bus-decl-valid")
	reg.ParticipantNodes = nodes
	reg.Sends = sends
	reg.Receives = receives

	if err := Register(reg); err != nil {
		t.Fatalf("Register failed: %v", err)
	}

	// Mutate caller slices after Register; Lookup must return original values.
	nodes[0].SchemaPath = "/mutated"
	sends[0].Tags[0].Values[0] = "mutated"
	receives[0].From[0] = "/mutated"

	got := Lookup("bus-decl-valid")
	if got == nil {
		t.Fatal("Lookup returned nil")
	}
	if got.ParticipantNodes[0].SchemaPath != "/bgp:bgp/bgp:peer" {
		t.Fatalf("ParticipantNodes mutated: %q", got.ParticipantNodes[0].SchemaPath)
	}
	if got.Sends[0].Tags[0].Values[0] != "received" {
		t.Fatalf("Sends tag mutated: %q", got.Sends[0].Tags[0].Values[0])
	}
	if got.Receives[0].From[0] != "/bgp" {
		t.Fatalf("Receives from mutated: %q", got.Receives[0].From[0])
	}
}

// VALIDATES: Register refuses malformed ParticipantNodes, Sends, and Receives
// (empty schema path, missing namespace, control characters, duplicates).
// PREVENTS: invalid bus declarations from passing init() registration.
func TestRegisterBusDeclarationsRefusesMalformed(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*Registration)
	}{
		{
			name: "empty schema path",
			mutate: func(r *Registration) {
				r.ParticipantNodes = []rpc.ParticipantNodeDecl{{Module: "ze-test"}}
			},
		},
		{
			name: "control char in schema path",
			mutate: func(r *Registration) {
				r.ParticipantNodes = []rpc.ParticipantNodeDecl{{SchemaPath: "/bgp\x1b:peer"}}
			},
		},
		{
			name: "duplicate schema path",
			mutate: func(r *Registration) {
				r.ParticipantNodes = []rpc.ParticipantNodeDecl{
					{SchemaPath: "/bgp:bgp/bgp:peer"},
					{SchemaPath: "/bgp:bgp/bgp:peer"},
				}
			},
		},
		{
			name: "send missing namespace",
			mutate: func(r *Registration) {
				r.Sends = []rpc.MessageSendDecl{{Message: "update"}}
			},
		},
		{
			name: "send control char",
			mutate: func(r *Registration) {
				r.Sends = []rpc.MessageSendDecl{{Message: "bgp:up\ndate"}}
			},
		},
		{
			name: "send duplicate message",
			mutate: func(r *Registration) {
				r.Sends = []rpc.MessageSendDecl{{Message: "bgp:update"}, {Message: "bgp:update"}}
			},
		},
		{
			name: "send bad tag value",
			mutate: func(r *Registration) {
				r.Sends = []rpc.MessageSendDecl{{
					Message: "bgp:update",
					Tags:    []rpc.MessageTagDecl{{Name: "direction", Values: []string{"bad\tval"}}},
				}}
			},
		},
		{
			name: "receive missing namespace",
			mutate: func(r *Registration) {
				r.Receives = []rpc.MessageReceiveDecl{{Message: "state"}}
			},
		},
		{
			name: "receive control char in from",
			mutate: func(r *Registration) {
				r.Receives = []rpc.MessageReceiveDecl{{
					Message: "bgp:state",
					From:    []string{"/bgp\x1b"},
				}}
			},
		},
		{
			name: "receive duplicate tag",
			mutate: func(r *Registration) {
				r.Receives = []rpc.MessageReceiveDecl{{
					Message: "bgp:update",
					Tags: []rpc.MessageTagDecl{
						{Name: "direction", Values: []string{"received"}},
						{Name: "direction", Values: []string{"sent"}},
					},
				}}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := Snapshot()
			Reset()
			t.Cleanup(func() { Restore(snap) })

			reg := validReg("bus-decl-bad")
			tc.mutate(&reg)
			if err := Register(reg); err == nil {
				t.Fatalf("Register accepted malformed declaration (%s)", tc.name)
			}
		})
	}
}

// VALIDATES: CheckBusDeclarationsMatch accepts identical declarations and
// refuses any mismatch in ParticipantNodes, Sends, or Receives.
// PREVENTS: a separate-process internal plugin silently drifting from its compiled registration.
func TestCheckBusDeclarationsMatch(t *testing.T) {
	compiled := &Registration{
		Name: "bgp-rs",
		ParticipantNodes: []rpc.ParticipantNodeDecl{{
			Module:     "ze-rs-conf",
			SchemaPath: "/bgp:bgp/rs:route-server",
		}},
		Sends: []rpc.MessageSendDecl{{
			Message: "bgp:update",
		}},
		Receives: []rpc.MessageReceiveDecl{{
			Message: "bgp:update",
			Tags:    []rpc.MessageTagDecl{{Name: "direction", Values: []string{"received"}}},
		}},
	}

	matching := &rpc.DeclareRegistrationInput{
		ParticipantNodes: []rpc.ParticipantNodeDecl{{
			Module:     "ze-rs-conf",
			SchemaPath: "/bgp:bgp/rs:route-server",
		}},
		Sends: []rpc.MessageSendDecl{{
			Message: "bgp:update",
		}},
		Receives: []rpc.MessageReceiveDecl{{
			Message: "bgp:update",
			Tags:    []rpc.MessageTagDecl{{Name: "direction", Values: []string{"received"}}},
		}},
	}
	if err := CheckBusDeclarationsMatch("my-rs", compiled, matching); err != nil {
		t.Fatalf("matching declarations refused: %v", err)
	}

	badNodes := *matching
	badNodes.ParticipantNodes = nil
	if err := CheckBusDeclarationsMatch("my-rs", compiled, &badNodes); err == nil {
		t.Fatal("expected error for mismatched ParticipantNodes")
	}

	badSends := *matching
	badSends.Sends = []rpc.MessageSendDecl{{Message: "bgp:raw"}}
	if err := CheckBusDeclarationsMatch("my-rs", compiled, &badSends); err == nil {
		t.Fatal("expected error for mismatched Sends")
	}

	badReceives := *matching
	badReceives.Receives = []rpc.MessageReceiveDecl{{Message: "bgp:update"}}
	if err := CheckBusDeclarationsMatch("my-rs", compiled, &badReceives); err == nil {
		t.Fatal("expected error for mismatched Receives")
	}
}
