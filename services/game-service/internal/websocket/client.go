package websocket

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 512 * 1024 // 512 KB
)

type Client struct {
	Hub        *Hub
	Conn       *websocket.Conn
	Send       chan []byte
	UserID     string
	InstanceID string
	IsCreator  bool
}

func NewClient(hub *Hub, conn *websocket.Conn, userID, instanceID string, isCreator bool) *Client {
	return &Client{
		Hub:        hub,
		Conn:       conn,
		Send:       make(chan []byte, 256),
		UserID:     userID,
		InstanceID: instanceID,
		IsCreator:  isCreator,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.Hub.Unregister <- c
		c.Conn.Close()
	}()

	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		var msg Message
		if err := json.Unmarshal(message, &msg); err != nil {
			log.Printf("Failed to unmarshal message: %v", err)
			c.SendError("Invalid message format")
			continue
		}

		c.Hub.HandleMessage <- &ClientMessage{
			Client:  c,
			Message: msg,
		}
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	log.Printf("WritePump started for user %s", c.UserID)

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				log.Printf("Send channel closed for user %s, closing connection", c.UserID)
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			log.Printf("Sending message to user %s: %s", c.UserID, string(message))

			w, err := c.Conn.NextWriter(websocket.TextMessage)
			if err != nil {
				log.Printf("Failed to get next writer for user %s: %v", c.UserID, err)
				return
			}
			w.Write(message)

			n := len(c.Send)
			for range n {
				w.Write([]byte{'\n'})
				w.Write(<-c.Send)
			}

			if err := w.Close(); err != nil {
				log.Printf("Failed to close writer for user %s: %v", c.UserID, err)
				return
			}

		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func (c *Client) SendMessage(msgType MessageType, payload any) {
	msg := Message{
		Type:    msgType,
		Payload: payload,
	}
	data, err := json.Marshal(msg)
	if err != nil {
		log.Printf("Failed to marshal message: %v", err)
		return
	}
	select {
	case c.Send <- data:
	default:
		log.Printf("Client send channel full, closing connection")
		close(c.Send)
	}
}

func (c *Client) SendError(message string) {
	c.SendMessage(MessageTypeError, ErrorPayload{Message: message})
}