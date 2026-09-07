// VALIDATES: spec-connected-static-reach-the-locrib -- the FIB writer a route
// producer needs is DECLARED by the writer and resolved through the registry,
// so no package holds a list of FIB plugins.
// PREVENTS: an unknown data plane answering with an empty plugin name a caller
// would start, and two plugins claiming one data plane leaving the winner to
// map order.

package registry

import (
	"errors"
	"strings"
	"testing"
)

// writerReg builds a registration that declares itself the writer for one data
// plane.
func writerReg(name, dataPlane string) Registration {
	reg := validReg(name)
	reg.ProgramsFIB = true
	reg.DataPlane = dataPlane
	return reg
}

// TestPluginForDataPlaneResolvesTheDeclaredWriter proves the resolution a route
// producer depends on: the data plane name goes in, the plugin that declared it
// comes out, and nothing else in the registry answers.
func TestPluginForDataPlaneResolvesTheDeclaredWriter(t *testing.T) {
	t.Cleanup(func() { Reset() })
	Reset()

	for _, r := range []Registration{
		writerReg("writer-netlink", "netlink"),
		writerReg("writer-vpp", "vpp"),
		validReg("producer"),
	} {
		if err := Register(r); err != nil {
			t.Fatal(err)
		}
	}

	name, ok := PluginForDataPlane("netlink")
	if !ok || name != "writer-netlink" {
		t.Fatalf("PluginForDataPlane(netlink) = %q, %v; want writer-netlink, true", name, ok)
	}
	name, ok = PluginForDataPlane("vpp")
	if !ok || name != "writer-vpp" {
		t.Fatalf("PluginForDataPlane(vpp) = %q, %v; want writer-vpp, true", name, ok)
	}
}

// TestPluginForDataPlaneReportsAnUnresolvedDataPlane proves the lookup says it
// cannot answer rather than returning an empty name. A caller that read "" as a
// plugin name would try to start a plugin that does not exist.
func TestPluginForDataPlaneReportsAnUnresolvedDataPlane(t *testing.T) {
	t.Cleanup(func() { Reset() })
	Reset()

	if err := Register(writerReg("writer-netlink", "netlink")); err != nil {
		t.Fatal(err)
	}

	for _, dataPlane := range []string{"", "p4-runtime", "dpdk"} {
		if name, ok := PluginForDataPlane(dataPlane); ok || name != "" {
			t.Fatalf("PluginForDataPlane(%q) = %q, %v; want \"\", false", dataPlane, name, ok)
		}
	}
}

// TestRegisterRefusesASecondWriterForOneDataPlane proves the "which writer
// serves this deployment" question cannot become a run-time choice: two
// declarations of one data plane are refused at registration.
func TestRegisterRefusesASecondWriterForOneDataPlane(t *testing.T) {
	t.Cleanup(func() { Reset() })
	Reset()

	if err := Register(writerReg("writer-netlink", "netlink")); err != nil {
		t.Fatal(err)
	}

	err := Register(writerReg("writer-netlink-again", "netlink"))
	if err == nil {
		t.Fatal("second writer for the netlink data plane was accepted")
	}
	if !errors.Is(err, ErrDuplicateDataPlane) {
		t.Fatalf("error = %v; want ErrDuplicateDataPlane", err)
	}
}

// TestRegisterRefusesADataPlaneWithoutTheFIBRole proves a declaration that
// could mean nothing is refused. A plugin naming a data plane it does not
// program would be resolved as a producer's writer and program nothing.
func TestRegisterRefusesADataPlaneWithoutTheFIBRole(t *testing.T) {
	t.Cleanup(func() { Reset() })
	Reset()

	reg := validReg("half-declared")
	reg.DataPlane = "netlink"

	err := Register(reg)
	if err == nil {
		t.Fatal("a data plane declared without ProgramsFIB was accepted")
	}
	if !strings.Contains(err.Error(), "does not program the FIB") {
		t.Fatalf("error = %v; want it to name the missing FIB role", err)
	}
}

// TestProgramsFIBIsAskedSeparatelyFromTheDataPlane proves the two declarations
// answer different questions. A FIB plugin that reaches no data plane is still
// the operator's choice, and is still never resolved as a producer's writer.
func TestProgramsFIBIsAskedSeparatelyFromTheDataPlane(t *testing.T) {
	t.Cleanup(func() { Reset() })
	Reset()

	noop := validReg("writer-without-a-data-plane")
	noop.ProgramsFIB = true
	if err := Register(noop); err != nil {
		t.Fatal(err)
	}

	if !ProgramsFIB("writer-without-a-data-plane") {
		t.Fatal("a FIB plugin with no data plane must still report that it programs the FIB")
	}
	if _, ok := PluginForDataPlane(""); ok {
		t.Fatal("a FIB plugin with no data plane must never be resolved as a writer")
	}
}

// TestNeedsDataPlaneReadsTheDeclaration proves the producer side of the pair:
// the need is a declaration on the producer, and a plugin nobody registered
// needs nothing.
func TestNeedsDataPlaneReadsTheDeclaration(t *testing.T) {
	t.Cleanup(func() { Reset() })
	Reset()

	producer := validReg("producer")
	producer.NeedsDataPlane = true
	if err := Register(producer); err != nil {
		t.Fatal(err)
	}
	if err := Register(validReg("bystander")); err != nil {
		t.Fatal(err)
	}

	if !NeedsDataPlane("producer") {
		t.Fatal("producer declared NeedsDataPlane and the registry says otherwise")
	}
	if NeedsDataPlane("bystander") {
		t.Fatal("bystander declared no need and the registry says it has one")
	}
	if NeedsDataPlane("never-registered") {
		t.Fatal("an unregistered plugin was reported as needing a data plane")
	}
}
