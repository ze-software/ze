//go:build !linux

package mlockexe

import "errors"

func lock(onFault bool) (int64, error) {
	return 0, errors.ErrUnsupported
}
