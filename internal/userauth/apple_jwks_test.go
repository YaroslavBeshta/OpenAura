package userauth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestJWKSAppleVerifier_Verify(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("rsa: %v", err)
	}
	kid := "test-kid"
	jwks := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := base64.RawURLEncoding.EncodeToString(key.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes())
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]string{{
				"kty": "RSA",
				"kid": kid,
				"use": "sig",
				"alg": "RS256",
				"n":   n,
				"e":   e,
			}},
		})
	}))
	t.Cleanup(jwks.Close)

	verifier := &JWKSAppleVerifier{
		ClientIDs: []string{"app.brocal.ios"},
		JWKSURL:   jwks.URL,
		HTTP:      jwks.Client(),
	}

	now := time.Now()
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, appleIDTokenClaims{
		Email: "ada@example.com",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    appleIssuer,
			Subject:   "apple-user-1",
			Audience:  jwt.ClaimStrings{"app.brocal.ios"},
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	})
	token.Header["kid"] = kid
	raw, err := token.SignedString(key)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	claims, err := verifier.Verify(context.Background(), raw)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if claims.Subject != "apple-user-1" || claims.Email != "ada@example.com" {
		t.Fatalf("claims=%+v", claims)
	}

	wrongAud := jwt.NewWithClaims(jwt.SigningMethodRS256, appleIDTokenClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    appleIssuer,
			Subject:   "apple-user-1",
			Audience:  jwt.ClaimStrings{"other.app"},
			ExpiresAt: jwt.NewNumericDate(now.Add(time.Hour)),
		},
	})
	wrongAud.Header["kid"] = kid
	bad, err := wrongAud.SignedString(key)
	if err != nil {
		t.Fatalf("sign bad aud: %v", err)
	}
	if _, err := verifier.Verify(context.Background(), bad); !errors.Is(err, ErrAppleAudience) && !errors.Is(err, ErrInvalidAppleToken) {
		if err == nil {
			t.Fatal("expected audience rejection")
		}
	}
}

func TestNewJWKSAppleVerifier(t *testing.T) {
	v := NewJWKSAppleVerifier([]string{"app.brocal.ios"})
	if len(v.ClientIDs) != 1 || v.ClientIDs[0] != "app.brocal.ios" {
		t.Fatalf("client ids = %v", v.ClientIDs)
	}
	if v.JWKSURL != appleJWKSURL {
		t.Fatalf("jwks url = %q, want %q", v.JWKSURL, appleJWKSURL)
	}
	if v.HTTP == nil || v.HTTP.Timeout != 10*time.Second {
		t.Fatalf("http client = %+v", v.HTTP)
	}
}

func TestJWKSAppleVerifier_NotConfigured(t *testing.T) {
	v := &JWKSAppleVerifier{}
	if _, err := v.Verify(context.Background(), "token"); !errors.Is(err, ErrAppleNotConfigured) {
		t.Fatalf("got %v", err)
	}
}

func TestStaticAppleVerifier(t *testing.T) {
	v := NewStaticAppleVerifier()
	v.Allow("tok", AppleClaims{Subject: "sub", Email: "a@b.co"})
	got, err := v.Verify(context.Background(), "tok")
	if err != nil || got.Subject != "sub" {
		t.Fatalf("got %+v err=%v", got, err)
	}
	if _, err := v.Verify(context.Background(), "nope"); !errors.Is(err, ErrInvalidAppleToken) {
		t.Fatalf("unknown token: %v", err)
	}
}
