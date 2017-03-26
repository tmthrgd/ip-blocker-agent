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

type handler struct {
	c *blocker.Client

	http.Handler
	block http.Handler

	whitelist bool
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
	return BlockWithHandler(c, h, errorHandler(code))
}

// BlockWithHandler wraps a given http.Handler and routes
// all clients that are contained in the blocklist to the
// block http.Handler and the rest to the h http.Handler.
func BlockWithHandler(c *blocker.Client, h http.Handler, block http.Handler) http.Handler {
	return &handler{
		c: c,

		Handler: h,
		block:   block,
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
	return WhitelistWithHandler(c, h, errorHandler(code))
}

// WhitelistWithHandler wraps a given http.Handler and
// routes all clients that are contained in the list to
// the h http.Handler and the rest to the block
// http.Handler.
func WhitelistWithHandler(c *blocker.Client, h http.Handler, block http.Handler) http.Handler {
	return &handler{
		c: c,

		Handler: h,
		block:   block,

		whitelist: true,
	}
}

func (h *handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	has, err := h.c.Contains(net.ParseIP(stripPort(r.RemoteAddr)))
	if err != nil {
		server := r.Context().Value(http.ServerContextKey).(*http.Server)
		if server.ErrorLog != nil {
			server.ErrorLog.Println(err)
		}

		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
	} else if has == h.whitelist {
		h.Handler.ServeHTTP(w, r)
	} else {
		h.block.ServeHTTP(w, r)
	}
}

type errorHandler int

func (code errorHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	http.Error(w, http.StatusText(int(code)), int(code))
}
