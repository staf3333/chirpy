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
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"github.com/joho/godotenv"
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
	db *database.DB
	jwtSecret string
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

func (cfg *apiConfig) userCreateHandler(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	type requestBody struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	params := requestBody{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding JSON: %s", err)
		w.WriteHeader(500)
		return
	}

	user, err := cfg.db.CreateUser(params.Email, params.Password)	
	if err != nil {
		// respondWithError(w, 500, "Error creating user in DB")
		respondWithError(w, 500, err.Error())
		return
	}
	respondWithJSON(w, 201, struct {
		Email string `json:"email"`
		ID int `json:"id"`
	}{
		Email: user.Email,
		ID: user.ID,
	})
}

func stripAuthHeaderPrefix(h string) string {
	return strings.Split(h, " ")[1]
}

func (cfg *apiConfig) userUpdateHandler(w http.ResponseWriter, r *http.Request) {
	
	defer r.Body.Close()
	type requestBody struct {
		Email string `json:"email"`
		Password string `json:"password"`
	}

	decoder := json.NewDecoder(r.Body)
	params := requestBody{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding JSON: %s", err)
		w.WriteHeader(500)
		return
	}

	authHeader := r.Header.Get("Authorization")
	var tokenString string
	if len(authHeader) == 0 {
		log.Printf("unauthorized attempt to modify resources")
		w.WriteHeader(401)
		return
	}
	tokenString = stripAuthHeaderPrefix(authHeader)

	type MyCustomClaims struct {
		jwt.RegisteredClaims
	}
	claims := &MyCustomClaims{}
	keyFunc := func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.jwtSecret), nil
	}
	token , err := jwt.ParseWithClaims(tokenString, claims, keyFunc)
	if err != nil {
		log.Printf("unauthorized attempt to modify resources, claims don't match")
		respondWithError(w, 401, err.Error())
		return
	}

	if !token.Valid {
		log.Printf("token has expired")
		respondWithError(w, 401, err.Error())
		return
	}
	
	userIDStr, err := token.Claims.GetSubject()
	if err != nil {
		log.Printf("issue getting subject from token")
	}
	userID, err := strconv.Atoi(userIDStr)
	if err != nil {
		log.Printf("Error converting id from string to int")
	}

	updatedUser, err := cfg.db.UpdateUser(userID, params.Email, params.Password)
	if err != nil {
		log.Printf("error updating user in the database")
		w.WriteHeader(500)
		return
	}

	respondWithJSON(w, 200, struct {
		Email string `json:"email"`
		ID int `json:"id"`
	}{
		Email: updatedUser.Email,
		ID: updatedUser.ID,
	})
}

func (cfg *apiConfig) loginHandler(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	type requestBody struct {
		Email string `json:"email"`
		Password string `json:"password"`
		Expiration *int `json:"expires_in_second,omitempty"`
	}

	decoder := json.NewDecoder(r.Body)
	params := requestBody{}
	err := decoder.Decode(&params)
	if err != nil {
		log.Printf("Error decoding JSON: %s", err)
		w.WriteHeader(500)
		return
	}

	user, err := cfg.db.LoginUser(params.Email, params.Password)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}

	var expirationDuration time.Duration
	if params.Expiration == nil || *params.Expiration > 86400 {
		expirationDuration = time.Duration(86400) * time.Second
	} else {
		expirationDuration = time.Duration(*params.Expiration) * time.Second
	}
	jwt := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer: "chirpy",
		IssuedAt: jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expirationDuration)),
		Subject: strconv.Itoa(user.ID),
	})

	tokenString, err := jwt.SignedString([]byte(cfg.jwtSecret))
	if err != nil {
		fmt.Println(err)
		log.Fatal("error generating users jwt token")
	}

	respondWithJSON(w, 200, struct {
		Email string `json:"email"`
		ID int `json:"id"`
		Token string `json:"token"`
	}{
		Email: user.Email,
		ID: user.ID,
		Token: tokenString, 
	})
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