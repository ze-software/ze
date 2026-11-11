// Design: docs/architecture/mpls/mpls-kernel.md -- the one label validation every label writer applies
package mplsfib

import (
	"errors"
	"strings"
	"testing"
)

func stackOf(depth int) []uint32 {
	labels := make([]uint32, depth)
	for i := range labels {
		labels[i] = uint32(i + 100)
	}
	return labels
}

// TestValidateOutLabels pins the outgoing-stack rule every label writer
// shares. The cases are the ones fib-kernel's own copy carried
// (TestMPLSLabelValidation, TestMPLSLabelValidationBoundary), moved here
// with that copy.
//
// VALIDATES: AC-23 -- one validator: an empty stack, a stack deeper than
// MaxLabelStack and a label above 2^20-1 are refused; 0, MaxLabel and a
// stack of exactly MaxLabelStack pass.
func TestValidateOutLabels(t *testing.T) {
	cases := []struct {
		name    string
		labels  []uint32
		wantErr string
		wantIs  error
	}{
		{name: "valid single label", labels: []uint32{100}},
		{name: "valid label stack", labels: []uint32{100, 200, 300}},
		{name: "label 0 valid", labels: []uint32{0}},
		{name: "label max valid", labels: []uint32{MaxLabel}},
		{name: "stack depth 16 valid", labels: stackOf(16)},
		{name: "empty stack", labels: []uint32{}, wantIs: ErrEmptyLabelStack},
		{name: "nil stack", labels: nil, wantIs: ErrEmptyLabelStack},
		{name: "label max+1", labels: []uint32{MaxLabel + 1}, wantErr: "exceeds 20-bit maximum"},
		{name: "label 1048576", labels: []uint32{1048576}, wantErr: "exceeds 20-bit maximum"},
		{name: "large label in middle", labels: []uint32{100, MaxLabel + 1, 200}, wantErr: "exceeds 20-bit maximum"},
		{name: "stack depth 17", labels: stackOf(17), wantErr: "exceeds limit"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateOutLabels(c.labels)
			if c.wantErr == "" && c.wantIs == nil {
				if err != nil {
					t.Fatalf("ValidateOutLabels(%v) = %v, want nil", c.labels, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("ValidateOutLabels(%v) accepted", c.labels)
			}
			if c.wantIs != nil && !errors.Is(err, c.wantIs) {
				t.Fatalf("ValidateOutLabels(%v) = %v, want %v", c.labels, err, c.wantIs)
			}
			if c.wantErr != "" && !strings.Contains(err.Error(), c.wantErr) {
				t.Fatalf("ValidateOutLabels(%v) = %v, want it to say %q", c.labels, err, c.wantErr)
			}
		})
	}
}

// TestMaxLabelIsTwentyBits pins the bound to the RFC 3032 field width.
func TestMaxLabelIsTwentyBits(t *testing.T) {
	if MaxLabel != 1048575 {
		t.Fatalf("MaxLabel = %d, want 1048575", MaxLabel)
	}
	if MaxLabelStack != 16 {
		t.Fatalf("MaxLabelStack = %d, want 16", MaxLabelStack)
	}
}

// TestValidateEntry pins the entry rules both writers apply before they
// program anything: a swap or pop carries no table and a 20-bit in-label, a
// swap a valid stack, a pop none, a push an empty (Implicit NULL) or valid
// stack, a remove only its key.
//
// VALIDATES: AC-23 -- the refusals fib-kernel applied, from one function.
func TestValidateEntry(t *testing.T) {
	cases := []struct {
		name  string
		entry Entry
		ok    bool
	}{
		{"swap", Entry{Action: ActionAdd, Op: OpSwap, InLabel: 100, OutLabels: []uint32{200}}, true},
		{"swap backup stack", Entry{Action: ActionAdd, Op: OpSwap, InLabel: 100, OutLabels: []uint32{300, 200}}, true},
		{"swap in-label max", Entry{Action: ActionAdd, Op: OpSwap, InLabel: MaxLabel, OutLabels: []uint32{200}}, true},
		{"swap in-label max+1", Entry{Action: ActionAdd, Op: OpSwap, InLabel: MaxLabel + 1, OutLabels: []uint32{200}}, false},
		{"swap empty stack", Entry{Action: ActionAdd, Op: OpSwap, InLabel: 100}, false},
		{"swap stack too deep", Entry{Action: ActionAdd, Op: OpSwap, InLabel: 100, OutLabels: stackOf(17)}, false},
		{"swap private table", Entry{Action: ActionAdd, Op: OpSwap, InLabel: 100, OutLabels: []uint32{200}, TableID: 7}, false},
		{"pop", Entry{Action: ActionAdd, Op: OpPop, InLabel: 100}, true},
		{"pop with out-label", Entry{Action: ActionAdd, Op: OpPop, InLabel: 100, OutLabels: []uint32{200}}, false},
		{"pop private table", Entry{Action: ActionAdd, Op: OpPop, InLabel: 100, TableID: 7}, false},
		{"push implicit null", Entry{Action: ActionAdd, Op: OpPush}, true},
		{"push stack", Entry{Action: ActionAdd, Op: OpPush, OutLabels: []uint32{200}, TableID: 7}, true},
		{"push label max+1", Entry{Action: ActionAdd, Op: OpPush, OutLabels: []uint32{MaxLabel + 1}}, false},
		{"remove swap", Entry{Action: ActionRemove, Op: OpSwap, InLabel: 100}, true},
		{"remove pop private table", Entry{Action: ActionRemove, Op: OpPop, InLabel: 100, TableID: 7}, false},
		{"remove in-label max+1", Entry{Action: ActionRemove, Op: OpSwap, InLabel: MaxLabel + 1}, false},
		{"remove push", Entry{Action: ActionRemove, Op: OpPush, TableID: 7}, true},
		{"remove label source", Entry{Action: ActionRemoveLabelSource, Source: 2}, true},
		{"unspecified action", Entry{Op: OpSwap, InLabel: 100, OutLabels: []uint32{200}}, false},
		{"unknown action", Entry{Action: 9, Op: OpSwap, InLabel: 100, OutLabels: []uint32{200}}, false},
		{"unspecified op", Entry{Action: ActionAdd}, false},
		{"unknown op", Entry{Action: ActionAdd, Op: 9}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := ValidateEntry(&c.entry)
			if c.ok && err != nil {
				t.Fatalf("ValidateEntry(%+v) = %v, want nil", c.entry, err)
			}
			if !c.ok && err == nil {
				t.Fatalf("ValidateEntry(%+v) accepted", c.entry)
			}
		})
	}
}
