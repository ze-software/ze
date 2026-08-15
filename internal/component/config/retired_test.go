package config

import (
	"strings"
	"testing"
)

// VALIDATES: AC-12 — a ze-native config that still spells the retired
// `process <name>` block is refused at parse, and the refusal names the
// keyword that replaces it.
func TestParseRefusesRetiredProcessKeyword(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}

	retired := `bgp {
	peer peer1 {
		process looking-glass {
			receive [ state ]
		}
	}
}
`
	if _, err := NewParser(schema).Parse(retired); err == nil {
		t.Fatal("the retired process keyword parsed; the parser must accept one spelling only")
	} else {
		msg := err.Error()
		if !strings.Contains(msg, "attach process") {
			t.Errorf("refusal does not name the replacement keyword: %s", msg)
		}
	}

	current := `bgp {
	peer peer1 {
		attach process looking-glass {
			receive [ state ]
		}
	}
}
`
	tree, err := NewParser(schema).Parse(current)
	if err != nil {
		t.Fatalf("attach process must parse: %v", err)
	}
	bgp := tree.GetContainer("bgp")
	if bgp == nil {
		t.Fatal("no bgp container")
	}
	peer := bgp.GetList("peer")["peer1"]
	if peer == nil {
		t.Fatal("no peer1 entry")
	}
	attach := peer.GetContainer("attach")
	if attach == nil {
		t.Fatal("no attach container under the peer")
	}
	if _, ok := attach.GetList("process")["looking-glass"]; !ok {
		t.Error("the attached process is missing from the peer tree")
	}
}

// VALIDATES: the flat spelling an operator writes is the one ze prints back,
// so a load-and-show round trip produces no diff (ze:flatten, flatten.go).
func TestFlattenedContainerRoundTrips(t *testing.T) {
	schema, err := YANGSchema()
	if err != nil {
		t.Fatalf("YANG schema: %v", err)
	}

	input := `bgp {
	peer peer1 {
		attach process alpha {
			receive [ state ]
		}
		attach process beta {
			send [ update ]
		}
	}
}
`
	tree, err := NewParser(schema).Parse(input)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out := Serialize(tree, schema)
	if !strings.Contains(out, "attach process alpha {") {
		t.Errorf("serialized output does not carry the flat spelling:\n%s", out)
	}
	if !strings.Contains(out, "attach process beta {") {
		t.Errorf("serialized output lost the second attachment:\n%s", out)
	}
	if strings.Contains(out, "attach {") {
		t.Errorf("serialized output nested the ze:flatten container:\n%s", out)
	}

	again, err := NewParser(schema).Parse(out)
	if err != nil {
		t.Fatalf("reparse of serialized output: %v", err)
	}
	if Serialize(again, schema) != out {
		t.Error("serialize is not stable across a reparse")
	}
}
