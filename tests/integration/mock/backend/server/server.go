package server

import (
	"fmt"
	"net/http"
	"os"
)

const ListenPort = "8080"

func getContainerID() (string, error) {
	if id := os.Getenv("HOSTNAME"); id != "" {
		return id, nil
	}
	return "", fmt.Errorf("container ID not found")
}

func Handler(w http.ResponseWriter, r *http.Request) {
	id, err := getContainerID()
	if err != nil {
		http.Error(w, fmt.Sprintf("Error: %v", err), http.StatusInternalServerError)
		return
	}

	response := id
	w.Write([]byte(response))
}
