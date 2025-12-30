package realtime

import (
	"sync"

	"github.com/gorilla/websocket"
)

// Hub manages WebSocket clients and broadcasts messages to them.
type Hub struct {
	connections map[*websocket.Conn]struct{}
	Register    chan *websocket.Conn
	Unregister  chan *websocket.Conn
	Broadcast   chan []byte
	mu          sync.Mutex
}

// NewHub constructs a Hub.
func NewHub() *Hub {
	return &Hub{
		connections: make(map[*websocket.Conn]struct{}),
		Register:    make(chan *websocket.Conn),
		Unregister:  make(chan *websocket.Conn),
		Broadcast:   make(chan []byte),
	}
}

// Run processes register/unregister/broadcast events.
func (h *Hub) Run() {
	for {
		select {
		case conn := <-h.Register:
			h.mu.Lock()
			h.connections[conn] = struct{}{}
			h.mu.Unlock()
		case conn := <-h.Unregister:
			h.mu.Lock()
			delete(h.connections, conn)
			h.mu.Unlock()
			conn.Close()
		case msg := <-h.Broadcast:
			h.mu.Lock()
			for conn := range h.connections {
				if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
					conn.Close()
					delete(h.connections, conn)
				}
			}
			h.mu.Unlock()
		}
	}
}
