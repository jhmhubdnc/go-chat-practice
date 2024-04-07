package network

import (
	. "chat_server_go/types"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

var upgrader = &websocket.Upgrader{ReadBufferSize: SocketBufferSize, WriteBufferSize: MessageBufferSize, CheckOrigin: func(r *http.Request) bool { return true }}

type Room struct {
	Forward chan *message // 수신되는 메시지를 보관

	Join  chan *client // Socket이 연결되는 경우 작동
	Leave chan *client // Socket이 끊어지는 경우 작동

	Clients map[*client]bool // 현재 방에 있는 Client 정보 저장
}

type message struct {
	Name    string
	Message string
	Time    int64
}

type client struct {
	Send   chan *message
	Room   *Room
	Name   string
	Socket *websocket.Conn
}

func NewRoom() *Room {
	return &Room{
		Forward: make(chan *message),
		Join:    make(chan *client),
		Leave:   make(chan *client),
		Clients: make(map[*client]bool),
	}
}

// 클라이언트 메시지 읽는 함수
func (c *client) Read() {
	defer c.Socket.Close()
	for {
		var msg *message
		err := c.Socket.ReadJSON(&msg)
		if err != nil {
			if !websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway) {
				break
			} else {
				panic(err)
			}
		} else {
			msg.Time = time.Now().Unix()
			msg.Name = c.Name
			c.Room.Forward <- msg
		}
	}
}

// 클라이언트 메시지 전송하는 함수
func (c *client) Write() {
	defer c.Socket.Close()
	for msg := range c.Send {
		err := c.Socket.WriteJSON(msg)
		if err != nil {
			panic(err)
		}
	}
}

func (r *Room) RunInit() {
	// Room에 있는 모든 채널들을 받는 역할
	for {
		select {
		case client := <-r.Join:
			r.Clients[client] = true
		case client := <-r.Leave:
			r.Clients[client] = false
			delete(r.Clients, client)
			close(client.Send)
		case msg := <-r.Forward:
			for client := range r.Clients {
				client.Send <- msg
			}
		}
	}
}

func (r *Room) SocketServe(c *gin.Context) {
	socket, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		panic(err)
	}

	userCookie, err := c.Request.Cookie("auth")
	if err != nil {
		panic(err)
	}

	client := &client{
		Socket: socket,
		Send:   make(chan *message, MessageBufferSize),
		Room:   r,
		Name:   userCookie.Value,
	}

	r.Join <- client

	defer func() { r.Leave <- client }()

	go client.Write()
	client.Read()
}
