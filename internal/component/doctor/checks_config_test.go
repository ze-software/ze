package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/ze-software/ze/internal/component/config"
)

// gnmiTreeOn builds an enabled environment.gnmi block bound to one endpoint.
func gnmiTreeOn(host, token string) *config.Tree {
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	gnmi := env.GetOrCreateContainer("gnmi")
	gnmi.Set("enabled", "true")
	if token != "" {
		gnmi.Set("token", token)
	}
	srv := config.NewTree()
	srv.Set("ip", host)
	srv.Set("port", "9339")
	gnmi.AddListEntry("server", "default", srv)
	return tree
}

// exposedGnmiTree is the audit finding in config form: gNMI bound to 0.0.0.0
// with no token, so it serves unauthenticated Get AND Set on every interface.
func exposedGnmiTree() *config.Tree { return gnmiTreeOn("0.0.0.0", "") }

// The gNMI exposure assertions that sat here (TestDoctorFlagsGnmiExposure,
// TestDoctorGnmiLoopbackAndTokenAreClean) moved with the semantic check to
// internal/component/config/doctor_test.go, TestCheckSemanticsFlagsGnmiExposure
// and TestCheckSemanticsGnmiLoopbackAndTokenAreClean.

func TestDoctorGnmiListenerIsProbed(t *testing.T) {
	// AC-6 second half, fallback path: the gNMI endpoint reaches the bind-probe
	// set even when schema discovery is unavailable.
	listeners := collectHardcodedListeners(exposedGnmiTree())
	found := false
	for i := range listeners {
		if listeners[i].service == "gnmi" {
			found = true
			assert.Equal(t, "0.0.0.0", listeners[i].host)
			assert.Equal(t, "9339", listeners[i].port)
		}
	}
	assert.True(t, found, "gNMI must appear in the doctor listener set")
}

func TestDoctorGnmiDefaultEndpointIsProbed(t *testing.T) {
	// VALIDATES: AC-6 -- the "gnmi" entry in
	// config.RegisterBuiltinListenerDefaults reaches the PRIMARY (schema
	// discovery) path. A gNMI block that names no server produces no endpoint of
	// its own, because CollectListeners skips a service with an empty server
	// list unless a default is registered for its name. The name must match the
	// one DiscoverListenerServices derives from the ze:listener mark in
	// ze-gnmi-conf.yang, so this test also pins that the registered string
	// "gnmi" is not dead.
	// PREVENTS: doctor silently probing nothing for the exposure this spec is
	// about -- gNMI enabled with no explicit address, which is exactly the
	// config that binds 0.0.0.0:9339.
	tree := config.NewTree()
	env := tree.GetOrCreateContainer("environment")
	gnmi := env.GetOrCreateContainer("gnmi")
	gnmi.Set("enabled", "true")

	listeners := collectSchemaListeners(tree)
	found := false
	for i := range listeners {
		if listeners[i].service == "gnmi" {
			found = true
			assert.Equal(t, "0.0.0.0", listeners[i].host)
			assert.Equal(t, "9339", listeners[i].port)
		}
	}
	assert.True(t, found,
		"a gNMI block with no server entry must still yield the default probe endpoint")
}
