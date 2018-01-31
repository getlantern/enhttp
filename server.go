package enhttp

import (
	"io"
	"net"
	"net/http"
	"sync"
)

// NewServerHandler creates an http.Handler that performs the server-side
// processing of enhttp. serverURL optionally specifies the unique URL at which
// this server can be reached (used for sticky routing).
func NewServerHandler(serverURL string) http.Handler {
	return &server{
		serverURL: serverURL,
		conns:     make(map[string]net.Conn, 1000),
	}
}

type server struct {
	serverURL string
	conns     map[string]net.Conn
	mx        sync.RWMutex
}

func (s *server) ServeHTTP(resp http.ResponseWriter, req *http.Request) {
	connID := req.Header.Get(ConnectionIDHeader)
	s.mx.RLock()
	conn := s.conns[connID]
	s.mx.RUnlock()
	first := conn == nil

	if first {
		// Connect to the origin
		origin := req.Header.Get(OriginHeader)
		var err error
		conn, err = net.Dial("tcp", origin)
		if err != nil {
			log.Errorf("Unable to dial %v: %v", origin, err)
			resp.WriteHeader(http.StatusBadGateway)
			return
		}

		// Remember the origin connection
		s.mx.Lock()
		s.conns[connID] = conn
		s.mx.Unlock()

		// Write the request body to origin
		_, err = io.Copy(conn, req.Body)
		if err != nil {
			log.Errorf("Error reading request body: %v", err)
			resp.WriteHeader(http.StatusBadRequest)
			return
		}
		req.Body.Close()

		// Set up the response for streaming
		resp.Header().Set("Connection", "Keep-Alive")
		resp.Header().Set("Transfer-Encoding", "chunked")
		if s.serverURL != "" {
			resp.Header().Set(ServerURL, s.serverURL)
		}
		resp.WriteHeader(http.StatusOK)

		// Force writing the HTTP response header to client
		resp.(http.Flusher).Flush()

		// Read from the origin and write data to client
		buf := make([]byte, 8192)
		for {
			n, err := conn.Read(buf)
			if n > 0 {
				resp.Write(buf[:n])
				resp.(http.Flusher).Flush()
			}
			if err != nil {
				break
			}
		}
		return
	}

	if req.Header.Get(Close) != "" {
		// Close the connection
		conn.Close()
		s.mx.Lock()
		delete(s.conns, connID)
		s.mx.Unlock()
		resp.WriteHeader(http.StatusOK)
		return
	}

	// Not first request, simply write request data to origin
	_, err := io.Copy(conn, req.Body)
	if err != nil {
		log.Errorf("Error reading request body: %v", err)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
	resp.WriteHeader(http.StatusOK)
}
