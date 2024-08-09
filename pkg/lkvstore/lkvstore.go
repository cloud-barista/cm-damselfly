// Local Key-Value Store based on sync.Map and file I/O
package lkvstore

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

var (
	lkvstore   sync.Map
	dbFilePath string
)

type Config struct {
	DbFilePath string
}

func Init(config Config) {
	if config.DbFilePath != "" {
		dbFilePath = config.DbFilePath
	} else {
		dbFilePath = ".lkvstore/lkvstore.db"
	}
}

// Save lkvstore to file
func SaveLkvStore() error {
	if dbFilePath == "" {
		return fmt.Errorf("db file path is not set")
	}

	file, err := os.Create(dbFilePath)
	if err != nil {
		return fmt.Errorf("failed to create db file: %w", err)
	}
	defer file.Close()

	tempMap := make(map[string]interface{})
	lkvstore.Range(func(key, value interface{}) bool {
		tempMap[key.(string)] = value
		return true
	})

	encoder := json.NewEncoder(file)
	if err := encoder.Encode(tempMap); err != nil {
		return fmt.Errorf("failed to encode map: %w", err)
	}

	return nil
}

// Load the info from file
func LoadLkvStore() error {
	if dbFilePath == "" {
		return fmt.Errorf("db file path is not set")
	}

	if _, err := os.Stat(dbFilePath); os.IsNotExist(err) {
		return fmt.Errorf("db file does not exist: %w", err)
	}

	file, err := os.Open(dbFilePath)
	if err != nil {
		return fmt.Errorf("failed to open db file: %w", err)
	}
	defer file.Close()

	var tempMap map[string]interface{}
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&tempMap); err != nil {
		return fmt.Errorf("failed to decode map: %w", err)
	}

	for key, value := range tempMap {
		lkvstore.Store(key, value)
	}

	return nil
}

// Get returns the value for a given key.
func Get(key string) (interface{}, bool) {
	return lkvstore.Load(key)
}

// GetWithPrefix returns the values for a given key prefix.
func GetWithPrefix(keyPrefix string) ([]interface{}, bool) {
	var results []interface{}
	var exists bool
	lkvstore.Range(func(key, value interface{}) bool {
		if strings.HasPrefix(key.(string), keyPrefix) {
			results = append(results, value)
			exists = true
		}
		return true
	})
	return results, exists
}

// Put the key-value pair.
func Put(key string, value interface{}) {
	lkvstore.Store(key, value)
}

// Delete the key-value pair for a given key.
func Delete(key string) {
	lkvstore.Delete(key)
}
