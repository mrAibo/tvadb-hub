package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"0.2.0", "0.1.0", 1},
		{"1.0.0", "1.0.0", 0},
		{"1.2.3", "2.0.0", -1},
		{"v2.1.0", "2.0.9", 1},
	}
	for _, tt := range tests {
		got, err := compareVersions(tt.a, tt.b)
		if err != nil {
			t.Fatalf("compareVersions(%q, %q): %v", tt.a, tt.b, err)
		}
		if got != tt.want {
			t.Fatalf("compareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestCheckReportsNewerRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("User-Agent"); got == "" {
			t.Fatal("expected User-Agent header")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name":"v0.2.0","html_url":"https://example.test/release","name":"TVADB Hub 0.2.0","published_at":"2026-09-29T10:00:00Z"}`))
	}))
	defer server.Close()

	service := &Service{client: server.Client(), endpoint: server.URL}
	info, err := service.Check(context.Background())
	if err != nil {
		t.Fatalf("Check returned error: %v", err)
	}
	if !info.UpdateAvailable {
		t.Fatal("expected update to be available")
	}
	if info.LatestVersion != "0.2.0" {
		t.Fatalf("LatestVersion = %q, want 0.2.0", info.LatestVersion)
	}
	if info.ReleaseURL != "https://example.test/release" {
		t.Fatalf("ReleaseURL = %q", info.ReleaseURL)
	}
}
