package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

const defaultJSONBodyLimit int64 = 1 << 20

func decodeJSONBody(w http.ResponseWriter, r *http.Request, dst any, limit int64) bool {
	return decodeJSONBodyWithOptions(w, r, dst, limit, false)
}

func decodeStrictJSONBody(w http.ResponseWriter, r *http.Request, dst any, limit int64) bool {
	return decodeJSONBodyWithOptions(w, r, dst, limit, true)
}

func decodeJSONBodyWithOptions(w http.ResponseWriter, r *http.Request, dst any, limit int64, strict bool) bool {
	if limit > 0 {
		r.Body = http.MaxBytesReader(w, r.Body, limit)
	}

	decoder := json.NewDecoder(r.Body)
	if strict {
		decoder.DisallowUnknownFields()
	}

	if err := decoder.Decode(dst); err != nil {
		switch {
		case errors.Is(err, io.EOF):
			if strict {
				http.Error(w, "Request body is required", http.StatusBadRequest)
			} else {
				http.Error(w, "Invalid request body", http.StatusBadRequest)
			}
		case err.Error() == "http: request body too large":
			http.Error(w, "Request body too large", http.StatusRequestEntityTooLarge)
		case strict && strings.HasPrefix(err.Error(), "json: unknown field "):
			http.Error(w, "Request body contains unknown fields", http.StatusBadRequest)
		default:
			http.Error(w, "Invalid request body", http.StatusBadRequest)
		}
		return false
	}

	if strict {
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			http.Error(w, "Request body must contain a single JSON object", http.StatusBadRequest)
			return false
		}
	}

	return true
}
