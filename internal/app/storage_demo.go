package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	chronokv "github.com/SmitUplenchwar2687/Chrono/pkg/kvstorage"
)

type storageWriteRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
	TTL   string `json:"ttl"`
}

type storageIncrementRequest struct {
	Key   string `json:"key"`
	Delta int64  `json:"delta"`
	TTL   string `json:"ttl"`
}

func storageDemoHandler(store chronokv.Storage) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleStorageRead(w, r, store)
		case http.MethodPut:
			handleStorageWrite(w, r, store)
		case http.MethodPost:
			handleStorageIncrement(w, r, store)
		default:
			w.Header().Set("Allow", "GET, PUT, POST")
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"error":   "method_not_allowed",
				"message": "use GET (read), PUT (write), POST (increment)",
			})
		}
	}
}

func handleStorageRead(w http.ResponseWriter, r *http.Request, store chronokv.Storage) {
	key := strings.TrimSpace(r.URL.Query().Get("key"))
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "invalid_request",
			"message": "query parameter 'key' is required",
		})
		return
	}

	value, err := store.Get(r.Context(), key)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "storage_error",
			"message": err.Error(),
		})
		return
	}

	resp := map[string]any{
		"operation": "read",
		"key":       key,
		"exists":    value != nil,
	}
	if value != nil {
		resp["value"] = string(value)
	}

	writeJSON(w, http.StatusOK, resp)
}

func handleStorageWrite(w http.ResponseWriter, r *http.Request, store chronokv.Storage) {
	var req storageWriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "invalid_request",
			"message": "invalid JSON body",
		})
		return
	}

	key := strings.TrimSpace(req.Key)
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "invalid_request",
			"message": "field 'key' is required",
		})
		return
	}

	ttl, err := parseOptionalDuration(req.TTL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "invalid_request",
			"message": err.Error(),
		})
		return
	}

	if err := store.Set(r.Context(), key, []byte(req.Value), ttl); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "storage_error",
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"operation": "write",
		"key":       key,
		"value":     req.Value,
		"ttl":       ttl.String(),
	})
}

func handleStorageIncrement(w http.ResponseWriter, r *http.Request, store chronokv.Storage) {
	var req storageIncrementRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "invalid_request",
			"message": "invalid JSON body",
		})
		return
	}

	key := strings.TrimSpace(req.Key)
	if key == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "invalid_request",
			"message": "field 'key' is required",
		})
		return
	}

	ttl, err := parseOptionalDuration(req.TTL)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{
			"error":   "invalid_request",
			"message": err.Error(),
		})
		return
	}

	delta := req.Delta
	if delta == 0 {
		delta = 1
	}

	value, err := store.Increment(r.Context(), key, delta, ttl)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error":   "storage_error",
			"message": err.Error(),
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"operation": "increment",
		"key":       key,
		"value":     value,
		"delta":     delta,
		"ttl":       ttl.String(),
	})
}

func parseOptionalDuration(raw string) (time.Duration, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, nil
	}
	d, err := time.ParseDuration(trimmed)
	if err != nil {
		return 0, err
	}
	if d < 0 {
		return 0, fmt.Errorf("ttl must be >= 0, got %s", d)
	}
	return d, nil
}
