package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func HPostRollDice(w http.ResponseWriter, r *http.Request) {
	user, game, err := getUserAndPacotGame(r)

	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
	}

	if game.State != "turnStart" {
		http.Error(w, "Wait for the turn to start", http.StatusUnauthorized)
	}

	if len(game.DiceRolls) > int(game.NActivePlayers) {
		http.Error(w, "Max rolls reached", http.StatusBadRequest)
		return
	}

	var newRoll DiceRoll
	// Decode the JSON body into the requestData
	if err := json.NewDecoder(r.Body).Decode(&newRoll); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if user.PlayerName != newRoll.PlayerName {
		http.Error(w, "You Can Not Roll For Others", http.StatusUnauthorized)
		return
	}

	nDice := game.GetPlayerDiceNumber(newRoll.PlayerName)
	if len(newRoll.Roll) != nDice {
		http.Error(w, fmt.Sprintf("You Only Have %v Dice Left", nDice), http.StatusBadRequest)
		return
	}

	for _, roll := range game.DiceRolls {
		if roll.PlayerName == newRoll.PlayerName {
			http.Error(w, "You Already Rolled, Wait", http.StatusBadRequest)
			return
		}
	}

	game.DiceRolls = append(game.DiceRolls, newRoll)

	fmt.Println(game.DiceRolls)

	if len(game.DiceRolls) == int(game.NActivePlayers) {
		game.State = "bid"
		WsGamePlayerReg.BroadcastJsonMessage(game.ID, "st_change", game.GetPubPacotGame())
		// WsGamePlayerReg.BroadcastJsonMessage(gameId, "st_change", game.GetPubPacotGame())
	}

	// fmt.Println(user, gameId)
}

func HPostExacto(w http.ResponseWriter, r *http.Request) {
	user, game, err := getUserAndPacotGame(r)

	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if game.State != "bid" {
		http.Error(w, "Wait for Bids to start", http.StatusUnauthorized)
		return
	}

	if game.CurrentPlayer != user.PlayerName {
		http.Error(w, "You Cant do this to Others", http.StatusUnauthorized)
		return
	}

	game.HandleExacto()
	game.EndTurnAndStartNewOne()
}

func HPostLiar(w http.ResponseWriter, r *http.Request) {
	user, game, err := getUserAndPacotGame(r)

	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if game.State != "bid" {
		http.Error(w, "Wait for Bids to start", http.StatusUnauthorized)
		return
	}

	if game.CurrentPlayer != user.PlayerName {
		http.Error(w, "You Cant do this to Others", http.StatusUnauthorized)
		return
	}

	game.HandleLiar()
	game.EndTurnAndStartNewOne()

}
func HPostGameBid(w http.ResponseWriter, r *http.Request) {
	user, game, err := getUserAndPacotGame(r)

	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	if game.State != "bid" {
		http.Error(w, "Wait for the turn to start", http.StatusUnauthorized)
		return
	}

	// Parse body to get the 'name'
	var newBid CurrBid
	// Decode the JSON body into the requestData
	if err := json.NewDecoder(r.Body).Decode(&newBid); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if newBid.PlayerName != user.PlayerName {
		http.Error(w, "You Can Not Bid For Others", http.StatusUnauthorized)
		return
	}

	if game.CurrentPlayer != user.PlayerName {
		http.Error(w, "Wait Your Turn 👎", http.StatusUnauthorized)
		return
	}

	if !isBidValid(game.CurrentBid.Bid, newBid.Bid) {
		http.Error(w, "Invalid Bid", http.StatusBadRequest)
		return
	}

	if err := game.NewBid(newBid); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	WsGamePlayerReg.BroadcastJsonMessage(game.ID, "new_bid", game.GetPubPacotGame())
}

