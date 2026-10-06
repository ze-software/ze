// Design: docs/architecture/traffic/fw-7-traffic-vpp.md -- external VPP restart reconciliation.
// Related: ops_linux.go -- production adapter exercised by these tests.
// Upstream: https://github.com/FDio/vpp/tree/v26.06/src/plugins/policer,
// policer_api.c and policer_op.c -- duplicate rejection and name-only discovery.

//go:build linux

package trafficvpp

import (
	"errors"
	"fmt"
	"testing"

	"go.fd.io/govpp/api"
	"go.fd.io/govpp/binapi/classify"
	interfaces "go.fd.io/govpp/binapi/interface"
	"go.fd.io/govpp/binapi/interface_types"
	"go.fd.io/govpp/binapi/policer"
	"go.fd.io/govpp/binapi/policer_types"
	"go.fd.io/govpp/codec"

	"github.com/ze-software/ze/internal/component/traffic"
)

// restartPolicer is external state, independent of the backend's trackers.
// Indices deliberately differ from dump order and survive a new backend.
type restartPolicer struct {
	index  uint32
	config policer_types.PolicerConfig
}

// restartChannel implements only the VPP policer consumer needed here. Adds
// reject duplicates, dumps return real PolicerDetails without an index, and
// deleting an object does not clear its binding (VPP 26.06 policer_del).
// Unexpected RPCs fail rather than manufacturing success.
type restartChannel struct {
	recordingChannel
	policers  map[string]restartPolicer
	outputs   map[interface_types.InterfaceIndex]string
	fail      map[string]error
	retval    map[string]int32
	nextIndex uint32
	adds      int
	updates   int
	deletes   int
	unbinds   int
	tables    map[uint32]map[string]uint32
	classify  map[interface_types.InterfaceIndex][2]uint32
	failNext  map[string][]error
}

func newRestartChannel() *restartChannel {
	return &restartChannel{
		policers: map[string]restartPolicer{
			"ze/eth0/c1":    {index: 73, config: policer_types.PolicerConfig{Cir: 99}},
			"operator/eth1": {index: 11, config: policer_types.PolicerConfig{Cir: 250}},
		},
		outputs:   map[interface_types.InterfaceIndex]string{5: "ze/eth0/c1", 8: "operator/eth1"},
		fail:      make(map[string]error),
		retval:    make(map[string]int32),
		nextIndex: 101,
		tables:    make(map[uint32]map[string]uint32),
		classify:  make(map[interface_types.InterfaceIndex][2]uint32),
		failNext:  make(map[string][]error),
	}
}

func (c *restartChannel) SendRequest(req api.Message) api.RequestCtx {
	return &restartRequest{channel: c, request: req}
}

func (c *restartChannel) SendMultiRequest(req api.Message) api.MultiRequestCtx {
	dump := &restartDump{err: c.fail[req.GetMessageName()]}
	switch req := req.(type) {
	case *interfaces.SwInterfaceDump:
		dump.replies = []api.Message{
			&interfaces.SwInterfaceDetails{InterfaceName: "eth0", SwIfIndex: 5},
			&interfaces.SwInterfaceDetails{InterfaceName: "eth1", SwIfIndex: 8},
		}
	case *policer.PolicerDump:
		for name := range c.policers {
			if req.MatchNameValid && req.MatchName != name {
				continue
			}
			dump.replies = append(dump.replies, &policer.PolicerDetails{Name: name})
		}
	case *policer.PolicerDumpV2:
		for name, p := range c.policers {
			if req.PolicerIndex == p.index || req.PolicerIndex == ^uint32(0) {
				dump.replies = append(dump.replies, &policer.PolicerDetails{
					Name: name, Cir: p.config.Cir, Eir: p.config.Eir, Cb: p.config.Cb, Eb: p.config.Eb,
					RateType: p.config.RateType, RoundType: p.config.RoundType, Type: p.config.Type,
					ColorAware: p.config.ColorAware, ConformAction: p.config.ConformAction,
					ExceedAction: p.config.ExceedAction, ViolateAction: p.config.ViolateAction,
				})
			}
		}
	default:
		dump.err = fmt.Errorf("unexpected dump %T", req)
	}
	return dump
}

