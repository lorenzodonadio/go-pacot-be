package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"sync"

	"github.com/gorilla/websocket"
)

type PacotPlayer struct {
	PlayerName string `json:"p_name"`
	NDice      int    `json:"n_dice"`
}

type DiceRoll struct {
	PlayerName string `json:"p_name"`
	Roll       []int  `json:"roll"`
}

type CurrBid struct {
	PlayerName string `json:"pn"`
	Bid        [2]int `json:"bid"`
}

type PacotGame struct {
	ID             string        `json:"id"`
	Players        []PacotPlayer `json:"pls"`
	CurrentPlayer  string        `json:"cp"`
	CurrentBid     CurrBid       `json:"cb"`
	NPlayers       int           `json:"np"`
	NActivePlayers int           `json:"nap"`
	State          string        `json:"st"`
	DiceRolls      []DiceRoll    `json:"dice_roll"`
}

type PacotGamePublic struct {
	ID             string        `json:"id"`
	Players        []PacotPlayer `json:"pls"`
	CurrentPlayer  string        `json:"cp"`
	CurrentBid     CurrBid       `json:"cb"`
	NPlayers       int           `json:"np"`
	NActivePlayers int           `json:"nap"`
	State          string        `json:"st"`
}

func (pg *PacotGame) GetPubPacotGame() PacotGamePublic {
	return PacotGamePublic{
		ID:             pg.ID,
		Players:        pg.Players,
		CurrentPlayer:  pg.CurrentPlayer,
		CurrentBid:     pg.CurrentBid,
		NPlayers:       pg.NPlayers,
		NActivePlayers: pg.NActivePlayers,
		State:          pg.State,
	}
}
func (pg *PacotGame) GetPlayerDiceNumber(playerName string) int {
	for _, p := range pg.Players {
		if p.PlayerName == playerName {
			return p.NDice
		}
	}
	return -1
}

func (pg *PacotGame) GetPlayerNameIndex(playerName string) int {
	for i, p := range pg.Players {
		if p.PlayerName == playerName {
			return i
		}
	}
	return -1
}

func (pg *PacotGame) NewBid(bid CurrBid) error {
	idx := pg.GetPlayerNameIndex(bid.PlayerName)
	if idx < 0 {
		return errors.New("Unacceptable Bid")
	}

	for i := 0; i < int(pg.NPlayers)-1; i++ {
		newPlayerIndex := (i + idx + 1) % len(pg.Players)
		if pg.Players[newPlayerIndex].NDice > 0 {
			pg.CurrentBid = bid
			pg.CurrentPlayer = pg.Players[newPlayerIndex].PlayerName
			return nil
		}
	}

	return errors.New("Unacceptable Bid")
}
func (pg *PacotGame) HandleExacto() {
	found := 0
	for _, dr := range pg.DiceRolls {
		for _, num := range dr.Roll {
			if num == 1 || num == pg.CurrentBid.Bid[1] {
				found++
			}
		}
	}

	idx := pg.GetPlayerNameIndex(pg.CurrentPlayer)

	if found == pg.CurrentBid.Bid[0] {

		pg.Players[idx].NDice++
		WsGamePlayerReg.BroadcastJsonMessage(pg.ID, "exacto", pg.Players[idx].PlayerName)
	} else {
		pg.Players[idx].NDice--
		WsGamePlayerReg.BroadcastJsonMessage(pg.ID, "lost_dice", pg.Players[idx])

		if pg.Players[idx].NDice == 0 {
			pg.NActivePlayers--
			WsGamePlayerReg.BroadcastJsonMessage(pg.ID, "lost_player", pg.CurrentPlayer)
		}
	}
}

func (pg *PacotGame) HandleLiar() {
	found := 0
	for _, dr := range pg.DiceRolls {
		for _, num := range dr.Roll {
			if num == 1 || num == pg.CurrentBid.Bid[1] {
				found++
			}
		}
	}

	var loserIdx int
	if found >= pg.CurrentBid.Bid[0] {
		loserIdx = pg.GetPlayerNameIndex(pg.CurrentPlayer)
	} else {
		loserIdx = pg.GetPlayerNameIndex(pg.CurrentBid.PlayerName)
	}

	pg.Players[loserIdx].NDice--
	pg.CurrentPlayer = pg.Players[loserIdx].PlayerName
	WsGamePlayerReg.BroadcastJsonMessage(pg.ID, "lost_dice", pg.Players[loserIdx])

	if pg.Players[loserIdx].NDice == 0 {
		pg.NActivePlayers--
		WsGamePlayerReg.BroadcastJsonMessage(pg.ID, "lost_player", pg.Players[loserIdx].PlayerName)
	}

}

