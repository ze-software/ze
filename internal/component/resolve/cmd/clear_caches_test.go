package cmd

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

type clearHandler func(*pluginserver.CommandContext, command.ValidatedArgs) (*plugin.Response, error)

func runClear(t *testing.T, handler clearHandler, args ...string) *plugin.Response {
	t.Helper()
	resp, err := handler(nil, commandtest.Args(args...))
	if err != nil {
		t.Fatalf("handler returned an error: %v", err)
	}
	return resp
}

// fakeOwners answers like *pluginserver.Server for a fixed set of plugins:
// removed holds each plugin's count, failing the plugins whose call fails, and
// blocking the plugins that never answer until the call's context ends.
type fakeOwners struct {
	removed  map[string]int
	failing  map[string]bool
	blocking map[string]bool
	asked    []rpc.ResolverCacheClearInput
}

func (f *fakeOwners) ResolverCacheOwners(string) []string {
	var names []string
	for name := range f.removed {
		names = append(names, name)
	}
	for name := range f.failing {
		names = append(names, name)
	}
	for name := range f.blocking {
		names = append(names, name)
	}
	return names
}

func (f *fakeOwners) CallResolverCacheClear(ctx context.Context, name string, input *rpc.ResolverCacheClearInput) (*rpc.ResolverCacheClearOutput, error) {
	f.asked = append(f.asked, *input)
	if f.blocking[name] {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if f.failing[name] {
		return nil, errors.New("connection closed")
	}
	return &rpc.ResolverCacheClearOutput{Removed: f.removed[name]}, nil
}

func doneData(t *testing.T, resp *plugin.Response) plugin.Map {
	t.Helper()
	if resp.Status != plugin.StatusDone {
		t.Fatalf("status = %s (%s), want done", resp.Status, resp.Error)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("data is %T, want plugin.Map", resp.Data)
	}
	return data
}

// VALIDATES: a clear-all sums exactly what each owner removed and lists each
// owner; method: the hub removes 3 and two plugins remove 2 and 0.
// PREVENTS: a count that includes clients no owner uses, or drops a plugin.
func TestClearAnswer_SumsEveryOwner(t *testing.T) {
	owners := &fakeOwners{removed: map[string]int{"bgp-filter-irr": 2, "firewall-irr": 0}}
	request := clearAllRequest(rpc.ResolverCacheIRR, func() int { return 3 })
	data := doneData(t, clearAnswer(context.Background(), owners, "clear irr cache all", request, "", nil))

	if data[keyAction] != actionClearAll || data[keyRemoved] != 5 {
		t.Fatalf("answer %v, want clear-all with removed 5", data)
	}
	rows, ok := data[keyOwners].([]plugin.Map)
	if !ok || len(rows) != 3 {
		t.Fatalf("owners = %v, want the hub and both plugins", data[keyOwners])
	}
	got := map[any]any{}
	for _, row := range rows {
		got[row[keyOwner]] = row[keyRemoved]
	}
	if got[ownerHub] != 3 || got["bgp-filter-irr"] != 2 || got["firewall-irr"] != 0 {
		t.Fatalf("owners = %v, want hub 3, bgp-filter-irr 2, firewall-irr 0", got)
	}
	for _, input := range owners.asked {
		if input.Cache != rpc.ResolverCacheIRR || input.Action != rpc.ResolverCacheClearAll || input.Key != "" {
			t.Fatalf("a plugin was asked %+v, want the irr clear-all", input)
		}
	}
}

// VALIDATES: a delete-entry is found when any owner removed an entry, and the
// plugins receive the parsed key; method: only a plugin holds the key.
// PREVENTS: found answered from the hub alone.
func TestClearAnswer_DeleteEntryFoundInPlugin(t *testing.T) {
	owners := &fakeOwners{removed: map[string]int{"bgp-filter-irr": 1}}
	request := deleteEntryRequest(rpc.ResolverCachePeeringDB, "65546", func() int { return 0 })
	data := doneData(t, clearAnswer(context.Background(), owners, "clear peeringdb cache asn 1.10", request, keyASNumber, uint32(65546)))

	if data[keyAction] != rpc.ResolverCacheDeleteEntry || data[keyASNumber] != uint32(65546) {
		t.Fatalf("answer %v, want delete-entry of as-number 65546", data)
	}
	if data[keyFound] != true || data[keyRemoved] != 1 {
		t.Fatalf("answer %v, want found true and removed 1", data)
	}
	if len(owners.asked) != 1 || owners.asked[0].Key != "65546" {
		t.Fatalf("plugins were asked %+v, want one delete-entry of 65546", owners.asked)
	}

	owners = &fakeOwners{removed: map[string]int{"bgp-filter-irr": 0}}
	data = doneData(t, clearAnswer(context.Background(), owners, "clear peeringdb cache asn 1.10", request, keyASNumber, uint32(65546)))
	if data[keyFound] != false || data[keyRemoved] != 0 {
		t.Fatalf("answer %v, want found false and removed 0 when no owner held it", data)
	}
}

// VALIDATES: a plugin that does not answer makes the clear an error naming it
// and the retry. PREVENTS: a failed owner read as one that held nothing.
func TestClearAnswer_FailedOwnerIsAnError(t *testing.T) {
	owners := &fakeOwners{removed: map[string]int{"bgp-filter-irr": 2}, failing: map[string]bool{"firewall-irr": true}}
	resp := clearAnswer(context.Background(), owners, "clear irr cache all", clearAllRequest(rpc.ResolverCacheIRR, nil), "", nil)
	if resp.Status != plugin.StatusError {
		t.Fatalf("status = %s, want error", resp.Status)
	}
	for _, want := range []string{"cleared 2 entries", "plugin firewall-irr did not answer", "connection closed", "show plugin list", "clear irr cache all"} {
		if !strings.Contains(resp.Error, want) {
			t.Errorf("error %q does not hold %q", resp.Error, want)
		}
	}
}

// VALIDATES: a plugin that never answers is given up on at ownerClearTimeout,
// and the clear is an error naming it, with the count and the rows of the
// owners that answered in both the message and the data; method: the timeout
// shortened to 50 ms, a plugin that blocks until its context ends, a
// background parent context so only the per-owner deadline can end the wait.
// PREVENTS: a hung plugin hanging the operator's command, or the error losing
// what the other owners removed.
func TestClearAnswer_BlockedOwnerTimesOut(t *testing.T) {
	saved := ownerClearTimeout
	ownerClearTimeout = 50 * time.Millisecond
	t.Cleanup(func() { ownerClearTimeout = saved })

	owners := &fakeOwners{removed: map[string]int{"bgp-filter-irr": 2}, blocking: map[string]bool{"firewall-irr": true}}
	request := clearAllRequest(rpc.ResolverCacheIRR, func() int { return 3 })
	start := time.Now()
	resp := clearAnswer(context.Background(), owners, "clear irr cache all", request, "", nil)
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("clear took %s, want it to give up on the blocked owner after 50ms", elapsed)
	}
	if resp.Status != plugin.StatusError {
		t.Fatalf("status = %s, want error", resp.Status)
	}
	for _, want := range []string{"cleared 5 entries", "plugin firewall-irr did not answer", context.DeadlineExceeded.Error()} {
		if !strings.Contains(resp.Error, want) {
			t.Errorf("error %q does not hold %q", resp.Error, want)
		}
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok {
		t.Fatalf("data is %T, want plugin.Map with the owners that answered", resp.Data)
	}
	if data[keyRemoved] != 5 {
		t.Errorf("data removed = %v, want 5", data[keyRemoved])
	}
	rows, ok := data[keyOwners].([]plugin.Map)
	if !ok || len(rows) != 2 {
		t.Fatalf("data owners = %v, want the hub and bgp-filter-irr", data[keyOwners])
	}
	got := map[any]any{}
	for _, row := range rows {
		got[row[keyOwner]] = row[keyRemoved]
	}
	if got[ownerHub] != 3 || got["bgp-filter-irr"] != 2 {
		t.Errorf("data owners = %v, want hub 3 and bgp-filter-irr 2", got)
	}
}

// VALIDATES: a form whose words parse reaches the owners, and with no plugin
// server it says so; method: each handler with a valid key and a nil context.
// PREVENTS: asn 0.65001 refused as a bad key, or a missing server answered as
// removed 0.
func TestClearCache_ValidWordsReachOwners(t *testing.T) {
	cases := map[string]struct {
		handler clearHandler
		args    []string
	}{
		"peeringdb all":   {handleClearPeeringDBCacheAll, nil},
		"peeringdb asdot": {handleClearPeeringDBCacheASN, []string{"1.10"}},
		"irr all":         {handleClearIRRCacheAll, nil},
		"irr as-set":      {handleClearIRRCacheASSet, []string{"AS-NOT-CACHED"}},
		"cymru all":       {handleClearCymruCacheAll, nil},
		"cymru address":   {handleClearCymruCacheAddress, []string{"2001:db8:0::5"}},
	}
	for name, tc := range cases {
		resp := runClear(t, tc.handler, tc.args...)
		if resp.Status != plugin.StatusError || !strings.Contains(resp.Error, "plugin server is not running") {
			t.Errorf("%s: %s %q, want the refusal that no plugin server runs", name, resp.Status, resp.Error)
		}
	}
	if resp := runClear(t, handleClearIRRCacheAll, "extra"); !strings.Contains(resp.Error, "takes no words after all") {
		t.Errorf("a word after all: %q, want the all-form refusal", resp.Error)
	}
}

// VALIDATES: a bad key is refused with what is wrong and the command that
// works. PREVENTS: a typo answered as found false, which reads as "not cached".
func TestClearCacheKeyed_RefusesBadKey(t *testing.T) {
	cases := []struct {
		name    string
		handler clearHandler
		args    []string
		usage   string
	}{
		{"asn word", handleClearPeeringDBCacheASN, []string{"AS-FOO"}, "clear peeringdb cache asn 65001"},
		{"asn missing", handleClearPeeringDBCacheASN, nil, "clear peeringdb cache asn 65001"},
		{"as-set control", handleClearIRRCacheASSet, []string{"AS-\x01"}, "clear irr cache as-set AS-EXAMPLE"},
		{"as-set two", handleClearIRRCacheASSet, []string{"AS-A", "AS-B"}, "clear irr cache as-set AS-EXAMPLE"},
		{"address word", handleClearCymruCacheAddress, []string{"router1"}, "clear cymru cache address 192.0.2.1"},
	}
	for _, tc := range cases {
		resp := runClear(t, tc.handler, tc.args...)
		if resp.Status != plugin.StatusError {
			t.Errorf("%s: status = %s, want error", tc.name, resp.Status)
			continue
		}
		if !strings.Contains(resp.Error, tc.usage) {
			t.Errorf("%s: error %q does not name the command that works, %q", tc.name, resp.Error, tc.usage)
		}
	}
}