type restartDump struct {
	replies []api.Message
	err     error
}

func (d *restartDump) ReceiveReply(reply api.Message) (bool, error) {
	if d.err != nil {
		return false, d.err
	}
	if len(d.replies) == 0 {
		return true, nil
	}
	err := restartReply(d.replies[0], reply)
	d.replies = d.replies[1:]
	return false, err
}

type restartRequest struct {
	channel *restartChannel
	request api.Message
}

func (r *restartRequest) ReceiveReply(reply api.Message) error {
	c := r.channel
	if failures := c.failNext[r.request.GetMessageName()]; len(failures) != 0 {
		c.failNext[r.request.GetMessageName()] = failures[1:]
		if failures[0] != nil {
			return failures[0]
		}
	}
	if err := c.fail[r.request.GetMessageName()]; err != nil {
		return err
	}
	rv := c.retval[r.request.GetMessageName()]
	switch req := r.request.(type) {
	case *policer.PolicerAddDel:
		p, exists := c.policers[req.Name]
		if req.IsAdd {
			c.adds++
			if exists {
				rv = int32(api.VALUE_EXIST)
			}
			if rv == 0 {
				p = restartPolicer{index: c.nextIndex, config: policer_types.PolicerConfig{
					Cir: req.Cir, Eir: req.Eir, Cb: req.Cb, Eb: req.Eb,
					RateType: req.RateType, RoundType: req.RoundType, Type: req.Type,
					ColorAware: req.ColorAware, ConformAction: req.ConformAction,
					ExceedAction: req.ExceedAction, ViolateAction: req.ViolateAction,
				}}
				c.nextIndex++
				c.policers[req.Name] = p
			}
		} else {
			c.deletes++
			if !exists {
				rv = int32(api.NO_SUCH_ENTRY)
			}
			if rv == 0 {
				delete(c.policers, req.Name)
			}
		}
		return restartReply(&policer.PolicerAddDelReply{Retval: rv, PolicerIndex: p.index}, reply)
	case *policer.PolicerUpdate:
		c.updates++
		name, found := c.nameAt(req.PolicerIndex)
		if !found {
			rv = int32(api.NO_SUCH_ENTRY)
		}
		if rv == 0 {
			c.policers[name] = restartPolicer{index: req.PolicerIndex, config: req.Infos}
		}
		return restartReply(&policer.PolicerUpdateReply{Retval: rv}, reply)
	case *policer.PolicerDel:
		c.deletes++
		name, found := c.nameAt(req.PolicerIndex)
		if !found {
			rv = int32(api.NO_SUCH_ENTRY)
		}
		if rv == 0 {
			delete(c.policers, name)
		}
		return restartReply(&policer.PolicerDelReply{Retval: rv}, reply)
	case *policer.PolicerOutput:
		if _, found := c.policers[req.Name]; !found {
			rv = int32(api.NO_SUCH_ENTRY)
		}
		if rv == 0 {
			if req.Apply {
				c.outputs[req.SwIfIndex] = req.Name
			} else {
				c.unbinds++
				delete(c.outputs, req.SwIfIndex)
			}
		}
		return restartReply(&policer.PolicerOutputReply{Retval: rv}, reply)
	case *classify.ClassifyAddDelTable:
		index := req.TableIndex
		if req.IsAdd {
			index = c.nextIndex
			c.nextIndex++
			c.tables[index] = make(map[string]uint32)
		} else {
			delete(c.tables, index)
		}
		return restartReply(&classify.ClassifyAddDelTableReply{NewTableIndex: index}, reply)
	case *classify.ClassifyAddDelSession:
		table, exists := c.tables[req.TableIndex]
		if !exists {
			return errors.New("session references absent table")
		}
		if req.IsAdd {
			if _, exists := c.nameAt(req.HitNextIndex); !exists {
				return errors.New("session references absent policer")
			}
			table[string(req.Match)] = req.HitNextIndex
		} else {
			delete(table, string(req.Match))
		}
		return restartReply(&classify.ClassifyAddDelSessionReply{}, reply)
	case *classify.PolicerClassifySetInterface:
		if req.IsAdd {
			for _, index := range []uint32{req.IP4TableIndex, req.IP6TableIndex} {
				if index != noTable {
					if _, exists := c.tables[index]; !exists {
						return errors.New("binding references absent table")
					}
				}
			}
			c.classify[req.SwIfIndex] = [2]uint32{req.IP4TableIndex, req.IP6TableIndex}
		} else {
			delete(c.classify, req.SwIfIndex)
		}
		return restartReply(&classify.PolicerClassifySetInterfaceReply{}, reply)
	default:
		return fmt.Errorf("unexpected request %T", req)
	}
}

