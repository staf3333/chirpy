package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)
type Chirp struct {
	ID int `json:"id"`
	Body string `json:"body"`
}

type DB struct {
	path string
	mux *sync.RWMutex
}

type DBStructure struct {
	Chirps map[int]Chirp `json:"chirps"`
	Users map[int]User `json:"users"`
}

type User struct {
	ID int `json:"id"`
	Email string `json:"email"`
}

// NewDB creates a new database connection
// and creates the database file if it doesn't exist
func NewDB(path string) (*DB, error) {
	db := &DB{
		path: path,
		mux:   &sync.RWMutex{},
	}
	err := db.ensureDB()
	return db, err
}

func (db *DB) CreateUser(email string) (User, error) {

	dbStruct, err := db.loadDB()
	if err != nil {
		fmt.Println("Error loading DBStructure for creating users")
		return User{}, err
	}

	id := len(dbStruct.Users) + 1
	newUser := User{
		ID: id,
		Email: email,
	}
	dbStruct.Users[id] = newUser
	err = db.writeDB(dbStruct)
	if err != nil {
		fmt.Println("Error saving user to database")
		return User{}, err
	}
	return newUser, nil

}

// CreateChirp creates a new chirp and saves it to disk
func (db *DB) CreateChirp(body string) (Chirp, error) {
	dbStruct, err := db.loadDB()
	if err != nil {
		fmt.Println("Error loading DBStructure for creating chirps")
		return Chirp{}, err
	}
	id := len(dbStruct.Chirps) + 1
	newChirp := Chirp{
		ID: id,
		Body: body,
	}
	dbStruct.Chirps[id] = newChirp
	err = db.writeDB(dbStruct)
	if err != nil {
		return Chirp{}, err
	}
	return newChirp, nil
}

// GetChirps returns all chirps in the database
func (db *DB) GetChirps() ([]Chirp, error) {
	fmt.Println("Attempting to load struct")
	dbStruct, err := db.loadDB()
	if err != nil {
		fmt.Println("Error loading DBStructure for getting Chirps")
		return nil, err
	}
	var chirps []Chirp
	for _, v := range dbStruct.Chirps {
		chirps = append(chirps, v)
	}
	return chirps, nil
}
func (db *DB) createDB() error {
	dbStructure := DBStructure{
		Chirps: map[int]Chirp{},
		Users: map[int]User{},
	}
	return db.writeDB(dbStructure)
}

// ensureDB creates a new database file if it doesn't exist
func (db *DB) ensureDB() error {
	_, err := os.ReadFile(db.path)
	if errors.Is(err, os.ErrNotExist) {
		return db.createDB()
	}
	return err
}

// loadDB reads the database file into memory
func (db *DB) loadDB() (DBStructure, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()

	chirpDB := DBStructure{}
	data, err := os.ReadFile(db.path)
	if errors.Is(err, os.ErrNotExist) {
		return chirpDB, err
	}
	// now unmarshall json into DBStructure
	err = json.Unmarshal(data, &chirpDB)
	if err != nil {
		fmt.Println("Error unmarshalling json")
		return chirpDB, err
	}
	return chirpDB, nil
}

// // writeDB writes the database file to disk
func (db *DB) writeDB(dbStructure DBStructure) error {
	db.mux.Lock()
	defer db.mux.Unlock()

	dat, err := json.Marshal(dbStructure)
	if err != nil {
		return err
	}

	err = os.WriteFile(db.path, dat, 0600)
	if err != nil {
		return err
	}
	return nil
}

