package userauth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	appleJWKSURL = "https://appleid.apple.com/auth/keys"
	appleIssuer  = "https://appleid.apple.com"
	appleJWKSTTL = 10 * time.Minute
)

type appleIDTokenClaims struct {
	Email string `json:"email"`
	jwt.RegisteredClaims
}

type appleJWKSet struct {
	Keys []appleJWK `json:"keys"`
}

type appleJWK struct {
	Kty string `json:"kty"`
	Kid string `json:"kid"`
	Use string `json:"use"`
	Alg string `json:"alg"`
	N   string `json:"n"`
	E   string `json:"e"`
}

// JWKSAppleVerifier verifies Apple identity tokens against Apple's JWKS.
type JWKSAppleVerifier struct {
	ClientIDs []string
	JWKSURL   string
	HTTP      *http.Client

	mu      sync.Mutex
	keys    map[string]*rsa.PublicKey
	fetched time.Time
}

func NewJWKSAppleVerifier(clientIDs []string) *JWKSAppleVerifier {
	return &JWKSAppleVerifier{
		ClientIDs: clientIDs,
		JWKSURL:   appleJWKSURL,
		HTTP:      &http.Client{Timeout: 10 * time.Second},
	}
}

func (v *JWKSAppleVerifier) Verify(ctx context.Context, identityToken string) (AppleClaims, error) {
	if v == nil || len(v.ClientIDs) == 0 {
		return AppleClaims{}, ErrAppleNotConfigured
	}
	identityToken = strings.TrimSpace(identityToken)
	if identityToken == "" {
		return AppleClaims{}, ErrInvalidAppleToken
	}

	var claims appleIDTokenClaims
	token, err := jwt.ParseWithClaims(identityToken, &claims, func(t *jwt.Token) (any, error) {
		if t.Method != jwt.SigningMethodRS256 {
			return nil, fmt.Errorf("%w: unexpected alg %v", ErrInvalidAppleToken, t.Header["alg"])
		}
		kid, _ := t.Header["kid"].(string)
		return v.keyForKid(ctx, kid)
	}, jwt.WithValidMethods([]string{jwt.SigningMethodRS256.Name}), jwt.WithIssuer(appleIssuer))
	if err != nil {
		return AppleClaims{}, fmt.Errorf("%w: %v", ErrInvalidAppleToken, err)
	}
	if !token.Valid {
		return AppleClaims{}, ErrInvalidAppleToken
	}

	if !audienceAllowed(claims.Audience, v.ClientIDs) {
		return AppleClaims{}, ErrAppleAudience
	}
	subject, err := appleSubject(claims.Subject)
	if err != nil {
		return AppleClaims{}, err
	}
	return AppleClaims{Subject: subject, Email: strings.TrimSpace(claims.Email)}, nil
}

func (v *JWKSAppleVerifier) keyForKid(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	if kid == "" {
		return nil, fmt.Errorf("%w: missing kid", ErrInvalidAppleToken)
	}
	if key := v.cachedKey(kid); key != nil {
		return key, nil
	}
	if err := v.refreshKeys(ctx); err != nil {
		return nil, err
	}
	if key := v.cachedKey(kid); key != nil {
		return key, nil
	}
	return nil, fmt.Errorf("%w: unknown kid", ErrInvalidAppleToken)
}

func (v *JWKSAppleVerifier) cachedKey(kid string) *rsa.PublicKey {
	v.mu.Lock()
	defer v.mu.Unlock()
	if v.keys == nil || time.Since(v.fetched) > appleJWKSTTL {
		return nil
	}
	return v.keys[kid]
}

func (v *JWKSAppleVerifier) refreshKeys(ctx context.Context) error {
	url := v.JWKSURL
	if url == "" {
		url = appleJWKSURL
	}
	client := v.HTTP
	if client == nil {
		client = http.DefaultClient
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("apple jwks request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("apple jwks fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("apple jwks status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return fmt.Errorf("apple jwks read: %w", err)
	}

	var set appleJWKSet
	if err := json.Unmarshal(body, &set); err != nil {
		return fmt.Errorf("apple jwks json: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(set.Keys))
	for _, jwk := range set.Keys {
		if jwk.Kty != "RSA" || jwk.Kid == "" {
			continue
		}
		pub, err := rsaPublicKeyFromJWK(jwk)
		if err != nil {
			return err
		}
		keys[jwk.Kid] = pub
	}

	v.mu.Lock()
	defer v.mu.Unlock()
	v.keys = keys
	v.fetched = time.Now()
	return nil
}

func rsaPublicKeyFromJWK(jwk appleJWK) (*rsa.PublicKey, error) {
	nBytes, err := base64.RawURLEncoding.DecodeString(jwk.N)
	if err != nil {
		return nil, fmt.Errorf("apple jwk n: %w", err)
	}
	eBytes, err := base64.RawURLEncoding.DecodeString(jwk.E)
	if err != nil {
		return nil, fmt.Errorf("apple jwk e: %w", err)
	}
	if len(nBytes) == 0 || len(eBytes) == 0 {
		return nil, fmt.Errorf("apple jwk missing modulus or exponent")
	}
	var e int
	for _, b := range eBytes {
		e = e<<8 | int(b)
	}
	return &rsa.PublicKey{
		N: new(big.Int).SetBytes(nBytes),
		E: e,
	}, nil
}

func audienceAllowed(aud jwt.ClaimStrings, clientIDs []string) bool {
	allowed := make(map[string]struct{}, len(clientIDs))
	for _, id := range clientIDs {
		id = strings.TrimSpace(id)
		if id != "" {
			allowed[id] = struct{}{}
		}
	}
	for _, a := range aud {
		if _, ok := allowed[strings.TrimSpace(a)]; ok {
			return true
		}
	}
	return false
}
