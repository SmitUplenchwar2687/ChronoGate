package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	chronoclock "github.com/SmitUplenchwar2687/Chrono/pkg/clock"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	chronorecorder "github.com/SmitUplenchwar2687/Chrono/pkg/recorder"
	chronostorage "github.com/SmitUplenchwar2687/Chrono/pkg/storage"
)

func TestRateLimitedRoutesAcrossAlgorithms(t *testing.T) {
	algorithms := []limiter.Algorithm{
		limiter.AlgorithmTokenBucket,
		limiter.AlgorithmSlidingWindow,
		limiter.AlgorithmFixedWindow,
	}

	for _, algorithm := range algorithms {
		t.Run(string(algorithm), func(t *testing.T) {
			vc := chronoclock.NewVirtualClock(time.Date(2026, 2, 8, 10, 0, 0, 0, time.UTC))
			cfg := Config{
				Algorithm: algorithm,
				Rate:      2,
				Window:    time.Minute,
				Burst:     2,
				Addr:      ":0",
			}

			lim, err := NewLimiter(cfg, vc)
			if err != nil {
				t.Fatalf("NewLimiter() error = %v", err)
			}

			rec := chronorecorder.New(nil)
			store := chronostorage.NewMemoryStorage(vc)
			handler := NewHandler(lim, vc, rec, store)

			assertPublicRoutes(t, handler)

			resp1 := executeRequest(handler, http.MethodGet, "/api/profile", "client-a", "", "", "198.51.100.2:4000")
			assertStatus(t, resp1, http.StatusOK)
			assertRateLimitHeadersPresent(t, resp1)
			if got := resp1.Header().Get("Retry-After"); got != "0" {
				t.Fatalf("first request Retry-After = %q, want 0", got)
			}

			resp2 := executeRequest(handler, http.MethodPost, "/api/orders", "client-a", "", `{"sku":"book"}`, "198.51.100.2:4000")
			assertStatus(t, resp2, http.StatusCreated)
			assertRateLimitHeadersPresent(t, resp2)

			resp3 := executeRequest(handler, http.MethodGet, "/api/profile", "client-a", "", "", "198.51.100.2:4000")
			assertStatus(t, resp3, http.StatusTooManyRequests)
			assertRateLimitHeadersPresent(t, resp3)
			assertDeniedResponse(t, resp3)

			respOtherKey := executeRequest(handler, http.MethodGet, "/api/profile", "client-b", "", "", "198.51.100.2:4000")
			assertStatus(t, respOtherKey, http.StatusOK)
			assertRateLimitHeadersPresent(t, respOtherKey)
		})
	}
}

func TestRateLimitKeyFallsBackToClientIP(t *testing.T) {
	vc := chronoclock.NewVirtualClock(time.Date(2026, 2, 8, 10, 0, 0, 0, time.UTC))
	cfg := Config{
		Algorithm: limiter.AlgorithmFixedWindow,
		Rate:      1,
		Window:    time.Minute,
		Burst:     1,
		Addr:      ":0",
	}

	lim, err := NewLimiter(cfg, vc)
	if err != nil {
		t.Fatalf("NewLimiter() error = %v", err)
	}

	rec := chronorecorder.New(nil)
	store := chronostorage.NewMemoryStorage(vc)
	handler := NewHandler(lim, vc, rec, store)

	resp1 := executeRequest(handler, http.MethodGet, "/api/profile", "", "198.51.100.10", "", "203.0.113.1:8080")
	assertStatus(t, resp1, http.StatusOK)

	resp2 := executeRequest(handler, http.MethodGet, "/api/profile", "", "198.51.100.10", "", "203.0.113.1:8080")
	assertStatus(t, resp2, http.StatusTooManyRequests)
	assertDeniedResponse(t, resp2)

	resp3 := executeRequest(handler, http.MethodGet, "/api/profile", "", "198.51.100.11", "", "203.0.113.1:8080")
	assertStatus(t, resp3, http.StatusOK)
}

func assertPublicRoutes(t *testing.T, handler http.Handler) {
	t.Helper()

	health := executeRequest(handler, http.MethodGet, "/health", "", "", "", "198.51.100.9:9999")
	assertStatus(t, health, http.StatusOK)
	if got := health.Header().Get("X-RateLimit-Limit"); got != "" {
		t.Fatalf("/health should not include X-RateLimit-Limit header, got %q", got)
	}

	public := executeRequest(handler, http.MethodGet, "/public", "", "", "", "198.51.100.9:9999")
	assertStatus(t, public, http.StatusOK)
	if got := public.Header().Get("X-RateLimit-Limit"); got != "" {
		t.Fatalf("/public should not include X-RateLimit-Limit header, got %q", got)
	}
}

func executeRequest(handler http.Handler, method, path, apiKey, forwardedFor, body, remoteAddr string) *httptest.ResponseRecorder {
	requestBody := strings.NewReader(body)
	req := httptest.NewRequest(method, path, requestBody)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	}
	if forwardedFor != "" {
		req.Header.Set("X-Forwarded-For", forwardedFor)
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if remoteAddr != "" {
		req.RemoteAddr = remoteAddr
	}

	resp := httptest.NewRecorder()
	handler.ServeHTTP(resp, req)
	return resp
}

func assertDeniedResponse(t *testing.T, resp *httptest.ResponseRecorder) {
	t.Helper()

	if ct := resp.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("deny Content-Type = %q, want application/json", ct)
	}

	retryAfter, err := strconv.Atoi(resp.Header().Get("Retry-After"))
	if err != nil {
		t.Fatalf("Retry-After should be integer, got %q", resp.Header().Get("Retry-After"))
	}
	if retryAfter <= 0 {
		t.Fatalf("Retry-After = %d, want > 0", retryAfter)
	}

	var payload map[string]string
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode denied payload: %v", err)
	}
	if payload["error"] != "rate_limited" {
		t.Fatalf("deny payload error = %q, want rate_limited", payload["error"])
	}
}

func assertRateLimitHeadersPresent(t *testing.T, resp *httptest.ResponseRecorder) {
	t.Helper()

	for _, header := range []string{"X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset", "Retry-After"} {
		if got := resp.Header().Get(header); got == "" {
			t.Fatalf("missing required header %s", header)
		}
	}

	if _, err := strconv.Atoi(resp.Header().Get("X-RateLimit-Limit")); err != nil {
		t.Fatalf("X-RateLimit-Limit should be integer, got %q", resp.Header().Get("X-RateLimit-Limit"))
	}
	if _, err := strconv.Atoi(resp.Header().Get("X-RateLimit-Remaining")); err != nil {
		t.Fatalf("X-RateLimit-Remaining should be integer, got %q", resp.Header().Get("X-RateLimit-Remaining"))
	}
	if _, err := strconv.ParseInt(resp.Header().Get("X-RateLimit-Reset"), 10, 64); err != nil {
		t.Fatalf("X-RateLimit-Reset should be integer epoch seconds, got %q", resp.Header().Get("X-RateLimit-Reset"))
	}
	if _, err := strconv.Atoi(resp.Header().Get("Retry-After")); err != nil {
		t.Fatalf("Retry-After should be integer, got %q", resp.Header().Get("Retry-After"))
	}
}

func assertStatus(t *testing.T, resp *httptest.ResponseRecorder, want int) {
	t.Helper()
	if resp.Code != want {
		t.Fatalf("status = %d, want %d", resp.Code, want)
	}
}
