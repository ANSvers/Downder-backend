package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

// Config : keep all configuration variables in one struct
type Config struct {
	Port           string
	FrontendOrigin string
	StorageDir     string
}

// Read Variables from env + Create Storage Folder
func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("⚠️ No .env file found. Reading from system environment.")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default Port
	}

	origin := os.Getenv("FRONTEND_ORIGIN")
	if origin == "" {
		origin = "http://localhost:3000" // Default Frontend Origin
	}

	storageDir := os.Getenv("STORAGE_DIR")
	if storageDir == "" {
		storageDir = "./tmp/downloads"
	}

	// Create Storage Directory if it doesn't exist
	err = os.MkdirAll(storageDir, os.ModePerm)
	if err != nil {
		log.Printf("Failed to create storage directory: %v\n", err)
	}

	return &Config{
		Port:           port,
		FrontendOrigin: origin,
		StorageDir:     storageDir,
	}
}
