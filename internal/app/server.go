package app

import (
	"fmt"
	"net/http"
	"time"

	chronoclock "github.com/SmitUplenchwar2687/Chrono/pkg/clock"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	chronorecorder "github.com/SmitUplenchwar2687/Chrono/pkg/recorder"
)

// NewHandler builds the ChronoGate HTTP handler.
func NewHandler(
	cfg Config,
	mainLimiter limiter.Limiter,
	clk chronoclock.Clock,
	rec *chronorecorder.Recorder,
	storageSet *StorageLimiterSet,
) http.Handler {
	if rec == nil {
		rec = chronorecorder.New(nil)
	}
	if storageSet == nil {
		storageSet = NewStorageLimiterSet(cfg, clk)
	}

	tokenCfg := cfg
	tokenCfg.Algorithm = limiter.AlgorithmTokenBucket
	tokenLimiter, _ := NewLimiter(tokenCfg, clk)

	slidingCfg := cfg
	slidingCfg.Algorithm = limiter.AlgorithmSlidingWindow
	slidingLimiter, _ := NewLimiter(slidingCfg, clk)

	fixedCfg := cfg
	fixedCfg.Algorithm = limiter.AlgorithmFixedWindow
	fixedLimiter, _ := NewLimiter(fixedCfg, clk)

	mux := http.NewServeMux()

	// Validates: pkg/config + general runtime health path
	mux.HandleFunc("/health", methodHandler(http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))

	// Validates: unrestricted public route behavior in a Chrono consumer app
	mux.HandleFunc("/public", methodHandler(http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"service": "chronogate",
			"message": "public endpoint",
		})
	}))

	// Validates: pkg/limiter + pkg/storage via selected backend StorageLimiter
	mux.Handle("/api/profile", wrapRecordedLimit(mainLimiter, clk, rec, http.HandlerFunc(methodHandler(http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"id":   "demo-user",
			"name": "Chrono Demo",
		})
	}))))

	// Validates: pkg/limiter + pkg/storage deny path under write route
	mux.Handle("/api/orders", wrapRecordedLimit(mainLimiter, clk, rec, http.HandlerFunc(methodHandler(http.MethodPost, func(w http.ResponseWriter, _ *http.Request) {
		orderID := fmt.Sprintf("ord_%d", clk.Now().UnixNano())
		writeJSON(w, http.StatusCreated, map[string]string{
			"order_id": orderID,
			"status":   "created",
		})
	}))))

	// Validates: pkg/limiter.NewTokenBucket
	mux.Handle("/api/token-bucket", wrapRecordedLimit(tokenLimiter, clk, rec, http.HandlerFunc(methodHandler(http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"algorithm": string(limiter.AlgorithmTokenBucket), "status": "allowed"})
	}))))

	// Validates: pkg/limiter.NewSlidingWindow
	mux.Handle("/api/sliding-window", wrapRecordedLimit(slidingLimiter, clk, rec, http.HandlerFunc(methodHandler(http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"algorithm": string(limiter.AlgorithmSlidingWindow), "status": "allowed"})
	}))))

	// Validates: pkg/limiter.NewFixedWindow
	mux.Handle("/api/fixed-window", wrapRecordedLimit(fixedLimiter, clk, rec, http.HandlerFunc(methodHandler(http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"algorithm": string(limiter.AlgorithmFixedWindow), "status": "allowed"})
	}))))

	// Validates: pkg/storage memory backend + pkg/limiter.StorageLimiter
	mux.HandleFunc("/api/storage/memory", methodHandler(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		serveStorageDecision(w, r, clk, "memory", storageSet.Memory, nil, "")
	}))

	// Validates: pkg/storage redis backend + pkg/limiter.StorageLimiter
	mux.HandleFunc("/api/storage/redis", methodHandler(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		serveStorageDecision(w, r, clk, "redis", storageSet.Redis, storageSet.RedisErr, "")
	}))

	// Validates: pkg/storage CRDT backend + pkg/limiter.StorageLimiter
	mux.HandleFunc("/api/storage/crdt", methodHandler(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		serveStorageDecision(w, r, clk, "crdt", storageSet.CRDT, storageSet.CRDTErr, "⚠️ EXPERIMENTAL - eventual consistency may cause minor discrepancies")
	}))

	// Validates: side-by-side backend behavior comparison (memory vs redis vs crdt)
	mux.HandleFunc("/api/storage/compare", methodHandler(http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		serveStorageCompare(w, r, clk, storageSet)
	}))

	// Validates: pkg/recorder export of recorded request traffic
	mux.HandleFunc("/api/recordings/export", methodHandler(http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := rec.ExportJSON(w); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error":   "export_failed",
				"message": err.Error(),
			})
		}
	}))

	return mux
}

