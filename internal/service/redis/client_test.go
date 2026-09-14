package redis

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateClientSetsTimeouts(t *testing.T) {
	s := &service{connectionInfo: &ConnectionInfo{Host: "127.0.0.1", Port: "6379"}}

	cl := s.createClient()
	require.NotNil(t, cl)
	defer cl.Close()

	opts := cl.Options()
	assert.Equal(t, defaultRedisClientTimeout, opts.DialTimeout)
	assert.Equal(t, defaultRedisClientTimeout, opts.ReadTimeout)
	assert.Equal(t, defaultRedisClientTimeout, opts.WriteTimeout)
}

// TestCreateClientUnresponsiveServerReturns verifies that a Redis server that
// accepts the connection but never answers does not stall the caller: the
// read timeout must turn the PING into an error instead of blocking forever.
func TestCreateClientUnresponsiveServerReturns(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	go func() {
		for {
			_, err := ln.Accept()
			if err != nil {
				return
			}
			// Hold the connection open without ever responding.
		}
	}()

	host, portStr, err := net.SplitHostPort(ln.Addr().String())
	require.NoError(t, err)

	s := &service{connectionInfo: &ConnectionInfo{Host: host, Port: portStr}}
	cl := s.createClient()
	require.NotNil(t, cl)
	defer cl.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	start := time.Now()
	err = cl.Ping(ctx).Err()
	elapsed := time.Since(start)

	require.Error(t, err, "PING against an unresponsive server must fail, not hang")
	assert.Less(t, elapsed, 10*time.Second, "read timeout should bound the wait (~5s), far below the 30s ctx budget")
}
