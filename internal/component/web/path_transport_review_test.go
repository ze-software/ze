// Design: docs/architecture/web-interface.md -- key paths survive browser routing.

package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/iface"
)

// TestConfigDotKeyCarrierRemainsDistinct pins the special dot carrier alongside
// literal marker and percent spellings, including the traversal refusal.
func TestConfigDotKeyCarrierRemainsDistinct(t *testing.T) {
	seen := make(map[string]string)
	for _, key := range []string{".", "~.", "~~.", "~", "%2E", "%", "/", "x/y"} {
		encoded := encodeConfigPath([]string{"bgp", "group", key})
		assert.NotEqual(t, "bgp/group/.", encoded)
		if previous, exists := seen[encoded]; exists {
			t.Fatalf("keys %q and %q share transport %q", previous, key, encoded)
		}
		seen[encoded] = key
		decoded, err := decodeConfigPath(encoded)
		require.NoError(t, err)
		assert.Equal(t, []string{"bgp", "group", key}, decoded)
	}
	assert.Equal(t, "bgp/group/~.", encodeConfigPath([]string{"bgp", "group", "."}))
	_, err := decodeConfigPath("bgp/group/..")
	assert.Error(t, err)
}

// TestWorkbenchInterfaceLinksKeepKeyTokens checks all interface link producers
// against the same path consumer used by their workbench handlers.
func TestWorkbenchInterfaceLinksKeepKeyTokens(t *testing.T) {
	key := "logical/100%?&+~."
	infos := []iface.InterfaceInfo{{Name: key, Addresses: []iface.AddrInfo{{Address: "192.0.2.1", PrefixLength: 24}}}}
	table := buildInterfaceTableData(infos, "")
	traffic := buildTrafficTableData(infos)
	addresses := buildAddressTableData(infos, "", "")
	for _, target := range []string{table.Rows[0].URL, traffic.Rows[0].URL, addresses.Rows[0].Actions[0].URL} {
		parsed, err := ParseURL(httptest.NewRequestWithContext(t.Context(), http.MethodGet, target, http.NoBody))
		require.NoError(t, err)
		assert.Equal(t, []string{"iface", "detail", key}, parsed.Path)
	}
}

// TestGraphicalDotKeyThroughWebMux follows a generated link through the same
// ServeMux as the listener, then saves and reloads that exact candidate entry.
func TestGraphicalDotKeyThroughWebMux(t *testing.T) {
	mgr, terminal := newYANGTerminal(t)
	webTerminalLine(t, terminal, "", `set bgp group . description original`)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	path := []string{"bgp", "group", "."}
	listing := buildFragmentData(mgr.schema, mgr.Tree("testuser"), path[:2])
	var entryURL string
	for _, column := range listing.Columns {
		for _, item := range column.NamedItems {
			if item.Name == "." {
				entryURL = item.URL
			}
		}
	}
	require.NotEmpty(t, entryURL)
	srv := &WebServer{mux: http.NewServeMux()}
	srv.HandleFunc("/show/", HandleFragment(renderer, mgr.schema, mgr.Tree("testuser"), mgr, false, nil))
	srv.HandleFunc("/config/set/", HandleConfigSetWithAuthorizer(mgr, mgr.schema, renderer, nil))
	shown := httptest.NewRecorder()
	srv.mux.ServeHTTP(shown, authedRequest(t, http.MethodGet, entryURL, nil))
	require.Equal(t, http.StatusOK, shown.Code, shown.Body.String())
	assert.Contains(t, shown.Body.String(), `value="original"`)
	changed := httptest.NewRecorder()
	save := authedRequest(t, http.MethodPost, configSetURL(encodeConfigPath(path)), url.Values{"leaf": {"description"}, "value": {"saved"}})
	save.Header.Set("HX-Request", htmxRequestTrue)
	srv.mux.ServeHTTP(changed, save)
	require.Equal(t, http.StatusOK, changed.Code, changed.Body.String())
	value, present := walkTree(mgr.Tree("testuser"), mgr.schema, path).Get("description")
	require.True(t, present)
	assert.Equal(t, "saved", value)
	reloaded := httptest.NewRecorder()
	srv.mux.ServeHTTP(reloaded, authedRequest(t, http.MethodGet, entryURL, nil))
	require.Equal(t, http.StatusOK, reloaded.Code, reloaded.Body.String())
	assert.Contains(t, reloaded.Body.String(), `value="saved"`)
}

