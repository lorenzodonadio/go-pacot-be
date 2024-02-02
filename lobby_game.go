package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"sync"

	"github.com/go-chi/chi/v5"
)

type LobbyInvite struct {
	From   string `json:"from"`
	To     string `json:"to"`
	GameId string `json:"game_id"`
	At     int    `json:"at"`
}

type GameLobby struct {
	ID        string   `json:"id"`
	CreatedBy string   `json:"created_by"`
	Players   []string `json:"players"`
}

type GameLobbyRegistry struct {
	GameLobby map[string]*GameLobby
	mu        sync.RWMutex // A mutex to ensure concurrent access to the map is safe
}

func NewGameLobbyRegistry() *GameLobbyRegistry {
	return &GameLobbyRegistry{
		GameLobby: make(map[string]*GameLobby),
	}
}

func (game *GameLobby) BroadcastMessage(message string) {
	// HELPERS
	payload := struct {
		Key string `json:"k"`
	}{
		Key: message,
	}
	for _, playerName := range game.Players {
		user, err := Users.FindUserByPlayerName(playerName)
		if err != nil {
			continue
		}
		lobbyConn, exists := WsLobbyReg.GetLobbyUser(user.ID)
		if exists == false {
			continue
		}

		lobbyConn.Conn.WriteJSON(payload)
	}

}
func (glr *GameLobbyRegistry) AddLobbyGame(user *User) *GameLobby {
	glr.mu.Lock()
	defer glr.mu.Unlock()

	id := GenShortID(12)

	glr.GameLobby[id] = &GameLobby{
		ID:        id,
		CreatedBy: user.PlayerName,
		Players:   []string{user.PlayerName}, //Max of 8 players
	}

	return glr.GameLobby[id]
}

func (glr *GameLobbyRegistry) AddPlayerToGame(user *User, gameId string) (*GameLobby, error) {
	glr.mu.Lock()
	defer glr.mu.Unlock()

	for _, lobbyGame := range glr.GameLobby {
		if lobbyGame.ID != gameId {
			continue
		}

		if slices.Contains(lobbyGame.Players, user.PlayerName) {
			return nil, errors.New("Player Already In Game")
		}

		lobbyGame.Players = append(lobbyGame.Players, user.PlayerName)

		return lobbyGame, nil
	}

	return nil, errors.New("Unable To Add Player")
}

func (glr *GameLobbyRegistry) GetGameById(gameId string) (*GameLobby, error) {
	glr.mu.Lock()
	defer glr.mu.Unlock()

	for _, lobbyGame := range glr.GameLobby {
		if lobbyGame.ID == gameId {
			return lobbyGame, nil
		}
	}

	return nil, errors.New("Unable To Add Player")
}

func (glr *GameLobbyRegistry) DeleteGame(gameId string) {
	glr.mu.Lock()
	defer glr.mu.Unlock()

	delete(glr.GameLobby, gameId)
}

func HPostCreateGame(w http.ResponseWriter, r *http.Request) {

	user, status := Users.FindUserByBearerToken(r)

	if status != UserFound {
		http.Error(w, "No Player Found", http.StatusUnauthorized)
		return
	}

	for _, game := range LobbyGameReg.GameLobby {
		if game.CreatedBy == user.PlayerName {
			http.Error(w, "You Already Created a Game", http.StatusConflict)
			return
		}
	}

	newGame := LobbyGameReg.AddLobbyGame(user)
	WriteJSON(w, newGame)
}

func HGetCreateGame(w http.ResponseWriter, r *http.Request) {
	WriteJSON(w, LobbyGameReg.GameLobby)
}

func HPostInviteGame(w http.ResponseWriter, r *http.Request) {

	user, status := Users.FindUserByBearerToken(r)

	if status != UserFound {
		http.Error(w, "No Player Found", http.StatusUnauthorized)
		return
	}

	// Parse body to get the 'name'
	var body LobbyInvite
	// Decode the JSON body into the requestData
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if body.From != user.PlayerName {
		http.Error(w, "You can only invite on your behalf", http.StatusUnauthorized)
		return
	}

	userToInvite, err := Users.FindUserByPlayerName(body.To)

	if err != nil {
		http.Error(w, "Invited Player Does Not Exist", http.StatusBadRequest)
		return
	}
	userToInviteConn, exists := WsLobbyReg.GetLobbyUser(userToInvite.ID)

	if exists == false {
		http.Error(w, "Invited Player Not Connected", http.StatusBadRequest)
		return
	}

	payload := struct {
		Key    string      `json:"k"`
		Invite LobbyInvite `json:"d"`
	}{
		Key:    "invite",
		Invite: body,
	}

	userToInviteConn.Conn.WriteJSON(payload)
}

func HPutAcceptInvite(w http.ResponseWriter, r *http.Request) {
	user, status := Users.FindUserByBearerToken(r)

	if status != UserFound {
		http.Error(w, "No Player Found", http.StatusUnauthorized)
		return
	}

	gameId := chi.URLParam(r, "gameId")
	playerName := r.URL.Query().Get("player_name")

	// Check if 'game_id' or 'player_name' are empty
	if gameId == "" || playerName == "" || playerName != user.PlayerName {
		http.Error(w, "Wrong params ", http.StatusBadRequest)
		return
	}

	game, err := LobbyGameReg.AddPlayerToGame(user, gameId)

	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	payload := struct {
		Key  string     `json:"k"`
		Game *GameLobby `json:"d"`
	}{
		Key:  "game_update",
		Game: game,
	}
	for _, playerName := range game.Players {
		user, err := Users.FindUserByPlayerName(playerName)
		if err != nil {
			continue
		}
		lobbyConn, exists := WsLobbyReg.GetLobbyUser(user.ID)
		if exists == false {
			continue
		}

		lobbyConn.Conn.WriteJSON(payload)
	}
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Invite accepted successfully"))
}

func HPostStartGame(w http.ResponseWriter, r *http.Request) {
	user, status := Users.FindUserByBearerToken(r)

	if status != UserFound {
		http.Error(w, "No Player Found", http.StatusUnauthorized)
		return
	}

	gameId := chi.URLParam(r, "gameId")

	game, err := LobbyGameReg.GetGameById(gameId)

	if err != nil {
		http.Error(w, "No Game Found", http.StatusBadRequest)
		return
	}

	if game.CreatedBy != user.PlayerName {
		http.Error(w, "Only The Creator Can Start The Game", http.StatusBadRequest)
		return
	}
	// now we are maintaining two structures, the game state and the players WS connection
	PacotGameReg.AddPacotGame(gameId, game.Players)
	WsGamePlayerReg.NewGame(gameId)

	game.BroadcastMessage("start")

}

func HDeleteLobbyGame(w http.ResponseWriter, r *http.Request) {
	user, status := Users.FindUserByBearerToken(r)

	if status != UserFound {
		http.Error(w, "No Player Found", http.StatusUnauthorized)
		return
	}

	gameId := chi.URLParam(r, "gameId")

	game, err := LobbyGameReg.GetGameById(gameId)

	if err != nil {
		http.Error(w, "No Game Found", http.StatusBadRequest)
		return
	}

	if game.CreatedBy != user.PlayerName {
		http.Error(w, "Only The Creator Can Delete The Game", http.StatusBadRequest)
		return
	}
	game.BroadcastMessage("delete")
	LobbyGameReg.DeleteGame(game.ID)

}
