package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Player struct {
	ID    string `json:"id"`
	X     int    `json:"x"`
	Y     int    `json:"y"`
	Color string `json:"color"`
	Char  string `json:"char"`
}

type GameState struct {
	Players map[string]*Player `json:"players"`
	Chars   map[string]string  `json:"chars"`
	Mutex   sync.Mutex
}

var state = GameState{
	Players: make(map[string]*Player),
	Chars:   make(map[string]string),
}
var upgrader = websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}
var connections = make(map[*websocket.Conn]bool)
var connMutex = sync.Mutex{}

func handleConnection(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		fmt.Println("Upgrade error:", err)
		return
	}
	defer conn.Close()

	connMutex.Lock()
	connections[conn] = true
	connMutex.Unlock()

	id := fmt.Sprintf("player-%d", rand.Intn(1000000))
	player := &Player{
		ID:    id,
		X:     rand.Intn(50),
		Y:     rand.Intn(50),
		Color: fmt.Sprintf("rgb(%d,%d,%d)", rand.Intn(256), rand.Intn(256), rand.Intn(256)),
		Char:  "",
	}

	state.Mutex.Lock()
	state.Players[id] = player
	state.Mutex.Unlock()

	defer func() {
		state.Mutex.Lock()
		delete(state.Players, id)
		state.Mutex.Unlock()

		connMutex.Lock()
		delete(connections, conn)
		connMutex.Unlock()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var input struct {
			DX   int    `json:"dx"`
			DY   int    `json:"dy"`
			Char string `json:"char"`
		}
		json.Unmarshal(msg, &input)

		state.Mutex.Lock()
		if p, ok := state.Players[id]; ok {
			p.X = (p.X + input.DX + 50) % 50
			p.Y = (p.Y + input.DY + 50) % 50
			if input.Char != "" {
				state.Chars[fmt.Sprintf("%d,%d", p.X, p.Y)] = input.Char
				/*p.X = (p.X + 1 + 50) % 50 */ // eher in die html packen?
			}
		}
		state.Mutex.Unlock()
	}
}

func broadcastGameState() {
	ticker := time.NewTicker(time.Second / 60)
	defer ticker.Stop()

	for range ticker.C {
		state.Mutex.Lock()
		data, _ := json.Marshal(struct {
			Players map[string]*Player `json:"players"`
			Chars   map[string]string  `json:"chars"`
		}{state.Players, state.Chars})
		state.Mutex.Unlock()

		connMutex.Lock()
		for conn := range connections {
			err := conn.WriteMessage(websocket.TextMessage, data)
			if err != nil {
				conn.Close()
				delete(connections, conn)
			}
		}
		connMutex.Unlock()
	}
}

func main() {
	http.Handle("/", http.FileServer(http.Dir(".")))
	http.HandleFunc("/ws", handleConnection)
	go broadcastGameState()

	fmt.Println("Server started on :8080")
	http.ListenAndServe(":8080", nil)
}
