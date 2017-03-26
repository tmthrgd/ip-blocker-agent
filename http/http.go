// Copyright 2017 Tom Thorogood. All rights reserved.
// Use of this source code is governed by a
// Modified BSD License license that can be found in
// the LICENSE file.

// Package httpblocker implements IP address blocking
// for net/http.
package httpblocker

import (
	"net"
	"net/http"

	"github.com/tmthrgd/ip-blocker-agent"
)

// Handler is a http.Handler that blocks clients
// with IP addresses that are/are-not in the block
// list.
type Handler struct {
	Client *blocker.Client

	// The http.Handler to invoke when the
	// client is not blocked.
	Handler http.Handler
	// The http.Handler to invoke when the
	// client is blocked.
	Blocked http.Handler

	// If true, only clients in the block
	// list are accepted.
	Whitelist bool
}

// Block wraps a given http.Handler and blocks all
// clients that are contained in the blocklist with
// a 403 Forbidden HTTP status code.
func Block(c *blocker.Client, h http.Handler) http.Handler {
	return BlockWithCode(c, h, http.StatusForbidden)
}

// BlockWithCode wraps a given http.Handler and blocks
// all clients that are contained in the blocklist with
// the given status code.
func BlockWithCode(c *blocker.Client, h http.Handler, code int) http.Handler {
	return &Handler{
		Client: c,

		Handler: h,
		Blocked: errorHandler(code),
	}
}

// Whitelist wraps a given http.Handler and blocks
// all clients that are not contained in the list
// with a 403 Forbidden HTTP status code.
func Whitelist(c *blocker.Client, h http.Handler) http.Handler {
	return WhitelistWithCode(c, h, http.StatusForbidden)
}

// WhitelistWithCode wraps a given http.Handler and
// blocks all clients that are not contained in the
// list with the given status code.
func WhitelistWithCode(c *blocker.Client, h http.Handler, code int) http.Handler {
	return &Handler{
		Client: c,

		Handler: h,
		Blocked: errorHandler(code),

		Whitelist: true,
	}
}

// ServeHTTP implements http.Handler.
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	has, err := h.Client.Contains(net.ParseIP(stripPort(r.RemoteAddr)))
	if err != nil {
		server := r.Context().Value(http.ServerContextKey).(*http.Server)
		if server.ErrorLog != nil {
			server.ErrorLog.Println(err)
		}

		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	} else if has == h.Whitelist {
		h.Handler.ServeHTTP(w, r)
	} else {
		h.Blocked.ServeHTTP(w, r)
	}
}

type errorHandler int

func (code errorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Error(w, http.StatusText(int(code)), int(code))
}
