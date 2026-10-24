package cli

// This test binary links the interface plugin (internal/component/iface) through
// the editor's own imports, and interface hard-depends on sysctl
// (Registration.Dependencies). A commit resolves the dependencies of every
// plugin the config's roots select (config.VerifyPluginConfig), as boot does, so
// without sysctl linked here every commit of an `interface { }` config is
// refused as a build missing sysctl, which no shipped build is. Linking it
// makes the test binary a build the product could be.
import _ "github.com/ze-software/ze/internal/component/sysctl"
