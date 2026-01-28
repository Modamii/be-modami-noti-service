package ws

import (
	"encoding/json"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

// Hub holds rooms (roomID -> connections) and broadcasts to a room.
type Hub struct {
	mu        sync.RWMutex
	rooms     map[string]map[*Conn]struct{}
	register  chan *Conn
	unregister chan *Conn
}

type Conn struct {
	UserID string
	Topic  string
	RoomID string // user:{userID} or topic:{topic}
	conn   *websocket.Conn
	send   chan []byte
}

func NewHub() *Hub {
	h := &Hub{
		rooms:      make(map[string]map[*Conn]struct{}),
		register:   make(chan *Conn),
		unregister: make(chan *Conn),
	}
	go h.run()
	return h
}

func (h *Hub) run() {
	for {
		select {
		case c := <-h.register:
			h.mu.Lock()
			if h.rooms[c.RoomID] == nil {
				h.rooms[c.RoomID] = make(map[*Conn]struct{})
			}
			h.rooms[c.RoomID][c] = struct{}{}
			h.mu.Unlock()
		case c := <-h.unregister:
			h.mu.Lock()
			if m, ok := h.rooms[c.RoomID]; ok {
				delete(m, c)
				if len(m) == 0 {
					delete(h.rooms, c.RoomID)
				}
			}
			close(c.send)
			h.mu.Unlock()
		}
	}
}

func (h *Hub) Register(conn *websocket.Conn, userID, topic string) *Conn {
	roomID := "user:" + userID
	if topic != "" {
		roomID = "topic:" + topic
	}
	c := &Conn{
		UserID: userID,
		Topic:  topic,
		RoomID: roomID,
		conn:   conn,
		send:   make(chan []byte, 256),
	}
	h.register <- c
	go c.writePump()
	return c
}

func (h *Hub) Unregister(c *Conn) {
	h.unregister <- c
}

// Broadcast sends event+payload to all connections in the room.
func (h *Hub) Broadcast(roomID, event string, payload map[string]interface{}) {
	msg := map[string]interface{}{"event": event, "payload": payload}
	b, err := json.Marshal(msg)
	if err != nil {
		log.Printf("ws Broadcast marshal: %v", err)
		return
	}
	h.mu.RLock()
	room := h.rooms[roomID]
	h.mu.RUnlock()
	if room == nil {
		return
	}
	for c := range room {
		select {
		case c.send <- b:
		default:
			h.Unregister(c)
		}
	}
}

func (c *Conn) writePump() {
	defer c.conn.Close()
	for b := range c.send {
		if err := c.conn.WriteMessage(websocket.TextMessage, b); err != nil {
			return
		}
	}
}
