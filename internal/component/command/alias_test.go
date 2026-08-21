package command

import (
	"strings"
	"testing"
)

// resetAliasTables clears the three registries an alias test writes to, before
// the test and again after it. A registry that survived a test would decide the
// next one. An in-tree refusal in this file is a panic and a plugin-facing one
// is an error, and either can leave the tables half written.
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

// pluginRefusal declares the aliases through the plugin-facing entry point and
// returns the text of the refusal. It fails the test when the declaration is
// accepted, and it fails the test when the entry point panics: a declaration
// arrives over a socket, so its author is told what to change and the daemon
// stays up.
func pluginRefusal(t *testing.T, declared ...PluginAlias) string {
	t.Helper()

	var err error
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				t.Fatalf("the plugin-facing entry point panicked on a declaration it must report: %v", recovered)
			}
		}()
		err = RegisterPluginAliases(declared)
	}()

	if err == nil {
		t.Fatal("the declaration was accepted, and a refused alias reports nothing at use time")
	}
	return err.Error()
}

// VALIDATES: every registration RegisterAliases refuses with a panic("BUG:") is
// refused with an error on the plugin-facing entry point, and the message names
// what the declaring author has to change.
// PREVENTS: a plugin's typo taking the daemon down. RegisterAliases states that
// only a registration in this repository reaches it and that no operator input
// does, and a plugin's declaration breaks that premise.
func TestRegisterPluginAliasesReturnsErrorNotPanic(t *testing.T) {
	cases := []struct {
		name     string
		setup    func()
		alias    Alias
		mentions []string
	}{
		{
			name:     "an alias with no name",
			alias:    Alias{Expansion: "display peers"},
			mentions: []string{"no name"},
		},
		{
			name:     "a name a pipe operator carries",
			alias:    Alias{Name: "count", Expansion: "display peers"},
			mentions: []string{"count", "pipe operator"},
		},
		{
			name:     "a name a pipe filter of an overlapping path carries",
			setup:    func() { RegisterPipeFilters([]string{"show test"}, PipeFilter{Name: "histogram"}) },
			alias:    Alias{Name: "histogram", Expansion: "display peers"},
			mentions: []string{"histogram", "show test", "pipe filter"},
		},
		{
			name:     "an expansion naming no operator",
			alias:    Alias{Name: "wide", Expansion: "sideways"},
			mentions: []string{"wide", "sideways", "not a pipe operator"},
		},
		{
			name:     "an expansion naming nothing at all",
			alias:    Alias{Name: "wide", Expansion: "  "},
			mentions: []string{"wide", "expands to nothing"},
		},
		{
			name:     "a declaration naming no command path",
			alias:    Alias{Name: "wide", Expansion: "display peers"},
			mentions: []string{"wide", "no command path"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			resetAliasTables(t)
			if testCase.setup != nil {
				testCase.setup()
			}

			command := "show test summary"
			if testCase.name == "a declaration naming no command path" {
				command = "   "
			}

			message := pluginRefusal(t, PluginAlias{Command: command, Alias: testCase.alias})
			requireMentions(t, message, testCase.mentions...)

			if aliases := AliasesForCommand("show test summary"); len(aliases) != 0 {
				t.Errorf("a refused declaration reached the registry: %v", aliases)
			}
		})
	}
}

// VALIDATES: AC-2. A plugin naming a built-in pipe operator is refused, and the
// message names the operator.
// PREVENTS: an alias nothing can reach. ParsePipe reads the built-in name
// first, so the operator answers every command and the alias is never resolved.
func TestRegisterPluginAliasesRefusesBuiltinOperatorName(t *testing.T) {
	resetAliasTables(t)

	message := pluginRefusal(t, PluginAlias{
		Command: "show test summary",
		Alias:   Alias{Name: "display", Description: "Taken", Expansion: "display peers"},
	})
	requireMentions(t, message, "display", "pipe operator")

	payload := `{"router-id":"192.0.2.254","peers":[{"address":"192.0.2.1"}]}`
	got := renderThroughPipes(t, "show test summary | display router-id | json", payload)
	if !strings.Contains(got, "192.0.2.254") {
		t.Errorf("the built-in operator stopped answering: %s", got)
	}
}

