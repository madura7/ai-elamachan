// Package health provides a minimal liveness signal for the ElaMachan backend.
// It is intentionally dependency-free so the skeleton builds and tests on a bare repo;
// real readiness checks (Postgres, Redis, Meilisearch) are added with the API service.
package health

// Status is the canonical health response payload.
type Status struct {
	Status  string `json:"status"`
	Service string `json:"service"`
}

// Check returns the current liveness status of the service.
func Check() Status {
	return Status{Status: "ok", Service: "elamachan-backend"}
}
