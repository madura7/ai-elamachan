package storage

import "testing"

func TestExtForContentType(t *testing.T) {
	cases := map[string]string{
		"image/jpeg": "jpg",
		"image/png":  "png",
		"image/webp": "webp",
		"image/gif":  "",
		"text/plain": "",
		"":           "",
	}
	for ct, want := range cases {
		if got := ExtForContentType(ct); got != want {
			t.Errorf("ExtForContentType(%q) = %q, want %q", ct, got, want)
		}
	}
}

func TestParseSizeEnv(t *testing.T) {
	const def int64 = 8 * 1024 * 1024
	cases := []struct {
		in   string
		want int64
	}{
		{"", def},
		{"abc", def},
		{"0", def},
		{"-5", def},
		{"1048576", 1048576},
	}
	for _, c := range cases {
		if got := ParseSizeEnv(c.in, def); got != c.want {
			t.Errorf("ParseSizeEnv(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestNewFromEnv_DisabledWhenUnset(t *testing.T) {
	t.Setenv("BLOB_ENDPOINT", "")
	store, err := NewFromEnv()
	if err != nil {
		t.Fatalf("expected nil error when BLOB_ENDPOINT unset, got %v", err)
	}
	if store != nil {
		t.Fatalf("expected nil store when BLOB_ENDPOINT unset")
	}
}

func TestNewFromEnv_ErrorsOnPartialConfig(t *testing.T) {
	t.Setenv("BLOB_ENDPOINT", "https://acct.r2.cloudflarestorage.com")
	t.Setenv("BLOB_BUCKET", "") // missing
	if _, err := NewFromEnv(); err == nil {
		t.Fatalf("expected error when BLOB_BUCKET missing")
	}
}
