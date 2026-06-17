package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "tryuio_objects_total",
			Help: "Total number of objects stored locally",
		},
		func() float64 {
			objects, err := storage.List()
			if err != nil {
				return 0
			}
			return float64(len(objects))
		},
	))

	var replicator *replication.Replicator
	if peersEnv := os.Getenv("PEERS"); peersEnv != "" {
		peers := strings.Split(peersEnv, ",")
		pm := replication.NewPeerManager(peers)
		replicator = replication.NewReplicator(pm, storage, apiKey)
		pm.Start(10 * time.Second)
		log.Printf("replication enabled, peers=%v", peers)
	}

	h := handler.NewObjectHandler(uc, apiKey, replicator)

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.Handle("/", h)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	addr := ":" + port
	log.Printf("starting server on %s, data_dir=%s", addr, dataDir)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