func (c *restartChannel) nameAt(index uint32) (string, bool) {
	for name, p := range c.policers {
		if p.index == index {
			return name, true
		}
	}
	return "", false
}

// restartReply round-trips the actual generated wire shape. In particular,
// no fake-only index can cross the PolicerDetails response boundary.
func restartReply(source, target api.Message) error {
	if source.GetMessageName() != target.GetMessageName() {
		return fmt.Errorf("reply %s decoded as %s", source.GetMessageName(), target.GetMessageName())
	}
	wire, err := codec.EncodeMsg(source, 1)
	if err != nil {
		return err
	}
	return codec.DecodeMsg(wire, target)
}

// TestRestartReconcilesNameOnlyPolicerDump drives a fresh backend through the
// production adapter against persistent external state, then removes config
// through another fresh backend. Duplicate rejection remains enabled.
func TestRestartReconcilesNameOnlyPolicerDump(t *testing.T) {
	ch := newRestartChannel()
	foreign := ch.policers["operator/eth1"]
	ch.policers["ze/eth0/old"] = restartPolicer{index: 32}
	ch.policers["ze/unrelated"] = restartPolicer{index: 48}
	b := newOpsBackend()
	if err := applyWithOpsLocked(b, newGovppOps(ch), eth0OneClassHTB()); err != nil {
		t.Fatalf("fresh daemon apply: %v", err)
	}
	live := ch.policers["ze/eth0/c1"]
	if live.index != 101 || live.config.Cir != 1000 {
		t.Fatalf("reconciled policer = %+v, want real create index 101 and CIR 1000", live)
	}
	if got := b.interfaceOutputPolicers["eth0"]["ze/eth0/c1"]; got != live.index {
		t.Fatalf("tracked index %d differs from live index %d", got, live.index)
	}
	if ch.outputs[5] != "ze/eth0/c1" {
		t.Fatalf("desired binding absent: %v", ch.outputs)
	}
	if _, present := ch.policers["ze/eth0/old"]; present {
		t.Fatal("obsolete owned policer survived startup")
	}
	if ch.adds != 1 {
		t.Fatalf("create requests = %d, want 1", ch.adds)
	}
	if err := applyWithOpsLocked(newOpsBackend(), newGovppOps(ch), map[string]traffic.InterfaceQoS{}); err != nil {
		t.Fatalf("fresh daemon with removed config: %v", err)
	}
	if _, present := ch.policers["ze/eth0/c1"]; present {
		t.Fatal("owned policer survived removed config")
	}
	if _, bound := ch.outputs[5]; bound {
		t.Fatal("owned output binding survived removed config")
	}
	if ch.policers["operator/eth1"] != foreign || ch.outputs[8] != "operator/eth1" {
		t.Fatalf("unrelated policer or binding changed: %+v, %v", ch.policers, ch.outputs)
	}
	if _, present := ch.policers["ze/unrelated"]; !present {
		t.Fatal("name outside the complete Ze ownership shape was deleted")
	}
}

