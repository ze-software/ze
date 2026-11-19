// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- final original snapshot
// Related: client_native_owner.go -- exact owner and post-join unit close

package pppoeclient

import "github.com/ze-software/ze/internal/component/l2tp/ppp"

// clientNativeFinalCounters preserves one explicit final outcome. A nil value
// means cleanup has not attempted a final sample. A failed reading contains its
// cause and cannot be mistaken for a valid all-zero traffic measurement.
type clientNativeFinalCounters interface{ clientNativeFinalCounters() }
type clientNativeCountersSampled struct{ counters ppp.SessionCounters }
type clientNativeCountersRefused struct{ cause error }

func (clientNativeCountersSampled) clientNativeFinalCounters() {}
func (clientNativeCountersRefused) clientNativeFinalCounters() {}

// captureFinalCounters MUST run after all protocol consumers join and before
// CloseUnit. It reads the same CreatedUnit that creation and the writer retain;
// neither a configured name nor UnitNumber can choose its counter producer.
func (owner *clientNativeOwner) captureFinalCounters() (result error) {
	defer func() {
		if failure := recover(); failure != nil {
			result = clientPanicFailure(failure)
			owner.finalCounters = clientNativeCountersRefused{cause: result}
		}
	}()
	counters, err := owner.unit.Counters()
	if err != nil {
		owner.finalCounters = clientNativeCountersRefused{cause: err}
		return err
	}
	owner.finalCounters = clientNativeCountersSampled{counters: counters}
	return nil
}
