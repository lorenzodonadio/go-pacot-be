package main

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func SetupRoutes(r *chi.Mux) {
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("✅ API OK ✅"))
	})

	r.Post("/login", HPostLogin)

	// Lobby

	r.Get("/lobby", HGetLobby)
	r.Get("/ws/lobby", HLobbyWS)

	r.Post("/create_game", HPostCreateGame)
	r.Get("/create_game", HGetCreateGame)

	r.Post("/invite_to_game", HPostInviteGame)
	//ACTUAL GAME
	r.Route("/game/{gameId}", func(r chi.Router) {
		// called from lobby
		r.Put("/accept_invite", HPutAcceptInvite)
		r.Post("/start", HPostStartGame)
		r.Delete("/delete", HDeleteLobbyGame)
		//called after game starts
		r.Post("/bid", HPostGameBid)
		r.Post("/roll", HPostRollDice)
		r.Post("/exacto", HPostExacto)
		r.Post("/liar", HPostLiar)
	})

	// WS
	r.Get("/ws/game/{gameId}", HGameWS)
	// DEBUG
	r.Get("/games", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println(&PacotGameReg.Games)
		fmt.Println(PacotGameReg.Games)
		WriteJSON(w, PacotGameReg.Games)
	})
	r.Get("/gamesws", func(w http.ResponseWriter, r *http.Request) {
		WriteJSON(w, WsGamePlayerReg.GamePlayers)
	})
}
