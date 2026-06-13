package aiassist

import (
	"errors"
	"os"
	"strconv"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// ErrNoAPIKey is returned by NewHandlerFromEnv when ANTHROPIC_API_KEY is unset.
// The key is a secret sourced from GCP Secret Manager in deployed environments
// (docs/secrets.md) and must never be committed.
var ErrNoAPIKey = errors.New("aiassist: ANTHROPIC_API_KEY is not set")

// NewHandlerFromEnv builds a production Handler from the environment:
//   - ANTHROPIC_API_KEY (required) — secret, from env / Secret Manager.
//   - AIASSIST_SPEND_CAP_CALLS (optional) — integer workspace spend cap,
//     expressed as a max number of model calls; <= 0 or unset means no cap.
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
	if cap := spendCapFromEnv(); cap > 0 {
		opts.Spend = NewSpendCap(cap)
	}
	return NewHandler(client, opts), nil
}

func spendCapFromEnv() int {
	v := os.Getenv("AIASSIST_SPEND_CAP_CALLS")
	if v == "" {
		return 0
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0
	}
	return n
}
