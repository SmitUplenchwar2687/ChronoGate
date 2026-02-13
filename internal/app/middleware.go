package app

import (
	"log"
	"math"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	chronoclock "github.com/SmitUplenchwar2687/Chrono/pkg/clock"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	chronorecorder "github.com/SmitUplenchwar2687/Chrono/pkg/recorder"
)

// RecordingMiddleware records request traffic using Chrono's recorder package.
func RecordingMiddleware(state *RecordingState, clk chronoclock.Clock) func(http.Handler) http.Handler {
	if state == nil {
		state = NewRecordingState(nil, true)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := state.Record(chronorecorder.TrafficRecord{
				Timestamp: clk.Now(),
				Key:       clientKeyFromRequest(r),
				Endpoint:  r.Method + " " + r.URL.Path,
			}); err != nil {
				log.Printf("record traffic: %v", err)
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RateLimitMiddleware enforces rate limiting for protected endpoints.
func RateLimitMiddleware(lim limiter.Limiter, clk chronoclock.Clock) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := clientKeyFromRequest(r)
			decision := lim.Allow(r.Context(), key)

			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(decision.Limit))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(decision.Remaining))
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(decision.ResetAt.Unix(), 10))

			if decision.Allowed {
				w.Header().Set("Retry-After", "0")
				next.ServeHTTP(w, r)
				return
			}

			retryAfter := retryAfterSeconds(decision.RetryAt, clk.Now())
			w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"error":   "rate_limited",
				"message": "too many requests",
				"key":     key,
			})
		})
	}
}

func clientKeyFromRequest(r *http.Request) string {
	if apiKey := strings.TrimSpace(r.Header.Get("X-API-Key")); apiKey != "" {
		return apiKey
	}

	if forwardedFor := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		if len(parts) > 0 {
			ip := strings.TrimSpace(parts[0])
			if ip != "" {
				return ip
			}
		}
	}

	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil && host != "" {
		return host
	}

	if remoteAddr := strings.TrimSpace(r.RemoteAddr); remoteAddr != "" {
		return remoteAddr
	}

	return "unknown"
}

func retryAfterSeconds(retryAt, now time.Time) int {
	if retryAt.IsZero() {
		return 1
	}

	seconds := retryAt.Sub(now).Seconds()
	if seconds <= 0 {
		return 1
	}

	return int(math.Ceil(seconds))
}
