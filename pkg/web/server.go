package web

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Client struct {
	conn *websocket.Conn
	send chan []byte
}

type Server struct {
	clients    map[*Client]bool
	broadcast  chan []byte
	register   chan *Client
	unregister chan *Client
	lock       sync.Mutex
	CmdHandler func(cmd string, params map[string]interface{})
}

func NewServer() *Server {
	return &Server{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

func (s *Server) Run() {
	for {
		select {
		case client := <-s.register:
			s.lock.Lock()
			s.clients[client] = true
			s.lock.Unlock()
		case client := <-s.unregister:
			s.lock.Lock()
			if _, ok := s.clients[client]; ok {
				delete(s.clients, client)
				close(client.send)
			}
			s.lock.Unlock()
		case message := <-s.broadcast:
			s.lock.Lock()
			for client := range s.clients {
				select {
				case client.send <- message:
				default:
					close(client.send)
					delete(s.clients, client)
				}
			}
			s.lock.Unlock()
		}
	}
}

func (s *Server) BroadcastAudio(pcm []byte) {
	// Send binary audio data to all clients
	s.lock.Lock()
	defer s.lock.Unlock()
	for client := range s.clients {
		err := client.conn.WriteMessage(websocket.BinaryMessage, pcm)
		if err != nil {
			log.Printf("error broadcasting audio: %v", err)
		}
	}
}

func (s *Server) BroadcastJSON(data interface{}) {
	msg, err := json.Marshal(data)
	if err != nil {
		return
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	for client := range s.clients {
		err := client.conn.WriteMessage(websocket.TextMessage, msg)
		if err != nil {
			log.Printf("error broadcasting json: %v", err)
		}
	}
}

func (s *Server) Start(addr string) {
	go s.Run()

	r := gin.Default()

	r.GET("/ws", func(c *gin.Context) {
		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			log.Printf("Web: WebSocket upgrade failed: %v", err)
			return
		}
		client := &Client{conn: conn, send: make(chan []byte, 256)}
		s.register <- client
		log.Printf("Web: Client connected from %s", conn.RemoteAddr())

		go s.writePump(client)
		go s.readPump(client)
	})

	// Serve static files without conflicting with /ws
	r.Static("/assets", "./web/dist/assets")
	r.StaticFile("/", "./web/dist/index.html")
	r.StaticFile("/vite.svg", "./web/dist/vite.svg")
	r.StaticFile("/audio-processor.js", "./web/dist/audio-processor.js")

	log.Printf("Web server starting on %s", addr)
	go r.Run(addr)
}

func (s *Server) writePump(c *Client) {
	defer func() {
		c.conn.Close()
	}()
	for {
		message, ok := <-c.send
		if !ok {
			c.conn.WriteMessage(websocket.CloseMessage, []byte{})
			return
		}
		err := c.conn.WriteMessage(websocket.TextMessage, message)
		if err != nil {
			return
		}
	}
}

func (s *Server) readPump(c *Client) {
	defer func() {
		s.unregister <- c
		c.conn.Close()
	}()
	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		
		var payload struct {
			Cmd    string                 `json:"cmd"`
			Params map[string]interface{} `json:"params"`
		}
		if err := json.Unmarshal(message, &payload); err == nil && s.CmdHandler != nil {
			s.CmdHandler(payload.Cmd, payload.Params)
		}
	}
}
