package listings

import "testing"

func TestBlobObjectKey(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		wantKey string
		wantOK  bool
	}{
		{
			name:    "valid public blob url",
			url:     "https://zufjds2j6eb21ij3.public.blob.vercel-storage.com/listings/abc/photo-x1y2.png",
			wantKey: "listings/abc/photo-x1y2.png",
			wantOK:  true,
		},
		{
			name:   "http scheme rejected",
			url:    "http://zufjds2j6eb21ij3.public.blob.vercel-storage.com/listings/abc/photo.png",
			wantOK: false,
		},
		{
			name:   "foreign host rejected",
			url:    "https://evil.example.com/listings/abc/photo.png",
			wantOK: false,
		},
		{
			name:   "lookalike host suffix rejected",
			url:    "https://public.blob.vercel-storage.com.evil.com/x.png",
			wantOK: false,
		},
		{
			name:   "empty path rejected",
			url:    "https://zufjds2j6eb21ij3.public.blob.vercel-storage.com/",
			wantOK: false,
		},
		{
			name:   "garbage rejected",
			url:    "not a url",
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			key, ok := blobObjectKey(tc.url)
			if ok != tc.wantOK {
				t.Fatalf("blobObjectKey(%q) ok = %v, want %v", tc.url, ok, tc.wantOK)
			}
			if ok && key != tc.wantKey {
				t.Errorf("blobObjectKey(%q) key = %q, want %q", tc.url, key, tc.wantKey)
			}
		})
	}
}