func (pg *PacotGame) EndTurnAndStartNewOne() {

	WsGamePlayerReg.BroadcastJsonMessage(pg.ID, "turn_end", pg.DiceRolls)

	isOver, winner := pg.CheckWinCondition()

	if isOver {
		pg.EndGame(winner)
		return
	}

	pg.State = "turnStart"
	pg.CurrentBid = CurrBid{PlayerName: "", Bid: [2]int{0, 0}}
	WsGamePlayerReg.BroadcastJsonMessage(pg.ID, "st_change", pg.GetPubPacotGame())
	pg.DiceRolls = make([]DiceRoll, 0)
}

func (pg *PacotGame) EndGame(msg string) {
	// Notify all players that the game has ended
	WsGamePlayerReg.BroadcastJsonMessage(pg.ID, "game_end", msg)

	// Remove the game from the registry
	// Assuming there's a global or accessible instance of PacotGameRegistry
	PacotGameReg.DeleteGame(pg.ID) // Adjust as necessary to fit your architecture
}

func (pg *PacotGame) CheckWinCondition() (bool, string) {
	var winner string
	activePlayers := 0

	// Iterate through the players to count how many have dice left
	for _, player := range pg.Players {
		if player.NDice > 0 {
			activePlayers++
			winner = player.PlayerName // Update the winner as the last found player with dice
		}

		// If more than one player still has dice, the game isn't over
		if activePlayers > 1 {
			return false, ""
		}
	}

	// If only one player has dice, the game is over, and that player is the winner
	if activePlayers == 1 {
		return true, winner
	}

	// If no players have dice left, which shouldn't happen in a typical game,
	// this condition could be handled differently, depending on your game rules.
	// For now, returning false, "" implies no winner, which could indicate a draw or an error state.
	return true, "ERROR - No winner"
}

// GAME REGISTRY IN MEMORY
type PacotGameRegistry struct {
	Games map[string]*PacotGame
	mu    sync.RWMutex
}

func NewPacotGameRegistry() *PacotGameRegistry {
	return &PacotGameRegistry{
		Games: make(map[string]*PacotGame),
	}
}

func (pgr *PacotGameRegistry) AddPacotGame(gameID string, playerNames []string) (*PacotGame, error) {
	pgr.mu.Lock()
	defer pgr.mu.Unlock()

	// Check if the game already exists
	if _, exists := pgr.Games[gameID]; exists {
		return nil, errors.New("Game with the same ID already exists")
	}

	// Create a new PacotGame with players and initial empty dice rolls

	players := make([]PacotPlayer, len(playerNames))
	// Initialize each player with 5 dice
	for i, playerName := range playerNames {
		players[i] = PacotPlayer{
			PlayerName: playerName,
			NDice:      5,
		}
	}

	// Pick a random current player from playerNames to start the game
	currPlayer := playerNames[rand.Intn(len(playerNames))]

	// init the game
	pgr.Games[gameID] = &PacotGame{
		ID:             gameID,
		Players:        players,
		NPlayers:       int(len(playerNames)),
		NActivePlayers: int(len(playerNames)),
		State:          "waiting", // Set the initial state as needed
		DiceRolls:      make([]DiceRoll, 0),
		CurrentPlayer:  currPlayer,
		CurrentBid:     CurrBid{PlayerName: "", Bid: [2]int{0, 0}},
	}

	return pgr.Games[gameID], nil
}

// DeleteGame removes a game from the registry by its ID
func (pgr *PacotGameRegistry) DeleteGame(gameID string) {
	pgr.mu.Lock() // Ensure thread safety with a write lock
	defer pgr.mu.Unlock()

	// Check if the game exists before trying to delete
	if _, exists := pgr.Games[gameID]; exists {
		delete(pgr.Games, gameID) // Remove the game from the map
		fmt.Printf("Game %s has been removed from the registry.\n", gameID)
	} else {
		fmt.Printf("Game %s not found in the registry, cannot remove.\n", gameID)
	}
}

// WS CONNECTION REGISTRY

