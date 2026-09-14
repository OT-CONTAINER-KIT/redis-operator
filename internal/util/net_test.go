package util

import (
	"net"
	"testing"
)

// Test_GetLocalIP_ReturnsValidIP verifies that GetLocalIP returns a string
// that parses as a valid IP address. The actual network path taken (IPv4 or
// IPv6) depends on the environment, so we only validate the format.
func Test_GetLocalIP_ReturnsValidIP(t *testing.T) {
	ip, err := GetLocalIP()
	if err != nil {
		t.Skipf("GetLocalIP requires network connectivity in this environment: %v", err)
	}

	if ip == "" {
		t.Fatal("GetLocalIP returned empty string")
	}

	parsed := net.ParseIP(ip)
	if parsed == nil {
		t.Fatalf("GetLocalIP returned invalid IP: %q", ip)
	}
}