// TestReapplyUpdatesConfirmedIndex proves an existing backend updates the live
// policer instead of issuing a duplicate add or deleting its working binding.
func TestReapplyUpdatesConfirmedIndex(t *testing.T) {
	ch := newRestartChannel()
	b := newOpsBackend()
	desired := eth0OneClassHTB()
	if err := applyWithOpsLocked(b, newGovppOps(ch), desired); err != nil {
		t.Fatal(err)
	}
	prior := ch.policers["ze/eth0/c1"].index
	desired["eth0"].Qdisc.Classes[0].Rate = 2_000_000
	deletes, unbinds := ch.deletes, ch.unbinds
	if err := applyWithOpsLocked(b, newGovppOps(ch), desired); err != nil {
		t.Fatal(err)
	}
	live := ch.policers["ze/eth0/c1"]
	if live.index != prior || live.config.Cir != 2000 || live.config.Eir != 2000 {
		t.Fatalf("updated live policer = %+v, prior index %d", live, prior)
	}
	if ch.adds != 1 || ch.updates != 1 || ch.deletes != deletes || ch.unbinds != unbinds {
		t.Fatalf("unexpected mutation counts: add=%d update=%d delete=%d unbind=%d", ch.adds, ch.updates, ch.deletes, ch.unbinds)
	}
}

func TestOutputClassRenameKeepsReplacementBinding(t *testing.T) {
	ch := newRestartChannel()
	b := newOpsBackend()
	desired := eth0OneClassHTB()
	if err := applyWithOpsLocked(b, newGovppOps(ch), desired); err != nil {
		t.Fatal(err)
	}
	desired["eth0"].Qdisc.Classes[0].Name = "replacement"
	desired["eth0"].Qdisc.Classes[0].Rate = 2_000_000
	if err := applyWithOpsLocked(b, newGovppOps(ch), desired); err != nil {
		t.Fatal(err)
	}
	if _, present := ch.policers["ze/eth0/c1"]; present {
		t.Fatal("renamed class retained its old policer")
	}
	replacement, present := ch.policers["ze/eth0/replacement"]
	if !present || replacement.config.Cir != 2000 {
		t.Fatalf("replacement policer = %+v, present=%v", replacement, present)
	}
	if ch.outputs[5] != "ze/eth0/replacement" {
		t.Fatalf("replacement output attachment = %q", ch.outputs[5])
	}
}

func TestOutputRenameFailureRestoresPreviousBinding(t *testing.T) {
	ch := newRestartChannel()
	b := newOpsBackend()
	desired := eth0OneClassHTB()
	other := desired["eth0"]
	other.Qdisc.Classes = []traffic.TrafficClass{other.Qdisc.Classes[0]}
	desired["eth1"] = other
	if err := applyWithOpsLocked(b, newGovppOps(ch), desired); err != nil {
		t.Fatal(err)
	}
	prior0, prior1 := ch.policers["ze/eth0/c1"], ch.policers["ze/eth1/c1"]
	for _, qos := range desired {
		qos.Qdisc.Classes[0].Name = "replacement"
	}
	// Whichever interface is visited first changes its output slot; the
	// second creation fails. Both old attachments must survive rollback.
	want := errors.New("later policer creation failed")
	ch.failNext["policer_add_del"] = []error{nil, want}
	if err := applyWithOpsLocked(b, newGovppOps(ch), desired); !errors.Is(err, want) {
		t.Fatalf("apply error = %v, want %v", err, want)
	}
	if ch.policers["ze/eth0/c1"] != prior0 || ch.policers["ze/eth1/c1"] != prior1 ||
		ch.outputs[5] != "ze/eth0/c1" || ch.outputs[8] != "ze/eth1/c1" {
		t.Fatalf("failed rename changed prior state: %+v, %v", ch.policers, ch.outputs)
	}
	for name := range desired {
		if _, present := ch.policers["ze/"+name+"/replacement"]; present {
			t.Fatal("failed rename retained its replacement")
		}
	}
}

// TestReapplyRejectsReusedForeignIndex simulates a VPP restart that reuses a
// cached index for another owner's policer. Readback must prevent its update.
func TestReapplyRejectsReusedForeignIndex(t *testing.T) {
	ch := newRestartChannel()
	b := newOpsBackend()
	if err := applyWithOpsLocked(b, newGovppOps(ch), eth0OneClassHTB()); err != nil {
		t.Fatal(err)
	}
	prior := ch.policers["ze/eth0/c1"].index
	delete(ch.policers, "ze/eth0/c1")
	foreign := restartPolicer{index: prior, config: policer_types.PolicerConfig{Cir: 123}}
	ch.policers["operator/reused"] = foreign
	if err := applyWithOpsLocked(b, newGovppOps(ch), eth0OneClassHTB()); err != nil {
		t.Fatal(err)
	}
	if ch.policers["operator/reused"] != foreign || ch.updates != 0 {
		t.Fatal("cached index updated a foreign policer")
	}
	if ch.policers["ze/eth0/c1"].index == prior || ch.outputs[5] != "ze/eth0/c1" {
		t.Fatal("missing owned policer was not recreated and rebound")
	}
}