type PlayerConn struct {
	PlayerName string
	Conn       *websocket.Conn
}

type GamePlayerRegistry struct {
	GamePlayers map[string][]PlayerConn
	mu          sync.RWMutex // A mutex to ensure concurrent access to the map is safe
}

// Singleton Please
func NewWsGamePlayerRegistry() *GamePlayerRegistry {
	return &GamePlayerRegistry{
		GamePlayers: make(map[string][]PlayerConn),
	}
}

func (gpr *GamePlayerRegistry) NewGame(gameID string) {
	gpr.mu.Lock()
	defer gpr.mu.Unlock()

	// Create a new game with no players and empty connections
	gpr.GamePlayers[gameID] = []PlayerConn{}
}

func (gpr *GamePlayerRegistry) DeleteGame(gameID string) {
	gpr.mu.Lock()
	defer gpr.mu.Unlock()

	// Delete the game entry, which also removes all players associated with the game
	delete(gpr.GamePlayers, gameID)
}

func (gpr *GamePlayerRegistry) AddPlayerToGame(gameID string, player PlayerConn) error {
	gpr.mu.Lock()
	defer gpr.mu.Unlock()
	players, exists := gpr.GamePlayers[gameID]
	if !exists {
		return errors.New("Game Does Not Exist")
	}

	// update connection
	for _, p := range players {
		if p.PlayerName == player.PlayerName {
			p.Conn.Close()
			p.Conn = player.Conn
			return nil
		}
	}
	// Add player to the specified game
	gpr.GamePlayers[gameID] = append(gpr.GamePlayers[gameID], player)
	return nil
}

func (gpr *GamePlayerRegistry) GetPlayersInGame(gameID string) []PlayerConn {
	gpr.mu.RLock()
	defer gpr.mu.RUnlock()

	return gpr.GamePlayers[gameID]
}

func (gpr *GamePlayerRegistry) RemovePlayerFromGame(gameID string, playerName string) {
	gpr.mu.Lock()
	defer gpr.mu.Unlock()
	fmt.Println("USER REMOVED FROM GAME WS: ", gameID, playerName)
	// Get the slice of players for the specified game
	players := gpr.GamePlayers[gameID]

	// Find the index of the player to remove
	indexToRemove := -1
	for i, p := range players {
		if p.PlayerName == playerName {
			indexToRemove = i
			break
		}
	}

	// If the player was found, remove them from the slice
	if indexToRemove != -1 {
		// Remove the player from the slice by swapping with the last player
		players[indexToRemove] = players[len(players)-1]
		players = players[:len(players)-1]

		// Update the player slice for the game
		gpr.GamePlayers[gameID] = players
	}
}

func (gpr *GamePlayerRegistry) BroadcastJsonMessage(gameId string, key string, data any) {
	gpr.mu.RLock() // Acquire a read lock
	defer gpr.mu.RUnlock()
	// Let's also construct a list of Lobby user IDs to send out
	game, exists := gpr.GamePlayers[gameId]

	if !exists {
		return
	}
	payload := struct {
		Key  string `json:"k"`
		Data any    `json:"d"`
	}{
		Key:  key,
		Data: data,
	}

	fmt.Println("PAYLOAD BROADCAST PLAYERS:")
	fmt.Println(payload)
	for _, player := range game {
		//in the client the payload shows empty
		if err := player.Conn.WriteJSON(payload); err != nil {
			log.Println("Error broadcasting user list:", err)
		}
	}
}

func (gpr *GamePlayerRegistry) BroadcastPlayers(gameId string) {
	gpr.mu.RLock() // Acquire a read lock
	defer gpr.mu.RUnlock()
	// Let's also construct a list of Lobby user IDs to send out
	game, exists := gpr.GamePlayers[gameId]

	if !exists {
		return
	}
	payload := struct {
		Key     string   `json:"k"`
		Players []string `json:"d"`
	}{
		Key:     "online_players",
		Players: make([]string, 0, len(game)),
	}

	for _, player := range game {
		payload.Players = append(payload.Players, player.PlayerName)
		// user.PlayerName
	}
	fmt.Println("PAYLOAD BROADCAST PLAYERS:")
	fmt.Println(payload)
	for _, player := range game {
		//in the client the payload shows empty
		if err := player.Conn.WriteJSON(payload); err != nil {
			log.Println("Error broadcasting user list:", err)
		}
	}
}
