package rpc

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestCurrentSelectionExactReply(t *testing.T) {
	in := CurrentSelectionInput{Destination: "192.0.2.1", DestinationSession: 7, DeadlineMS: time.Now().Add(time.Second).UnixMilli(),
		Keys: []CurrentSelectionKey{{Family: "ipv4 unicast", KeyNLRI: []byte{24, 192, 0, 2}, Revision: 3}}}
	require.NoError(t, in.Validate(time.Now()))
	out := CurrentSelectionOutput{Destination: in.Destination, DestinationSession: 7, Cut: 500,
		Results: []CurrentSelectionResult{{Family: in.Keys[0].Family, KeyNLRI: in.Keys[0].KeyNLRI, Revision: 3,
			Presence: CurrentSelectionPresent, CurrentRevision: 3}}}
	require.NoError(t, out.Validate(in))
	out.Results[0].Revision = 4
	require.Error(t, out.Validate(in))
}

func TestCurrentSelectionRejectsMalformedAndOversized(t *testing.T) {
	in := CurrentSelectionInput{Destination: "192.0.2.1", DestinationSession: 7, DeadlineMS: time.Now().Add(time.Second).UnixMilli(),
		Keys: []CurrentSelectionKey{{Family: "ipv4 unicast", KeyNLRI: []byte{0}, Revision: 3}}}
	for _, change := range []func(*CurrentSelectionInput){
		func(in *CurrentSelectionInput) { in.Destination = "not-an-address" },
		func(in *CurrentSelectionInput) { in.DestinationSession = 0 },
		func(in *CurrentSelectionInput) { in.DeadlineMS = 0 },
		func(in *CurrentSelectionInput) { in.Keys = nil },
		func(in *CurrentSelectionInput) { in.Keys[0].Revision = 0 },
		func(in *CurrentSelectionInput) { in.Keys[0].KeyNLRI = nil },
		func(in *CurrentSelectionInput) { in.Keys[0].Family = "" },
		func(in *CurrentSelectionInput) { in.Keys[0].Family = strings.Repeat("x", MaxMessageSize) },
	} {
		copy := in
		copy.Keys = append([]CurrentSelectionKey(nil), in.Keys...)
		change(&copy)
		require.Error(t, copy.Validate(time.Now()))
	}
	// Explicit empty-state cut/revision zeros are valid; missing zeros are not.
	var out CurrentSelectionOutput
	require.Error(t, json.Unmarshal([]byte(`{"destination":"192.0.2.1","destination-session":7,"results":[]}`), &out))
	require.NoError(t, json.Unmarshal([]byte(`{"destination":"192.0.2.1","destination-session":7,"cut":0,"results":[]}`), &out))
}

func TestCurrentSelectionAbsenceNeedsNoHistory(t *testing.T) {
	in := CurrentSelectionInput{Destination: "192.0.2.1", DestinationSession: 7,
		Keys: []CurrentSelectionKey{{Family: "ipv4 unicast", KeyNLRI: []byte{0}, Revision: 11}}}
	out := CurrentSelectionOutput{Destination: in.Destination, DestinationSession: 7, Cut: 500,
		Results: []CurrentSelectionResult{{Family: in.Keys[0].Family, KeyNLRI: in.Keys[0].KeyNLRI, Revision: 11, Presence: CurrentSelectionAbsent}}}
	require.NoError(t, out.Validate(in))
	out.Results[0].Presence = CurrentSelectionUnspecified
	require.Error(t, out.Validate(in))
}

func TestCurrentSelectionReplyRejectsReboundScope(t *testing.T) {
	in := CurrentSelectionInput{Destination: "192.0.2.1", DestinationSession: 7,
		Keys: []CurrentSelectionKey{{Family: "ipv4 unicast", KeyNLRI: []byte{0}, Revision: 3}}}
	for _, change := range []func(*CurrentSelectionOutput){
		func(out *CurrentSelectionOutput) { out.Destination = "192.0.2.2" },
		func(out *CurrentSelectionOutput) { out.DestinationSession = 8 },
		func(out *CurrentSelectionOutput) { out.Results = nil },
		func(out *CurrentSelectionOutput) { out.Results[0].Family = "ipv6 unicast" },
		func(out *CurrentSelectionOutput) { out.Results[0].KeyNLRI = []byte{1} },
		func(out *CurrentSelectionOutput) { out.Results[0].Presence = 99 },
		func(out *CurrentSelectionOutput) { out.Results[0].CurrentRevision = 0 },
		func(out *CurrentSelectionOutput) { out.Results[0].CurrentRevision = 501 },
		func(out *CurrentSelectionOutput) { out.Results[0].Presence = CurrentSelectionAbsent },
	} {
		out := CurrentSelectionOutput{Destination: in.Destination, DestinationSession: 7, Cut: 500,
			Results: []CurrentSelectionResult{{Family: in.Keys[0].Family, KeyNLRI: in.Keys[0].KeyNLRI, Revision: 3,
				Presence: CurrentSelectionPresent, CurrentRevision: 3}}}
		change(&out)
		require.Error(t, out.Validate(in))
	}
}
