package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
)

// HTTP 状态码常量
const (
	StatusOK                  = http.StatusOK
	StatusBadRequest          = http.StatusBadRequest
	StatusUnauthorized        = http.StatusUnauthorized
	StatusNotFound            = http.StatusNotFound
	StatusTooManyRequests     = http.StatusTooManyRequests
	StatusInternalServerError = http.StatusInternalServerError
	StatusMethodNotAllowed    = http.StatusMethodNotAllowed
)

// 请求头常量
const (
	HeaderSyncID        = "X-Sync-ID"
	HeaderUserKey       = "X-User-Key"
	HeaderVersionNumber = "X-Version-Number"
	HeaderContentType   = "Content-Type"
)

// Content-Type 常量
const (
	ContentTypeJSON   = "application/json"
	ContentTypeBinary = "application/octet-stream"
)

// contextKey 用于上下文键
type contextKey string

const contextKeyUser = contextKey("user")

// Handler HTTP处理器
type Handler struct {
	service     *SyncService
	rateLimiter *RateLimiter
}

// NewHandler 创建处理器
func NewHandler(service *SyncService, rateLimiter *RateLimiter) *Handler {
	return &Handler{
		service:     service,
		rateLimiter: rateLimiter,
	}
}

// 响应辅助方法

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

// authMiddleware 认证中间件
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

// UploadHandler 上传处理器
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

	version, err := h.service.SaveVersion(user.ID, r.Body)
	if err != nil {
		h.writeError(w, StatusInternalServerError, err.Error())
		return
	}

	_ = h.service.UpdateLastSyncAt(user.ID)

	h.writeJSON(w, StatusOK, map[string]any{
		"version_number": version.VersionNumber,
		"file_size":      version.FileSize,
		"created_at":     version.CreatedAt,
	})
}

// DownloadHandler 下载处理器
func (h *Handler) DownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, StatusMethodNotAllowed, "method not allowed")
		return
	}

	user := h.getUserFromContext(r)

	versionNumber, err := h.getVersionNumber(r, user.ID)
	if err != nil {
		h.writeError(w, StatusNotFound, err.Error())
		return
	}

	version, err := h.service.GetVersion(user.ID, versionNumber)
	if err != nil {
		h.writeError(w, StatusNotFound, "version not found")
		return
	}

	reader, err := h.service.GetVersionReader(user.ID, versionNumber)
	if err != nil {
		h.writeError(w, StatusInternalServerError, err.Error())
		return
	}
	defer reader.Close()

	w.Header().Set(HeaderContentType, ContentTypeBinary)

	w.Header().Set(HeaderVersionNumber, strconv.Itoa(version.VersionNumber))

	_, _ = io.Copy(w, reader)
}

// getVersionNumber 获取版本号（从查询参数或最新版本）
func (h *Handler) getVersionNumber(r *http.Request, userID string) (int, error) {
	versionStr := r.URL.Query().Get("version")
	if versionStr != "" {
		return strconv.Atoi(versionStr)
	}

	// 默认下载最新版本
	version, err := h.service.GetLatestVersion(userID)
	if err != nil {
		return 0, err
	}
	return version.VersionNumber, nil
}

// VersionsHandler 处理版本相关的请求（GET/DELETE）
func (h *Handler) VersionsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.ListVersionsHandler(w, r)
	case http.MethodDelete:
		h.DeleteVersionHandler(w, r)
	default:
		h.writeError(w, StatusMethodNotAllowed, "method not allowed")
	}
}

// ListVersionsHandler 版本列表处理器
func (h *Handler) ListVersionsHandler(w http.ResponseWriter, r *http.Request) {
	user := h.getUserFromContext(r)

	// 解析分页参数
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

	versions, err := h.service.ListVersions(user.ID, limit, offset)
	if err != nil {
		h.writeError(w, StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, StatusOK, map[string]any{
		"versions": versions,
	})
}

// DeleteVersionHandler 删除版本处理器
func (h *Handler) DeleteVersionHandler(w http.ResponseWriter, r *http.Request) {
	user := h.getUserFromContext(r)

	versionStr := r.URL.Query().Get("version")
	if versionStr == "" {
		h.writeError(w, StatusBadRequest, "version parameter required")
		return
	}

	versionNumber, err := strconv.Atoi(versionStr)
	if err != nil {
		h.writeError(w, StatusBadRequest, "invalid version number")
		return
	}

	if err := h.service.DeleteVersion(user.ID, versionNumber); err != nil {
		h.writeError(w, StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, StatusOK, map[string]string{
		"message": "version deleted successfully",
	})
}

// HealthHandler 健康检查处理器
func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, StatusOK, map[string]string{
		"status": "ok",
	})
}

// Routes 注册路由
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()

	// 健康检查（无需认证）
	mux.HandleFunc("/health", h.HealthHandler)

	// 需要认证的路由
	mux.HandleFunc("/upload", h.authMiddleware(h.UploadHandler))
	mux.HandleFunc("/download", h.authMiddleware(h.DownloadHandler))
	mux.HandleFunc("/versions", h.authMiddleware(h.VersionsHandler))

	return mux
}
