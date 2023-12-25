package main

import (
	"log"
	"net/http"
	"strconv"
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
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) fileServerHitsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	// need to convert string into array of bytes
	hits := strconv.Itoa(cfg.fileServerHits)
	w.Write([]byte("Hits: " + hits))
}

func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	// need to convert string into array of bytes
	cfg.fileServerHits = 0
	w.Write([]byte("Reset hits to 0!"))
}


func main() {
	cfg := &apiConfig{}
	mux := http.NewServeMux()
	fileServerWithMetrics := cfg.middlewareMetricsInc(http.FileServer(http.Dir(".")))
	mux.Handle("/app/", http.StripPrefix("/app/", fileServerWithMetrics))
	mux.HandleFunc("/healthz", readinessHandler)
	mux.HandleFunc("/metrics", cfg.fileServerHitsHandler)
	mux.HandleFunc("/reset", cfg.resetHandler)
	corsMux := middlewareCors(mux)
	s := &http.Server{
		Addr:           ":8080",
		Handler:        corsMux,
	}
	log.Fatal(s.ListenAndServe())
}