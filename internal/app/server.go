package app

import (
	"fmt"
	"net/http"

	chronoclock "github.com/SmitUplenchwar2687/Chrono/pkg/clock"
	"github.com/SmitUplenchwar2687/Chrono/pkg/limiter"
	chronorecorder "github.com/SmitUplenchwar2687/Chrono/pkg/recorder"
	chronostorage "github.com/SmitUplenchwar2687/Chrono/pkg/storage"
)

// NewHandler builds the ChronoGate HTTP handler.
func NewHandler(
	lim limiter.Limiter,
	clk chronoclock.Clock,
	rec *chronorecorder.Recorder,
	store chronostorage.Storage,
) http.Handler {
	if rec == nil {
		rec = chronorecorder.New(nil)
	}
	if store == nil {
		store = chronostorage.NewMemoryStorage(clk)
	}

	mux := http.NewServeMux()
	protected := RateLimitMiddleware(lim, clk)

	mux.HandleFunc("/health", methodHandler(http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}))

	mux.HandleFunc("/public", methodHandler(http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"service": "chronogate",
			"message": "public endpoint",
		})
	}))

	mux.Handle("/api/profile", protected(http.HandlerFunc(methodHandler(http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"id":   "demo-user",
			"name": "Chrono Demo",
		})
	}))))

	mux.Handle("/api/orders", protected(http.HandlerFunc(methodHandler(http.MethodPost, func(w http.ResponseWriter, _ *http.Request) {
		orderID := fmt.Sprintf("ord_%d", clk.Now().UnixNano())
		writeJSON(w, http.StatusCreated, map[string]string{
			"order_id": orderID,
			"status":   "created",
		})
	}))))

	mux.HandleFunc("/api/recordings/export", methodHandler(http.MethodGet, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := rec.ExportJSON(w); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"error":   "export_failed",
				"message": err.Error(),
			})
		}
	}))

	mux.HandleFunc("/api/storage/demo", storageDemoHandler(store))

	return RecordingMiddleware(rec, clk)(mux)
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
