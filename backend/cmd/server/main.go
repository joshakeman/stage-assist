package main

import (
	"log"
	"net/http"

	"github.com/joshakeman/stage-assist/backend/internal/api"
)

func main() {
	mux := api.NewMux()
	log.Println("listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
