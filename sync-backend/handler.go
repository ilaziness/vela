package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

const (
	StatusOK                  = http.StatusOK
	StatusBadRequest          = http.StatusBadRequest
	StatusUnauthorized        = http.StatusUnauthorized
	StatusNotFound            = http.StatusNotFound
	StatusTooManyRequests     = http.StatusTooManyRequests
	StatusInternalServerError = http.StatusInternalServerError
	StatusMethodNotAllowed    = http.StatusMethodNotAllowed
)

const (
	HeaderSyncID      = "X-Sync-ID"
	HeaderUserKey     = "X-User-Key"
	HeaderContentType = "Content-Type"
)

const (
	ContentTypeJSON   = "application/json"
	ContentTypeBinary = "application/octet-stream"
)

type contextKey string

const contextKeyUser = contextKey("user")

type Handler struct {
	service     *SyncService
	rateLimiter *RateLimiter
}

func NewHandler(service *SyncService, rateLimiter *RateLimiter) *Handler {
	return &Handler{
		service:     service,
		rateLimiter: rateLimiter,
	}
}

func (h *Handler) writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set(HeaderContentType, ContentTypeJSON)
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (h *Handler) writeError(w http.ResponseWriter, status int, message string) {
	http.Error(w, message, status)
}

func (h *Handler) getUserFromContext(r *http.Request) *User {
	return r.Context().Value(contextKeyUser).(*User)
}

func (h *Handler) authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		syncID := r.Header.Get(HeaderSyncID)
		userKey := r.Header.Get(HeaderUserKey)

		if syncID == "" || userKey == "" {
			h.writeError(w, StatusUnauthorized, "missing authentication headers")
			return
		}

		user, err := h.service.VerifyUser(syncID, userKey)
		if err != nil {
			h.writeError(w, StatusUnauthorized, "invalid credentials")
			return
		}

		ctx := context.WithValue(r.Context(), contextKeyUser, user)
		next(w, r.WithContext(ctx))
	}
}

func (h *Handler) UploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, StatusMethodNotAllowed, "method not allowed")
		return
	}

	user := h.getUserFromContext(r)
	if err := h.rateLimiter.Check(user.ID); err != nil {
		h.writeError(w, StatusTooManyRequests, err.Error())
		return
	}

	name := r.URL.Query().Get("name")
	if name == "" {
		h.writeError(w, StatusBadRequest, "name parameter required")
		return
	}

	info, err := h.service.SaveBackup(user.ID, name, r.Body)
	if err != nil {
		h.writeError(w, StatusInternalServerError, err.Error())
		return
	}

	_ = h.service.UpdateLastSyncAt(user.ID)
	h.writeJSON(w, StatusOK, info)
}

func (h *Handler) DownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, StatusMethodNotAllowed, "method not allowed")
		return
	}

	user := h.getUserFromContext(r)
	name := r.URL.Query().Get("name")
	if name == "" {
		h.writeError(w, StatusBadRequest, "name parameter required")
		return
	}

	reader, err := h.service.GetBackupReader(user.ID, name)
	if err != nil {
		h.writeError(w, StatusNotFound, "backup not found")
		return
	}
	defer reader.Close()

	w.Header().Set(HeaderContentType, ContentTypeBinary)
	w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	_, _ = io.Copy(w, reader)
}

func (h *Handler) BackupsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.ListBackupsHandler(w, r)
	case http.MethodDelete:
		h.DeleteBackupHandler(w, r)
	default:
		h.writeError(w, StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) ListBackupsHandler(w http.ResponseWriter, r *http.Request) {
	user := h.getUserFromContext(r)

	limit := 0
	offset := 0
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	backups, err := h.service.ListBackups(user.ID, limit, offset)
	if err != nil {
		h.writeError(w, StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, StatusOK, map[string]any{
		"backups": backups,
	})
}

func (h *Handler) DeleteBackupHandler(w http.ResponseWriter, r *http.Request) {
	user := h.getUserFromContext(r)
	name := r.URL.Query().Get("name")
	if name == "" {
		h.writeError(w, StatusBadRequest, "name parameter required")
		return
	}

	if err := h.service.DeleteBackup(user.ID, name); err != nil {
		h.writeError(w, StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, StatusOK, map[string]string{
		"message": "backup deleted",
	})
}

func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, StatusOK, map[string]string{
		"status": "ok",
	})
}

func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", h.HealthHandler)
	mux.HandleFunc("/upload", h.authMiddleware(h.UploadHandler))
	mux.HandleFunc("/download", h.authMiddleware(h.DownloadHandler))
	mux.HandleFunc("/backups", h.authMiddleware(h.BackupsHandler))
	return mux
}
