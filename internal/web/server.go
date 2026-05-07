package web

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

//go:embed static
var staticFiles embed.FS

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Message типы сообщений для клиента
type MessageType string

const (
	MessageTypeLog       MessageType = "log"
	MessageTypeCondition MessageType = "condition"
)

// Message представляет сообщение для клиента
type Message struct {
	Type    MessageType `json:"type"`
	Payload interface{} `json:"payload"`
}

// ConditionPayload представляет состояние условий
type ConditionPayload struct {
	Type    string `json:"type"`
	Reason  string `json:"reason"`
	Status  string `json:"status"`
	Message string `json:"message"`
}

// Hub управляет всеми WebSocket соединениями
type Hub struct {
	mu      sync.RWMutex
	clients map[*Client]struct{}

	// Буферы для новых клиентов
	logBuffer       []string
	conditionBuffer []ConditionPayload

	broadcast  chan Message
	register   chan *Client
	unregister chan *Client
}

func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]struct{}),
		broadcast:  make(chan Message, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run запускает основной цикл Hub
func (h *Hub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for client := range h.clients {
				close(client.send)
			}
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			// Отправляем буферизованное состояние новому клиенту
			snapshot := h.buildSnapshot()
			h.mu.Unlock()
			for _, msg := range snapshot {
				client.send <- msg
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.Lock()
			h.updateBuffer(msg)
			clients := make([]*Client, 0, len(h.clients))
			for client := range h.clients {
				clients = append(clients, client)
			}
			h.mu.Unlock()

			for _, client := range clients {
				select {
				case client.send <- msg:
				default:
					// Клиент не успевает читать — отключаем
					h.unregister <- client
				}
			}
		}
	}
}

func (h *Hub) buildSnapshot() []Message {
	msgs := make([]Message, 0, len(h.logBuffer)+1)
	for _, log := range h.logBuffer {
		msgs = append(msgs, Message{
			Type:    MessageTypeLog,
			Payload: log,
		})
	}
	if len(h.conditionBuffer) > 0 {
		msgs = append(msgs, Message{
			Type:    MessageTypeCondition,
			Payload: h.conditionBuffer,
		})
	}
	return msgs
}

func (h *Hub) updateBuffer(msg Message) {
	switch msg.Type {
	case MessageTypeLog:
		if log, ok := msg.Payload.(string); ok {
			h.logBuffer = append(h.logBuffer, log)
			// Ограничиваем размер буфера логов
			if len(h.logBuffer) > 1000 {
				h.logBuffer = h.logBuffer[len(h.logBuffer)-1000:]
			}
		}
	case MessageTypeCondition:
		if conditions, ok := msg.Payload.([]ConditionPayload); ok {
			h.conditionBuffer = conditions
		}
	}
}

// Server HTTP сервер с WebSocket поддержкой
type Server struct {
	hub    *Hub
	server *http.Server
}

func NewServer(addr string) *Server {
	hub := NewHub()
	mux := http.NewServeMux()

	staticFS, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/", http.FileServer(http.FS(staticFS)))
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		serveWS(hub, w, r)
	})

	return &Server{
		hub: hub,
		server: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
}

func (s *Server) Start(ctx context.Context) error {
	go s.hub.Run(ctx)

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(shutdownCtx)
	}()

	if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

// Broadcast отправляет сообщение всем клиентам
func (s *Server) Broadcast(msg Message) {
	select {
	case s.hub.broadcast <- msg:
	default:
	}
}
