package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/net/websocket"

	"coinchord/internal/app"
	"coinchord/internal/config"
	"coinchord/internal/kits"
	"coinchord/internal/tutor"
)

func start(t *testing.T) *httptest.Server {
	t.Helper()
	p, err := config.ForPreset(kits.Presets[0])
	if err != nil {
		t.Fatal(err)
	}
	a := app.New(p, app.Options{ProfilePath: filepath.Join(t.TempDir(), "p.json"), Stories: tutor.Builtin})
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	go a.Run(ctx)
	h, err := Handler(a)
	if err != nil {
		t.Fatal(err)
	}
	s := httptest.NewServer(h)
	t.Cleanup(s.Close)
	return s
}

func TestWebSocket(t *testing.T) {
	s := start(t)
	addr := strings.TrimPrefix(s.URL, "http://")
	ws, err := websocket.Dial("ws://"+addr+"/ws", "", s.URL)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	ws.SetDeadline(time.Now().Add(2 * time.Second))
	var first string
	if err := websocket.Message.Receive(ws, &first); err != nil || !strings.HasPrefix(first, `{`) || !strings.Contains(first, `"t":"state"`) {
		t.Fatalf("first message %.80q %v", first, err)
	}
	if err := websocket.JSON.Send(ws, app.Cmd{Do: "page", Name: "layers"}); err != nil {
		t.Fatal(err)
	}
	for {
		var m string
		if err := websocket.Message.Receive(ws, &m); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(m, `"t":"state"`) {
			return // the command was handled and a new state came back
		}
	}
}

func TestRejectsOtherSites(t *testing.T) {
	s := start(t)
	addr := strings.TrimPrefix(s.URL, "http://")
	if _, err := websocket.Dial("ws://"+addr+"/ws", "", "https://evil.example"); err == nil {
		t.Fatal("a foreign origin must not connect")
	}
	req, _ := http.NewRequest("GET", s.URL+"/", nil)
	req.Host = "evil.example"
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusForbidden {
		t.Fatalf("foreign host got %d", res.StatusCode)
	}
	res, err = http.Get(s.URL + "/")
	if err != nil || res.StatusCode != http.StatusOK {
		t.Fatalf("own page: %v %v", err, res.StatusCode)
	}
	res.Body.Close()
}