func HGameWS(w http.ResponseWriter, r *http.Request) {
	gameId := chi.URLParam(r, "gameId")
	userId := r.URL.Query().Get("userId")

	if userId == "" || gameId == "" {
		http.Error(w, "Missing query parameters", http.StatusBadRequest)
		return
	}

	game, exists := PacotGameReg.Games[gameId]
	// Now you have the 'userId' and can use it as needed
	if !exists {
		http.Error(w, "Game not found", http.StatusBadRequest)
		return
	}

	user, err := Users.FindUserByID(userId)
	// user, status := Users.FindUserByBearerToken(r)
	if err != nil {
		http.Error(w, "userId not found", http.StatusBadRequest)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer func() {
		conn.Close()
		WsGamePlayerReg.RemovePlayerFromGame(gameId, user.PlayerName)
		WsGamePlayerReg.BroadcastPlayers(gameId)
	}()

	// Add the user to the WS Reg
	WsGamePlayerReg.AddPlayerToGame(gameId, PlayerConn{PlayerName: user.PlayerName, Conn: conn})
	// Update everyone online
	WsGamePlayerReg.BroadcastPlayers(gameId)

	// Check if everyone join and update game state

	if len(WsGamePlayerReg.GetPlayersInGame(gameId)) == int(game.NPlayers) {
		game.State = "turnStart"
		WsGamePlayerReg.BroadcastJsonMessage(gameId, "st_change", game.GetPubPacotGame())
	}

	for {
		// Handle incoming messages if needed
		mt, p, err := conn.ReadMessage()
		if err != nil {
			log.Println(err)
			return
		}

		fmt.Println(mt, p)
		// for now just echo everything
		for _, playerConn := range WsGamePlayerReg.GetPlayersInGame(gameId) {
			if err := playerConn.Conn.WriteMessage(mt, p); err != nil {
				log.Println("Error broadcasting message:", err)
				// Handle the error: you might decide to remove the user or ignore the error.
			}
		}
		// TODO: Handle incoming messages if you need bidirectional communication.
	}
}

// HELPERS

// isBidValid checks the validity of a new bid against the current bid in a Liar's Dice game.
// It returns true if the new bid is valid, and false otherwise.
//
// Parameters:
//   - curr: The current bid in the format [AMOUNT, DICE NUMBER].
//   - new: The new bid in the format [AMOUNT, DICE NUMBER] to be validated against the current bid.
//
// Returns:
//   - true if the new bid is valid and can be made, otherwise false.
//
// Rules for Valid Bids:
//  1. If the dice numbers in the current bid and the new bid are equal, the new bid must have a higher amount to be valid.
//  2. If the current bid's dice number is 1 (PACOT), the new bid must have an amount greater than twice the current bid's amount.
//  3. If the new bid's dice number is 1 (PACOT), the new bid must have an amount greater than half of the current bid's amount.
//  4. If the dice numbers are different, the new bid's dice number must be greater than or equal to the current bid's dice number,
//     and the new bid's amount must be greater than or equal to the current bid's amount.
//
// Example:
//
//	curr := [3, 4]  // Current bid: 3 fours
//	new := [4, 5]   // New bid: 4 fives
//	result := isBidValid(curr, new)  // Result is true, the new bid is valid.
func isBidValid(curr [2]int, new [2]int) bool {

	if new[1] == curr[1] { //are the dice numbers in curr and new bid equal?
		return new[0] > curr[0]
	} else if curr[1] == 1 { //Is current bid PACOT
		return new[0] > curr[0]*2
	} else if new[1] == 1 { //Is new bid PACOT
		return 2*new[0] >= curr[0]
	} else {
		if new[1] < curr[1] {
			return false
		} else {
			return new[0] >= curr[0]
		}
	}
}

func getUserByBearerTokenAndGameId(r *http.Request) (*User, string, error) {
	user, status := Users.FindUserByBearerToken(r)

	if status != UserFound {
		return nil, "", errors.New("No Player Found")
	}
	gameId := chi.URLParam(r, "gameId")

	fmt.Println("TRY GAME ID: ", gameId)
	fmt.Println(r.URL)
	if gameId == "" {
		return nil, "", errors.New("No GameId Found")
	}

	return user, gameId, nil
}

func getUserAndPacotGame(r *http.Request) (*User, *PacotGame, error) {

	user, gameId, err := getUserByBearerTokenAndGameId(r)

	if err != nil {
		return nil, nil, err
	}

	game, exists := PacotGameReg.Games[gameId]

	if !exists {
		return nil, nil, errors.New("No Game Found")
	}

	return user, game, nil
}
