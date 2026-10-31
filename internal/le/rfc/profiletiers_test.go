package rfc

import (
	"reflect"
	"strings"
	"testing"
)

// TestProfileTiersOfAnswersTheDeclaredTiers proves ProfileTiersOf answers each
// profile's two tiers from the summaries' own declarations, the one place
// membership is built: a use case's `Use cases` and `Optional use cases`, and
// an aggregate's members split by `Aggregate tier`. A name with no profile file
// is refused naming the valid ones.
//
// Method: one use case and one aggregate over three summaries, each tier
// holding a different stem, so a swapped or merged tier is a difference.
func TestProfileTiersOfAnswersTheDeclaredTiers(t *testing.T) {
	profiles := Profiles{
		UseCases:   map[string]UseCase{"edge": {Name: "edge", Title: "Edge", Release: true}},
		Aggregates: map[string]Aggregate{"ospf": {Name: "ospf", Title: "OSPF"}},
	}
	metas := map[string]Meta{
		"rfc2328": {Aggregate: "ospf", UseCases: []string{"edge"}},
		"rfc4577": {Aggregate: "ospf", AggregateOptional: true, OptionalUseCases: []string{"edge"}},
		"rfc4576": {Aggregate: "ospf", AggregateOptional: true},
	}
	for _, want := range []ProfileTiers{
		{Name: "edge", Kind: ProfileKindUseCase, Title: "Edge", Release: true,
			Required: []string{"rfc2328"}, Optional: []string{"rfc4577"}},
		{Name: "ospf", Kind: ProfileKindTechnology, Title: "OSPF",
			Required: []string{"rfc2328"}, Optional: []string{"rfc4576", "rfc4577"}},
	} {
		got, err := ProfileTiersOf(profiles, metas, want.Name)
		if err != nil {
			t.Fatalf("%s: %v", want.Name, err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s tiers\n got: %+v\nwant: %+v", want.Name, got, want)
		}
	}
	if ProfileKindUseCase != "use-case" || ProfileKindTechnology != "technology" {
		t.Errorf("the kind words are %q and %q, not the words a profile file declares",
			ProfileKindUseCase, ProfileKindTechnology)
	}
	_, err := ProfileTiersOf(profiles, metas, "core")
	if err == nil {
		t.Fatal("an unknown profile answered tiers")
	}
	for _, named := range []string{"core", "edge", "ospf"} {
		if !strings.Contains(err.Error(), named) {
			t.Errorf("the refusal does not name %q: %v", named, err)
		}
	}
}
