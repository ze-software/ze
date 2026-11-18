// Design: docs/plugin-development/schema.md -- extensions that name a registration
// Related: externalyang.go -- the other modules the external-yang test plugin can bring
// Related: ../fixture/plugin_fixture_external_yang_unheld.go -- the fixture variant that declares this module

package externalyang

// UnheldValidateName is the ze:validate function the unheld module names. No
// Ze package registers a validator of this name, so the daemon does not hold
// it, and a test asserts the refusal names it.
const UnheldValidateName = "acme-no-such-check"

// unheldValidateText is a config module whose one leaf names a ze:validate
// function ze does not hold. The daemon refuses it at the join: a validator
// the daemon lacks would check nothing, and the leaf would then accept every
// value its type admits.
const unheldValidateText = `module acme-unheld-conf {
    namespace "urn:acme:unheld:conf";
    prefix unheld;

    import ze-extensions {
        prefix ze;
    }

    description "Configuration of the acme unheld monitor, an external plugin.";

    revision 2026-11-17 {
        description "Initial revision";
    }

    container acme-unheld {
        presence "The acme unheld monitor is configured";
        description "Settings of the acme unheld monitor.";

        leaf target {
            type string {
                length "1..64";
            }
            ze:validate "` + UnheldValidateName + `";
            description "The host the monitor probes.";
        }
    }
}
`

// UnheldValidate answers the module whose leaf names a ze:validate function
// the daemon does not hold.
func UnheldValidate() Module {
	return Module{Name: "acme-unheld-conf", Namespace: "urn:acme:unheld:conf", Text: unheldValidateText, Refused: true}
}
