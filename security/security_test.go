package security

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

const testSecret = "test_webhook_secret_key_12345"

func TestNewVerifier_Validation(t *testing.T) {
	_, err := NewVerifier("")
	if err == nil {
		t.Fatal("expected error on empty secret, got nil")
	}

	_, err = NewVerifier("   ")
	if err == nil {
		t.Fatal("expected error on whitespace secret, got nil")
	}

	v, err := NewVerifier(testSecret)
	if err != nil {
		t.Fatalf("unexpected error creating verifier: %v", err)
	}
	if v == nil {
		t.Fatal("expected non-nil verifier")
	}
}

func TestSecurityMiddleware_ValidSignature(t *testing.T) {
	verifier, err := NewVerifier(testSecret)
	if err != nil {
		t.Fatalf("failed to create verifier: %v", err)
	}

	rawBody := []byte(`{"data":{"payment":{"payment_group":"credit_card"}}}`)
	timestamp := "1691990400"
	validSignature := verifier.ComputeSignature(timestamp, rawBody)

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		bodyFromCtx, err := GetRawBody(r)
		if err != nil {
			t.Fatalf("failed to retrieve body from context: %v", err)
		}
		if !bytes.Equal(bodyFromCtx, rawBody) {
			t.Errorf("expected body %s, got %s", string(rawBody), string(bodyFromCtx))
		}
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(rawBody))
	req.Header.Set(HeaderTimestamp, timestamp)
	req.Header.Set(HeaderSignature, validSignature)

	recorder := httptest.NewRecorder()
	verifier.Middleware(handler).ServeHTTP(recorder, req)

	if !called {
		t.Error("expected downstream handler to be called")
	}
	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}
}

func TestSecurityMiddleware_TamperedBody(t *testing.T) {
	verifier, _ := NewVerifier(testSecret)

	originalBody := []byte(`{"data":{"payment":{"payment_group":"credit_card"}}}`)
	tamperedBody := []byte(`{"data":{"payment":{"payment_group":"credit_card_TAMPERED"}}}`)
	timestamp := "1691990400"
	signatureForOriginal := verifier.ComputeSignature(timestamp, originalBody)

	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	})

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(tamperedBody))
	req.Header.Set(HeaderTimestamp, timestamp)
	req.Header.Set(HeaderSignature, signatureForOriginal)

	recorder := httptest.NewRecorder()
	verifier.Middleware(handler).ServeHTTP(recorder, req)

	if called {
		t.Error("expected downstream handler NOT to be called on tampered body")
	}
	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 Unauthorized, got %d", recorder.Code)
	}
}

func TestSecurityMiddleware_MissingHeaders(t *testing.T) {
	verifier, _ := NewVerifier(testSecret)
	rawBody := []byte(`{"data":{}}`)
	timestamp := "1691990400"
	signature := verifier.ComputeSignature(timestamp, rawBody)

	// Missing signature header
	req1 := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(rawBody))
	req1.Header.Set(HeaderTimestamp, timestamp)
	rec1 := httptest.NewRecorder()
	verifier.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rec1, req1)
	if rec1.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 on missing signature, got %d", rec1.Code)
	}

	// Missing timestamp header
	req2 := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(rawBody))
	req2.Header.Set(HeaderSignature, signature)
	rec2 := httptest.NewRecorder()
	verifier.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(rec2, req2)
	if rec2.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 on missing timestamp, got %d", rec2.Code)
	}
}

func TestSecurityMiddleware_WrongSecret(t *testing.T) {
	attackerVerifier, _ := NewVerifier("wrong_attacker_secret")
	serverVerifier, _ := NewVerifier(testSecret)

	rawBody := []byte(`{"data":{"payment":{"payment_group":"upi"}}}`)
	timestamp := "1691990400"
	invalidSignature := attackerVerifier.ComputeSignature(timestamp, rawBody)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(rawBody))
	req.Header.Set(HeaderTimestamp, timestamp)
	req.Header.Set(HeaderSignature, invalidSignature)

	recorder := httptest.NewRecorder()
	serverVerifier.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})).ServeHTTP(recorder, req)

	if recorder.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 on wrong secret, got %d", recorder.Code)
	}
}

func TestSecurityMiddleware_PassNonPOST(t *testing.T) {
	verifier, _ := NewVerifier(testSecret)
	called := false
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	recorder := httptest.NewRecorder()
	verifier.Middleware(handler).ServeHTTP(recorder, req)

	if !called {
		t.Error("expected non-POST request to pass through middleware")
	}
	if recorder.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", recorder.Code)
	}
}