// VALIDATES: AC-3, and that the population of this check is OVERLAPPING command
// paths. foldFilters resolves a command's own filter for the whole subtree the
// filter covers, so an alias admitted anywhere in that subtree is dark.
// PREVENTS: the check being read as the exact path, which would admit an alias
// under a filter's subtree where nothing reports that the filter always wins.
func TestRegisterPluginAliasesRefusesFilterNameOnOverlappingPath(t *testing.T) {
	t.Run("a filter on a shorter path", func(t *testing.T) {
		resetAliasTables(t)

		RegisterPipeFilters([]string{"show bgp rib"}, PipeFilter{Name: "histogram", Description: "Count routes by prefix length"})

		message := pluginRefusal(t, PluginAlias{
			Command: "show bgp rib best",
			Alias:   Alias{Name: "histogram", Expansion: "display prefix"},
		})
		requireMentions(t, message, "histogram", "show bgp rib", "pipe filter")
	})

	t.Run("a filter on a longer path", func(t *testing.T) {
		resetAliasTables(t)

		RegisterPipeFilters([]string{"show bgp rib best"}, PipeFilter{Name: "histogram", Description: "Count routes by prefix length"})

		message := pluginRefusal(t, PluginAlias{
			Command: "show bgp rib",
			Alias:   Alias{Name: "histogram", Expansion: "display prefix"},
		})
		requireMentions(t, message, "histogram", "show bgp rib best", "pipe filter")
	})

	t.Run("a filter on a path that does not overlap", func(t *testing.T) {
		resetAliasTables(t)

		RegisterPipeFilters([]string{"show bgp rib"}, PipeFilter{Name: "histogram", Description: "Count routes by prefix length"})

		if err := RegisterPluginAliases([]PluginAlias{{
			Command: "show bgp health",
			Alias:   Alias{Name: "histogram", Expansion: "display peers"},
		}}); err != nil {
			t.Fatalf("an alias on a path the filter cannot reach was refused: %v", err)
		}
		if aliases := AliasesForCommand("show bgp health"); len(aliases) != 1 {
			t.Errorf("AliasesForCommand = %v, want the alias on the unrelated path", aliases)
		}
	})
}

// VALIDATES: R-1. `summary` on `show bgp rpki` is accepted while `show bgp`
// already carries `summary`, and each path answers its own.
// PREVENTS: the alias-versus-alias check being read as overlapping paths, which
// refuses the consumer this channel exists to serve. A longer path shadows a
// shorter one, and shadowing is the declared inheritance mechanism.
func TestRegisterPluginAliasesAllowsSameNameOnLongerPath(t *testing.T) {
	resetAliasTables(t)

	RegisterAliases([]string{"show bgp"}, Alias{
		Name:        "summary",
		Description: "The aggregate fields, without the peer rows",
		Expansion:   "display router-id",
	})

	if err := RegisterPluginAliases([]PluginAlias{{
		Command: "show bgp rpki",
		Alias:   Alias{Name: "summary", Description: "The RPKI counters", Expansion: "display vrp-count"},
	}}); err != nil {
		t.Fatalf("the longer path was refused the name the shorter path carries: %v", err)
	}

	payload := `{"router-id":"192.0.2.254","vrp-count":7}`

	longer := renderThroughPipes(t, "show bgp rpki | summary | json", payload)
	if !strings.Contains(longer, "vrp-count") {
		t.Errorf("the longer path did not answer its own expansion: %s", longer)
	}
	if strings.Contains(longer, "192.0.2.254") {
		t.Errorf("the longer path answered the shorter path's expansion: %s", longer)
	}

	shorter := renderThroughPipes(t, "show bgp | summary | json", payload)
	if !strings.Contains(shorter, "192.0.2.254") {
		t.Errorf("the shorter path lost its own expansion: %s", shorter)
	}
	if strings.Contains(shorter, "vrp-count") {
		t.Errorf("the shorter path answered the longer path's expansion: %s", shorter)
	}
}

