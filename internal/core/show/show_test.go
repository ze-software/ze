// VALIDATES: AC-1 through AC-5, AC-9, AC-11 through AC-13
// PREVENTS: enricher key collision, panic propagation, ordering regression

package show

import (
	"testing"
)

func TestRegisterAndEnrich(t *testing.T) {
	t.Cleanup(ResetForTest)

	called := false
	MustRegister("show subscriber detail", "cos", Enricher{
		Detail: func(base map[string]any) {
			called = true
			base["cos"] = "residential"
		},
	})

	base := map[string]any{"id": "s1"}
	Enrich("show subscriber detail", base)

	if !called {
		t.Fatal("enricher Detail was not called")
	}
	if base["cos"] != "residential" {
		t.Fatalf("expected cos=residential, got %v", base["cos"])
	}
	if base["id"] != "s1" {
		t.Fatal("base key was removed")
	}
}

func TestEnrichNoRegistrations(t *testing.T) {
	t.Cleanup(ResetForTest)

	base := map[string]any{"id": "s1", "state": "active"}
	Enrich("show subscriber detail", base)

	if len(base) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(base))
	}
}

func TestEnrichMultipleEnrichers(t *testing.T) {
	t.Cleanup(ResetForTest)

	MustRegister("show subscriber detail", "cos", Enricher{
		Detail: func(base map[string]any) { base["cos"] = "gold" },
	})
	MustRegister("show subscriber detail", "radius", Enricher{
		Detail: func(base map[string]any) { base["radius"] = "attrs" },
	})

	base := map[string]any{"id": "s1"}
	Enrich("show subscriber detail", base)

	if base["cos"] != "gold" {
		t.Fatalf("expected cos=gold, got %v", base["cos"])
	}
	if base["radius"] != "attrs" {
		t.Fatalf("expected radius=attrs, got %v", base["radius"])
	}
}

func TestRegisterDuplicateKeyRejects(t *testing.T) {
	t.Cleanup(ResetForTest)

	err := Register("show subscriber detail", "cos", Enricher{
		Detail: func(base map[string]any) {},
	})
	if err != nil {
		t.Fatalf("first registration should succeed: %v", err)
	}

	err = Register("show subscriber detail", "cos", Enricher{
		Detail: func(base map[string]any) {},
	})
	if err == nil {
		t.Fatal("expected error on duplicate key, got nil")
	}
}

func TestEnrichBrief(t *testing.T) {
	t.Cleanup(ResetForTest)

	MustRegister("show subscriber", "cos", Enricher{
		Detail: func(base map[string]any) { base["cos-detail"] = "full" },
		Brief:  func(base map[string]any) { base["cos"] = "residential" },
	})

	base := map[string]any{"id": "s1"}
	EnrichBrief("show subscriber", base)

	if base["cos"] != "residential" {
		t.Fatalf("expected cos=residential, got %v", base["cos"])
	}
	if _, ok := base["cos-detail"]; ok {
		t.Fatal("Detail should not be called by EnrichBrief")
	}
}

func TestEnrichModifiesBaseInPlace(t *testing.T) {
	t.Cleanup(ResetForTest)

	MustRegister("cmd", "e1", Enricher{
		Detail: func(base map[string]any) { base["added"] = true },
	})

	base := map[string]any{"existing": "value"}
	Enrich("cmd", base)

	if base["added"] != true {
		t.Fatal("enricher did not modify base in place")
	}
	if base["existing"] != "value" {
		t.Fatal("existing key was removed")
	}
}

func TestResetForTest(t *testing.T) {
	MustRegister("cmd", "e1", Enricher{
		Detail: func(base map[string]any) { base["x"] = 1 },
	})

	ResetForTest()

	base := map[string]any{}
	Enrich("cmd", base)

	if len(base) != 0 {
		t.Fatal("expected empty map after ResetForTest, enricher still registered")
	}
}

