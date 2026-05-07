package auth

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/wirepanel/wirepanel/shared/featgate"
	"github.com/wirepanel/wirepanel/shared/perms"
)

type Identity struct {
	UserID      string             `json:"user_id"`
	Username    string             `json:"username"`
	RoleIDs     []string           `json:"role_ids"`
	Permissions []perms.Permission `json:"permissions"`
}

func (i *Identity) Has(p perms.Permission) bool {
	if i == nil {
		return false
	}
	return perms.Has(i.Permissions, p)
}

type ctxKey int

const ctxIdentityKey ctxKey = 1

type Handler struct {
	jwtSecret []byte
	provider  featgate.Provider

	rateMu sync.Mutex
	rate   map[string]*bucket
}

type bucket struct {
	tokens float64
	last   time.Time
}

func NewHandler(secret, env string, provider featgate.Provider) *Handler {
	if env == "production" && (secret == "" || secret == "dev-secret-change-me") {
		log.Fatal("WP_JWT_SECRET must be set in production")
	}
	if secret == "" {
		secret = "dev-secret-change-me"
	}
	return &Handler{
		jwtSecret: []byte(secret),
		provider:  provider,
		rate:      make(map[string]*bucket),
	}
}

func (h *Handler) Edition() featgate.Edition { return h.provider.Edition() }

func (h *Handler) ResolveIdentity(u *featgate.User) *Identity {
	id := &Identity{
		UserID:   u.ID,
		Username: u.Username,
		RoleIDs:  u.RoleIDs,
	}
	if h.provider.Edition() == featgate.EditionCommunity {
		id.Permissions = []perms.Permission{perms.WildcardAll}
		return id
	}
	seen := map[perms.Permission]bool{}
	for _, rid := range u.RoleIDs {
		r, ok := h.provider.Roles().Get(rid)
		if !ok {
			continue
		}
		for _, p := range r.Permissions {
			if !seen[p] {
				seen[p] = true
				id.Permissions = append(id.Permissions, p)
			}
		}
	}
	return id
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Token       string             `json:"token"`
	UserID      string             `json:"user_id"`
	Username    string             `json:"username"`
	Edition     featgate.Edition   `json:"edition"`
	Permissions []perms.Permission `json:"permissions"`
}

func (h *Handler) LoginHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !h.allowLogin(clientIP(r)) {
			http.Error(w, "rate limited", http.StatusTooManyRequests)
			return
		}
		var req LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		u, ok := h.provider.Users().VerifyPassword(req.Username, req.Password)
		if !ok {
			time.Sleep(150 * time.Millisecond)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id := h.ResolveIdentity(u)
		token := h.sign(id, 24*time.Hour)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(LoginResponse{
			Token:       token,
			UserID:      id.UserID,
			Username:    id.Username,
			Edition:     h.provider.Edition(),
			Permissions: id.Permissions,
		})
	}
}

func (h *Handler) Me() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := IdentityFromContext(r.Context())
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"user_id":     id.UserID,
			"username":    id.Username,
			"role_ids":    id.RoleIDs,
			"permissions": id.Permissions,
			"edition":     h.provider.Edition(),
		})
	}
}

func (h *Handler) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := extractToken(r)
		if token == "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		claims, ok := h.verify(token)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		uid, _ := claims["sub"].(string)
		u, ok := h.provider.Users().Get(uid)
		if !ok {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		id := h.ResolveIdentity(u)
		ctx := context.WithValue(r.Context(), ctxIdentityKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (h *Handler) Require(p perms.Permission, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := IdentityFromContext(r.Context())
		if !id.Has(p) {
			http.Error(w, "forbidden: missing permission "+string(p), http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func IdentityFromContext(ctx context.Context) *Identity {
	v, _ := ctx.Value(ctxIdentityKey).(*Identity)
	if v == nil {
		return &Identity{}
	}
	return v
}

func extractToken(r *http.Request) string {
	if a := r.Header.Get("Authorization"); strings.HasPrefix(a, "Bearer ") {
		return strings.TrimPrefix(a, "Bearer ")
	}
	if q := r.URL.Query().Get("access_token"); q != "" {
		return q
	}
	return ""
}

func (h *Handler) sign(id *Identity, dur time.Duration) string {
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	exp := time.Now().Add(dur).Unix()
	payload, _ := json.Marshal(map[string]any{
		"sub": id.UserID,
		"usr": id.Username,
		"exp": exp,
		"iat": time.Now().Unix(),
	})
	payloadEnc := base64.RawURLEncoding.EncodeToString(payload)
	msg := header + "." + payloadEnc
	mac := hmac.New(sha256.New, h.jwtSecret)
	mac.Write([]byte(msg))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	return msg + "." + sig
}

func (h *Handler) verify(token string) (map[string]any, bool) {
	parts := strings.SplitN(token, ".", 3)
	if len(parts) != 3 {
		return nil, false
	}
	mac := hmac.New(sha256.New, h.jwtSecret)
	mac.Write([]byte(parts[0] + "." + parts[1]))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	if subtle.ConstantTimeCompare([]byte(parts[2]), []byte(expected)) != 1 {
		return nil, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, false
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, false
	}
	if exp, ok := claims["exp"].(float64); ok {
		if time.Now().Unix() > int64(exp) {
			return nil, false
		}
	}
	return claims, true
}

func (h *Handler) allowLogin(ip string) bool {
	const rate = 5.0 / 60.0
	const burst = 5.0
	now := time.Now()
	h.rateMu.Lock()
	defer h.rateMu.Unlock()
	b, ok := h.rate[ip]
	if !ok {
		b = &bucket{tokens: burst, last: now}
		h.rate[ip] = b
	}
	elapsed := now.Sub(b.last).Seconds()
	b.tokens = minF(burst, b.tokens+elapsed*rate)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens -= 1
	return true
}

func clientIP(r *http.Request) string {
	if x := r.Header.Get("X-Forwarded-For"); x != "" {
		if i := strings.IndexByte(x, ','); i > 0 {
			return strings.TrimSpace(x[:i])
		}
		return strings.TrimSpace(x)
	}
	host := r.RemoteAddr
	if i := strings.LastIndexByte(host, ':'); i > 0 {
		host = host[:i]
	}
	return host
}

func minF(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
