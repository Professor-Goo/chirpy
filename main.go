package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"

	"time"

	"github.com/Professor-Goo/chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
}

type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

type Chirp struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    uuid.UUID `json:"user_id"`
}

func main() {
	// Load environment variables
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")

	// Open database connection
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal("Error opening database:", err)
	}
	dbQueries := database.New(db)

	// Create API config to hold state
	apiCfg := &apiConfig{
		db:       dbQueries,
		platform: os.Getenv("PLATFORM"),
	}

	// Create a new ServeMux
	mux := http.NewServeMux()

	// API endpoints (under /api namespace)
	mux.HandleFunc("GET /api/healthz", handlerReadiness)
	mux.HandleFunc("POST /api/users", apiCfg.handlerCreateUser)
	mux.HandleFunc("GET /api/chirps", apiCfg.handlerGetAllChirps)
	mux.HandleFunc("POST /api/chirps", apiCfg.handlerCreateChirp)
	mux.HandleFunc("GET /api/chirps/{chirpID}", apiCfg.handlerGetChirp)

	// Admin endpoints (under /admin namespace)
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerMetrics)
	mux.HandleFunc("POST /admin/reset", apiCfg.handlerReset)

	// Fileserver for frontend (under /app namespace)
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))

	// Create and configure the server
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Start the server
	server.ListenAndServe()
}

// middlewareMetricsInc increments the fileserver hits counter
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// handlerMetrics returns an HTML page with the hit count
func (cfg *apiConfig) handlerMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	html := fmt.Sprintf(`<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`, cfg.fileserverHits.Load())
	w.Write([]byte(html))
}

// handlerReset resets the fileserver hits counter to 0 and deletes all users
func (cfg *apiConfig) handlerReset(w http.ResponseWriter, r *http.Request) {
	if cfg.platform != "dev" {
		respondWithError(w, 403, "Forbidden")
		return
	}

	cfg.fileserverHits.Store(0)
	err := cfg.db.DeleteAllUsers(r.Context())
	if err != nil {
		log.Printf("Error deleting users: %s", err)
		respondWithError(w, 500, "Something went wrong")
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
}

// handlerReadiness is a readiness endpoint handler
func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

// cleanProfanity replaces profane words with ****
func cleanProfanity(text string) string {
	profaneWords := map[string]bool{
		"kerfuffle": true,
		"sharbert":  true,
		"fornax":    true,
	}

	words := strings.Split(text, " ")
	for i, word := range words {
		if profaneWords[strings.ToLower(word)] {
			words[i] = "****"
		}
	}
	return strings.Join(words, " ")
}

// respondWithError sends a JSON error response
func respondWithError(w http.ResponseWriter, code int, msg string) {
	type errorResponse struct {
		Error string `json:"error"`
	}
	respondWithJSON(w, code, errorResponse{Error: msg})
}

// respondWithJSON sends a JSON response
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

// handlerCreateUser creates a new user
func (cfg *apiConfig) handlerCreateUser(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Email string `json:"email"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		respondWithError(w, 500, "Something went wrong")
		return
	}

	dbUser, err := cfg.db.CreateUser(r.Context(), params.Email)
	if err != nil {
		log.Printf("Error creating user: %s", err)
		respondWithError(w, 500, "Something went wrong")
		return
	}

	user := User{
		ID:        dbUser.ID,
		CreatedAt: dbUser.CreatedAt,
		UpdatedAt: dbUser.UpdatedAt,
		Email:     dbUser.Email,
	}

	respondWithJSON(w, 201, user)
}

// handlerCreateChirp creates a new chirp
func (cfg *apiConfig) handlerCreateChirp(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body   string    `json:"body"`
		UserID uuid.UUID `json:"user_id"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding parameters: %s", err)
		respondWithError(w, 500, "Something went wrong")
		return
	}

	const maxChirpLength = 140
	if len(params.Body) > maxChirpLength {
		respondWithError(w, 400, "Chirp is too long")
		return
	}

	cleanedBody := cleanProfanity(params.Body)

	dbChirp, err := cfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
		Body:   cleanedBody,
		UserID: params.UserID,
	})
	if err != nil {
		log.Printf("Error creating chirp: %s", err)
		respondWithError(w, 500, "Something went wrong")
		return
	}

	chirp := Chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
	}

	respondWithJSON(w, 201, chirp)
}

// handlerGetAllChirps returns all chirps
func (cfg *apiConfig) handlerGetAllChirps(w http.ResponseWriter, r *http.Request) {
	dbChirps, err := cfg.db.GetAllChirps(r.Context())
	if err != nil {
		log.Printf("Error getting chirps: %s", err)
		respondWithError(w, 500, "Something went wrong")
		return
	}

	chirps := make([]Chirp, len(dbChirps))
	for i, dbChirp := range dbChirps {
		chirps[i] = Chirp{
			ID:        dbChirp.ID,
			CreatedAt: dbChirp.CreatedAt,
			UpdatedAt: dbChirp.UpdatedAt,
			Body:      dbChirp.Body,
			UserID:    dbChirp.UserID,
		}
	}

	respondWithJSON(w, 200, chirps)
}

// handlerGetChirp returns a single chirp by ID
func (cfg *apiConfig) handlerGetChirp(w http.ResponseWriter, r *http.Request) {
	chirpIDStr := r.PathValue("chirpID")
	chirpID, err := uuid.Parse(chirpIDStr)
	if err != nil {
		respondWithError(w, 404, "Not found")
		return
	}

	dbChirp, err := cfg.db.GetChirpByID(r.Context(), chirpID)
	if err != nil {
		respondWithError(w, 404, "Not found")
		return
	}

	chirp := Chirp{
		ID:        dbChirp.ID,
		CreatedAt: dbChirp.CreatedAt,
		UpdatedAt: dbChirp.UpdatedAt,
		Body:      dbChirp.Body,
		UserID:    dbChirp.UserID,
	}

	respondWithJSON(w, 200, chirp)
}
