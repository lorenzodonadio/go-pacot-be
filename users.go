package main

import (
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"
)

type User struct {
	PlayerName string    `json:"player_name"`
	UserName   string    `json:"user_name"`
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
}

type UserPublic struct {
	PlayerName string `json:"player"`
	UserName   string `json:"name"`
}

func (u *User) GetPubUser() UserPublic {
	return UserPublic{PlayerName: u.PlayerName, UserName: u.UserName}
}

type UserRegistry struct {
	Registry map[string]User
}

// Colors is an array containing tropical color names.
var colors = [13]string{"Red", "Blue", "Gold", "Pink", "Aqua", "Lime", "Tan", "Sky", "Sea", "Rose", "Sun", "Ocre", "Teal"}

// Adjectives is an array containing pirate-themed adjectives.
var adjectives = [14]string{"Rusty", "Groggy", "Scurvy", "Wonky", "Ragged", "Blimey", "Grubby", "Snarky", "Blind", "Weird", "Greedy", "Angry", "Crazy", "Drunk"}

// Plants is an array containing short tropical plant names.
var plants = [14]string{"Palm", "Fern", "Moss", "Aloe", "Bamboo", "Yucca", "Agave", "Orchid", "Taro", "Kelp", "Reed", "Cacti", "Coconut", "Pinapple"}

func genRandName() string {
	//TODO avoid collissions and implemente the max 2548 users
	i := rand.Intn(len(colors))
	j := rand.Intn(len(adjectives))
	w := rand.Intn(len(plants))
	return fmt.Sprintf("%v-%v-%v", colors[i], adjectives[j], plants[w])
}

func NewUserRegistry() *UserRegistry {
	return &UserRegistry{
		Registry: make(map[string]User),
	}
}

func GenNewUser(userName string) User {
	return User{
		PlayerName: genRandName(),
		UserName:   userName,
		ID:         "po_" + GenShortID(12),
		CreatedAt:  time.Now(),
	}
}

func (ur *UserRegistry) AddUser(userName string) *User {
	user := GenNewUser(userName)
	ur.Registry[user.ID] = user
	return &user
}

func (ur *UserRegistry) RemoveUserByID(id string) {
	delete(ur.Registry, id)
}

func (ur *UserRegistry) FindUserByID(id string) (*User, error) {
	if user, exists := ur.Registry[id]; exists {
		return &user, nil
	}
	return nil, errors.New("UserNotFound")
}

func (ur *UserRegistry) FindUserByPlayerName(playerName string) (*User, error) {

	for _, usr := range ur.Registry {
		if usr.PlayerName == playerName {
			return &usr, nil
		}
	}
	// if user, exists := ur.Registry[id]; exists {
	// 	return &user, nil
	// }

	return nil, errors.New("UserNotFound")
}

type UserLookupStatus int

const (
	NoAuthHeader UserLookupStatus = iota
	NoUserForAuthHeader
	UserFound
)

func (ur *UserRegistry) FindUserByBearerToken(r *http.Request) (*User, UserLookupStatus) {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return nil, NoAuthHeader
	}
	// Check if the Authorization header starts with "Bearer "
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, NoAuthHeader
	}
	// Extract the token from the header
	token := strings.TrimPrefix(authHeader, "Bearer ")

	user, err := ur.FindUserByID(token)
	if err != nil {
		return nil, NoUserForAuthHeader
	}

	return user, UserFound
}