// VALIDATES: AC-4, and that the population of this check is the EXACT command
// path. lookupAlias reads the set on the longest registered prefix and never
// falls back, so two aliases of one name collide only where one hides the other.
// PREVENTS: a plugin taking a name a command already answers to. The registry
// stored one set per path and replaced what the path held, so the aliases
// `show bgp` carries were one declaration away from being lost in silence.
func TestRegisterPluginAliasesRefusesSameNameOnSamePath(t *testing.T) {
	declareInTree := func() {
		RegisterAliases([]string{"show bgp"},
			Alias{Name: "summary", Description: "In tree", Expansion: "display router-id"},
			Alias{Name: "peers", Description: "In tree", Expansion: "display peers"},
		)
	}

	t.Run("the name is refused", func(t *testing.T) {
		resetAliasTables(t)
		declareInTree()

		message := pluginRefusal(t, PluginAlias{
			Command: "show bgp",
			Alias:   Alias{Name: "summary", Description: "Plugin", Expansion: "display vrp-count"},
		})
		requireMentions(t, message, "summary", "show bgp")
	})

	t.Run("the alias already serving keeps serving unchanged", func(t *testing.T) {
		resetAliasTables(t)
		declareInTree()

		pluginRefusal(t, PluginAlias{
			Command: "show bgp",
			Alias:   Alias{Name: "summary", Description: "Plugin", Expansion: "display vrp-count"},
		})

		aliases := AliasesForCommand("show bgp")
		if len(aliases) != 2 {
			t.Fatalf("AliasesForCommand = %v, want the two the command already carried", aliases)
		}
		for _, alias := range aliases {
			if alias.Description != "In tree" {
				t.Errorf("the alias %s was replaced: %+v", alias.Name, alias)
			}
		}

		payload := `{"router-id":"192.0.2.254","vrp-count":7,"peers":[{"address":"192.0.2.1"}]}`
		got := renderThroughPipes(t, "show bgp | summary | json", payload)
		if !strings.Contains(got, "192.0.2.254") || strings.Contains(got, "vrp-count") {
			t.Errorf("`show bgp | summary` no longer answers what it answered: %s", got)
		}
	})

	t.Run("the path is read in the spelling the registry stores", func(t *testing.T) {
		resetAliasTables(t)
		declareInTree()

		message := pluginRefusal(t, PluginAlias{
			Command: "  SHOW   BGP  ",
			Alias:   Alias{Name: " Summary ", Expansion: "display vrp-count"},
		})
		requireMentions(t, message, "summary", "show bgp")
	})
}

// VALIDATES: AC-6. An expansion naming another alias is refused, and the
// message says why.
// PREVENTS: an alias that half expands. expandAliases runs one pass and never
// reads its own result, so the second name would reach the operator as an
// unknown pipe operator.
func TestRegisterPluginAliasesRefusesExpansionNamingAnAlias(t *testing.T) {
	resetAliasTables(t)

	RegisterAliases([]string{"show test summary"}, Alias{Name: "peers", Expansion: "display peers"})

	message := pluginRefusal(t, PluginAlias{
		Command: "show test summary",
		Alias:   Alias{Name: "wide", Expansion: "peers"},
	})
	requireMentions(t, message, "wide", "peers", "another alias")
}

// VALIDATES: AC-5. An expansion naming a word that is no pipe operator is
// refused, and the message names the word.
// PREVENTS: the word reaching ApplyPipes, where the operator is told their own
// alias does not exist.
func TestRegisterPluginAliasesRefusesUnknownOperatorInExpansion(t *testing.T) {
	resetAliasTables(t)

	message := pluginRefusal(t, PluginAlias{
		Command: "show test summary",
		Alias:   Alias{Name: "wide", Expansion: "display kind | sideways"},
	})
	requireMentions(t, message, "wide", "sideways", "not a pipe operator")
}

// VALIDATES: AC-15. Two aliases of one name on one path in a single message are
// refused, and neither is registered.
// PREVENTS: the later entry winning in silence. The message is built into one
// set per path, so a map assignment would drop the earlier declaration and the
// author would never learn which of the two the daemon serves.
func TestRegisterPluginAliasesRefusesDuplicateNameInOneBatch(t *testing.T) {
	t.Run("the same name on the same path", func(t *testing.T) {
		resetAliasTables(t)

		message := pluginRefusal(t,
			PluginAlias{Command: "show test summary", Alias: Alias{Name: "totals", Description: "First", Expansion: "display kind"}},
			PluginAlias{Command: "show test summary", Alias: Alias{Name: "totals", Description: "Second", Expansion: "display vrp-count"}},
		)
		requireMentions(t, message, "totals", "show test summary")

		if aliases := AliasesForCommand("show test summary"); len(aliases) != 0 {
			t.Errorf("a refused message reached the registry: %v", aliases)
		}
	})

	t.Run("the same name on two paths", func(t *testing.T) {
		resetAliasTables(t)

		if err := RegisterPluginAliases([]PluginAlias{
			{Command: "show test summary", Alias: Alias{Name: "totals", Expansion: "display kind"}},
			{Command: "show test detail", Alias: Alias{Name: "totals", Expansion: "display vrp-count"}},
		}); err != nil {
			t.Fatalf("one name on two paths was refused: %v", err)
		}
		if aliases := AliasesForCommand("show test summary"); len(aliases) != 1 {
			t.Errorf("AliasesForCommand(show test summary) = %v, want the one alias", aliases)
		}
		if aliases := AliasesForCommand("show test detail"); len(aliases) != 1 {
			t.Errorf("AliasesForCommand(show test detail) = %v, want the one alias", aliases)
		}
	})
}

