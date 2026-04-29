package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "codeberg.org/thomas-mangin/ze/internal/component/firewall/schema"
	_ "codeberg.org/thomas-mangin/ze/internal/component/iface/schema"
)

func TestScenario_BGPPeerCreateAndCommit(t *testing.T) {
	mgr, schema := newHandlerTestManager(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)

	addHandler := HandleConfigAdd(mgr, schema, renderer)
	setHandler := HandleConfigSet(mgr, schema, renderer)
	commitHandler := HandleConfigCommit(mgr, renderer, nil)

	addPeer := func(name, remoteIP, remoteASN string) {
		t.Helper()
		form := url.Values{
			"name":                       {name},
			"field:connection/remote/ip": {remoteIP},
			"field:session/asn/remote":   {remoteASN},
		}
		req := postConfigRequest(t, "/config/add/bgp/peer/", form, "alice")
		rec := httptest.NewRecorder()
		addHandler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "add peer %s", name)
	}

	setLeaf := func(path, leaf, value string) {
		t.Helper()
		form := url.Values{"leaf": {leaf}, "value": {value}}
		req := postConfigRequest(t, "/config/set/"+path, form, "alice")
		rec := httptest.NewRecorder()
		setHandler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusSeeOther, rec.Code, "set %s/%s=%s", path, leaf, value)
	}

	setLeaf("bgp/", "router-id", "10.0.0.1")

	addPeer("isp-transit-a", "198.51.100.1", "64496")
	setLeaf("bgp/peer/isp-transit-a/connection/local/", "ip", "198.51.100.2")

	addPeer("isp-transit-b", "203.0.113.1", "64497")
	addPeer("ixp-peer-cdn", "185.1.76.10", "13335")
	addPeer("core-rtr-1", "10.0.0.2", "65000")
	addPeer("customer-acme", "172.16.1.2", "65100")

	count := mgr.ChangeCount("alice")
	assert.Greater(t, count, 0, "should have pending changes")

	req := postConfigRequest(t, "/config/commit/", url.Values{}, "alice")
	rec := httptest.NewRecorder()
	commitHandler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusSeeOther, rec.Code, "commit should succeed")
	assert.Equal(t, 0, mgr.ChangeCount("alice"), "no pending changes after commit")
}

func TestScenario_InterfaceCreateWithUnit(t *testing.T) {
	mgr, schema := newHandlerTestManager(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)

	addHandler := HandleConfigAdd(mgr, schema, renderer)
	setHandler := HandleConfigSet(mgr, schema, renderer)
	commitHandler := HandleConfigCommit(mgr, renderer, nil)

	addEntry := func(path string, form url.Values) int {
		t.Helper()
		req := postConfigRequest(t, "/config/add/"+path, form, "alice")
		rec := httptest.NewRecorder()
		addHandler.ServeHTTP(rec, req)
		return rec.Code
	}

	setLeaf := func(path, leaf, value string) {
		t.Helper()
		form := url.Values{"leaf": {leaf}, "value": {value}}
		req := postConfigRequest(t, "/config/set/"+path, form, "alice")
		rec := httptest.NewRecorder()
		setHandler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusSeeOther, rec.Code, "set %s/%s=%s", path, leaf, value)
	}

	code := addEntry("interface/ethernet/", url.Values{
		"name":              {"uplink-isp-a"},
		"field:mac-address": {"00:1a:2b:3c:4d:01"},
	})
	assert.Equal(t, http.StatusOK, code, "add ethernet interface")

	setLeaf("interface/ethernet/uplink-isp-a/", "description", "Transit link to ISP-A")
	setLeaf("interface/ethernet/uplink-isp-a/", "mtu", "9000")

	code = addEntry("interface/ethernet/uplink-isp-a/unit/", url.Values{"name": {"0"}})
	assert.Equal(t, http.StatusOK, code, "add unit 0")

	setLeaf("interface/ethernet/uplink-isp-a/unit/0/", "address", "198.51.100.2/30")

	count := mgr.ChangeCount("alice")
	assert.Greater(t, count, 0, "should have pending changes")

	req := postConfigRequest(t, "/config/commit/", url.Values{}, "alice")
	rec := httptest.NewRecorder()
	commitHandler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusSeeOther, rec.Code, "commit should succeed")
}

