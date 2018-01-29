package enhttp

import (
	"bytes"
	"io"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/getlantern/errors"
	"github.com/getlantern/uuid"
)

// NewDialer creates a new dialer that dials out using the enhttp protocol,
// tunneling via the server specified by serverURL. An http.Client must be
// specified to configure the underlying HTTP behavior.
func NewDialer(client *http.Client, serverURL string) func(string, string) (net.Conn, error) {
	return func(network, addr string) (net.Conn, error) {
		return &conn{
			id:           uuid.NewRandom().String(),
			origin:       addr,
			client:       client,
			serverURL:    serverURL,
			readDeadline: time.Now().Add(10 * 365 * 24 * time.Hour).UnixNano(),
			received:     make(chan *result, 10),
		}, nil
	}
}

type errTimeout string

func (err errTimeout) Error() string {
	return string(err)
}

func (err errTimeout) Timeout() bool {
	return true
}

func (err errTimeout) Temporary() bool {
	return true
}

type result struct {
	b   []byte
	err error
}

type conn struct {
	id           string
	origin       string
	client       *http.Client
	serverURL    string
	readDeadline int64
	received     chan *result
	unread       []byte
	receiveOnce  sync.Once
	closeOnce    sync.Once
	mx           sync.RWMutex
}

func (c *conn) Write(b []byte) (n int, err error) {
	c.mx.RLock()
	serverURL := c.serverURL
	c.mx.RUnlock()
	req, err := http.NewRequest(http.MethodPost, serverURL, bytes.NewReader(b))
	if err != nil {
		return 0, log.Errorf("Error constructing request: %v", err)
	}
	req.Header.Set(ConnectionIDHeader, c.id)
	req.Header.Set(OriginHeader, c.origin)
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, errors.New("Error posting data: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		return 0, errors.New("Unexpected response status posting data: %d", resp.StatusCode)
	}
	updatedServerURL := resp.Header.Get(ServerURL)
	if updatedServerURL != "" {
		c.mx.Lock()
		c.serverURL = updatedServerURL
		c.mx.Unlock()
	}
	c.receiveOnce.Do(func() {
		go c.receive(resp)
	})
	return len(b), nil
}

func (c *conn) receive(resp *http.Response) {
	defer resp.Body.Close()
	for {
		b := make([]byte, 8192)
		n, err := resp.Body.Read(b)
		c.received <- &result{b[:n], err}
		if err != nil {
			return
		}
	}
}

func (c *conn) Read(b []byte) (int, error) {
	if len(c.unread) > 0 {
		return c.readFromUnread(b)
	}

	deadline := timeFromInt(atomic.LoadInt64(&c.readDeadline))
	timeout := deadline.Sub(time.Now())
	select {
	case result, open := <-c.received:
		if !open {
			return 0, io.EOF
		}
		if result.err != nil {
			return 0, result.err
		}
		c.unread = result.b
		return c.Read(b)
	case <-time.After(timeout):
		return 0, errTimeout("read timeout")
	}
}

func (c *conn) readFromUnread(b []byte) (int, error) {
	copied := copy(b, c.unread)
	c.unread = c.unread[copied:]
	if len(b) <= copied {
		return copied, nil
	}

	// We've consumed unread but have room for more
	select {
	case result, open := <-c.received:
		if !open {
			return copied, io.EOF
		}
		if result.err != nil {
			return copied, result.err
		}
		c.unread = result.b
		n, err := c.Read(b[copied:])
		return copied + n, err
	default:
		// don't block, just return what we have
		return copied, nil
	}
}

func (c *conn) SetDeadline(t time.Time) error {
	return c.SetReadDeadline(t)
}

func (c *conn) SetReadDeadline(t time.Time) error {
	atomic.StoreInt64(&c.readDeadline, t.UnixNano())
	return nil
}

func (c *conn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (c *conn) LocalAddr() net.Addr {
	return nil
}

func (c *conn) RemoteAddr() net.Addr {
	return nil
}

func (c *conn) Close() error {
	c.closeOnce.Do(func() {
		// TODO: actually close this thing somehow
	})
	return nil
}

func timeFromInt(ts int64) time.Time {
	s := ts / int64(time.Second)
	ns := ts % int64(time.Second)
	return time.Unix(s, ns)
}
