//go:build integration
// +build integration

package main

//go:generate make -C . backend-docker

import (
	"balance/tests/integration/mock/backend/server"
	"fmt"
	"net/http"
)

func main() {
	http.HandleFunc("/", server.Handler)
	if err := http.ListenAndServe("0.0.0.0"+":"+server.ListenPort, nil); err != nil {
		fmt.Println("failed running server", err)
	}
}
