package enhttp

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	text = "hello encapsulated world"
)

func TestRoundTrip(t *testing.T) {
	// echo server
	el, err := net.Listen("tcp", "127.0.0.1:0")
	if !assert.NoError(t, err) {
		return
	}
	defer el.Close()

	go func() {
		for {
			conn, err := el.Accept()
			if err != nil {
				t.Fatalf("Unable to accept: %v", err)
			}
			go func() {
				b := make([]byte, 1)
				for {
					_, err := conn.Read(b)
					if err != nil {
						return
					}
					conn.Write(b)
				}
				conn.Close()
			}()
		}
	}()

	// enhttp server (two separate listeners, single server)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if !assert.NoError(t, err) {
		return
	}
	defer l.Close()

	l2, err := net.Listen("tcp", "127.0.0.1:0")
	if !assert.NoError(t, err) {
		return
	}
	defer l2.Close()

	// second enhttp server
	hs := &http.Server{
		Handler: NewServerHandler(fmt.Sprintf("http://%v/", l2.Addr())),
	}
	go hs.Serve(l)
	go hs.Serve(l2)

	// enhttp dialer
	dialer := NewDialer(&http.Client{}, fmt.Sprintf("http://%v/", l.Addr()))

	conn, err := dialer("tcp", el.Addr().String())
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	for i := 0; i < 100; i++ {
		n, err := conn.Write([]byte(text))
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, len(text), n)

		b := make([]byte, len(text))
		n, err = io.ReadFull(conn, b)
		if !assert.NoError(t, err) {
			return
		}
		assert.Equal(t, text, string(b[:n]))
	}
	conn.Close()
}
