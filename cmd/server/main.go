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

	storage := infrastructure.NewLocalStorage(dataDir)
	uc := usecase.NewObjectUsecase(storage)
	h := handler.NewObjectHandler(uc)

	log.Printf("starting server on :8080, data_dir=%s", dataDir)
	if err := http.ListenAndServe(":8080", h); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
