//go:build linux

package firewallnft

import (
	"testing"

	"github.com/google/nftables"
	"github.com/google/nftables/expr"
	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/firewall"
)

// TestLowerCtHelperObject proves a ct helper object and its assignment lower
// to the expressions nft itself writes for `ct helper "x" { type "ftp"
// protocol tcp; }` and `ct helper set "x"`.
//
// VALIDATES: the object carries the kernel helper name and the transport's IP
// protocol number, and no L3 protocol, so the kernel takes the table's family
// (an inet object resolves both helpers); the action is an objref of type
// NFT_OBJECT_CT_HELPER naming the object; a transport other than tcp or udp
// and an empty object name are refused.
// PREVENTS: an assignment the kernel resolves to no helper, or an IPv4-only
// L3 protocol pinned on an inet object.
func TestLowerCtHelperObject(t *testing.T) {
	table := &nftables.Table{Name: "ze_conntrack_helpers", Family: nftables.TableFamilyINet}

	tests := []struct {
		protocol string
		l4       uint8
	}{
		{"tcp", unix.IPPROTO_TCP},
		{"udp", unix.IPPROTO_UDP},
	}
	for _, tt := range tests {
		obj, err := lowerCtHelper(table, &firewall.CtHelper{Name: "ftp-" + tt.protocol, Helper: "ftp", Protocol: tt.protocol})
		if err != nil {
			t.Fatalf("lowerCtHelper(%s): %v", tt.protocol, err)
		}
		if obj.Table != table || obj.Name != "ftp-"+tt.protocol || obj.Type != nftables.ObjTypeCtHelper {
			t.Fatalf("object = %+v, want table %s, name ftp-%s, type ct helper", obj, table.Name, tt.protocol)
		}
		helper, ok := obj.Obj.(*expr.CtHelper)
		if !ok {
			t.Fatalf("object body %T, want *expr.CtHelper", obj.Obj)
		}
		if helper.Name != "ftp" || helper.L4Proto != tt.l4 || helper.L3Proto != 0 {
			t.Fatalf("ct helper = %+v, want name ftp, l4 %d, l3 unset", helper, tt.l4)
		}
	}

	if _, err := lowerCtHelper(table, &firewall.CtHelper{Name: "x", Helper: "ftp", Protocol: "sctp"}); err == nil {
		t.Fatal("lowerCtHelper accepted sctp")
	}

	exprs, err := lowerAction(firewall.SetCtHelper{Name: "ftp-tcp"}, unix.NFPROTO_UNSPEC)
	if err != nil {
		t.Fatalf("lowerAction(SetCtHelper): %v", err)
	}
	if len(exprs) != 1 {
		t.Fatalf("lowerAction(SetCtHelper) = %d expressions, want 1", len(exprs))
	}
	ref, ok := exprs[0].(*expr.Objref)
	if !ok {
		t.Fatalf("expression %T, want *expr.Objref", exprs[0])
	}
	if ref.Type != unix.NFT_OBJECT_CT_HELPER || ref.Name != "ftp-tcp" {
		t.Fatalf("objref = %+v, want type %d name ftp-tcp", ref, unix.NFT_OBJECT_CT_HELPER)
	}

	if _, err := lowerAction(firewall.SetCtHelper{}, unix.NFPROTO_UNSPEC); err == nil {
		t.Fatal("lowerAction accepted a ct helper assignment with no object name")
	}
}
