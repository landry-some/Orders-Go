package realtime

import (
	"net/http"
	"net/http/httptest"
	"net"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestHub_Broadcast(t *testing.T) {
	t.Parallel()

	hub := NewHub()
	go hub.Run()

	upgrader := websocket.Upgrader{}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("listener not permitted in this environment: %v", err)
	}

	srv := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Fatalf("upgrade: %v", err)
		}
		hub.Register <- conn
	}))
	srv.Listener = ln
	srv.Start()
	t.Cleanup(srv.Close)

	wsURL := "ws" + srv.URL[len("http"):]
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() {
		conn.Close()
	})

	msg := []byte("hello world")
	select {
	case hub.Broadcast <- msg:
	case <-time.After(time.Second):
		t.Fatalf("timed out sending to hub")
	}

	readCh := make(chan []byte, 1)
	go func() {
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read message: %v", err)
			return
		}
		readCh <- data
	}()

	select {
	case got := <-readCh:
		if string(got) != string(msg) {
			t.Fatalf("expected %q, got %q", msg, got)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for broadcast")
	}
}
