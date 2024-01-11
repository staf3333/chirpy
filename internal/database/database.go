package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"
)
type Chirp struct {
	id int
	body string
}

type DB struct {
	path string
	mux *sync.RWMutex
}

type DBStructure struct {
	Chirps map[int]Chirp `json:"chirps"`
}


// NewDB creates a new database connection
// and creates the database file if it doesn't exist
func NewDB(path string) (*DB, error) {
	_, err := os.Stat("database.json")
	if errors.Is(err, os.ErrNotExist) {
		db := &DB{
			path: path,
		}
		db.ensureDB()
		return db, nil
	} else if err != nil {
		fmt.Println("Error loading file")
		return nil, err
	}
	// returns reference to new DB
	return &DB{path: path}, nil
}

// CreateChirp creates a new chirp and saves it to disk
func (db *DB) CreateChirp(body string) (Chirp, error) {
	db.mux.Lock()
	defer db.mux.Unlock()
	dbStruct, err := db.loadDB()
	if err != nil {
		fmt.Println("Error loading DBStructure for creating chirps")
		return Chirp{}, err
	}
	id := len(dbStruct.Chirps) + 1
	// newChirp := Chirp{
	// 	id: id,
	// 	body: body,
	// }
	return Chirp{
		id: id,
		body: body,
	}, nil
}

// GetChirps returns all chirps in the database
func (db *DB) GetChirps() ([]Chirp, error) {
	db.mux.RLock()
	defer db.mux.RUnlock()
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

// ensureDB creates a new database file if it doesn't exist
func (db *DB) ensureDB() error {
	err := os.WriteFile("database.json", []byte(""), 0666)
	if err != nil {
		fmt.Println("Error creating db file")
		return err
	}
	return nil
}

// loadDB reads the database file into memory
func (db *DB) loadDB() (DBStructure, error) {
	data, err := os.ReadFile(db.path)
	if err != nil {
		fmt.Println("Error reading bytes from db")
		return DBStructure{}, err
	}
	// now unmarshall json into DBStructure
	chirpDB := DBStructure{}
	err = json.Unmarshal(data, &chirpDB)
	if err != nil {
		fmt.Println("Error unmarshalling json")
		return DBStructure{}, err
	}
	return chirpDB, nil
}

// // writeDB writes the database file to disk
// func (db *DB) writeDB(dbStructure DBStructure) error 

