//go:build !freebsd && !linux && !windows
// +build !freebsd,!linux,!windows

package libnetwork

func getInitializers() []initializer {
	return nil
}
