package health

import "testing"

func TestCheck(t *testing.T) {
	got := Check()
	if got.Status != "ok" {
		t.Errorf("Check().Status = %q, want %q", got.Status, "ok")
	}
	if got.Service != "elamachan-backend" {
		t.Errorf("Check().Service = %q, want %q", got.Service, "elamachan-backend")
	}
}