func TestScenario_FirewallChainAndMatchLeaves(t *testing.T) {
	mgr, schema := newHandlerTestManager(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)

	addHandler := HandleConfigAdd(mgr, schema, renderer)
	setHandler := HandleConfigSet(mgr, schema, renderer)
	commitHandler := HandleConfigCommit(mgr, renderer, nil)

	addEntry := func(path string, form url.Values) int {
		t.Helper()
		req := postConfigRequest(t, "/config/add/"+path, form, "alice")
		rec := httptest.NewRecorder()
		addHandler.ServeHTTP(rec, req)
		return rec.Code
	}

	setLeaf := func(path, leaf, value string) {
		t.Helper()
		form := url.Values{"leaf": {leaf}, "value": {value}}
		req := postConfigRequest(t, "/config/set/"+path, form, "alice")
		rec := httptest.NewRecorder()
		setHandler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusSeeOther, rec.Code, "set %s/%s=%s", path, leaf, value)
	}

	code := addEntry("firewall/table/", url.Values{"name": {"filter"}})
	assert.Equal(t, http.StatusOK, code, "add firewall table")

	setLeaf("firewall/table/filter/", "family", "inet")

	code = addEntry("firewall/table/filter/chain/", url.Values{"name": {"input"}})
	assert.Equal(t, http.StatusOK, code, "add input chain")

	setLeaf("firewall/table/filter/chain/input/", "type", "filter")
	setLeaf("firewall/table/filter/chain/input/", "hook", "input")
	setLeaf("firewall/table/filter/chain/input/", "priority", "0")
	setLeaf("firewall/table/filter/chain/input/", "policy", "drop")

	code = addEntry("firewall/table/filter/chain/input/term/", url.Values{"name": {"allow-bgp"}})
	assert.Equal(t, http.StatusOK, code, "add bgp term")

	setLeaf("firewall/table/filter/chain/input/term/allow-bgp/from/", "protocol", "tcp")
	setLeaf("firewall/table/filter/chain/input/term/allow-bgp/from/", "destination-port", "179")

	code = addEntry("firewall/table/filter/chain/input/term/", url.Values{"name": {"allow-ssh"}})
	assert.Equal(t, http.StatusOK, code, "add ssh term")

	setLeaf("firewall/table/filter/chain/input/term/allow-ssh/from/", "source-address", "10.0.0.0/8")
	setLeaf("firewall/table/filter/chain/input/term/allow-ssh/from/", "protocol", "tcp")
	setLeaf("firewall/table/filter/chain/input/term/allow-ssh/from/", "destination-port", "22")

	setLeaf("firewall/table/filter/chain/input/term/allow-ssh/then/log/", "prefix", "SSH-ACCESS")

	count := mgr.ChangeCount("alice")
	assert.Greater(t, count, 10, "should have many pending changes")

	req := postConfigRequest(t, "/config/commit/", url.Values{}, "alice")
	rec := httptest.NewRecorder()
	commitHandler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusSeeOther, rec.Code, "commit should succeed")
	assert.Equal(t, 0, mgr.ChangeCount("alice"), "no pending changes after commit")
}

func TestScenario_NATChainWithSourceLeaf(t *testing.T) {
	mgr, schema := newHandlerTestManager(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)

	addHandler := HandleConfigAdd(mgr, schema, renderer)
	setHandler := HandleConfigSet(mgr, schema, renderer)
	commitHandler := HandleConfigCommit(mgr, renderer, nil)

	addEntry := func(path string, form url.Values) int {
		t.Helper()
		req := postConfigRequest(t, "/config/add/"+path, form, "alice")
		rec := httptest.NewRecorder()
		addHandler.ServeHTTP(rec, req)
		return rec.Code
	}

	setLeaf := func(path, leaf, value string) {
		t.Helper()
		form := url.Values{"leaf": {leaf}, "value": {value}}
		req := postConfigRequest(t, "/config/set/"+path, form, "alice")
		rec := httptest.NewRecorder()
		setHandler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusSeeOther, rec.Code, "set %s/%s=%s", path, leaf, value)
	}

	code := addEntry("firewall/table/", url.Values{"name": {"nat"}})
	assert.Equal(t, http.StatusOK, code, "add nat table")

	setLeaf("firewall/table/nat/", "family", "inet")

	code = addEntry("firewall/table/nat/chain/", url.Values{"name": {"postrouting"}})
	assert.Equal(t, http.StatusOK, code, "add postrouting chain")

	setLeaf("firewall/table/nat/chain/postrouting/", "type", "nat")
	setLeaf("firewall/table/nat/chain/postrouting/", "hook", "postrouting")
	setLeaf("firewall/table/nat/chain/postrouting/", "priority", "100")
	setLeaf("firewall/table/nat/chain/postrouting/", "policy", "accept")

	code = addEntry("firewall/table/nat/chain/postrouting/term/", url.Values{"name": {"snat-customer"}})
	assert.Equal(t, http.StatusOK, code, "add snat term")

	setLeaf("firewall/table/nat/chain/postrouting/term/snat-customer/from/", "source-address", "172.16.0.0/12")
	setLeaf("firewall/table/nat/chain/postrouting/term/snat-customer/then/snat/", "to", "198.51.100.2")

	req := postConfigRequest(t, "/config/commit/", url.Values{}, "alice")
	rec := httptest.NewRecorder()
	commitHandler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusSeeOther, rec.Code, "commit should succeed")
	assert.Equal(t, 0, mgr.ChangeCount("alice"), "no pending changes after commit")
}

