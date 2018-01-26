package enhttp

import (
	"github.com/getlantern/golog"
)

const (
	ConnectionIDHeader = "X-En-Conn-Id"
	OriginHeader       = "X-Origin"
	ServerURL          = "X-Server-URL"
)

var (
	log = golog.LoggerFor("enhttp")
)
