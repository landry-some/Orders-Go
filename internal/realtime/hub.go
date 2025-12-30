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

// HubBroadcaster adapts a Hub to the Broadcaster interface.
type HubBroadcaster struct {
	hub *Hub
}

// NewHubBroadcaster constructs a broadcaster backed by a Hub.
func NewHubBroadcaster(h *Hub) *HubBroadcaster {
	return &HubBroadcaster{hub: h}
}

// Broadcast sends the message to the hub's broadcast channel non-blocking.
func (b *HubBroadcaster) Broadcast(msg []byte) {
	select {
	case b.hub.Broadcast <- msg:
	default:
		// drop if subscribers are slow
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
