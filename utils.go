package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/http"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

func GenShortID(length int) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// func WriteJSON(w http.ResponseWriter, data any) {
// 	jsonData, err := json.Marshal(data)
// 	if err != nil {
// 		fmt.Println("Error converting data to JSON:", err)
// 		w.WriteHeader(http.StatusInternalServerError)
// 		w.Write([]byte(fmt.Sprintf("Error writing JSON %s", err.Error())))
// 		return
// 	}
// 	w.Header().Set("Content-Type", "application/json")
// 	w.Write(jsonData)
// }

func WriteJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK) // You may set the appropriate status code

	if err := json.NewEncoder(w).Encode(data); err != nil {
		fmt.Println("Error writing JSON:", err)
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(fmt.Sprintf("Error writing JSON: %s", err.Error())))
	}
}