// TestRestartPropagatesAPIFailures proves discovery, unbind, delete and create
// failures remain errors, including a duplicate-add response after discovery.
func TestRestartPropagatesAPIFailures(t *testing.T) {
	for _, operation := range []string{"policer_dump", "policer_output", "policer_add_del"} {
		t.Run(operation, func(t *testing.T) {
			ch := newRestartChannel()
			want := errors.New("consumer unavailable")
			ch.fail[operation] = want
			if err := applyWithOpsLocked(newOpsBackend(), newGovppOps(ch), eth0OneClassHTB()); !errors.Is(err, want) {
				t.Fatalf("apply error = %v, want %v", err, want)
			}
			if _, present := ch.policers["ze/eth0/c1"]; !present {
				t.Fatal("failed cleanup deleted the existing policer")
			}
		})
	}
	t.Run("duplicate is not adoption", func(t *testing.T) {
		ch := newRestartChannel()
		delete(ch.policers, "ze/eth0/c1")
		ch.retval["policer_add_del"] = int32(api.VALUE_EXIST)
		if err := applyWithOpsLocked(newOpsBackend(), newGovppOps(ch), eth0OneClassHTB()); !errors.Is(err, api.VALUE_EXIST) {
			t.Fatalf("duplicate create error = %v, want VALUE_EXIST", err)
		}
	})
}

// TestReapplyPropagatesReadbackAndUpdateErrors injects transport and VPP retval
// failures after a working apply. The existing object and binding must survive.
func TestReapplyPropagatesReadbackAndUpdateErrors(t *testing.T) {
	for _, operation := range []string{"policer_dump_v2", "policer_update"} {
		t.Run(operation, func(t *testing.T) {
			ch := newRestartChannel()
			b := newOpsBackend()
			if err := applyWithOpsLocked(b, newGovppOps(ch), eth0OneClassHTB()); err != nil {
				t.Fatal(err)
			}
			prior := ch.policers["ze/eth0/c1"]
			want := errors.New("readback or update unavailable")
			ch.fail[operation] = want
			if err := applyWithOpsLocked(b, newGovppOps(ch), eth0OneClassHTB()); !errors.Is(err, want) {
				t.Fatalf("apply error = %v, want %v", err, want)
			}
			if ch.policers["ze/eth0/c1"] != prior || ch.outputs[5] != "ze/eth0/c1" {
				t.Fatal("failed update removed the existing object or binding")
			}
		})
	}
	t.Run("update retval", func(t *testing.T) {
		ch := newRestartChannel()
		b := newOpsBackend()
		if err := applyWithOpsLocked(b, newGovppOps(ch), eth0OneClassHTB()); err != nil {
			t.Fatal(err)
		}
		ch.retval["policer_update"] = int32(api.INVALID_VALUE)
		if err := applyWithOpsLocked(b, newGovppOps(ch), eth0OneClassHTB()); !errors.Is(err, api.INVALID_VALUE) {
			t.Fatalf("update error = %v, want INVALID_VALUE", err)
		}
	})
}

// TestOwnedPolicerInterfaceNames covers full ownership names, including VPP
// interface names with slashes, and names that do not identify a Ze class.
func TestOwnedPolicerInterfaceNames(t *testing.T) {
	for _, tt := range []struct {
		name  string
		iface string
		owned bool
	}{
		{"ze/eth0/c1", "eth0", true},
		{"ze/GigabitEthernet0/1/0/c1", "GigabitEthernet0/1/0", true},
		{"operator/eth0/c1", "", false},
		{"ze/unrelated", "", false},
		{"ze//c1", "", false},
		{"ze/eth0/", "", false},
	} {
		got, owned := ifaceNameFromPolicerName(tt.name)
		if got != tt.iface || owned != tt.owned {
			t.Errorf("ownership of %q = (%q, %v), want (%q, %v)", tt.name, got, owned, tt.iface, tt.owned)
		}
	}
}

