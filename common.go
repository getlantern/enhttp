package enhttp

import (
	"github.com/getlantern/golog"
)

const (
	ConnectionIDHeader = "X-En-Conn-Id"
	OriginHeader       = "X-Origin"
)

var (
	log = golog.LoggerFor("enhttp")
)
