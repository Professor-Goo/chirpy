package main

import (
	"net/http"
)

func main() {
	// Create a new ServeMux
	mux := http.NewServeMux()

	// Add readiness endpoint
	mux.HandleFunc("/healthz", handlerReadiness)

	// Add fileserver handler with /app prefix
	mux.Handle("/app/", http.StripPrefix("/app", http.FileServer(http.Dir("."))))

	// Create and configure the server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Start the server
	server.ListenAndServe()
}

// handlerReadiness is a readiness endpoint handler
func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}
