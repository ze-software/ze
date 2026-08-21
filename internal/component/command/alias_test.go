package command

import (
	"strings"
	"testing"
)

// resetAliasTables clears the three registries an alias test writes to, before
// the test and again after it. A registry that survived a test would decide the
// next one. Every refusal in this file is a panic, which leaves the tables half
// written.
func resetAliasTables(t *testing.T) {
	t.Helper()

	reset := func() {
		ResetAliasesForTest()
		ResetPipeFiltersForTest()
		ResetColumnsForTest()
	}
	reset()
	t.Cleanup(reset)
}

// refusalMessage runs register and returns the message it panicked with. It
// fails the test when the registration is accepted, because a silent
// acceptance is the failure every refusal test in this file is about.
func refusalMessage(t *testing.T, register func()) string {
	t.Helper()

	message := ""
	func() {
		defer func() {
			recovered := recover()
			if recovered == nil {
				return
			}
			text, ok := recovered.(string)
			if !ok {
				t.Fatalf("panic value is %T, want the string a panic(\"BUG:\") carries", recovered)
			}
			message = text
		}()
		register()
	}()

	if message == "" {
		t.Fatal("the registration was accepted, and a shadowed alias reports nothing at use time")
	}
	return message
}

// requireMentions fails when the message leaves out a word the reader needs to
// act on it.
func requireMentions(t *testing.T, message string, words ...string) {
	t.Helper()

	for _, word := range words {
		if !strings.Contains(message, word) {
			t.Errorf("the refusal does not name %q: %s", word, message)
		}
	}
}

// VALIDATES: AC-7. An alias registered for every command resolves for a command
// that declares none of its own.
// PREVENTS: the global table being registered on the empty command path, where
// commandRegistry.register drops it and commandMatchesPrefix would refuse it
// anyway. Such a registration matches nothing and reports nothing, so the
// assertion here is that the answer CHANGED.
func TestAliasResolvesGlobal(t *testing.T) {
	resetAliasTables(t)

	RegisterAliases(nil, Alias{Name: "peers", Description: "The peer rows alone", Expansion: "display peers"})

	payload := `{"router-id":"192.0.2.254","local-as":65000,"peers":[{"address":"192.0.2.1","state":"established"}]}`
	got := renderThroughPipes(t, "show test summary | peers | json", payload)

	if !strings.Contains(got, "192.0.2.1") {
		t.Errorf("the peer rows the alias asked for are missing: %s", got)
	}
	if strings.Contains(got, "192.0.2.254") {
		t.Errorf("the alias did not resolve, so the aggregates are still there: %s", got)
	}
	if aliases := AliasesForCommand("show test summary"); len(aliases) != 1 || aliases[0].Name != "peers" {
		t.Errorf("AliasesForCommand = %v, want the one global alias", aliases)
	}
}

// VALIDATES: AC-11. A command-specific alias wins over a global one of the same
// name, by the longest-prefix rule the column registry uses.
// PREVENTS: the two tables being merged in either fixed order, which would make
// one of them unreachable for every name the other carries.
func TestAliasCommandSpecificBeatsGlobal(t *testing.T) {
	resetAliasTables(t)

	RegisterAliases(nil, Alias{Name: "peers", Description: "Global", Expansion: "display local-as"})
	RegisterAliases([]string{"show test summary"}, Alias{Name: "peers", Description: "Per command", Expansion: "display peers"})

	payload := `{"router-id":"192.0.2.254","local-as":65000,"peers":[{"address":"192.0.2.1","state":"established"}]}`

	specific := renderThroughPipes(t, "show test summary | peers | json", payload)
	if !strings.Contains(specific, "192.0.2.1") {
		t.Errorf("the command-specific alias lost to the global one: %s", specific)
	}

	global := renderThroughPipes(t, "show test other | peers | json", payload)
	if !strings.Contains(global, "65000") || strings.Contains(global, "192.0.2.1") {
		t.Errorf("a command with no alias of its own did not reach the global one: %s", global)
	}

	aliases := AliasesForCommand("show test summary")
	if len(aliases) != 1 || aliases[0].Description != "Per command" {
		t.Errorf("AliasesForCommand = %v, want the command-specific alias alone", aliases)
	}
}

