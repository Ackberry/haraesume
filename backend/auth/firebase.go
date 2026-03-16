package auth

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"backend/config"
	"backend/httputil"
)

type contextKey string

const (
	userIDContextKey    contextKey = "auth_user_id"
	userEmailContextKey contextKey = "auth_user_email"
)

type Validator struct {
	issuer   string
	audience string
	jwksURL  string
	client   *http.Client
	cacheTTL time.Duration

	mu         sync.RWMutex
	keys       map[string]*rsa.PublicKey
	lastJWKSAt time.Time
}

type jwtHeader struct {
	Alg string `json:"alg"`
	Kid string `json:"kid"`
}

type JWTClaims struct {
	Iss   string `json:"iss"`
	Aud   any    `json:"aud"`
	Sub   string `json:"sub"`
	Email string `json:"email"`
	Exp   int64  `json:"exp"`
	Nbf   int64  `json:"nbf,omitempty"`
	Iat   int64  `json:"iat,omitempty"`
}

type jwksDocument struct {
	Keys []jwk `json:"keys"`
}

type jwk struct {
	Kid string `json:"kid"`
	Kty string `json:"kty"`
	Alg string `json:"alg"`
	Use string `json:"use"`
	N   string `json:"n"`
	E   string `json:"e"`
}

func NewValidatorFromEnv() (*Validator, error) {
	provider := strings.ToLower(strings.TrimSpace(config.GetEnv("AUTH_PROVIDER")))
	if provider == "" {
		return nil, nil
	}
	if provider != "firebase" {
		return nil, fmt.Errorf("unsupported AUTH_PROVIDER: %s", provider)
	}

	projectID := strings.TrimSpace(config.GetEnv("FIREBASE_PROJECT_ID"))
	if projectID == "" {
		return nil, errors.New("FIREBASE_PROJECT_ID is required when AUTH_PROVIDER=firebase")
	}

	issuer := "https://securetoken.google.com/" + projectID

	return &Validator{
		issuer:   issuer,
		audience: projectID,
		jwksURL:  "https://www.googleapis.com/service_accounts/v1/jwk/securetoken@system.gserviceaccount.com",
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		cacheTTL: 10 * time.Minute,
		keys:     make(map[string]*rsa.PublicKey),
	}, nil
}

func UserIDFromContext(ctx context.Context) (string, bool) {
	value := ctx.Value(userIDContextKey)
	userID, ok := value.(string)
	if !ok || strings.TrimSpace(userID) == "" {
		return "", false
	}
	return userID, true
}

func UserEmailFromContext(ctx context.Context) string {
	value := ctx.Value(userEmailContextKey)
	email, ok := value.(string)
	if !ok {
		return ""
	}
	return email
}

func RequestUserID(r *http.Request) string {
	if userID, ok := UserIDFromContext(r.Context()); ok {
		return userID
	}
	return config.AnonymousUserID
}

