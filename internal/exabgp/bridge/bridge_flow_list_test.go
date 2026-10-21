package bridge

import "testing"

// TestFlowValueListKeepsExaBGPMeaning proves an ExaBGP flow value list reaches
// ze's update text as a list of alternatives, in both the family form and the
// `flow route` form.
//
// VALIDATES: ExaBGP 6.0 reads `[ =80 =443 ]` as 80 OR 443 and `>8080&<8088` as
// one AND term (configuration/grammar/types/flow.py, _bracketed and
// _expression). Ze's update text reads successive values as OR and "&" as AND
// (docs/architecture/api/architecture.md, FlowSpec NLRI), so the bridge drops
// the brackets and passes each term through unchanged.
// PREVENTS: the family form sending `[` and `]` into update text, which ze
// refuses as a value, so an ExaBGP script's list route was never announced.
func TestFlowValueListKeepsExaBGPMeaning(t *testing.T) {
	t.Parallel()
	const head = "send bgp 10.0.0.1 update text extended-community [rate-limit:0] mp ipv4/flow add destination-ipv4 10.0.0.0/24 "
	const flowHead = "send bgp * update text extended-community [rate-limit:0] mp ipv4/flow add destination-ipv4 10.0.0.0/24 "
	for _, tc := range []struct {
		line string
		want string
	}{
		{
			"neighbor 10.0.0.1 announce ipv4 flow destination-ipv4 10.0.0.0/24 destination-port [ =80 =443 ] extended-community [rate-limit:0]",
			head + "destination-port =80 =443",
		},
		{
			"neighbor 10.0.0.1 announce ipv4 flow destination-ipv4 10.0.0.0/24 destination-port [=80 =443] extended-community [rate-limit:0]",
			head + "destination-port =80 =443",
		},
		{
			"neighbor 10.0.0.1 announce ipv4 flow destination-ipv4 10.0.0.0/24 destination-port [ >8080&<8088 =3128 ] extended-community [rate-limit:0]",
			head + "destination-port >8080&<8088 =3128",
		},
		{
			"announce flow route { match { destination 10.0.0.0/24; destination-port [ =80 =443 ]; } then { discard; } }",
			flowHead + "destination-port =80 =443",
		},
		{
			"announce flow route { match { destination 10.0.0.0/24; destination-port [ >8080&<8088 =3128 ]; } then { discard; } }",
			flowHead + "destination-port >8080&<8088 =3128",
		},
	} {
		if got := translatedCommand(t, tc.line); got != tc.want {
			t.Errorf("%s\n got %s\nwant %s", tc.line, got, tc.want)
		}
	}
}
