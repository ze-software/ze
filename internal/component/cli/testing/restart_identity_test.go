// Design: docs/architecture/testing/ci-format.md -- editor session restarts.

package testing

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestETRestartPreservesSelectedSessionIdentity(t *testing.T) {
	for _, initial := range []string{"", "option=session:user=initial:origin=local\n", "option=mode:value=command\n"} {
		t.Run(initial, func(t *testing.T) {
			result := runWaitFile(t, initial+
				"session=first:user=operator-one,origin=ssh\n"+
				"input=type:text=set bgp router-id 5.6.7.8\ninput=enter\nexpect=error:none\n"+
				"restart=editor\ninput=type:text=who\ninput=enter\nexpect=error:none\n"+
				"session=second:user=operator-two,origin=console\n"+
				"input=type:text=set bgp session asn local 65002\ninput=enter\nexpect=error:none\n"+
				"session=first\ninput=type:text=set bgp router-id 9.8.7.6\ninput=enter\nexpect=error:none\n"+
				"expect=key:path=file/active/test.conf.change.operator-one:contains=#operator-one @ssh\n"+
				"expect=key:path=file/active/test.conf.change.operator-one:contains=9.8.7.6\n"+
				"session=second\nrestart=editor\nsession=first\nsession=second\n"+
				"input=type:text=who\ninput=enter\nexpect=error:none\n"+
				"input=type:text=set bgp session asn local 65003\ninput=enter\nexpect=error:none\n"+
				"expect=key:path=file/active/test.conf.change.operator-two:contains=#operator-two @console\n"+
				"expect=key:path=file/active/test.conf.change.operator-two:contains=65003\n")
			require.True(t, result.Passed, "%s", result.Error)
		})
	}
}
