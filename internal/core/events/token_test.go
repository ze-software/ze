// VALIDATES: config type tokens resolve to an event type plus a direction.
// PREVENTS: a hyphenated type name being misread as a type in one direction.
package events

import "testing"

// TestSplitTypeTokenPlainTypeMeansBoth verifies that a token naming a
// registered type keeps its whole name and grants both directions.
//
// VALIDATES: a plain token is the type, in both directions.
// PREVENTS: a config that says "update" being read as one direction only.
func TestSplitTypeTokenPlainTypeMeansBoth(t *testing.T) {
	name, dir, ok := SplitTypeToken("bgp", "update")
	if !ok {
		t.Fatal("update is a registered bgp event type and must resolve")
	}
	if name != "update" {
		t.Errorf("event type = %q, want update", name)
	}
	if dir != DirBoth {
		t.Errorf("direction = %v, want DirBoth", dir)
	}
}

// TestSplitTypeTokenDirectionSuffix verifies that a token the registry does
// not know is split on its direction suffix.
//
// VALIDATES: "update-received" and "update-sent" name one type each way.
// PREVENTS: a direction-carrying token being refused, which is the gap that
// leaves eight in-tree plugins unable to state their subscription.
func TestSplitTypeTokenDirectionSuffix(t *testing.T) {
	tests := []struct {
		token string
		want  Direction
	}{
		{"update-received", DirReceived},
		{"update-sent", DirSent},
		{"open-received", DirReceived},
		{"state-sent", DirSent},
	}
	for _, tc := range tests {
		name, dir, ok := SplitTypeToken("bgp", tc.token)
		if !ok {
			t.Errorf("%s: must resolve", tc.token)
			continue
		}
		if dir != tc.want {
			t.Errorf("%s: direction = %v, want %v", tc.token, dir, tc.want)
		}
		if got := DirectionToken(name, dir); got != tc.token {
			t.Errorf("%s: round trip = %q", tc.token, got)
		}
	}
}

// TestSplitTypeTokenRegistryWinsOverSplit verifies R-11: a registered type
// whose name ends in a direction word keeps its whole name.
//
// The test registers BOTH "flap" and "flap-sent", so a splitter that cut the
// suffix first would find "flap" registered and answer with it. Only
// registry-first resolution answers "flap-sent".
//
// VALIDATES: R-11, the whole token is resolved before any suffix is cut.
// PREVENTS: `receive [ update-rpki ]` being read as "update" in some
// direction the day a plugin registers a type ending in "-sent".
func TestSplitTypeTokenRegistryWinsOverSplit(t *testing.T) {
	if err := RegisterEventType("bgp", "flap"); err != nil {
		t.Fatalf("register flap: %v", err)
	}
	if err := RegisterEventType("bgp", "flap-sent"); err != nil {
		t.Fatalf("register flap-sent: %v", err)
	}

	name, dir, ok := SplitTypeToken("bgp", "flap-sent")
	if !ok {
		t.Fatal("flap-sent is registered and must resolve")
	}
	if name != "flap-sent" {
		t.Errorf("event type = %q, want flap-sent: the registry must be consulted before the suffix is cut", name)
	}
	if dir != DirBoth {
		t.Errorf("direction = %v, want DirBoth: a registered type carries no direction", dir)
	}
}

// TestSplitTypeTokenRefusesUnknown verifies that a token naming no registered
// type is refused, with or without a direction suffix.
//
// VALIDATES: the resolver fails closed.
// PREVENTS: a typo in a config leaf granting an event type that cannot exist.
func TestSplitTypeTokenRefusesUnknown(t *testing.T) {
	for _, token := range []string{"bogus", "bogus-sent", "bogus-received", "*", "*-sent", "-sent", "all"} {
		if _, _, ok := SplitTypeToken("bgp", token); ok {
			t.Errorf("%q must not resolve to an event type", token)
		}
	}
}

// TestSplitTypeTokenRefusesUnknownNamespace verifies the namespace is honored.
//
// VALIDATES: a type registered in one namespace does not resolve in another.
// PREVENTS: a receive list granting an event the peer's namespace never emits.
func TestSplitTypeTokenRefusesUnknownNamespace(t *testing.T) {
	if _, _, ok := SplitTypeToken("no-such-namespace", "update"); ok {
		t.Error("an unknown namespace must resolve nothing")
	}
}

// TestDirectionTokenSpellsEachDirection verifies the token a completion list
// offers for each direction.
//
// VALIDATES: DirectionToken is the inverse of SplitTypeToken.
// PREVENTS: completion offering a token the parser then refuses.
func TestDirectionTokenSpellsEachDirection(t *testing.T) {
	tests := []struct {
		dir  Direction
		want string
	}{
		{DirReceived, "update-received"},
		{DirSent, "update-sent"},
		{DirBoth, "update"},
		{DirUnspecified, "update"},
	}
	for _, tc := range tests {
		if got := DirectionToken("update", tc.dir); got != tc.want {
			t.Errorf("DirectionToken(update, %v) = %q, want %q", tc.dir, got, tc.want)
		}
	}
}

// TestDirectionWordHintNamesTheReplacement verifies the message a bare
// direction word gets.
//
// VALIDATES: "sent" alone is refused with the spelling that replaces it.
// PREVENTS: an operator who wrote the retired `receive [ sent ]` being told
// only that the token is invalid.
func TestDirectionWordHintNamesTheReplacement(t *testing.T) {
	hint := DirectionWordHint("sent")
	if hint == "" {
		t.Fatal("a bare direction word must be explained")
	}
	if !contains(hint, "update-sent") {
		t.Errorf("hint = %q, must name the replacement spelling", hint)
	}
	if DirectionWordHint("received") == "" {
		t.Error("received is a direction word too")
	}
	if DirectionWordHint("both") == "" {
		t.Error("both is a direction word too")
	}
	if DirectionWordHint("update") != "" {
		t.Error("an event type is not a direction word")
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
