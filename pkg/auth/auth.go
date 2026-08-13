package auth

import (
	"crypto/subtle"
	"net/http"
	"sync"
	"time"

	"void-panel/pkg/config"
)

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]time.Time
}

var store = &SessionStore{
	sessions: make(map[string]time.Time),
}

const CookieName = "void_session"
const SessionDuration = 24 * time.Hour

func Authenticate(username, password string) bool {
	cfg := config.GlobalConfig
	if cfg == nil {
		return false
	}
	userMatch := subtle.ConstantTimeCompare([]byte(username), []byte(cfg.Username)) == 1
	passMatch := subtle.ConstantTimeCompare([]byte(password), []byte(cfg.Password)) == 1
	return userMatch && passMatch
}

func CreateSession(w http.ResponseWriter) string {
	token := config.GlobalConfig.SessionSecret + "-" + time.Now().Format("20060102150405999999")
	store.mu.Lock()
	store.sessions[token] = time.Now().Add(SessionDuration)
	store.mu.Unlock()

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    token,
		Path:     "/",
		Expires:  time.Now().Add(SessionDuration),
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})

	return token
}

func IsAuthenticated(r *http.Request) bool {
	if config.GlobalConfig != nil && !config.GlobalConfig.AuthEnabled {
		return true
	}

	cookie, err := r.Cookie(CookieName)
	if err != nil || cookie.Value == "" {
		// Also check Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader != "" && isValidToken(authHeader) {
			return true
		}
		return false
	}

	return isValidToken(cookie.Value)
}

func isValidToken(token string) bool {
	store.mu.RLock()
	defer store.mu.RUnlock()

	expiry, exists := store.sessions[token]
	if !exists {
		return false
	}

	if time.Now().After(expiry) {
		return false
	}

	return true
}

func Logout(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(CookieName)
	if err == nil && cookie.Value != "" {
		store.mu.Lock()
		delete(store.sessions, cookie.Value)
		store.mu.Unlock()
	}

	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
	})
}

func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !IsAuthenticated(r) {
			if r.Header.Get("Accept") == "application/json" || r.Header.Get("X-Requested-With") == "XMLHttpRequest" {
				http.Error(w, `{"error":"Unauthorized"}`, http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next(w, r)
	}
}