// Failed rebind and later-class failures must restore live configuration, not
// merely retain the backend's old index tracker.
func TestReapplyFailureRestoresLiveConfiguration(t *testing.T) {
	for _, laterClass := range []bool{false, true} {
		t.Run(fmt.Sprintf("later-class-%v", laterClass), func(t *testing.T) {
			ch := newRestartChannel()
			b := newOpsBackend()
			desired := eth0OneClassHTB()
			if err := applyWithOpsLocked(b, newGovppOps(ch), desired); err != nil {
				t.Fatal(err)
			}
			// Readback, rather than cached desired, is the rollback authority.
			prior := ch.policers["ze/eth0/c1"]
			prior.config.Cir = 777
			prior.config.Cb = 12345
			ch.policers["ze/eth0/c1"] = prior
			desired["eth0"].Qdisc.Classes[0].Rate = 2_000_000
			want := errors.New("apply failure")
			if laterClass {
				qos := desired["eth0"]
				qos.Qdisc.Classes = append(qos.Qdisc.Classes, traffic.TrafficClass{Name: "c2", Rate: 3_000_000})
				desired["eth0"] = qos
				ch.fail["policer_add_del"] = want
			} else {
				ch.fail["policer_output"] = want
			}
			if err := applyWithOpsLocked(b, newGovppOps(ch), desired); !errors.Is(err, want) {
				t.Fatalf("apply error = %v, want %v", err, want)
			}
			if ch.policers["ze/eth0/c1"] != prior || ch.outputs[5] != "ze/eth0/c1" {
				t.Fatalf("failed apply changed live state: %+v, %v", ch.policers, ch.outputs)
			}
		})
	}
}

// TestReapplyReportsRestoreFailure injects distinct apply and recovery failures
// and checks both errors and the actual unrecovered live rate.
func TestReapplyReportsRestoreFailure(t *testing.T) {
	ch := newRestartChannel()
	b := newOpsBackend()
	desired := eth0OneClassHTB()
	if err := applyWithOpsLocked(b, newGovppOps(ch), desired); err != nil {
		t.Fatal(err)
	}
	desired["eth0"].Qdisc.Classes[0].Rate = 2_000_000
	applyFailure, restoreFailure := errors.New("rebind failed"), errors.New("restore failed")
	ch.fail["policer_output"] = applyFailure
	ch.failNext["policer_update"] = []error{nil, restoreFailure}
	err := applyWithOpsLocked(b, newGovppOps(ch), desired)
	if !errors.Is(err, applyFailure) || !errors.Is(err, restoreFailure) {
		t.Fatalf("error = %v, want both failures", err)
	}
	if ch.policers["ze/eth0/c1"].config.Cir != 2000 || ch.outputs[5] != "ze/eth0/c1" {
		t.Fatal("consumer did not preserve the failed-restore state")
	}
}

