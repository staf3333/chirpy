package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

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

func (cfg *apiConfig) loginHandler(w http.ResponseWriter, r *http.Request) {

	defer r.Body.Close()
	type requestBody struct {
		Email string `json:"email"`
		Password string `json:"password"`
		Expiration *int `json:"expires_in_seconds,omitempty"`
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

	// var expirationDuration time.Duration
	// if params.Expiration == nil || *params.Expiration > 86400 {
	// 	expirationDuration = time.Duration(86400) * time.Second
	// } else {
	// 	expirationDuration = time.Duration(*params.Expiration) * time.Second
	// }

	accessTokenExpirationDuration := time.Duration(1) * time.Hour
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer: "chirpy-access",
		IssuedAt: jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenExpirationDuration)),
		Subject: strconv.Itoa(user.ID),
	})

	accessTokenString, err := accessToken.SignedString([]byte(cfg.jwtSecret))
	if err != nil {
		log.Fatal("error generating users jwt token")
	}

	refreshTokenExpirationDuration := time.Duration(24 * 60) * time.Hour
	refreshToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer: "chirpy-refresh",
		IssuedAt: jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(refreshTokenExpirationDuration)),
		Subject: strconv.Itoa(user.ID),
	})

	refreshTokenString, err := refreshToken.SignedString([]byte(cfg.jwtSecret))
	if err != nil {
		log.Fatal("error generating users jwt token")
	}

	respondWithJSON(w, 200, struct {
		Email string `json:"email"`
		ID int `json:"id"`
		Token string `json:"token"`
		RefreshToken string `json:"refresh_token"`
	}{
		Email: user.Email,
		ID: user.ID,
		Token: accessTokenString,
		RefreshToken: refreshTokenString, 
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

	// get token string from authorization header
	tokenString = stripAuthHeaderPrefix(authHeader)

	type MyCustomClaims struct {
		jwt.RegisteredClaims
	}
	claims := &MyCustomClaims{}

	// Keyfunc will be used by the Parse methods as a 
	// callback function to supply the key for verification. 
	// The function receives the parsed, but unverified Token. 
	// This allows you to use properties in the Header 
	// of the token (such as `kid`) to identify which key to use.
	keyFunc := func (token *jwt.Token) (interface{}, error) {
		return []byte(cfg.jwtSecret), nil
	}
	token, err := jwt.ParseWithClaims(tokenString, claims, keyFunc)
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

	issuerString, err := token.Claims.GetIssuer()
	if err != nil {
		log.Printf("issue getting issuer from token")
		respondWithError(w, 401, err.Error())
		return
	}

	if issuerString == "chirpy-refresh" {
		log.Printf("can't use a refresh token to modify user")
		respondWithError(w, 401, "can't use refresh token bruh")
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