//go:build windows

package libnetwork

import (
	"context"
	"net/netip"
)

// Stub implementations for DNS related functions

func (sb *Sandbox) setupResolutionFiles(_ context.Context) error {
	return nil
}

func (sb *Sandbox) restoreHostsPath() {}

func (sb *Sandbox) restoreResolvConfPath() {}

func (sb *Sandbox) addHostsEntries(_ context.Context, ifaceIP []netip.Addr) error {
	return nil
}

func (sb *Sandbox) deleteHostsEntries(ifaceAddrs []netip.Addr) {}

func (sb *Sandbox) updateDNS(ipv6Enabled bool) error {
	return nil
}
