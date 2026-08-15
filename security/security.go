package security

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// Header constants defined by Cashfree webhook specification.
const (
	HeaderSignature = "x-webhook-signature"
	HeaderTimestamp = "x-webhook-timestamp"
	EnvWebhookSecret = "CASHFREE_WEBHOOK_SECRET"
)

type contextKey string

// RawBodyKey is the request context key used to pass the verified raw request body downstream.
const RawBodyKey contextKey = "raw_webhook_body"

// ErrMissingSecret indicates the webhook secret environment variable is unconfigured.
var ErrMissingSecret = errors.New("webhook secret is not configured")

// Verifier encapsulates webhook HMAC-SHA256 signature verification logic.
type Verifier struct {
	secret []byte
}

// NewVerifier creates a new Verifier with the given secret.
// Operator Safety: Returns an error if the secret is empty, preventing insecure zero-secret operation.
func NewVerifier(secret string) (*Verifier, error) {
	trimmed := strings.TrimSpace(secret)
	if trimmed == "" {
		return nil, ErrMissingSecret
	}
	return &Verifier{secret: []byte(trimmed)}, nil
}

// NewVerifierFromEnv reads the webhook secret from the CASHFREE_WEBHOOK_SECRET environment variable.
func NewVerifierFromEnv() (*Verifier, error) {
	secret := os.Getenv(EnvWebhookSecret)
	return NewVerifier(secret)
}

// ComputeSignature calculates the HMAC-SHA256 signature over (timestamp + rawBody) encoded in base64.
// Cashfree standard: base64(hmac_sha256(secret, timestamp + rawBody))
func (v *Verifier) ComputeSignature(timestamp string, rawBody []byte) string {
	mac := hmac.New(sha256.New, v.secret)
	mac.Write([]byte(timestamp))
	mac.Write(rawBody)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// Verify checks whether the provided signature matches the computed HMAC-SHA256.
// Defensive Programming: Uses constant-time comparison (hmac.Equal) to prevent timing attacks.
// It also checks hex encoding as a defensive fallback if base64 does not match.
func (v *Verifier) Verify(signature, timestamp string, rawBody []byte) bool {
	if signature == "" || timestamp == "" || len(rawBody) == 0 {
		return false
	}

	mac := hmac.New(sha256.New, v.secret)
	mac.Write([]byte(timestamp))
	mac.Write(rawBody)
	expectedMAC := mac.Sum(nil)

	// Cashfree standard: Base64
	expectedBase64 := base64.StdEncoding.EncodeToString(expectedMAC)
	if hmac.Equal([]byte(signature), []byte(expectedBase64)) {
		return true
	}

	// Defensive fallback: Hex-encoded signature comparison
	expectedHex := hex.EncodeToString(expectedMAC)
	if hmac.Equal([]byte(strings.ToLower(signature)), []byte(expectedHex)) {
		return true
	}

	return false
}

// Middleware creates an HTTP middleware that verifies incoming Cashfree webhook signatures.
// Defensive Programming & Operator Safety:
// 1. Reads raw bytes before any JSON parsing/deserialization to guarantee byte-for-byte integrity.
// 2. Reconstructs r.Body using io.NopCloser so downstream handlers can re-read it.
// 3. Attaches the verified raw body to the request context for zero-overhead access.
// 4. Returns standard HTTP 401 Unauthorized upon verification failure without panicking.
func (v *Verifier) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only enforce signature verification on POST requests
		if r.Method != http.MethodPost {
			next.ServeHTTP(w, r)
			return
		}

		timestamp := r.Header.Get(HeaderTimestamp)
		if timestamp == "" {
			writeSecurityError(w, http.StatusUnauthorized, fmt.Sprintf("missing required header: %s", HeaderTimestamp))
			return
		}

		signature := r.Header.Get(HeaderSignature)
		if signature == "" {
			writeSecurityError(w, http.StatusUnauthorized, fmt.Sprintf("missing required header: %s", HeaderSignature))
			return
		}

		// Read the un-transformed raw request body
		rawBody, err := io.ReadAll(r.Body)
		defer r.Body.Close()
		if err != nil {
			writeSecurityError(w, http.StatusBadRequest, "failed to read request body")
			return
		}

		if len(rawBody) == 0 {
			writeSecurityError(w, http.StatusBadRequest, "request body cannot be empty")
			return
		}

		// Verify signature using the exact raw bytes
		if !v.Verify(signature, timestamp, rawBody) {
			writeSecurityError(w, http.StatusUnauthorized, "invalid webhook signature")
			return
		}

		// Restore r.Body for downstream handlers and inject rawBody into context
		r.Body = io.NopCloser(bytes.NewReader(rawBody))
		ctx := context.WithValue(r.Context(), RawBodyKey, rawBody)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// GetRawBody retrieves the verified raw request body from context, or reads it from r.Body.
func GetRawBody(r *http.Request) ([]byte, error) {
	if val := r.Context().Value(RawBodyKey); val != nil {
		if b, ok := val.([]byte); ok {
			return b, nil
		}
	}
	return io.ReadAll(r.Body)
}

func writeSecurityError(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(map[string]string{
		"status": "error",
		"error":  message,
	})
}
