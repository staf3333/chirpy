package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/staf3333/chirpy/internal/database"
)

func middlewareCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// create function that matches this signature
func readinessHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	// need to convert string into array of bytes
	w.Write([]byte("OK"))
}

type apiConfig struct {
	fileServerHits int
	chirpID int
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileServerHits += 1
		w.Header().Set("Cache-Control", "No-Cache")
		next.ServeHTTP(w, r)
	})
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

func (cfg *apiConfig) chirpValidationHandler(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	type requestBody struct {
		Body string `json:"body"`
	}

	type responseBody struct {
		Body string `json:"cleaned_body"`
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
		newChirp, err := db.CreateChirp(params.Body)
		if err != nil {
			log.Fatal("error creating chirp")
		}
		fmt.Println(newChirp)
		respondWithJSON(w, 200, responseBody{
			Body: cleanBody(params.Body),
		})
	}
}

func apiRoutes(cfg *apiConfig) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/healthz", readinessHandler)
	r.Get("/reset", cfg.resetHandler)
	r.Post("/chirps", cfg.chirpValidationHandler)
	return r
}
func adminRoutes(cfg *apiConfig) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/metrics", cfg.metricsHtmlHandler)
	return r
}
func main() {
	cfg := &apiConfig{}
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