package aiassist

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ErrNoAPIKey is returned by NewHandlerFromEnv when ANTHROPIC_API_KEY is unset.
// The key is a secret sourced from GCP Secret Manager in deployed environments
// (docs/secrets.md) and must never be committed.
var ErrNoAPIKey = errors.New("aiassist: ANTHROPIC_API_KEY is not set")

// NewHandlerFromEnv builds a production Handler from the environment:
//   - ANTHROPIC_API_KEY (required) — secret, from env / Secret Manager.
//   - AIASSIST_SPEND_CAP_CALLS (optional) — workspace spend cap, expressed as a
//     max number of billable model calls per window; <= 0 or unset means no cap.
//   - AIASSIST_SPEND_CAP_WINDOW_SECONDS (optional) — refill window for the cap;
//     <= 0 or unset uses the default (1 hour).
//
// The model id is intentionally NOT read from the environment: it is pinned to
// claude-haiku-4-5 (modelID) per VER-42.
func NewHandlerFromEnv() (*Handler, error) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		return nil, ErrNoAPIKey
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	opts := Options{}
	if cap := envInt("AIASSIST_SPEND_CAP_CALLS"); cap > 0 {
		window := time.Duration(envInt("AIASSIST_SPEND_CAP_WINDOW_SECONDS")) * time.Second
		opts.Spend = NewSpendCap(cap, window)
	}
	return NewHandler(client, opts), nil
}

func envInt(key string) int {
	v := os.Getenv(key)
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}
