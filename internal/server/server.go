// Package server serves the CoinChord web interface and its WebSocket.
package server

import (
	"embed"
	"io/fs"
	"net"
	"net/http"
	"net/url"

	"golang.org/x/net/websocket"

	"coinchord/internal/app"
)

//go:embed dist
var dist embed.FS

// Handler serves the interface at / and the live connection at /ws.
func Handler(a *app.App) (http.Handler, error) {
	ui, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, err
	}
	mux := http.NewServeMux()
	mux.Handle("/", http.FileServerFS(ui))
	mux.Handle("/ws", websocket.Server{Handshake: sameOrigin, Handler: func(ws *websocket.Conn) { serve(a, ws) }})
	return localOnly(mux), nil
}

// localOnly refuses requests whose Host is not a loopback name. It stops
// DNS rebinding: a web page must never reach a server that types for you.
func localOnly(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !loopback(r.Host) {
			http.Error(w, "CoinChord only answers on localhost", http.StatusForbidden)
			return
		}
		h.ServeHTTP(w, r)
	})
}

func loopback(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// sameOrigin accepts WebSocket connections only from our own page.
func sameOrigin(cfg *websocket.Config, r *http.Request) error {
	o, err := url.Parse(r.Header.Get("Origin"))
	if err != nil || o.Host != r.Host || !loopback(o.Host) {
		return websocket.ErrBadWebSocketOrigin
	}
	cfg.Origin = o
	return nil
}

func serve(a *app.App, ws *websocket.Conn) {
	msgs, leave := a.Join()
	defer leave()
	go func() {
		for b := range msgs {
			if err := websocket.Message.Send(ws, string(b)); err != nil {
				ws.Close()
				return
			}
		}
		ws.Close() // the engine dropped us; the page reconnects
	}()
	for {
		var c app.Cmd
		if err := websocket.JSON.Receive(ws, &c); err != nil {
			return
		}
		a.Do(c)
	}
}
