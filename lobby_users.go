package main

import (
	"fmt"
	"log"
	"net/http"
	"sync"

	"github.com/gorilla/websocket"
)

type LobbyUserConn struct {
	PlayerName string
	Conn       *websocket.Conn
}

type LobbyRegistry struct {
	LobbyUsers map[string]*LobbyUserConn
	mu         sync.RWMutex // A mutex to ensure concurrent access to the map is safe
}

func HGetLobby(w http.ResponseWriter, r *http.Request) {
	fmt.Println(Users)

	user, status := Users.FindUserByBearerToken(r)
	if status != UserFound {
		w.Write([]byte("UserNotFound"))
	}

	WriteJSON(w, user)
}

func HLobbyWS(w http.ResponseWriter, r *http.Request) {
	userId := r.URL.Query().Get("userId")
	if userId == "" {
		http.Error(w, "userId not found", http.StatusBadRequest)
		return
	}
	// Now you have the 'userId' and can use it as needed
	user, err := Users.FindUserByID(userId)
	// user, status := Users.FindUserByBearerToken(r)
	if err != nil {
		http.Error(w, "userId not found", http.StatusBadRequest)
		return
	}

	fmt.Println("USER FOUND: ", user)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	fmt.Println("CONNECTION STABLISHED")
	defer func() {
		// Cleanup when the user disconnects
		conn.Close()
		WsLobbyReg.RemoveLobbyUser(user.ID)
		// delete(WsLobbyReg.Registry, userID)
		WsLobbyReg.broadcastLobbyList()
	}()
	fmt.Println("DEFER STABLISHED")

	// Add the user to the LobbyRegistry
	WsLobbyReg.AddLobbyUser(user, conn)
	// .Users.AddOnlineUser(userId, conn)
	// ... rest of the WebSocket handling ...

	WsLobbyReg.broadcastLobbyList()
	fmt.Println(WsLobbyReg.LobbyUsers)

	for {
		// Handle incoming messages if needed
		mt, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		fmt.Println(mt, p)
		// for now just echo everything
		for _, onlineUser := range WsLobbyReg.LobbyUsers {
			if err := onlineUser.Conn.WriteMessage(mt, p); err != nil {
				log.Println("Error broadcasting message:", err)
				// Handle the error: you might decide to remove the user or ignore the error.
			}
		}
		// TODO: Handle incoming messages if you need bidirectional communication.
	}
}

// Singleton Please
func NewWsLobbyRegistry() *LobbyRegistry {
	return &LobbyRegistry{
		LobbyUsers: make(map[string]*LobbyUserConn),
	}
}

func (or *LobbyRegistry) AddLobbyUser(user *User, conn *websocket.Conn) (*LobbyUserConn, error) {
	fmt.Println("USER FOUND: ", user)
	or.mu.Lock()
	defer or.mu.Unlock()

	lobbyUser := &LobbyUserConn{
		PlayerName: user.PlayerName,
		Conn:       conn,
	}
	or.LobbyUsers[user.ID] = lobbyUser
	return lobbyUser, nil
}

func (or *LobbyRegistry) RemoveLobbyUser(id string) {
	or.mu.Lock()
	defer or.mu.Unlock()

	delete(or.LobbyUsers, id)
}

func (or *LobbyRegistry) GetLobbyUser(id string) (*LobbyUserConn, bool) {
	or.mu.RLock()
	defer or.mu.RUnlock()

	user, exists := or.LobbyUsers[id]
	return user, exists
}

func (or *LobbyRegistry) broadcastLobbyList() {
	or.mu.RLock() // Acquire a read lock
	defer or.mu.RUnlock()
	// Let's also construct a list of Lobby user IDs to send out
	payload := struct {
		Key        string       `json:"k"`
		LobbyUsers []UserPublic `json:"d"`
	}{
		Key:        "lobby_users",
		LobbyUsers: make([]UserPublic, 0, len(or.LobbyUsers)),
	}
	// LobbyUserNames := make([]string, 0, len(or.LobbyUsers))

	for id := range or.LobbyUsers {
		user, err := Users.FindUserByID(id)
		fmt.Println("Found user: ", user)
		if err != nil {
			continue
		}
		payload.LobbyUsers = append(payload.LobbyUsers, user.GetPubUser())
		// user.PlayerName
	}
	// "USERLIST:"
	// payload := struct {
	// 	LobbyUsers []string `json:"users"`
	// }{
	// 	LobbyUsers: LobbyUserNames,
	// }

	for _, LobbyUser := range or.LobbyUsers {
		//in the client the payload shows empty
		if err := LobbyUser.Conn.WriteJSON(payload); err != nil {
			log.Println("Error broadcasting user list:", err)
		}
		// if err := LobbyUser.Conn.WriteMessage(1, []byte(LobbyUser.PlayerName)); err != nil {
		// 	log.Println("Error broadcasting user list:", err)
		// }
	}
}

// payload := LobbyUsersMessage{LobbyUserNames: LobbyUserNames}
// fmt.Println(payload) //this prints correctly
// for _, LobbyUser := range or.LobbyUsers {
// 	//in the client the payload shows empty
// 	if err := LobbyUser.Conn.WriteJSON(payload); err != nil {
// 		log.Println("Error broadcasting user list:", err)
// 		// Handle the error: you might decide to remove the user or ignore the error.
// 	}
// }
