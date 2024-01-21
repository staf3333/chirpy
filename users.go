package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
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