// VALIDATES: AC-9, R-2. An alias name a pipe filter of the same command already
// carries is refused at registration, whichever of the two registers second.
// PREVENTS: the filter winning in silence at use time. foldFilters resolves a
// command's own filter before anything generic, so the operator sees the filter
// and never learns the alias existed.
func TestAliasRegistrationRefusesFilterCollision(t *testing.T) {
	t.Run("the filter registered first", func(t *testing.T) {
		resetAliasTables(t)

		RegisterPipeFilters([]string{"show bgp rib"}, PipeFilter{Name: "histogram", Description: "Count routes by prefix length"})

		message := refusalMessage(t, func() {
			RegisterAliases([]string{"show bgp rib"}, Alias{Name: "histogram", Expansion: "display prefix"})
		})
		requireMentions(t, message, "histogram", "show bgp rib", "pipe filter")
	})

	t.Run("the alias registered first", func(t *testing.T) {
		resetAliasTables(t)

		RegisterAliases([]string{"show bgp rib"}, Alias{Name: "histogram", Expansion: "display prefix"})

		message := refusalMessage(t, func() {
			RegisterPipeFilters([]string{"show bgp rib"}, PipeFilter{Name: "histogram", Description: "Count routes by prefix length"})
		})
		requireMentions(t, message, "histogram", "show bgp rib", "pipe alias")
	})

	t.Run("a global alias against a filter of one command", func(t *testing.T) {
		resetAliasTables(t)

		RegisterPipeFilters([]string{"show bgp rib"}, PipeFilter{Name: "histogram", Description: "Count routes by prefix length"})

		message := refusalMessage(t, func() {
			RegisterAliases(nil, Alias{Name: "histogram", Expansion: "display prefix"})
		})
		requireMentions(t, message, "histogram", "show bgp rib")
	})

	t.Run("paths that do not overlap are no collision", func(t *testing.T) {
		resetAliasTables(t)

		RegisterPipeFilters([]string{"show bgp rib"}, PipeFilter{Name: "histogram", Description: "Count routes by prefix length"})
		RegisterAliases([]string{"show bgp health"}, Alias{Name: "histogram", Expansion: "display peers"})

		if aliases := AliasesForCommand("show bgp health"); len(aliases) != 1 {
			t.Errorf("AliasesForCommand = %v, want the alias on the unrelated path", aliases)
		}
	})
}

// VALIDATES: AC-10, A-4. An alias whose expansion names another alias is
// refused at registration, and so is one that names no operator at all.
// PREVENTS: an alias that half expands. Expansion is one pass, so a name
// reached by a second alias would arrive at ValidatePipes as an unknown
// operator, telling the operator their own alias does not exist.
func TestAliasRecursionIsRefused(t *testing.T) {
	t.Run("an expansion naming a registered alias", func(t *testing.T) {
		resetAliasTables(t)

		RegisterAliases(nil, Alias{Name: "peers", Expansion: "display peers"})

		message := refusalMessage(t, func() {
			RegisterAliases(nil, Alias{Name: "wide", Expansion: "peers"})
		})
		requireMentions(t, message, "wide", "peers", "another alias")
	})

	t.Run("an expansion naming nothing known", func(t *testing.T) {
		resetAliasTables(t)

		message := refusalMessage(t, func() {
			RegisterAliases(nil, Alias{Name: "wide", Expansion: "sideways"})
		})
		requireMentions(t, message, "wide", "sideways", "not a pipe operator")
	})

	t.Run("an empty expansion", func(t *testing.T) {
		resetAliasTables(t)

		message := refusalMessage(t, func() {
			RegisterAliases(nil, Alias{Name: "wide", Expansion: "  "})
		})
		requireMentions(t, message, "wide", "expands to nothing")
	})
}

// VALIDATES: A-4. Expansion is one pass, and a second pass over its result
// changes nothing.
// PREVENTS: a re-entrant rewrite arriving with the aliases. Nothing in the pipe
// layer re-parses its own output today, and this test is what says so after the
// alias table exists.
func TestAliasExpandsOnce(t *testing.T) {
	resetAliasTables(t)

	RegisterAliases(nil, Alias{Name: "peers", Expansion: "display peers | count"})

	_, ops := ParsePipe("show test summary | peers | json")
	once := expandAliases("show test summary", ops)

	kinds := make([]pipeKind, 0, len(once))
	for _, op := range once {
		kinds = append(kinds, op.kind)
	}
	want := []pipeKind{pipeDisplay, pipeCount, pipeJSON}
	if len(kinds) != len(want) {
		t.Fatalf("expanded chain = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("expanded chain = %v, want %v", kinds, want)
		}
	}
	if once[0].arg != "peers" {
		t.Errorf("the expansion lost its field name: %q", once[0].arg)
	}

	twice := expandAliases("show test summary", once)
	if len(twice) != len(once) {
		t.Fatalf("a second pass changed the chain: %d ops, want %d", len(twice), len(once))
	}
	for i := range once {
		if twice[i] != once[i] {
			t.Errorf("a second pass rewrote op %d: %+v, want %+v", i, twice[i], once[i])
		}
	}
}

// VALIDATES: an alias carries no argument, and a word after its name is
// refused by name.
// PREVENTS: the word being dropped in silence, which would answer an operator
// who typed a filter with the unfiltered table and no explanation.
func TestAliasTakesNoArgument(t *testing.T) {
	resetAliasTables(t)

	RegisterAliases(nil, Alias{Name: "peers", Expansion: "display peers"})

	_, _, errMsg := ProcessPipesChecked("show test summary | peers established")
	if errMsg == "" {
		t.Fatal("an argument after an alias was accepted, so the word went nowhere")
	}
	requireMentions(t, errMsg, "peers", "argument")
}

// VALIDATES: a name a pipe operator already carries is refused.
// PREVENTS: an alias that can never resolve. ParsePipe reads the operator
// first, so knownPipeOps wins every collision at use time.
func TestAliasRegistrationRefusesOperatorName(t *testing.T) {
	resetAliasTables(t)

	message := refusalMessage(t, func() {
		RegisterAliases(nil, Alias{Name: "display", Expansion: "display peers"})
	})
	requireMentions(t, message, "display", "pipe operator")
}