// TestWorkbenchPeerPathsKeepKeyTokens checks producer-built grouped peer links
// and related-tool context against the canonical path consumer.
func TestWorkbenchPeerPathsKeepKeyTokens(t *testing.T) {
	renderer, err := NewRenderer()
	require.NoError(t, err)
	peer := config.NewTree()
	peer.Set("remote", "192.0.2.1")
	name, group := "peer/100%?", "group/100%?"
	entry := extractPeerEntry(name, peer, group)
	path := []string{"bgp", "group", group, "peer", name}
	parsed, err := ParseURL(httptest.NewRequestWithContext(t.Context(), http.MethodGet, entry.EditURL, http.NoBody))
	require.NoError(t, err)
	assert.Equal(t, path, parsed.Path)
	entry.RemoteIP = "192.0.2.1"
	actions := string(buildPeerActionsHTML(renderer, entry))
	assert.Contains(t, actions, encodeConfigPath(path))
	table := buildBGPPeersTableData([]peerEntry{entry}, group, nil)
	parsed, err = ParseURL(httptest.NewRequestWithContext(t.Context(), http.MethodGet, table.AddURL, http.NoBody))
	require.NoError(t, err)
	assert.Equal(t, path[:4], parsed.Path)
	detail := buildBGPPeerDetailData(renderer, entry)
	assert.Equal(t, table.AddURL, detail.CloseURL)
}

// TestWorkbenchNamedLinksKeepKeyTokens covers the remaining named config-link
// producers and firewall query filters without involving a firewall backend.
func TestWorkbenchNamedLinksKeepKeyTokens(t *testing.T) {
	key := "key/100%?&+~."
	groups := buildBGPGroupsTableData([]groupEntry{{Name: key}})
	policies := buildBGPPolicyTableData([]policyEntry{{Name: key, Type: "prefix-list"}})
	users := buildUsersTableData([]userEntry{{Name: key}})
	profiles := buildSysctlProfilesTableData([]sysctlProfileEntry{{Name: key}})
	tables := buildFirewallTablesTableData([]tableEntry{{Name: key}})
	chains := buildFirewallChainsTableData([]chainEntry{{Table: key, Name: key}}, key)
	rules := buildFirewallRulesTableData([]ruleEntry{{Table: key, Chain: key, Name: key}}, key, key)
	sets := buildFirewallSetsTableData([]setEntry{{Table: key, Name: key}})
	for _, item := range []struct {
		target string
		path   []string
	}{
		{groups.Rows[0].URL, []string{"bgp", "group", key}},
		{groups.Rows[0].Actions[0].URL, []string{"bgp", "group", key, "peer"}},
		{policies.Rows[0].URL, []string{"policy", "prefix-list", key}},
		{users.Rows[0].URL, []string{"system", "authentication", "user", key}},
		{profiles.Rows[0].URL, []string{"sysctl", "profile", key}},
		{tables.Rows[0].Actions[1].URL, []string{"firewall", "table", key}},
		{chains.Rows[0].Actions[1].URL, []string{"firewall", "table", key, "chain", key}},
		{chains.AddURL, []string{"firewall", "table", key, "chain"}},
		{rules.Rows[0].Actions[0].URL, []string{"firewall", "table", key, "chain", key, "rule", key}},
		{rules.AddURL, []string{"firewall", "table", key, "chain", key, "term"}},
		{sets.Rows[0].Actions[0].URL, []string{"firewall", "table", key, "set", key}},
		{ifaceCountersURL(key), []string{"iface", "counters", key}},
	} {
		parsed, err := ParseURL(httptest.NewRequestWithContext(t.Context(), http.MethodGet, item.target, http.NoBody))
		require.NoError(t, err, item.target)
		assert.Equal(t, item.path, parsed.Path, item.target)
	}
	for _, target := range []string{tables.Rows[0].URL, chains.Rows[0].URL} {
		parsed, err := url.Parse(target)
		require.NoError(t, err)
		assert.Equal(t, key, parsed.Query().Get("table"))
		if strings.HasPrefix(target, "/show/firewall/rule/") {
			assert.Equal(t, key, parsed.Query().Get("chain"))
		}
	}
}

