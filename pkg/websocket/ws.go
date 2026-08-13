package websocket

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"

	"McsmTools/pkg/auth"
	"McsmTools/pkg/mcserver"
	"McsmTools/pkg/metrics"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true // Allow connection
	},
}

type WSMessage struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload,omitempty"`
	Command string      `json:"command,omitempty"`
}

type Hub struct {
	clients   map[*websocket.Conn]bool
	broadcast chan WSMessage
	register  chan *websocket.Conn
	unregister chan *websocket.Conn
	mu        sync.RWMutex
}

var globalHub *Hub

func init() {
	globalHub = &Hub{
		clients:    make(map[*websocket.Conn]bool),
		broadcast:  make(chan WSMessage, 256),
		register:   make(chan *websocket.Conn),
		unregister: make(chan *websocket.Conn),
	}
	go globalHub.run()
	go globalHub.metricsTicker()
}

func GetHub() *Hub {
	return globalHub
}

func (h *Hub) run() {
	logCh := mcserver.GetManager().SubscribeLogs()
	for {
		select {
		case conn := <-h.register:
			h.mu.Lock()
			h.clients[conn] = true
			h.mu.Unlock()

			// Send log history immediately upon connection
			history := mcserver.GetManager().GetLogHistory()
			_ = conn.WriteJSON(WSMessage{
				Type:    "history",
				Payload: history,
			})

		case conn := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[conn]; ok {
				delete(h.clients, conn)
				conn.Close()
			}
			h.mu.Unlock()

		case logLine := <-logCh:
			h.BroadcastMessage(WSMessage{
				Type:    "log",
				Payload: logLine,
			})

		case msg := <-h.broadcast:
			h.mu.RLock()
			for conn := range h.clients {
				err := conn.WriteJSON(msg)
				if err != nil {
					conn.Close()
					delete(h.clients, conn)
				}
			}
			h.mu.RUnlock()
		}
	}
}

func (h *Hub) metricsTicker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		m, err := metrics.CollectMetrics()
		if err == nil {
			h.BroadcastMessage(WSMessage{
				Type:    "metrics",
				Payload: m,
			})
		}
	}
}

func (h *Hub) BroadcastMessage(msg WSMessage) {
	h.broadcast <- msg
}

func ServeWS(w http.ResponseWriter, r *http.Request) {
	if !auth.IsAuthenticated(r) {
		http.Error(w, "Unauthorized WebSocket connection", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS Error] Upgrade failed: %v", err)
		return
	}

	globalHub.register <- conn

	go func() {
		defer func() {
			globalHub.unregister <- conn
		}()

		for {
			_, messageBytes, err := conn.ReadMessage()
			if err != nil {
				break
			}

			var msg WSMessage
			if err := json.Unmarshal(messageBytes, &msg); err != nil {
				continue
			}

			handleIncomingMessage(msg)
		}
	}()
}

func handleIncomingMessage(msg WSMessage) {
	mgr := mcserver.GetManager()
	switch msg.Type {
	case "command":
		if msg.Command != "" {
			_ = mgr.SendCommand(msg.Command)
		}
	case "start":
		_ = mgr.Start()
	case "stop":
		_ = mgr.Stop()
	case "restart":
		_ = mgr.Restart()
	case "kill":
		_ = mgr.Kill()
	}
}
