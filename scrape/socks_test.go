package scrape_test

import (
	"bytes"
	"io"
	"net"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// authenticatedSOCKSProxy handles SOCKS5 username/password CONNECT requests
// for fixture.invalid and forwards them only to the local fixture.
func authenticatedSOCKSProxy(t *testing.T, fixture string) string {
	t.Helper()

	var config net.ListenConfig

	listener, err := config.Listen(t.Context(), "tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, listener.Close()) })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}

			go serveSOCKSFixture(conn, fixture)
		}
	}()

	endpoint := url.URL{Scheme: "socks5h", Host: listener.Addr().String(), User: url.UserPassword("user", "password")}

	return endpoint.String()
}

func serveSOCKSFixture(conn net.Conn, fixture string) {
	defer conn.Close()

	if err := conn.SetDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return
	}

	// The Go SOCKS dialer advertises no authentication and username/password.
	if !exchangeSOCKS(conn, []byte{5, 2, 0, 2}, []byte{5, 2}) {
		return
	}

	if !exchangeSOCKS(conn, []byte("\x01\x04user\x08password"), []byte{1, 0}) {
		return
	}

	request := append([]byte{5, 1, 0, 3, 15}, []byte("fixture.invalid")...)
	request = append(request, 0, 80)

	if !exchangeSOCKS(conn, request, []byte{5, 0, 0, 1, 127, 0, 0, 1, 0, 80}) {
		return
	}

	upstream, err := net.DialTimeout("tcp", fixture, time.Second)
	if err != nil {
		return
	}
	defer upstream.Close()

	go func() {
		_, _ = io.Copy(upstream, conn)
		_ = upstream.Close()
	}()

	_, _ = io.Copy(conn, upstream)
}

func exchangeSOCKS(conn net.Conn, expected, reply []byte) bool {
	input := make([]byte, len(expected))
	if _, err := io.ReadFull(conn, input); err != nil || !bytes.Equal(input, expected) {
		return false
	}

	_, err := conn.Write(reply)

	return err == nil
}
