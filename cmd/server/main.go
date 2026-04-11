package main

import (
	"log"
	"net/http"
	"os"

	"github.com/tryuuu/tryuio/internal/handler"
	"github.com/tryuuu/tryuio/internal/infrastructure"
	"github.com/tryuuu/tryuio/internal/usecase"
)

func main() {
	dataDir := os.Getenv("DATA_DIR")
	if dataDir == "" {
		dataDir = "/data"
	}

	apiKey := os.Getenv("API_KEY")
	if apiKey == "" {
		log.Fatal("API_KEY is required")
	}

	storage := infrastructure.NewLocalStorage(dataDir)
	uc := usecase.NewObjectUsecase(storage)
	h := handler.NewObjectHandler(uc, apiKey)

	log.Printf("starting server on :8080, data_dir=%s", dataDir)
	if err := http.ListenAndServe(":8080", h); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
