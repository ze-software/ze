// Design: docs/research/l2tpv2-ze-integration.md -- native channel/unit namespace pairing

package kernel

import (
	"errors"
	"os"
	"sync"
)

// Only actual producer-held namespace files implement this outside tests.
type sessionNamespaceProof interface {
	sameSessionNamespace(sessionNamespaceProof) error
}

type capturedSessionNamespace struct {
	file *os.File
	once sync.Once
	err  error
}

func (namespace *capturedSessionNamespace) Close() error {
	if namespace == nil {
		return nil
	}
	namespace.once.Do(func() {
		if namespace.file != nil {
			namespace.err = namespace.file.Close()
		}
	})
	return namespace.err
}

func (namespace *capturedSessionNamespace) sameSessionNamespace(other sessionNamespaceProof) error {
	original, ok := other.(*capturedSessionNamespace)
	if !ok {
		return errors.New("ppp: channel/unit has no native namespace receipt")
	}
	if namespace == nil {
		return errors.New("ppp: channel/unit has no native namespace receipt")
	}
	if original == nil {
		return errors.New("ppp: channel/unit has no native namespace receipt")
	}
	if namespace.file == nil {
		return errors.New("ppp: channel/unit has no native namespace receipt")
	}
	if original.file == nil {
		return errors.New("ppp: channel/unit has no native namespace receipt")
	}
	left, err := namespace.file.Stat()
	if err != nil {
		return err
	}
	right, err := original.file.Stat()
	if err != nil {
		return err
	}
	if !os.SameFile(left, right) {
		return errors.New("ppp: original channel and unit belong to different namespaces")
	}
	return nil
}

func validateSessionNamespace(channel, unit sessionNamespaceProof) error {
	if nilSessionDependency(channel) {
		return errors.New("ppp: missing original channel/unit namespace receipt")
	}
	if nilSessionDependency(unit) {
		return errors.New("ppp: missing original channel/unit namespace receipt")
	}
	return channel.sameSessionNamespace(unit)
}
