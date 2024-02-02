package main

import (
	"encoding/json"
	"fmt"
	"net/http"
)

func HPostLogin(w http.ResponseWriter, r *http.Request) {
	fmt.Println(r.RemoteAddr)

	user, status := Users.FindUserByBearerToken(r)
	fmt.Println(status)
	if status == UserFound {
		WriteJSON(w, user)
		return
	}
	// Parse body to get the 'name'
	var requestData struct {
		Name string `json:"name"`
	}

	// Decode the JSON body into the requestData
	err := json.NewDecoder(r.Body).Decode(&requestData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user = Users.AddUser(requestData.Name)
	WriteJSON(w, user)
}
