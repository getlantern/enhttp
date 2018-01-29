// Package enhttp provides an implementation of net.Conn that encapsulates
// traffic in one or more HTTP requests. It is conceptually similar to the older
// https://github.com/getlantern/enproxy but differs in that it supports HTTP
// servers which don't support Transfer-Encoding: Chunked on uploads.
//
// enhttp was created to facilitate domain-fronting within Lantern.
//
// enhttp creates virtual connections that are identified by a unique GUID.
// Every write to the connection is implemented as an HTTP POST. The first POST
// establishes the virtual connection and subsequent HTTP POSTs are tied to that
// connection by the X-En-Conn-Id header.
//
// To support load-balancing, servers can optionally return an X-Server-URL
// header that uniquely identifies the server that handled the initial POST. If
// present, enhttp clients send all subsequent POSTs to that URL.
package enhttp

import (
	"github.com/getlantern/golog"
)

const (
	// ConnectionIDHeader identifies a virtual connection.
	ConnectionIDHeader = "X-En-Conn-Id"

	// OriginHeader identifies the origin server to which we're trying to tunnel.
	OriginHeader = "X-Origin"

	// ServerURL optionally specifies the URL to the server that handled the first
	// POST.
	ServerURL = "X-Server-URL"
)

var (
	log = golog.LoggerFor("enhttp")
)