func TestScenario_FullRouterSetup(t *testing.T) {
	mgr, schema := newHandlerTestManager(t)
	renderer, err := NewRenderer()
	require.NoError(t, err)

	addHandler := HandleConfigAdd(mgr, schema, renderer)
	setHandler := HandleConfigSet(mgr, schema, renderer)
	commitHandler := HandleConfigCommit(mgr, renderer, nil)

	add := func(path string, form url.Values) {
		t.Helper()
		req := postConfigRequest(t, "/config/add/"+path, form, "alice")
		rec := httptest.NewRecorder()
		addHandler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "add %s", path)
	}

	set := func(path, leaf, value string) {
		t.Helper()
		form := url.Values{"leaf": {leaf}, "value": {value}}
		req := postConfigRequest(t, "/config/set/"+path, form, "alice")
		rec := httptest.NewRecorder()
		setHandler.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusSeeOther, rec.Code, "set %s/%s", path, leaf)
	}

	set("bgp/", "router-id", "10.0.0.1")

	add("bgp/peer/", url.Values{
		"name":                       {"isp-transit"},
		"field:connection/remote/ip": {"198.51.100.1"},
		"field:session/asn/remote":   {"64496"},
	})
	add("bgp/peer/", url.Values{
		"name":                       {"customer-acme"},
		"field:connection/remote/ip": {"172.16.1.2"},
		"field:session/asn/remote":   {"65100"},
	})

	add("interface/ethernet/", url.Values{
		"name":              {"uplink"},
		"field:mac-address": {"00:1a:2b:3c:4d:01"},
	})
	set("interface/ethernet/uplink/", "description", "ISP uplink")
	add("interface/ethernet/uplink/unit/", url.Values{"name": {"0"}})
	set("interface/ethernet/uplink/unit/0/", "address", "198.51.100.2/30")

	add("interface/ethernet/", url.Values{
		"name":              {"customer"},
		"field:mac-address": {"00:1a:2b:3c:4d:02"},
	})
	add("interface/ethernet/customer/unit/", url.Values{"name": {"0"}})
	set("interface/ethernet/customer/unit/0/", "address", "172.16.1.1/30")

	add("firewall/table/", url.Values{"name": {"filter"}})
	set("firewall/table/filter/", "family", "inet")

	add("firewall/table/filter/chain/", url.Values{"name": {"input"}})
	set("firewall/table/filter/chain/input/", "type", "filter")
	set("firewall/table/filter/chain/input/", "hook", "input")
	set("firewall/table/filter/chain/input/", "priority", "0")
	set("firewall/table/filter/chain/input/", "policy", "drop")

	add("firewall/table/filter/chain/input/term/", url.Values{"name": {"allow-bgp"}})
	set("firewall/table/filter/chain/input/term/allow-bgp/from/", "protocol", "tcp")
	set("firewall/table/filter/chain/input/term/allow-bgp/from/", "destination-port", "179")

	count := mgr.ChangeCount("alice")
	assert.Greater(t, count, 15, "full router setup should produce many changes (got %d)", count)

	req := postConfigRequest(t, "/config/commit/", url.Values{}, "alice")
	rec := httptest.NewRecorder()
	commitHandler.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusSeeOther, rec.Code, "full router commit should succeed")
	assert.Equal(t, 0, mgr.ChangeCount("alice"), "no pending changes after commit")
}
