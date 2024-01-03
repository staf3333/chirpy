package main

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
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

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type response struct {
		Body string `json:"error"`
	}
	respBody := response{
		Body: msg,
	}
	dat, err := json.Marshal(respBody)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
	w.Write(dat)
}
	

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	type response struct {
		Body interface{} `json:"cleaned_body"`
	}
	respBody := response{
		Body: payload,
	}
	dat, err := json.Marshal(respBody)
	if err != nil {
		log.Printf("Error marshalling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
	w.Write(dat)
}

func chirpValidationHandler(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	if len(params.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")	
	} else {
		respondWithJSON(w, 200, cleanBody(params.Body))
	}
}

func apiRoutes(cfg *apiConfig) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/healthz", readinessHandler)
	r.Post("/validate_chirp", chirpValidationHandler)
	r.Get("/reset", cfg.resetHandler)
	return r
}
func adminRoutes(cfg *apiConfig) *chi.Mux {
	r := chi.NewRouter()
	// fs := http.FileServer(http.Dir("./static/metrics"))
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