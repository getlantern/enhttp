package enhttp

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"runtime"
	"runtime/pprof"
	"testing"
	"time"

	"github.com/getlantern/fdcount"
	"github.com/stretchr/testify/assert"
)

const (
	text = "hello encapsulated world"
)

func TestRoundTrip(t *testing.T) {
	_, counter, err := fdcount.Matching("TCP")
	goroutines := runtime.NumGoroutine()
	defer func() {
		time.Sleep(1 * time.Second)
		if assert.NoError(t, counter.AssertDelta(0), "All TCP sockets should have been closed") {
			if !assert.Equal(t, goroutines+1, runtime.NumGoroutine(), "No goroutines except the one spawned by NewServerHandler should be leaked") {
				pprof.Lookup("goroutine").WriteTo(os.Stdout, 1)
			}
		}
	}()

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
				return
			}
			go func() {
				defer conn.Close()

				b := make([]byte, 1)
				for {
					_, err := conn.Read(b)
					if err != nil {
						return
					}
					conn.Write(b)
				}
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

	hs := &http.Server{
		Handler: NewServerHandler(2*time.Second, fmt.Sprintf("http://%v/", l2.Addr())),
	}

	go hs.Serve(l)
	go hs.Serve(l2)
	// enhttp dialer
	dialer := NewDialer(&http.Client{
		Transport: &http.Transport{
			DisableKeepAlives: true,
		},
	}, fmt.Sprintf("http://%v/", l.Addr()))

	conn, err := dialer("tcp", el.Addr().String())
	if !assert.NoError(t, err) {
		return
	}
	defer conn.Close()

	assert.True(t, IsENHTTP(conn))

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

	err = conn.Close()
	assert.NoError(t, err)

	// Create a new connection that we don't close in order to test that reaping
	// works.
	conn, err = dialer("tcp", el.Addr().String())
	if !assert.NoError(t, err) {
		return
	}
	conn.Write([]byte(text))
	go io.Copy(ioutil.Discard, conn)

	log.Debugf("Echo server is: %v", el.Addr())
	log.Debugf("enhttp 1 server is: %v", l.Addr())
	log.Debugf("enhttp 2 server is: %v", l2.Addr())
}