func TestEnrichBriefSkipsNilBrief(t *testing.T) {
	t.Cleanup(ResetForTest)

	MustRegister("show subscriber", "cos", Enricher{
		Detail: func(base map[string]any) { base["detail-only"] = true },
	})

	base := map[string]any{"id": "s1"}
	EnrichBrief("show subscriber", base)

	if _, ok := base["detail-only"]; ok {
		t.Fatal("Detail should not be called when Brief is nil")
	}
	if len(base) != 1 {
		t.Fatalf("expected 1 key, got %d", len(base))
	}
}

func TestEnrichRecoversPanic(t *testing.T) {
	t.Cleanup(ResetForTest)

	MustRegister("cmd", "a-panics", Enricher{
		Detail: func(base map[string]any) { panic("boom") },
	})
	MustRegister("cmd", "b-works", Enricher{
		Detail: func(base map[string]any) { base["b"] = "ok" },
	})

	base := map[string]any{}
	Enrich("cmd", base)

	if base["b"] != "ok" {
		t.Fatal("second enricher should still be called after first panics")
	}
}

func TestEnrichBriefRecoversPanic(t *testing.T) {
	t.Cleanup(ResetForTest)

	MustRegister("cmd", "a-panics", Enricher{
		Brief: func(base map[string]any) { panic("brief boom") },
	})
	MustRegister("cmd", "b-works", Enricher{
		Brief: func(base map[string]any) { base["b"] = "ok" },
	})

	base := map[string]any{}
	EnrichBrief("cmd", base)

	if base["b"] != "ok" {
		t.Fatal("second brief enricher should still be called after first panics")
	}
}

func TestEnrichSkipsNilDetail(t *testing.T) {
	t.Cleanup(ResetForTest)

	MustRegister("cmd", "brief-only", Enricher{
		Brief: func(base map[string]any) { base["brief"] = true },
	})

	base := map[string]any{}
	Enrich("cmd", base)

	if _, ok := base["brief"]; ok {
		t.Fatal("Brief should not be called by Enrich")
	}
}

func TestUnregister(t *testing.T) {
	t.Cleanup(ResetForTest)

	MustRegister("cmd", "a", Enricher{
		Detail: func(base map[string]any) { base["a"] = true },
	})
	MustRegister("cmd", "b", Enricher{
		Detail: func(base map[string]any) { base["b"] = true },
	})

	Unregister("cmd", "a")

	base := map[string]any{}
	Enrich("cmd", base)

	if _, ok := base["a"]; ok {
		t.Fatal("enricher 'a' should have been unregistered")
	}
	if base["b"] != true {
		t.Fatal("enricher 'b' should still be registered")
	}
}

func TestUnregisterNonExistent(t *testing.T) {
	t.Cleanup(ResetForTest)

	Unregister("cmd", "nonexistent")
	Unregister("nonexistent-cmd", "key")
}

func TestUnregisterLastForCommand(t *testing.T) {
	t.Cleanup(ResetForTest)

	MustRegister("cmd", "only", Enricher{
		Detail: func(base map[string]any) { base["only"] = true },
	})

	Unregister("cmd", "only")

	base := map[string]any{}
	Enrich("cmd", base)

	if len(base) != 0 {
		t.Fatalf("expected empty map after unregistering last enricher, got %d keys", len(base))
	}
}

func TestEnrichOrderAlphabetical(t *testing.T) {
	t.Cleanup(ResetForTest)

	var order []string
	MustRegister("cmd", "zebra", Enricher{
		Detail: func(base map[string]any) { order = append(order, "zebra") },
	})
	MustRegister("cmd", "alpha", Enricher{
		Detail: func(base map[string]any) { order = append(order, "alpha") },
	})
	MustRegister("cmd", "middle", Enricher{
		Detail: func(base map[string]any) { order = append(order, "middle") },
	})

	Enrich("cmd", map[string]any{})

	if len(order) != 3 {
		t.Fatalf("expected 3 calls, got %d", len(order))
	}
	if order[0] != "alpha" || order[1] != "middle" || order[2] != "zebra" {
		t.Fatalf("expected [alpha middle zebra], got %v", order)
	}
}
