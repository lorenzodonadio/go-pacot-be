package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		fmt.Println("OriginURL: ", r.URL)
		return true // or validate the origin if needed
	},
}

var Users = NewUserRegistry()
var WsLobbyReg = NewWsLobbyRegistry()
var LobbyGameReg = NewGameLobbyRegistry()
var PacotGameReg = NewPacotGameRegistry()
var WsGamePlayerReg = NewWsGamePlayerRegistry()

func main() {

	r := chi.NewRouter()
	UseMiddleware(r)
	// Serve static files from the "src" directory
	SetupRoutes(r)

	fmt.Println("Server running on port :8080")
	http.ListenAndServe(":8080", r)
}
