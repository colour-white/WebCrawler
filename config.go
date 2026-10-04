package main

import (
	"encoding/json"
	"log"
	"os"
)

type config struct {
	StartURL              string   `json:"startURL"`
	Workers               int      `json:"workers"`
	PagesCountToProcess   int      `json:"pagesCountToProcess"`
	WorkerCooldown        int      `json:"workerCooldown"`
	MaxConcurrentRequests int      `json:"maxConcurrentRequests"`
	DB                    DBConfig `json:"db"`
}

type DBConfig struct {
	ConnectionString string `json:"connectionString"`
	Name             string `json:"name"`
	CollectionName   string `json:"collectionName"`
}

func loadConfig(configPath string) error {

	file, err := os.Open(configPath)

	if err != nil {
		log.Fatal(err.Error())
	}

	defer file.Close()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	err = decoder.Decode(&Config)
	return err

}
