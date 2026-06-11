// Command api is the ElaMachan backend HTTP service entrypoint.
// This skeleton exposes a single /healthz endpoint; routing, persistence, auth,
// search, and the Claude-assisted listing flow are layered on in follow-up issues.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/madura7/ai-elamachan/backend/internal/health"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(health.Check())
	})

	port := os.Getenv("BACKEND_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("elamachan-backend listening on :%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