// TestOutputToClassifyMigrationState checks live attachments and session targets
// through the production adapter, including recovery after an unbind failure.
func TestOutputToClassifyMigrationState(t *testing.T) {
	for _, filter := range []traffic.TrafficFilter{
		{Type: traffic.FilterProtocol, Value: 6},
		{Type: traffic.FilterDSCP, Value: 46},
	} {
		for _, failUnbind := range []bool{false, true} {
			t.Run(fmt.Sprintf("%v/fail-%v", filter.Type, failUnbind), func(t *testing.T) {
				ch := newRestartChannel()
				b := newOpsBackend()
				desired := eth0OneClassHTB()
				if err := applyWithOpsLocked(b, newGovppOps(ch), desired); err != nil {
					t.Fatal(err)
				}
				prior := ch.policers["ze/eth0/c1"]
				desired["eth0"].Qdisc.Classes[0].Filters = []traffic.TrafficFilter{filter}
				desired["eth0"].Qdisc.Classes[0].Rate = 2_000_000
				want := errors.New("migration unbind failed")
				if failUnbind {
					ch.fail["policer_output"] = want
				}
				err := applyWithOpsLocked(b, newGovppOps(ch), desired)
				if failUnbind {
					if !errors.Is(err, want) {
						t.Fatalf("migration error = %v", err)
					}
					if ch.policers["ze/eth0/c1"] != prior || ch.outputs[5] != "ze/eth0/c1" || len(ch.classify) != 0 || len(ch.tables) != 0 {
						t.Fatalf("migration rollback state: %+v, %v, %v, %v", ch.policers, ch.outputs, ch.classify, ch.tables)
					}
					return
				}
				if err != nil {
					t.Fatal(err)
				}
				if _, bound := ch.outputs[5]; bound {
					t.Fatal("obsolete output attachment survived migration")
				}
				live := ch.policers["ze/eth0/c1"]
				if live.index != prior.index || live.config.Cir != 2000 {
					t.Fatalf("migration replaced or misconfigured policer: %+v", live)
				}
				heads, bound := ch.classify[5]
				if !bound {
					t.Fatal("classify attachment absent")
				}
				for _, head := range heads {
					sessions := ch.tables[head]
					if len(sessions) != 1 {
						t.Fatalf("head %d has sessions %v", head, sessions)
					}
					for _, index := range sessions {
						if index != live.index {
							t.Fatalf("session targets %d, want %d", index, live.index)
						}
					}
				}
				if ch.outputs[8] != "operator/eth1" {
					t.Fatal("migration changed unrelated interface")
				}
			})
		}
	}
}

// Map order is deliberately irrelevant: the second interface fails after the
// first has installed its new classify binding (and removed output if present).
func TestLaterInterfaceFailureRestoresPolicersAndAttachments(t *testing.T) {
	for _, initiallyClassified := range []bool{false, true} {
		t.Run(fmt.Sprintf("classified-%v", initiallyClassified), func(t *testing.T) {
			ch := newRestartChannel()
			b := newOpsBackend()
			desired := eth0OneClassHTB()
			if initiallyClassified {
				desired = eth0OneClassProtoHTB()
			}
			second := eth0OneClassHTB()["eth0"]
			if initiallyClassified {
				second = eth0OneClassProtoHTB()["eth0"]
			}
			second.Interface = "eth1"
			desired["eth1"] = second
			if err := applyWithOpsLocked(b, newGovppOps(ch), desired); err != nil {
				t.Fatal(err)
			}
			prior0, prior1 := ch.policers["ze/eth0/c1"], ch.policers["ze/eth1/c1"]
			heads0, heads1 := ch.classify[5], ch.classify[8]
			tableCount := len(ch.tables)
			for _, qos := range desired {
				qos.Qdisc.Classes[0].Rate = 2_000_000
				qos.Qdisc.Classes[0].Filters = []traffic.TrafficFilter{{Type: traffic.FilterDSCP, Value: 46}}
			}
			want := errors.New("second interface failed")
			if initiallyClassified {
				ch.failNext["policer_classify_set_interface"] = []error{nil, want}
			} else {
				ch.failNext["policer_output"] = []error{nil, want}
			}
			if err := applyWithOpsLocked(b, newGovppOps(ch), desired); !errors.Is(err, want) {
				t.Fatalf("apply error = %v, want %v", err, want)
			}
			if ch.policers["ze/eth0/c1"] != prior0 || ch.policers["ze/eth1/c1"] != prior1 {
				t.Fatalf("prior configurations not restored: %+v", ch.policers)
			}
			if len(ch.tables) != tableCount {
				t.Fatalf("table count = %d, want %d", len(ch.tables), tableCount)
			}
			if initiallyClassified {
				if ch.classify[5] != heads0 || ch.classify[8] != heads1 {
					t.Fatalf("prior classify attachments not restored: %v", ch.classify)
				}
				for _, heads := range [][2]uint32{heads0, heads1} {
					for _, head := range heads {
						if len(ch.tables[head]) != 1 {
							t.Fatalf("prior classify sessions not retained: %v", ch.tables)
						}
					}
				}
			} else if ch.outputs[5] != "ze/eth0/c1" || ch.outputs[8] != "ze/eth1/c1" || len(ch.classify) != 0 {
				t.Fatalf("prior output attachments not restored: %v, %v", ch.outputs, ch.classify)
			}
		})
	}
}