func wrapRecordedLimit(lim limiter.Limiter, clk chronoclock.Clock, rec *chronorecorder.Recorder, next http.Handler) http.Handler {
	if lim == nil {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{
				"error":   "limiter_unavailable",
				"message": "limiter is not configured",
			})
		})
	}
	return RecordingMiddleware(rec, clk)(RateLimitMiddleware(lim, clk)(next))
}

func methodHandler(method string, next func(http.ResponseWriter, *http.Request)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != method {
			w.Header().Set("Allow", method)
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error":   "method_not_allowed",
				"message": "method not allowed",
			})
			return
		}
		next(w, r)
	}
}

type compareResult struct {
	Allowed   bool    `json:"allowed"`
	Remaining int     `json:"remaining"`
	ResetAt   string  `json:"reset_at"`
	LatencyMS float64 `json:"latency_ms"`
	Error     string  `json:"error,omitempty"`
	Note      string  `json:"note,omitempty"`
}

func serveStorageDecision(w http.ResponseWriter, r *http.Request, clk chronoclock.Clock, backend string, lim limiter.Limiter, limErr error, note string) {
	if limErr != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error":   "backend_unavailable",
			"backend": backend,
			"message": limErr.Error(),
		})
		return
	}
	if lim == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"error":   "backend_unavailable",
			"backend": backend,
			"message": "limiter not initialized",
		})
		return
	}

	key := clientKeyFromRequest(r)
	start := time.Now()
	decision := lim.Allow(r.Context(), key)
	latency := time.Since(start)

	w.Header().Set("X-RateLimit-Limit", fmt.Sprintf("%d", decision.Limit))
	w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", decision.Remaining))
	w.Header().Set("X-RateLimit-Reset", fmt.Sprintf("%d", decision.ResetAt.Unix()))
	if decision.Allowed {
		w.Header().Set("Retry-After", "0")
	} else {
		retryAfter := retryAfterSeconds(decision.RetryAt, clk.Now())
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
	}

	status := http.StatusOK
	if !decision.Allowed {
		status = http.StatusTooManyRequests
	}

	payload := map[string]any{
		"backend":     backend,
		"key":         key,
		"allowed":     decision.Allowed,
		"remaining":   decision.Remaining,
		"limit":       decision.Limit,
		"reset_at":    decision.ResetAt.UTC().Format(time.RFC3339),
		"latency_ms":  float64(latency.Microseconds()) / 1000.0,
		"retry_after": w.Header().Get("Retry-After"),
	}
	if note != "" {
		payload["note"] = note
	}
	writeJSON(w, status, payload)
}

func serveStorageCompare(w http.ResponseWriter, r *http.Request, clk chronoclock.Clock, set *StorageLimiterSet) {
	key := clientKeyFromRequest(r)

	run := func(name string, lim limiter.Limiter, err error, note string) compareResult {
		if err != nil {
			return compareResult{Error: err.Error(), Note: note}
		}
		if lim == nil {
			return compareResult{Error: "limiter not initialized", Note: note}
		}

		start := time.Now()
		decision := lim.Allow(r.Context(), key)
		latency := time.Since(start)

		res := compareResult{
			Allowed:   decision.Allowed,
			Remaining: decision.Remaining,
			ResetAt:   decision.ResetAt.UTC().Format(time.RFC3339),
			LatencyMS: float64(latency.Microseconds()) / 1000.0,
			Note:      note,
		}
		if !decision.Allowed {
			_ = retryAfterSeconds(decision.RetryAt, clk.Now())
		}
		_ = name
		return res
	}

	memoryRes := run("memory", set.Memory, nil, "")
	redisRes := run("redis", set.Redis, set.RedisErr, "")
	crdtRes := run("crdt", set.CRDT, set.CRDTErr, "⚠️ EXPERIMENTAL - eventual consistency may cause minor discrepancies")

	consistent := true
	var baseline *compareResult
	for _, res := range []compareResult{memoryRes, redisRes, crdtRes} {
		if res.Error != "" {
			continue
		}
		if baseline == nil {
			copyRes := res
			baseline = &copyRes
			continue
		}
		if baseline.Allowed != res.Allowed || baseline.Remaining != res.Remaining || baseline.ResetAt != res.ResetAt {
			consistent = false
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"request": map[string]any{
			"user_id": key,
			"limit":   0,
			"window":  "configured",
		},
		"results": map[string]any{
			"memory": memoryRes,
			"redis":  redisRes,
			"crdt":   crdtRes,
		},
		"consistent": consistent,
	})
}
