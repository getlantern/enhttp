package enhttp

import (
	"io"
	"net"
	"net/http"
	"sync"
)

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
		origin := req.Header.Get(OriginHeader)
		var err error
		conn, err = net.Dial("tcp", origin)
		if err != nil {
			log.Errorf("Unable to dial %v: %v", origin, err)
			resp.WriteHeader(http.StatusBadGateway)
			return
		}
		s.mx.Lock()
		s.conns[connID] = conn
		s.mx.Unlock()
		_, err = io.Copy(conn, req.Body)
		if err != nil {
			log.Errorf("Error reading request body: %v", err)
			resp.WriteHeader(http.StatusBadRequest)
			return
		}
		req.Body.Close()
		resp.Header().Set("Connection", "Keep-Alive")
		resp.Header().Set("Transfer-Encoding", "chunked")
		if s.serverURL != "" {
			resp.Header().Set(ServerURL, s.serverURL)
		}
		resp.WriteHeader(http.StatusOK)
		resp.(http.Flusher).Flush()
		buf := make([]byte, 8192)
	readLoop:
		for {
			n, err := conn.Read(buf)
			if err != nil {
				break readLoop
			}
			resp.Write(buf[:n])
			resp.(http.Flusher).Flush()
		}
		return
	}

	_, err := io.Copy(conn, req.Body)
	if err != nil {
		log.Errorf("Error reading request body: %v", err)
		resp.WriteHeader(http.StatusBadRequest)
		return
	}
	resp.WriteHeader(http.StatusOK)
}