func RequireAuth(v *Validator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if v == nil {
				ctx := context.WithValue(r.Context(), userIDContextKey, config.AnonymousUserID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
			if authHeader == "" {
				httputil.WriteError(w, http.StatusUnauthorized, "Missing Authorization header")
				return
			}
			if !strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
				httputil.WriteError(w, http.StatusUnauthorized, "Authorization header must be Bearer token")
				return
			}
			token := strings.TrimSpace(authHeader[len("Bearer "):])
			if token == "" {
				httputil.WriteError(w, http.StatusUnauthorized, "Missing bearer token")
				return
			}

			claims, err := v.ValidateToken(r.Context(), token)
			if err != nil {
				log.Printf("auth validation failed: %v", err)
				httputil.WriteError(w, http.StatusUnauthorized, "Unauthorized")
				return
			}
			if strings.TrimSpace(claims.Sub) == "" {
				httputil.WriteError(w, http.StatusUnauthorized, "Unauthorized")
				return
			}

			ctx := context.WithValue(r.Context(), userIDContextKey, claims.Sub)
			ctx = context.WithValue(ctx, userEmailContextKey, claims.Email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WarmUp pre-fetches the JWKS keys so the first authenticated request
// doesn't block on a network round-trip to googleapis.com.
func (v *Validator) WarmUp(ctx context.Context) error {
	return v.refreshJWKs(ctx)
}

func (v *Validator) ValidateToken(ctx context.Context, token string) (*JWTClaims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, errors.New("invalid jwt format")
	}

	headerBytes, err := decodeBase64URL(parts[0])
	if err != nil {
		return nil, fmt.Errorf("invalid jwt header: %w", err)
	}

	var header jwtHeader
	if err := json.Unmarshal(headerBytes, &header); err != nil {
		return nil, fmt.Errorf("invalid jwt header json: %w", err)
	}
	if !strings.EqualFold(header.Alg, "RS256") {
		return nil, fmt.Errorf("unsupported jwt alg: %s", header.Alg)
	}
	if strings.TrimSpace(header.Kid) == "" {
		return nil, errors.New("jwt missing kid")
	}

	claimsBytes, err := decodeBase64URL(parts[1])
	if err != nil {
		return nil, fmt.Errorf("invalid jwt claims: %w", err)
	}

	var claims JWTClaims
	if err := json.Unmarshal(claimsBytes, &claims); err != nil {
		return nil, fmt.Errorf("invalid jwt claims json: %w", err)
	}

	signature, err := decodeBase64URL(parts[2])
	if err != nil {
		return nil, fmt.Errorf("invalid jwt signature: %w", err)
	}

	publicKey, err := v.getPublicKey(ctx, header.Kid)
	if err != nil {
		return nil, err
	}

	signingInput := []byte(parts[0] + "." + parts[1])
	hash := sha256.Sum256(signingInput)
	if err := rsa.VerifyPKCS1v15(publicKey, crypto.SHA256, hash[:], signature); err != nil {
		return nil, fmt.Errorf("jwt signature verification failed: %w", err)
	}

	if normalizeIssuer(strings.TrimSpace(claims.Iss)) != normalizeIssuer(v.issuer) {
		return nil, fmt.Errorf("issuer mismatch")
	}
	if !claimHasAudience(claims.Aud, v.audience) {
		return nil, fmt.Errorf("audience mismatch")
	}

	const clockSkew = int64(60)
	now := time.Now().Unix()

	if claims.Exp == 0 {
		return nil, errors.New("jwt missing exp")
	}
	if now > claims.Exp+clockSkew {
		return nil, errors.New("token expired")
	}
	if claims.Nbf != 0 && now+clockSkew < claims.Nbf {
		return nil, errors.New("token not active yet")
	}
	if claims.Iat != 0 && claims.Iat > now+clockSkew {
		return nil, errors.New("token issued in the future")
	}

	return &claims, nil
}

func (v *Validator) getPublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	v.mu.RLock()
	key, ok := v.keys[kid]
	isFresh := time.Since(v.lastJWKSAt) < v.cacheTTL
	v.mu.RUnlock()
	if ok && isFresh {
		return key, nil
	}

	if err := v.refreshJWKs(ctx); err != nil {
		if ok {
			return key, nil
		}
		return nil, err
	}

	v.mu.RLock()
	defer v.mu.RUnlock()
	updated, found := v.keys[kid]
	if !found {
		return nil, fmt.Errorf("jwks key %s not found", kid)
	}
	return updated, nil
}

func (v *Validator) refreshJWKs(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, v.jwksURL, nil)
	if err != nil {
		return err
	}

	resp, err := v.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("jwks request failed: %s", resp.Status)
	}

	var doc jwksDocument
	if err := json.NewDecoder(resp.Body).Decode(&doc); err != nil {
		return err
	}
	if len(doc.Keys) == 0 {
		return errors.New("jwks response contained no keys")
	}

	next := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, key := range doc.Keys {
		if key.Kid == "" || key.Kty != "RSA" || key.N == "" || key.E == "" {
			continue
		}
		pub, err := parseRSAPublicKeyFromJWK(key.N, key.E)
		if err != nil {
			continue
		}
		next[key.Kid] = pub
	}
	if len(next) == 0 {
		return errors.New("jwks had no usable rsa keys")
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	v.keys = next
	v.lastJWKSAt = time.Now()
	return nil
}

func parseRSAPublicKeyFromJWK(nB64, eB64 string) (*rsa.PublicKey, error) {
	nBytes, err := decodeBase64URL(nB64)
	if err != nil {
		return nil, err
	}
	eBytes, err := decodeBase64URL(eB64)
	if err != nil {
		return nil, err
	}
	if len(eBytes) == 0 {
		return nil, errors.New("empty rsa exponent")
	}

	exponent := 0
	for _, b := range eBytes {
		exponent = (exponent << 8) + int(b)
	}
	if exponent <= 0 {
		return nil, errors.New("invalid rsa exponent")
	}

	modulus := new(big.Int).SetBytes(nBytes)
	if modulus.Sign() <= 0 {
		return nil, errors.New("invalid rsa modulus")
	}

	return &rsa.PublicKey{N: modulus, E: exponent}, nil
}

func decodeBase64URL(input string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(input)
}

func claimHasAudience(rawAud any, expected string) bool {
	switch aud := rawAud.(type) {
	case string:
		return aud == expected
	case []any:
		for _, item := range aud {
			if str, ok := item.(string); ok && str == expected {
				return true
			}
		}
	}
	return false
}

func normalizeIssuer(raw string) string {
	issuer := strings.TrimSpace(raw)
	return strings.TrimRight(issuer, "/")
}

func IsAdminUID(userID string) bool {
	adminUIDs := strings.TrimSpace(config.GetEnv("ADMIN_UIDS"))
	if adminUIDs == "" {
		return false
	}
	for _, uid := range strings.Split(adminUIDs, ",") {
		if strings.TrimSpace(uid) == userID {
			return true
		}
	}
	return false
}
