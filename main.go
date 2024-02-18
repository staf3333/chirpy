package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"github.com/staf3333/chirpy/internal/database"
)

// create function that matches this signature
func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	// need to convert string into array of bytes
	w.Write([]byte("OK"))
}

type apiConfig struct {
	fileServerHits int
	db *database.DB
	jwtSecret string
}

func outputMetricsHtml(w http.ResponseWriter, filename string, data interface{}) {
	t, err := template.ParseFiles(filename)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if err := t.Execute(w, data); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
}

func (cfg *apiConfig) metricsHtmlHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)

	hits := map[string]interface{}{
		"Hits": cfg.fileServerHits,
	}
	outputMetricsHtml(w, "static/metrics.html", hits)
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	// need to convert string into array of bytes
	cfg.fileServerHits = 0
}

func cleanBody(body string) string {
	badWords := map[string]struct{}{}
	badWords["kerfuffle"] = struct{}{}
	badWords["sharbert"] = struct{}{}
	badWords["fornax"] = struct{}{}
	words := strings.Split(body, " ")
	// range through words and check if word
	for i, word := range words {
		if _, ok := badWords[strings.ToLower(word)]; ok {
			words[i] = "****"
		}
	}
	return strings.Join(words, " ")
}

func respondWithError(w http.ResponseWriter, code int, msg string) error {
	return respondWithJSON(w, code, map[string]string{"error": msg})
}
	

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) error {
	response, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return err
	}
	w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
	w.Write(response)
	return nil
}

func chirpValidationHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	type requestBody struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	params := requestBody{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	if len(params.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")	
	} else {
		db, err := database.NewDB("database.json")
		if err != nil {
			log.Fatal("error loading db")
		}
		cleanBodyStr := cleanBody(params.Body)
		newChirp, err := db.CreateChirp(cleanBodyStr)
		if err != nil {
			fmt.Println("ran into error creating chirp")
			log.Fatal("error creating chirp")
		}
		respondWithJSON(w, 201, struct {
			Body string `json:"body"`
			ID int `json:"id"`
		}{
			Body: newChirp.Body,
			ID: newChirp.ID,
		})
	}
}

// create a function that handles get request (gets chirps and responds with some json)
func (cfg *apiConfig) chirpsGetHandler(w http.ResponseWriter, r *http.Request) {
	// don't need to do any checks, can just respond with some json
	chirps, err := cfg.db.GetChirps()
	if err != nil {
		fmt.Println("Error getting chirps from database")
	}
	err = respondWithJSON(w, 200, chirps)
	if err != nil {
		fmt.Println("Error responding to the client")
	}
}

func (cfg *apiConfig) chirpsWithIDHandler(w http.ResponseWriter, r *http.Request) {

	// get the ID from the url params (will be a string so need to cast to int)
	chirpID, err := strconv.Atoi(chi.URLParam(r, "id"))
	if err != nil {
		fmt.Println("Error converting ID to string")
	}

	// get all the chirps and find the one with the id you want
	chirps, err := cfg.db.GetChirps()
	if err != nil {
		fmt.Println("Error getting chirps from database")
	}
	for _, chirp := range chirps {
		if chirp.ID == chirpID {
			err = respondWithJSON(w, 200, chirp)
			if err != nil {
				fmt.Println("found chirp but trouble responding")
			}
			return
		}
	}

	// if chirp not found, respond with error
	respondWithError(w, 404, "")

}

func chirpsRoutes(cfg *apiConfig) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/", chirpValidationHandler)
	r.Get("/", cfg.chirpsGetHandler)
	r.Get("/{id}", cfg.chirpsWithIDHandler)
	return r
}

func usersRoutes(cfg *apiConfig) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/", cfg.userCreateHandler)
	r.Put("/", cfg.userUpdateHandler)
	return r
}

func apiRoutes(cfg *apiConfig) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/healthz", readinessHandler)
	r.Get("/reset", cfg.resetHandler)
	r.Post("/login", cfg.loginHandler)
	r.Post("/refresh", cfg.refreshHandler)
	r.Post("/revoke", cfg.revokeHandler)
	r.Mount("/chirps", chirpsRoutes(cfg))
	r.Mount("/users", usersRoutes(cfg))
	return r
}

func adminRoutes(cfg *apiConfig) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/metrics", cfg.metricsHtmlHandler)
	return r
}

func main() {
	db, err := database.NewDB("database.json")
	if err != nil {
		log.Fatal("error loading DB")
	}
	err = godotenv.Load()
	jwtSecret := os.Getenv("JWT_SECRET")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	cfg := &apiConfig{
		db: db,
		jwtSecret: jwtSecret,
	}
	r := chi.NewRouter()
	// mux := http.NewServeMux()
	fsHandler := cfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir("."))))
	r.Handle("/app/*", fsHandler)
	r.Handle("/app", fsHandler)
	r.Mount("/api", apiRoutes(cfg))
	r.Mount("/admin", adminRoutes(cfg))
	corsR := middlewareCors(r)
	s := &http.Server{
		Addr:           ":8080",
		Handler:        corsR,
	}
	log.Fatal(s.ListenAndServe())
}