// TestWorkbenchGroupedPeerOpensEditor follows the grouped peer table's Edit
// link through the real workbench handler, including its editable child path.
func TestWorkbenchGroupedPeerOpensEditor(t *testing.T) {
	mgr, terminal := newYANGTerminal(t)
	for _, line := range []string{
		`set bgp group "group/100%" peer "peer-100" description grouped-description`,
		`set bgp group "group/100%" peer "peer-100" connection remote ip 192.0.2.41`,
		`set bgp group other peer elsewhere description outside-group`,
	} {
		answer := webTerminalLine(t, terminal, "", line)
		require.True(t, strings.HasPrefix(answer.Output, "set "), "seed command %q failed: %s", line, answer.Output)
	}
	peerPath := []string{"bgp", "group", "group/100%", "peer", "peer-100"}
	peer := walkTree(mgr.Tree("testuser"), mgr.schema, peerPath)
	require.NotNil(t, peer, "seeded grouped peer is absent from the typed candidate")
	description, present := peer.Get("description")
	require.True(t, present)
	require.Equal(t, "grouped-description", description)
	remote := walkTree(mgr.Tree("testuser"), mgr.schema, slices.Concat(peerPath, []string{"connection", "remote"}))
	require.NotNil(t, remote)
	address, present := remote.Get("ip")
	require.True(t, present)
	require.Equal(t, "192.0.2.41", address)
	other := walkTree(mgr.Tree("testuser"), mgr.schema, []string{"bgp", "group", "other", "peer", "elsewhere"})
	require.NotNil(t, other, "the group-filter control must exist before requests")
	outside, present := other.Get("description")
	require.True(t, present)
	require.Equal(t, "outside-group", outside)
	renderer, err := NewRenderer()
	require.NoError(t, err)
	handler := HandleWorkbench(renderer, mgr.schema, mgr.Tree("testuser"), mgr, false)
	listPath := []string{"bgp", "group", "group/100%", "peer"}
	list := httptest.NewRecorder()
	handler(list, authedRequest(t, http.MethodGet, showPathPrefix+encodeConfigPath(listPath)+"/", nil))
	require.Equal(t, http.StatusOK, list.Code, list.Body.String())
	assert.Contains(t, list.Body.String(), "BGP Peers")
	assert.NotContains(t, list.Body.String(), "elsewhere", "group list includes another group's peer")
	entryURL := showPathPrefix + encodeConfigPath(peerPath) + "/"
	assert.Contains(t, list.Body.String(), entryURL, "the scoped table must offer the exact peer Edit URL")
	for _, selected := range []struct {
		name  string
		path  []string
		leaf  string
		value string
	}{
		{"entry", peerPath, "description", "grouped-description"},
		{"child", slices.Concat(peerPath, []string{"connection", "remote"}), "ip", "192.0.2.41"},
	} {
		t.Run(selected.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler(rec, authedRequest(t, http.MethodGet, showPathPrefix+encodeConfigPath(selected.path)+"/", nil))
			require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), `id="`+fieldInputID(encodeConfigPath(selected.path), selected.leaf)+`"`)
			assert.Contains(t, rec.Body.String(), `value="`+selected.value+`"`)
			assert.NotContains(t, rec.Body.String(), "BGP Peers", "a named peer or child reopened the scoped list")
		})
	}
}
