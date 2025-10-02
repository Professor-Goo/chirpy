package main

import (
	"net/http"
)

func main() {
	// Create a new ServeMux
	mux := http.NewServeMux()

	// Add fileserver handler for root path
	mux.Handle("/", http.FileServer(http.Dir(".")))

	// Create and configure the server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Start the server
	server.ListenAndServe()
}
