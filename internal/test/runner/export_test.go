// Related: harness_exec.go -- the child environment key checked by an external test.

package runner

// StoreTrimVariableForTest exposes the actual child environment key to the
// external contract test without adding a production dependency on le.
const StoreTrimVariableForTest = storeTrimVariable