// VALIDATES: one refused declaration refuses the whole message, whichever
// position it sits in.
// PREVENTS: a partial registration the declaring author never asked for and
// cannot see. Stage 1 either accepts the message or the plugin does not start.
func TestRegisterPluginAliasesIsAllOrNothing(t *testing.T) {
	resetAliasTables(t)

	err := RegisterPluginAliases([]PluginAlias{
		{Command: "show test summary", Alias: Alias{Name: "totals", Expansion: "display kind"}},
		{Command: "show test detail", Alias: Alias{Name: "wide", Expansion: "sideways"}},
		{Command: "show test other", Alias: Alias{Name: "rows", Expansion: "display peers"}},
	})
	if err == nil {
		t.Fatal("a message carrying one bad declaration was accepted")
	}
	requireMentions(t, err.Error(), "sideways")

	for _, command := range []string{"show test summary", "show test detail", "show test other"} {
		if aliases := AliasesForCommand(command); len(aliases) != 0 {
			t.Errorf("%s carries %v, and a refused message registers nothing", command, aliases)
		}
	}
}

// VALIDATES: a declaration adds to the command path and never replaces what the
// path already carries.
// PREVENTS: the hazard the exact-path name check alone leaves open.
// commandRegistry.register stores one value per path, so a declaration carrying
// a name nobody holds still dropped every alias that path already answered to.
// `show bgp rpki` carries the empty declaration the in-tree BGP command plugin
// puts on every child of `show bgp`, so the consumer this channel exists to
// serve registers onto an occupied path.
func TestRegisterPluginAliasesKeepsWhatThePathAlreadyCarries(t *testing.T) {
	t.Run("beside the aliases the path answers to", func(t *testing.T) {
		resetAliasTables(t)

		RegisterAliases([]string{"show bgp"},
			Alias{Name: "summary", Description: "In tree", Expansion: "display router-id"},
			Alias{Name: "peers", Description: "In tree", Expansion: "display peers"},
		)

		if err := RegisterPluginAliases([]PluginAlias{{
			Command: "show bgp",
			Alias:   Alias{Name: "wide", Description: "Plugin", Expansion: "display vrp-count"},
		}}); err != nil {
			t.Fatalf("a name the path does not carry was refused: %v", err)
		}

		aliases := AliasesForCommand("show bgp")
		if len(aliases) != 3 {
			t.Fatalf("AliasesForCommand = %v, want the two in-tree aliases and the declared one", aliases)
		}

		payload := `{"router-id":"192.0.2.254","vrp-count":7,"peers":[{"address":"192.0.2.1"}]}`
		got := renderThroughPipes(t, "show bgp | summary | json", payload)
		if !strings.Contains(got, "192.0.2.254") {
			t.Errorf("the alias the path already answered to was dropped: %s", got)
		}
	})

	t.Run("beside the empty declaration that stops inheritance", func(t *testing.T) {
		resetAliasTables(t)

		RegisterAliases([]string{"show bgp"}, Alias{Name: "summary", Description: "In tree", Expansion: "display router-id"})
		RegisterAliases([]string{"show bgp rpki"})

		if err := RegisterPluginAliases([]PluginAlias{{
			Command: "show bgp rpki",
			Alias:   Alias{Name: "cache", Description: "Plugin", Expansion: "display servers"},
		}}); err != nil {
			t.Fatalf("a declaration on a path carrying the empty declaration was refused: %v", err)
		}

		aliases := AliasesForCommand("show bgp rpki")
		if len(aliases) != 1 || aliases[0].Name != "cache" {
			t.Errorf("AliasesForCommand = %v, want the declared alias alone", aliases)
		}
	})
}
