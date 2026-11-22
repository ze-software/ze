// Design: docs/functional-tests.md -- executable tripwire for parse-error refusal tests

package fixture

func init() {
	Register("parse/write-tripwire", parseWriteTripwire)
}
