package util

import (
	"context"
	"fmt"
	"net"
	"time"
)

func GetLocalIP(ctx context.Context) (string, error) {
	// Safety net so this can’t hang forever.
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	udpAddr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || udpAddr.IP == nil {
		return "", fmt.Errorf("unexpected local addr: %T %v", conn.LocalAddr(), conn.LocalAddr())
	}
	return udpAddr.IP.String(), nil
}
