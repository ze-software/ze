//go:build !linux || !cgo || !seccomp

package seccomp

import (
	"errors"

	"github.com/opencontainers/runc/libcontainer/configs"
	"github.com/opencontainers/runtime-spec/specs-go"
)

var ErrSeccompNotEnabled = errors.New("seccomp: config provided but seccomp not supported")

// ze patch (runc-06-seccomp-seam): the filter is Go code in the tree of the
// program that links this library, so that it is linted and tested there.
// That code registers its loader here, and InitSeccomp calls it.
var registeredInit func(config *configs.Seccomp) (int, error)

// RegisterInit registers the function InitSeccomp calls to build and load the
// filter of a profile. Like InitSeccomp it answers the seccomp notify
// descriptor, or -1 when the profile needs none. It MUST be called from a
// package init function, once: a nil function or a second registration is a
// programming error and panics, so nothing replaces a filter loader later.
func RegisterInit(load func(config *configs.Seccomp) (int, error)) {
	if load == nil || registeredInit != nil {
		panic("seccomp: RegisterInit needs one non-nil function, registered once")
	}
	registeredInit = load
}

// InitSeccomp loads the filter of config through the registered function. It
// refuses a profile when no function is registered.
func InitSeccomp(config *configs.Seccomp) (int, error) {
	if config != nil {
		if registeredInit == nil {
			return -1, ErrSeccompNotEnabled
		}
		return registeredInit(config)
	}
	return -1, nil
}

// FlagSupported tells if a provided seccomp flag is supported.
func FlagSupported(_ specs.LinuxSeccompFlag) error {
	return ErrSeccompNotEnabled
}

// Version returns major, minor, and micro.
func Version() (uint, uint, uint) {
	return 0, 0, 0
}

// Enabled is true if seccomp support is compiled in.
const Enabled = false
