//go:build !linux

package system

// WriteResolvConf is a no-op on non-Linux platforms.
// On Linux, this writes DNS servers to the resolv.conf at path.
func WriteResolvConf(_ string, _ []string) error {
	return nil
}
