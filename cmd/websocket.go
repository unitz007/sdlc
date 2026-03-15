package cmd

import (
    "encoding/json"
    "net/http"
    "sync"

    "github.com/gorilla/websocket"
)

// Hub maintains active websocket clients and broadcasts messages.
type Hub struct {
    // Registered clients.
    clients map[*websocket.Conn]bool
    // Broadcast channel for messages.
    broadcast chan []byte
    // Register channel for new connections.
    register chan *websocket.Conn
    // Unregister channel for closed connections.
    unregister chan *websocket.Conn
    mu sync.Mutex // protects clients map during iteration
}

func newHub() *Hub {
    return &Hub{
        clients:    make(map[*websocket.Conn]bool),
        broadcast:  make(chan []byte, 256),
        register:   make(chan *websocket.Conn),
        unregister: make(chan *websocket.Conn),
    }
}

var (
    hub      = newHub()
    upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
)

func init() {
    // start hub event loop
    go hub.run()
    // start broadcaster that listens for status updates and forwards to hub
    go func() {
        for range WaitForUpdate() {
            // Marshal the current status snapshot.
            data, err := json.Marshal(GetAll())
            if err != nil {
                continue
            }
            // Send to hub broadcast channel (non‑blocking).
            select {
            case hub.broadcast <- data:
            default:
                // if buffer is full, drop the update to avoid blocking.
            }
        }
    }()
}

// run processes hub registrations, unregistrations and broadcast messages.
func (h *Hub) run() {
    for {
        select {
        case c := <-h.register:
            h.mu.Lock()
            h.clients[c] = true
            h.mu.Unlock()
        case c := <-h.unregister:
            h.mu.Lock()
            if _, ok := h.clients[c]; ok {
                delete(h.clients, c)
                c.Close()
            }
            h.mu.Unlock()
        case msg := <-h.broadcast:
            h.mu.Lock()
            for c := range h.clients {
                if err := c.WriteMessage(websocket.TextMessage, msg); err != nil {
                    // Remove faulty client.
                    delete(h.clients, c)
                    c.Close()
                }
            }
            h.mu.Unlock()
        }
    }
}

// wsHandler upgrades an HTTP connection to a WebSocket and registers the client.
func wsHandler(w http.ResponseWriter, r *http.Request) {
    conn, err := upgrader.Upgrade(w, r, nil)
    if err != nil {
        http.Error(w, "WebSocket upgrade failed", http.StatusBadRequest)
        return
    }
    // Register the new client.
    hub.register <- conn
    // Send initial snapshot.
    if data, err := json.Marshal(GetAll()); err == nil {
        _ = conn.WriteMessage(websocket.TextMessage, data)
    }
    // Keep the connection alive. Read loop discards incoming messages.
    for {
        if _, _, err := conn.NextReader(); err != nil {
            // Client disconnected or error; unregister.
            hub.unregister <- conn
            break
        }
    }
}
