package reactor

import (
	"fmt"
	"net/netip"
	"reflect"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// dynamicPeerDivergence names every PeerSettings field a dynamic peer is allowed
// NOT to inherit from its group template, and says why.
//
// A field absent from this table MUST arrive by inheritance. That is the whole
// contract of buildDynamicPeerSettings, and the test below is what holds anybody
// to it: adding a field to PeerSettings and forgetting the dynamic path is the
// defect class this table exists to close. Four separate defects found on
// 2026-08-13 were one field-by-field copy list that nobody kept current.
//
// The reason column is not decoration. A divergence has to be arguable from the
// protocol or from the peer map's keying; "it was easier" is not a reason, and a
// reader who cannot follow the reason must delete the row and watch what
// breaks.
// divergence is one field a dynamic peer is allowed NOT to inherit: the value it
// must hold instead, and why the group template cannot supply it.
type divergence struct {
	want   any
	reason string
}

// dynamicPeerDivergence names EVERY such field. A field absent from this table
// MUST arrive by inheritance.
//
// That is the whole contract of buildDynamicPeerSettings, and the test below is
// what holds anybody to it: adding a field to PeerSettings and forgetting the
// dynamic path is the defect class this table exists to close. Four separate
// defects found on 2026-08-13 were one field-by-field copy list nobody kept
// current.
//
// The reason is not decoration. A divergence has to be arguable from the
// protocol or from the peer map's keying; "it was easier" is not a reason, and a
// reader who cannot follow one must delete the row and watch what breaks.
func dynamicPeerDivergence(groupName string, addr netip.Addr) map[string]divergence {
	var tb textbuf.Buffer
	return map[string]divergence{
		"Name":       {tb.Str("dyn-").Addr(addr).String(), "a dynamic peer names itself after the address it connected from; no configuration names it"},
		"GroupName":  {groupName, "the group whose range the accepted connection matched"},
		"Address":    {addr, "the remote address arrives with the TCP connection, not from configuration"},
		"Port":       {uint16(DefaultBGPPort), "the peer map is keyed on (address, port) and the remote source port is ephemeral, so every dynamic peer keys on the canonical port. The template's own Port is what the group LISTENS on"},
		"PeerAS":     {uint32(0), "RFC 4271 Section 4.2: the peer's AS arrives in its OPEN, and resolveDynamicPeerSettings writes it there"},
		"Connection": {ConnectionPassive, "`ip dynamic` requires `connect false`: the group only ever accepts"},
		"IsDynamic":  {true, "the flag that says this peer was built from a group rather than configured"},
	}
}

// TestDynamicPeerInheritsEveryPeerSettingsField walks PeerSettings by reflection
// and refuses a field that a dynamic peer neither inherits from its group
// template nor declares in dynamicPeerDivergence.
//
// VALIDATES: buildDynamicPeerSettings carries every configured setting from the
// group template to each member built from it.
// PREVENTS: the defect CLASS behind four dynamic-peer bugs found on 2026-08-13.
// A field-by-field copy list stood in buildDynamicPeerSettings, and a setting
// nobody remembered to add to it was dropped in silence: no build error, no
// config error, no log line, and a session that establishes and then behaves
// unlike every static peer configured the same way. This test fails on the next
// field added, before it can become the fifth bug.
func TestDynamicPeerInheritsEveryPeerSettingsField(t *testing.T) {
	tmpl := fullyPopulatedPeerSettings(t)
	dg := &DynamicGroupConfig{
		GroupName: "ix",
		Ranges:    []netip.Prefix{netip.MustParsePrefix("192.0.2.0/24")},
		Settings:  tmpl,
	}
	r := &Reactor{}

	addr := netip.MustParseAddr("192.0.2.7")
	got, err := r.buildDynamicPeerSettings(dg, addr)
	require.NoError(t, err)

	tv := reflect.ValueOf(*tmpl)
	gv := reflect.ValueOf(*got)
	typ := tv.Type()
	diverges := dynamicPeerDivergence(dg.GroupName, addr)

	for i := range typ.NumField() {
		name := typ.Field(i).Name
		fromTemplate, built := tv.Field(i).Interface(), gv.Field(i).Interface()

		if d, ok := diverges[name]; ok {
			assert.Equalf(t, d.want, built, "PeerSettings.%s must hold the value the divergence states: %s", name, d.reason)
			// The template holds something else, so the row above is testing a
			// real divergence rather than restating what inheritance would give.
			assert.NotEqualf(t, fromTemplate, d.want,
				"PeerSettings.%s is declared a divergence but the template already holds that value, "+
					"so the row proves nothing. Give the template a different value, or delete the row.", name)
			continue
		}

		assert.Equalf(t, fromTemplate, built,
			"PeerSettings.%s is not inherited by a dynamic peer.\n"+
				"Every field a group template configures reaches its members unless it is named in "+
				"dynamicPeerDivergence with a reason (reactor_dynamic_inherit_test.go). "+
				"A field silently dropped here is a setting an operator configured and an IXP member never got.",
			name)
	}
}

// TestDynamicPeerClonesTheMapsItsSiblingsWouldShare pins the other half of a
// whole-struct copy: a shallow copy shares every map header with the template,
// so a per-peer write would reach every sibling built from the same group.
//
// VALIDATES: the seven per-family prefix maps are cloned per member.
// PREVENTS: one member's prefix accounting changing enforcement for the rest of
// the group -- the failure mode of sharing a map between peers built from one
// template, which for an IXP route server is every peer it has.
func TestDynamicPeerClonesTheMapsItsSiblingsWouldShare(t *testing.T) {
	tmpl := fullyPopulatedPeerSettings(t)
	dg := &DynamicGroupConfig{GroupName: "ix", Settings: tmpl}
	r := &Reactor{}

	first, err := r.buildDynamicPeerSettings(dg, netip.MustParseAddr("192.0.2.7"))
	require.NoError(t, err)
	second, err := r.buildDynamicPeerSettings(dg, netip.MustParseAddr("192.0.2.8"))
	require.NoError(t, err)

	first.PrefixMaximum["ipv4/unicast"] = 1
	first.PrefixCount["ipv4/unicast"] = PrefixCountInstalled
	first.PrefixUpdated["ipv4/unicast"] = "1999-01-01"

	assert.Equal(t, uint32(10000), second.PrefixMaximum["ipv4/unicast"], "a sibling must not see another peer's prefix maximum")
	assert.Equal(t, PrefixCountOffered, second.PrefixCount["ipv4/unicast"], "a sibling must not see another peer's count mode")
	assert.Equal(t, "2026-01-01", second.PrefixUpdated["ipv4/unicast"], "a sibling must not see another peer's refresh date")
	assert.Equal(t, uint32(10000), tmpl.PrefixMaximum["ipv4/unicast"], "the template itself must not be written through")
}

// fullyPopulatedPeerSettings builds a template whose every field holds a value
// distinguishable from its zero value, so an uninherited field shows up as a
// difference rather than as two zeros that happen to match.
//
// It walks PeerSettings by reflection, so a field added to the struct is filled
// here without anybody editing this function. A type it cannot fill fails the
// test loudly: a silently skipped field would leave the guard above vacuous for
// exactly the field somebody just added.
func fullyPopulatedPeerSettings(t *testing.T) *PeerSettings {
	t.Helper()

	ps := &PeerSettings{}
	v := reflect.ValueOf(ps).Elem()
	typ := v.Type()

	for i := range typ.NumField() {
		field := v.Field(i)
		name := typ.Field(i).Name
		require.Truef(t, field.CanSet(), "PeerSettings.%s is unexported; the guard cannot see it", name)
		require.NoErrorf(t, fillDistinctValue(field, i+1), "PeerSettings.%s", name)
	}

	// Three maps carry a fixed value the sibling-isolation test reads back.
	ps.PrefixMaximum = map[string]uint32{"ipv4/unicast": 10000}
	ps.PrefixCount = map[string]PrefixCountMode{"ipv4/unicast": PrefixCountOffered}
	ps.PrefixUpdated = map[string]string{"ipv4/unicast": "2026-01-01"}

	// Every divergent field holds something the built peer must NOT keep, so the
	// divergence rows above assert a real substitution rather than a coincidence.
	ps.Name = "template"
	ps.GroupName = "some-other-group"
	ps.Address = netip.MustParseAddr("192.0.2.254")
	ps.Port = 1179
	ps.PeerAS = 65001
	ps.Connection = ConnectionBoth
	ps.IsDynamic = false
	return ps
}

// fillDistinctValue writes a non-zero value into one field, using seed to keep
// numeric fields distinguishable from each other.
func fillDistinctValue(field reflect.Value, seed int) error {
	// Types whose zero value is reachable through reflection but whose contents
	// are not settable field by field.
	switch field.Type() {
	case reflect.TypeFor[netip.Addr]():
		field.Set(reflect.ValueOf(netip.AddrFrom4([4]byte{198, 51, 100, byte(seed)})))
		return nil
	case reflect.TypeFor[netip.Prefix]():
		field.Set(reflect.ValueOf(netip.PrefixFrom(netip.AddrFrom4([4]byte{203, 0, 113, 0}), 24)))
		return nil
	case reflect.TypeFor[time.Duration]():
		// RFC 4271 Section 4.2 and RFC 9687 bound the timers; a plain seed would
		// not be a legal hold time. The value only has to differ from zero.
		field.Set(reflect.ValueOf(time.Duration(seed+600) * time.Second))
		return nil
	case reflect.TypeFor[[]capability.Capability]():
		field.Set(reflect.ValueOf([]capability.Capability{&capability.Multiprotocol{AFI: family.AFI(1), SAFI: family.SAFI(1)}}))
		return nil
	case reflect.TypeFor[[]capability.Family]():
		field.Set(reflect.ValueOf([]capability.Family{{AFI: family.AFI(2), SAFI: family.SAFI(1)}}))
		return nil
	case reflect.TypeFor[[]capability.Code]():
		field.Set(reflect.ValueOf([]capability.Code{capability.CodeRouteRefresh}))
		return nil
	case reflect.TypeFor[[]filterapi.FilterRef]():
		field.Set(reflect.ValueOf([]filterapi.FilterRef{{Name: "bgp-filter-prefix:MEMBERS"}}))
		return nil
	}

	switch field.Kind() { //nolint:exhaustive // every kind PeerSettings uses is listed; the default fails loudly
	case reflect.String:
		var tb textbuf.Buffer
		field.SetString(tb.Str("sentinel-").Int(int64(seed)).String())
	case reflect.Bool:
		field.SetBool(true)
	case reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uint:
		field.SetUint(uint64(seed))
	case reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Int:
		field.SetInt(int64(seed))
	case reflect.Struct:
		for i := range field.NumField() {
			if !field.Field(i).CanSet() {
				return fmt.Errorf("unexported field in %s", field.Type())
			}
			if err := fillDistinctValue(field.Field(i), seed+i); err != nil {
				return err
			}
		}
	case reflect.Pointer:
		field.Set(reflect.New(field.Type().Elem()))
		return fillDistinctValue(field.Elem(), seed)
	case reflect.Map:
		m := reflect.MakeMap(field.Type())
		key := reflect.New(field.Type().Key()).Elem()
		if err := fillDistinctValue(key, seed); err != nil {
			return err
		}
		val := reflect.New(field.Type().Elem()).Elem()
		if err := fillDistinctValue(val, seed); err != nil {
			return err
		}
		m.SetMapIndex(key, val)
		field.Set(m)
	case reflect.Slice:
		elem := reflect.New(field.Type().Elem()).Elem()
		if err := fillDistinctValue(elem, seed); err != nil {
			return err
		}
		field.Set(reflect.Append(reflect.MakeSlice(field.Type(), 0, 1), elem))
	case reflect.Array:
		for i := range field.Len() {
			if err := fillDistinctValue(field.Index(i), seed+i); err != nil {
				return err
			}
		}
	case reflect.Interface:
		// An interface field holds no value to copy wrongly unless something is
		// in it, so leaving it nil would make the guard vacuous for that field.
		// bgptypes.RouteNextHop is the only one, and its concrete forms live in
		// another package, so the value comes from the constructor.
		if field.Type() == reflect.TypeFor[bgptypes.RouteNextHop]() {
			field.Set(reflect.ValueOf(bgptypes.NewNextHopExplicit(netip.AddrFrom4([4]byte{198, 51, 100, byte(seed)}))))
			return nil
		}
		return fmt.Errorf("no distinct value known for interface %s", field.Type())
	default:
		return fmt.Errorf("no distinct value known for kind %s (%s): teach fillDistinctValue about it "+
			"rather than skipping it, or the inheritance guard is vacuous for this field", field.Kind(), field.Type())
	}
	return nil
}
