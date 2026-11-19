// Design: docs/architecture/l2tp/bng-5-pppoe.md -- VPP startup and config wiring

package pppoevpp

import (
	"bytes"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	_ "github.com/ze-software/ze/internal/component/l2tp/pppoe"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	vppcomp "github.com/ze-software/ze/internal/component/vpp"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

// Registering the provider must make a valid VPP/LCP candidate reachable
// through the original-tree validator, rather than leaving the old refusal.
func TestVPPPPPoEValidationAllowsPreparedBackend(t *testing.T) {
	tree:=vppPPPoETree(t,`interface { backend vpp } vpp { enabled true; lcp { enabled true } } pppoe { interface access0 {} }`)
	if _,err:=config.ValidateCustomSections(tree);err!=nil{t.Fatalf("registered VPP candidate refused: %v",err)}
}

// A disabled control-plane mirror must refuse before any socket or API call.
func TestVPPPPPoEValidationRequiresLCP(t *testing.T) {
	tree:=vppPPPoETree(t,`interface { backend vpp } vpp { enabled true; lcp { enabled false } } pppoe { interface access0 {} }`)
	_,err:=config.ValidateCustomSections(tree)
	if err==nil{t.Fatal("VPP subscriber configuration accepted without LCP")}
	if !strings.Contains(err.Error(),"pppoe/interface/access0"){t.Fatalf("refusal has no access path: %v",err)}
	if !strings.HasSuffix(err.Error(),"set vpp lcp enabled true"){t.Fatalf("refusal has no LCP repair: %v",err)}
}

// The VPP process must be enabled before a registered provider can serve.
func TestVPPPPPoEValidationRequiresVPP(t *testing.T) {
	tree:=vppPPPoETree(t,`interface { backend vpp } vpp { enabled false } pppoe { interface access0 {} }`)
	_,err:=config.ValidateCustomSections(tree)
	if err==nil{t.Fatal("VPP subscriber configuration accepted with VPP disabled")}
	if !strings.HasSuffix(err.Error(),"set vpp enabled true"){t.Fatalf("refusal has no VPP repair: %v",err)}
}

// The actual composition declaration must deliver both read roots to the VPP
// plugin without making them roots it owns or auto-loads from.
func TestVPPPPPoEStartupDeclaresReadRoots(t *testing.T) {
	registered:=registry.Lookup("vpp")
	if registered==nil{t.Fatal("VPP plugin is not registered")}
	for _,root:=range []string{"interface","pppoe"}{
		if !slices.Contains(registered.ConfigReads,root){t.Fatalf("VPP does not read %q",root)}
		if slices.Contains(registered.ConfigRoots,root){t.Fatalf("VPP claimed ownership of %q",root)}
	}
}

// The outer startup parser must retain a prerequisite's concrete repair.
func TestVPPPPPoEStartupKeepsPrerequisiteRepair(t *testing.T) {
	_,err:=vppcomp.ParseConfigSections([]sdk.ConfigSection{
		{Root:"vpp",Data:`{"vpp":{"enabled":"false"}}`},
		{Root:"interface",Data:`{"interface":{"backend":"vpp"}}`},
		{Root:"pppoe",Data:`{"pppoe":{"interface":{"access0":{}}}}`},
	})
	if err==nil{t.Fatal("disabled VPP accepted")}
	if !strings.HasSuffix(err.Error(),"set vpp enabled true"){t.Fatalf("prerequisite repair was replaced: %v",err)}
}

// The configured feature reaches the real renderer with its patched plugin
// and IPv6-control startup flag. A same-spelled value elsewhere cannot select
// the data plane.
func TestVPPPPPoEStartupSelectsOnlyAccessBackend(t *testing.T) {
	for _,backend:=range []string{"vpp","netlink"}{
		t.Run(backend,func(t *testing.T){
			settings,err:=vppcomp.ParseConfigSections([]sdk.ConfigSection{
				{Root:"vpp",Data:`{"vpp":{"enabled":"true"}}`},
				{Root:"interface",Data:`{"interface":{"backend":"`+backend+`"}}`},
				{Root:"pppoe",Data:`{"pppoe":{"ac-name":"vpp","interface":{"access0":{}}}}`},
			})
			if err!=nil{t.Fatal(err)}
			var out bytes.Buffer
			if err:=vppcomp.GenerateStartupConf(&out,settings,vppcomp.CPUInventory{});err!=nil{t.Fatal(err)}
			for _,value:=range []string{"plugin pppoe_plugin.so", "enable-pass-nd-and-dhcpv6"}{
				if strings.Contains(out.String(),value)!=(backend=="vpp"){t.Fatalf("backend %q unexpected %q:\n%s",backend,value,out.String())}
			}
		})
	}
}

func vppPPPoETree(t *testing.T,source string)*config.Tree{
	t.Helper()
	schema,err:=config.YANGSchema()
	if err!=nil{t.Fatal(err)}
	tree,err:=config.NewParser(schema).Parse(source)
	if err!=nil{t.Fatal(err)}
	return tree
}
