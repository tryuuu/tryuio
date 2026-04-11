package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/tryuuu/tryuio/internal/handler"
	"github.com/tryuuu/tryuio/internal/infrastructure"
	"github.com/tryuuu/tryuio/internal/replication"
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

	var replicator *replication.Replicator
	if peersEnv := os.Getenv("PEERS"); peersEnv != "" {
		peers := strings.Split(peersEnv, ",")
		pm := replication.NewPeerManager(peers)
		replicator = replication.NewReplicator(pm, storage, apiKey)
		pm.Start(10 * time.Second)
		log.Printf("replication enabled, peers=%v", peers)
	}

	h := handler.NewObjectHandler(uc, apiKey, replicator)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("starting server on %s, data_dir=%s", addr, dataDir)
	if err := http.ListenAndServe(addr, h); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
