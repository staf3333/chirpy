package main

import (
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func (cfg *apiConfig) refreshHandler(w http.ResponseWriter, r *http.Request) {

	authHeader := r.Header.Get("Authorization")
	if len(authHeader) == 0 {
		log.Printf("unauthorized attempt to modify resources")
		w.WriteHeader(401)
		return
	}
	// get token string from authorization header
	tokenString := stripAuthHeaderPrefix(authHeader)

	type MyCustomClaims struct {
		jwt.RegisteredClaims
	}
	claims := &MyCustomClaims{}

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

	if issuerString != "chirpy-refresh" {
		log.Printf("wrong type of token")
		respondWithError(w, 401, "wrong type of token bruh")
		return
	}

	// check if refresh token has been revoked by looking in the db
	isRevoked, err := cfg.db.GetRevokeToken(tokenString)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}
	if isRevoked {
		respondWithError(w, 401, "this token has been revoked, sorry")
		return
	}

	userIDStr, err := token.Claims.GetSubject()
	if err != nil {
		log.Printf("issue getting subject from token")
		respondWithError(w, 401, err.Error())
		return
	}

	// here is where I need to create a new access token to return
	accessTokenExpirationDuration := time.Duration(1) * time.Hour
	accessToken := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer: "chirpy-access",
		IssuedAt: jwt.NewNumericDate(time.Now()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenExpirationDuration)),
		Subject: userIDStr,
	})

	accessTokenString, err := accessToken.SignedString([]byte(cfg.jwtSecret))
	if err != nil {
		log.Fatal("error generating users jwt token")
	}

	respondWithJSON(w, 200, struct {
		Token string `json:"token"`
	}{
		Token: accessTokenString,
	})
}

func (cfg *apiConfig) revokeHandler(w http.ResponseWriter, r *http.Request) {

	authHeader := r.Header.Get("Authorization")
	if len(authHeader) == 0 {
		log.Printf("unauthorized attempt to modify resources")
		w.WriteHeader(401)
		return
	}
	// get token string from authorization header
	tokenString := stripAuthHeaderPrefix(authHeader)

	type MyCustomClaims struct {
		jwt.RegisteredClaims
	}
	claims := &MyCustomClaims{}

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

	if issuerString != "chirpy-refresh" {
		log.Printf("wrong type of token")
		respondWithError(w, 401, "wrong type of token bruh")
		return
	}

	err = cfg.db.AddRevokeToken(tokenString, time.Now())
	if err != nil {
		respondWithError(w, 401, err.Error())
	}

	w.WriteHeader(200)
}