// Design: docs/architecture/api/process-protocol.md -- exact marker receipts.
package rpc

import (
	"strings"
	"testing"
	"time"
)

func selectedRefreshFixture() (SelectRefreshInput, *SelectRefreshOutput) {
	input := SelectRefreshInput{Destination: "192.0.2.7", PeerOwner: 17,
		DestinationSession: 71, Family: "ipv4/unicast", Action: SelectedRefreshBegin,
		DeadlineMS: time.Now().Add(time.Second).UnixMilli()}
	output := &SelectRefreshOutput{Destination: input.Destination, PeerOwner: input.PeerOwner,
		DestinationSession: input.DestinationSession, Family: input.Family, Action: input.Action,
		Outcome: SelectedRefreshWritten}
	return input, output
}

// Every receipt echoes one exact destination and marker; an omitted or forged
// echo cannot satisfy the selecting worker's completion barrier.
func TestSelectedRefreshReceiptRequiresExactScope(t *testing.T) {
	input, output := selectedRefreshFixture()
	if err := output.Validate(input); err != nil {
		t.Fatal(err)
	}
	cases := []func(*SelectRefreshOutput){
		func(out *SelectRefreshOutput) { out.Destination = "192.0.2.8" },
		func(out *SelectRefreshOutput) { out.PeerOwner++ },
		func(out *SelectRefreshOutput) { out.DestinationSession++ },
		func(out *SelectRefreshOutput) { out.Family = "ipv6/unicast" },
		func(out *SelectRefreshOutput) { out.Action = SelectedRefreshEnd },
		func(out *SelectRefreshOutput) { out.Outcome = SelectedRefreshOutcomeUnspecified },
		func(out *SelectRefreshOutput) { out.Outcome = 99 },
	}
	for _, mutate := range cases {
		_, result := selectedRefreshFixture()
		mutate(result)
		if err := result.Validate(input); err == nil {
			t.Fatal("mismatched marker receipt accepted")
		}
	}
	var missing *SelectRefreshOutput
	if err := missing.Validate(input); err == nil {
		t.Fatal("missing marker receipt accepted")
	}
	output.Outcome = SelectedRefreshNotNegotiated
	if err := output.Validate(input); err != nil {
		t.Fatal(err)
	}
}

func TestSelectedRefreshInputRefusesUnrecordedScope(t *testing.T) {
	mutations := []func(*SelectRefreshInput){
		func(input *SelectRefreshInput) { input.Destination = "" },
		func(input *SelectRefreshInput) { input.Destination = strings.Repeat("x", 129) },
		func(input *SelectRefreshInput) { input.Family = "" },
		func(input *SelectRefreshInput) { input.Family = strings.Repeat("x", 65) },
		func(input *SelectRefreshInput) { input.PeerOwner = 0 },
		func(input *SelectRefreshInput) { input.DestinationSession = 0 },
		func(input *SelectRefreshInput) { input.Action = SelectedRefreshUnspecified },
		func(input *SelectRefreshInput) { input.Action = 99 },
		func(input *SelectRefreshInput) { input.DeadlineMS = 0 },
	}
	for _, mutate := range mutations {
		input, _ := selectedRefreshFixture()
		mutate(&input)
		if err := input.Validate(time.Now()); err == nil {
			t.Fatal("unrecorded marker request accepted")
		}
	}
}
