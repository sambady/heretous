package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"time"

	"github.com/sambady/heretous/internal/config"
	"github.com/sambady/heretous/internal/session"
)

const sessionCookieName = "heretous_session"

type sessionAPI struct {
	store  *session.Store
	cfg    config.Session
	logger *slog.Logger
}

func (a *sessionAPI) guest(w http.ResponseWriter, r *http.Request) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeAPIError(w, http.StatusUnsupportedMediaType, "invalid_content_type", "Ожидается JSON.")
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var input struct {
		Name string `json:"name"`
	}
	if err := decoder.Decode(&input); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeAPIError(w, http.StatusRequestEntityTooLarge, "body_too_large", "Слишком большой запрос.")
		} else {
			writeAPIError(w, http.StatusBadRequest, "invalid_json", "Не удалось прочитать имя.")
		}
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeAPIError(w, http.StatusBadRequest, "invalid_json", "Ожидается один JSON-объект.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	current, token, err := a.store.CreateGuest(ctx, input.Name, requestToken(r))
	if errors.Is(err, session.ErrInvalidName) {
		writeAPIError(w, http.StatusBadRequest, "invalid_name", "Введите имя от 1 до 32 символов без управляющих знаков.")
		return
	}
	if err != nil {
		a.unavailable(w, "create guest session", err)
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: token, Path: "/",
		HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
		Expires: current.ExpiresAt, MaxAge: int(a.cfg.TTL / time.Second),
	})
	writeJSON(w, http.StatusCreated, current)
}

func (a *sessionAPI) current(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	current, err := a.store.Current(ctx, requestToken(r))
	if errors.Is(err, session.ErrUnauthenticated) {
		a.clearCookie(w)
		writeAPIError(w, http.StatusUnauthorized, "unauthenticated", "Сессия завершена. Войдите ещё раз.")
		return
	}
	if err != nil {
		a.unavailable(w, "read session", err)
		return
	}
	writeJSON(w, http.StatusOK, current)
}

func (a *sessionAPI) logout(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	if err := a.store.Revoke(ctx, requestToken(r)); err != nil {
		a.unavailable(w, "revoke session", err)
		return
	}
	a.clearCookie(w)
	w.WriteHeader(http.StatusNoContent)
}

func (a *sessionAPI) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name: sessionCookieName, Value: "", Path: "/", MaxAge: -1,
		Expires: time.Unix(1, 0).UTC(), HttpOnly: true,
		Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode,
	})
}

func (a *sessionAPI) unavailable(w http.ResponseWriter, operation string, err error) {
	a.logger.Error(operation, "error", err)
	writeAPIError(w, http.StatusServiceUnavailable, "unavailable", "Сервис временно недоступен. Попробуйте ещё раз.")
}

func requestToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]any{"error": map[string]string{"code": code, "message": message}})
}
