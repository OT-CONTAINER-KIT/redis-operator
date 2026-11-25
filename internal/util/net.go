package util

import (
	"context"
	"errors"
	"net"
	"time"
)

// GetLocalIP returns the IP address of the local network interface that would
// be used to reach an external host. It tries IPv4 first (8.8.8.8) and falls
// back to IPv6 (2001:4860:4860::8888, Google's public DNS64 server) so that
// it works correctly on IPv4-only, IPv6-only and dual-stack clusters.
// This replaces the previous implementation which hard-coded an IPv4 target,
// silently returning an error on pure-IPv6 clusters (#1898).
func GetLocalIP() (string, error) {
	dialer := net.Dialer{}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var lastErr error
	for _, target := range []string{"8.8.8.8:80", "[2001:4860:4860::8888]:80"} {
		conn, err := dialer.DialContext(ctx, "udp", target)
		if err != nil {
			lastErr = err
			continue
		}
		defer conn.Close()
		localAddr := conn.LocalAddr().(*net.UDPAddr)
		return localAddr.IP.String(), nil
	}
	return "", errors.New("failed to determine local IP: " + lastErr.Error())
}
