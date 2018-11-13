package main

import (
	"fmt"
	"flag"
	"net/http"
	"encoding/json"
	"github.com/gorilla/websocket"
	"github.com/satori/go.uuid"
)


//Client : Object client connect
type Client struct {
	id string
	socket *websocket.Conn
	send chan []byte 
}

//ClientManager : manager all client 
type ClientManager struct {
	clients map[*Client]bool
	broadcast chan []byte
	register chan *Client
	unRegister chan *Client
}

//Message : message send to client
type Message struct {
	Sender string `json:"sender, omitempty"`
	Content string `json:"content, omitempty"`
}

var manager = ClientManager {
	clients: make(map[*Client]bool),
	broadcast: make(chan []byte),
	register: make(chan *Client),
	unRegister: make(chan *Client),
}


func(manager *ClientManager)start() {
	for {
		select {
			case conn :=  <-manager.register ://register
				manager.clients[conn] = true
				jsonMessage, _ := json.Marshal(&Message{Sender: conn.id, Content: "new user connected"})
				manager.send(jsonMessage, conn)
			case conn := <-manager.unRegister://un register
				if _, ok := manager.clients[conn] ; ok {
					close(conn.send)
					delete(manager.clients, conn)
					jsonMessage, _ := json.Marshal(&Message{Sender: conn.id, Content: "user disconnected"})
					manager.send(jsonMessage, conn)
				}
			case message := <- manager.broadcast : //broadcast
				for conn := range manager.clients {
					select {
						case conn.send <- message:
						default :
							close(conn.send)
							delete(manager.clients, conn) 
					}
				}
		}
	}
}

func(manager *ClientManager) send(message []byte, ignore *Client) {
	for conn := range manager.clients {
		if conn != ignore {
			conn.send <- message
		}
	}
}


func(client *Client) read() {
	defer func() {
		manager.unRegister<-client
		client.socket.Close()
	}()

	for {
		_, message, err := client.socket.ReadMessage()
		if err != nil {
			manager.unRegister <- client
			client.socket.Close()
			break
		}
		jsonMessage, _ := json.Marshal(&Message{Sender: client.id, Content: string(message)})
		manager.broadcast <- jsonMessage
	}
}

func(client *Client) write() {
	defer func() {
		client.socket.Close()
	}()

	for {
		select {
		case message, ok := <- client.send:
				if !ok {
					client.socket.WriteMessage(websocket.CloseMessage, []byte{})
					return
				}

				client.socket.WriteMessage(websocket.TextMessage, message)
		}
	}
}


func wsPage(res http.ResponseWriter, req *http.Request)  {
	conn, err := (&websocket.Upgrader{CheckOrigin: func(r *http.Request) bool{ return true }}).Upgrade(res,req, nil)
	if err != nil {
		http.NotFound(res,req)
		return
	}

	uid , err := uuid.NewV4()
	if err != nil {
		fmt.Println("UID V4 Create Error", err)
		return
	}

	client := &Client{id: uid.String(), socket: conn, send: make(chan []byte)}
	manager.register <- client
	go client.read()
	go client.write()
}


var PORT = flag.Int("port",7878, "Port server listener")
func main() {
	flag.Parse()
	
	host := fmt.Sprintf(":%d",*PORT)
	fmt.Println("Start Server....", host) 
	go manager.start()
	http.HandleFunc("/ws", wsPage)//ws://id/ws
	http.ListenAndServe(host, nil)
}