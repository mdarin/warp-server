package web

import (
	"context"
	"log"
	"net/http"
	"strings"
	"time"
	"warp-server/pkg/controlloop"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
)

// Client представляет одно WebSocket соединение
type Client struct {
	hub  *Hub
	conn *websocket.Conn
	send chan Message
}

func serveWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("websocket upgrade error: %v", err)
		return
	}

	client := &Client{
		hub:  hub,
		conn: conn,
		send: make(chan Message, 256),
	}

	hub.register <- client

	go client.writePump()
	go client.readPump()
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteJSON(msg); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// Читаем входящие сообщения (для корректного закрытия соединения)
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}
}

// CreateWebTUI — аналог CreateTUI для WebSocket
func CreateWebTUI(
	ctx context.Context,
	addr string,
	logs <-chan string,
	conditions <-chan []controlloop.Condition,
) error {
	srv := NewServer(addr)

	// Пересылаем логи
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-logs:
				if !ok {
					return
				}
				srv.Broadcast(Message{
					Type:    MessageTypeLog,
					Payload: msg,
				})
			}
		}
	}()

	// Пересылаем условия
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case c, ok := <-conditions:
				if !ok {
					return
				}
				payload := make([]ConditionPayload, 0, len(c))
				for _, cond := range c {
					payload = append(payload, ConditionPayload{
						Type:    cond.Type,
						Reason:  strings.ToUpper(cond.Reason),
						Status:  cond.Status,
						Message: cond.Message,
					})
				}
				srv.Broadcast(Message{
					Type:    MessageTypeCondition,
					Payload: payload,
				})
			}
		}
	}()

	return srv.Start(ctx)
}
