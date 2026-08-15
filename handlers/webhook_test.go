package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"BS_backend_screening/engine"
	"BS_backend_screening/pipeline"
	"BS_backend_screening/security"
)

const testSecret = "webhook_secret_key_12345"

func setupTestServer(t *testing.T) (http.Handler, *security.Verifier) {
	t.Helper()
	templateDir := filepath.Join("..", "templates")
	eng, err := engine.NewEngine(templateDir)
	if err != nil {
		t.Fatalf("failed to init template engine: %v", err)
	}

	pipe, err := pipeline.NewPipeline(eng)
	if err != nil {
		t.Fatalf("failed to init pipeline: %v", err)
	}

	verifier, err := security.NewVerifier(testSecret)
	if err != nil {
		t.Fatalf("failed to init verifier: %v", err)
	}

	handler := NewWebhookHandler(pipe)
	protectedHandler := verifier.Middleware(handler)

	return protectedHandler, verifier
}

func TestWebhookHandler_E2E_Success_Card(t *testing.T) {
	handler, verifier := setupTestServer(t)

	payload := []byte(`{
		"type": "PAYMENT_SUCCESS_WEBHOOK",
		"event_time": "2023-08-14T10:00:00+05:30",
		"data": {
			"order": {
				"order_id": "order_card_101",
				"order_amount": 2500.00,
				"order_currency": "INR"
			},
			"payment": {
				"cf_payment_id": "cf_card_999",
				"payment_status": "SUCCESS",
				"payment_amount": 2500.00,
				"payment_currency": "INR",
				"payment_group": "credit_card",
				"payment_method": {
					"card": {
						"channel": "link",
						"card_number": "XXXXXXXXXXXX4738",
						"card_network": "visa",
						"card_type": "credit_card",
						"card_bank_name": "HDFC Bank"
					}
				}
			},
			"customer_details": {
				"customer_email": "cardholder@hdfcbank.com",
				"customer_name": "Alice"
			}
		}
	}`)

	timestamp := "1691990400"
	sig := verifier.ComputeSignature(timestamp, payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(security.HeaderTimestamp, timestamp)
	req.Header.Set(security.HeaderSignature, sig)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("expected status 'success', got '%s'", resp.Status)
	}
	if resp.Data == nil || resp.Data.PaymentGroup != "credit_card" {
		t.Errorf("expected payment_group 'credit_card', got '%v'", resp.Data)
	}
	if resp.Data.OrderID != "order_card_101" {
		t.Errorf("expected order_id 'order_card_101', got '%s'", resp.Data.OrderID)
	}
}

func TestWebhookHandler_E2E_Success_UPI(t *testing.T) {
	handler, verifier := setupTestServer(t)

	payload := []byte(`{
		"type": "PAYMENT_SUCCESS_WEBHOOK",
		"event_time": "2023-08-14T10:00:00+05:30",
		"data": {
			"order": {
				"order_id": "order_upi_202",
				"order_amount": 750.00,
				"order_currency": "INR"
			},
			"payment": {
				"cf_payment_id": "cf_upi_888",
				"payment_status": "SUCCESS",
				"payment_amount": 750.00,
				"payment_currency": "INR",
				"payment_group": "upi",
				"payment_method": {
					"upi": {
						"channel": "collect",
						"upi_id": "merchant@okaxis"
					}
				}
			},
			"customer_details": {
				"customer_email": "payer@upi.com",
				"customer_name": "Bob"
			}
		}
	}`)

	timestamp := "1691990400"
	sig := verifier.ComputeSignature(timestamp, payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(security.HeaderTimestamp, timestamp)
	req.Header.Set(security.HeaderSignature, sig)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("expected status 'success', got '%s'", resp.Status)
	}
	if resp.Data == nil || resp.Data.PaymentGroup != "upi" {
		t.Errorf("expected payment_group 'upi', got '%v'", resp.Data)
	}
}

func TestWebhookHandler_E2E_Success_NetBanking(t *testing.T) {
	handler, verifier := setupTestServer(t)

	payload := []byte(`{
		"type": "PAYMENT_SUCCESS_WEBHOOK",
		"event_time": "2023-08-14T10:00:00+05:30",
		"data": {
			"order": {
				"order_id": "order_nb_303",
				"order_amount": 12000.00,
				"order_currency": "INR"
			},
			"payment": {
				"cf_payment_id": "cf_nb_777",
				"payment_status": "SUCCESS",
				"payment_amount": 12000.00,
				"payment_currency": "INR",
				"payment_group": "net_banking",
				"payment_method": {
					"netbanking": {
						"channel": "nb",
						"netbanking_bank_name": "ICICI Bank"
					}
				}
			},
			"customer_details": {
				"customer_email": "corporate@icici.com",
				"customer_name": "Charlie"
			}
		}
	}`)

	timestamp := "1691990400"
	sig := verifier.ComputeSignature(timestamp, payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(security.HeaderTimestamp, timestamp)
	req.Header.Set(security.HeaderSignature, sig)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d. Body: %s", rec.Code, rec.Body.String())
	}

	var resp Response
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Status != "success" {
		t.Errorf("expected status 'success', got '%s'", resp.Status)
	}
	if resp.Data == nil || resp.Data.PaymentGroup != "net_banking" {
		t.Errorf("expected payment_group 'net_banking', got '%v'", resp.Data)
	}
}

func TestWebhookHandler_E2E_InvalidSignature(t *testing.T) {
	handler, _ := setupTestServer(t)

	payload := []byte(`{"type": "PAYMENT_SUCCESS_WEBHOOK", "data": {"payment": {"payment_group": "upi"}}}`)
	timestamp := "1691990400"

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set(security.HeaderTimestamp, timestamp)
	req.Header.Set(security.HeaderSignature, "invalid_forged_signature")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401 Unauthorized for forged signature, got %d", rec.Code)
	}
}

func TestWebhookHandler_E2E_UnsupportedGroup(t *testing.T) {
	handler, verifier := setupTestServer(t)

	payload := []byte(`{
		"type": "PAYMENT_SUCCESS_WEBHOOK",
		"data": {
			"payment": {
				"payment_group": "unsupported_foreign_method",
				"payment_method": {}
			}
		}
	}`)

	timestamp := "1691990400"
	sig := verifier.ComputeSignature(timestamp, payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set(security.HeaderTimestamp, timestamp)
	req.Header.Set(security.HeaderSignature, sig)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("expected status 422 Unprocessable Entity, got %d", rec.Code)
	}
}

func TestWebhookHandler_E2E_InvalidJSON(t *testing.T) {
	handler, verifier := setupTestServer(t)

	payload := []byte(`{invalid_malformed_json}`)
	timestamp := "1691990400"
	sig := verifier.ComputeSignature(timestamp, payload)

	req := httptest.NewRequest(http.MethodPost, "/webhook", bytes.NewReader(payload))
	req.Header.Set(security.HeaderTimestamp, timestamp)
	req.Header.Set(security.HeaderSignature, sig)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status 400 Bad Request for malformed JSON, got %d", rec.Code)
	}
}

func TestWebhookHandler_E2E_MethodNotAllowed(t *testing.T) {
	handler, _ := setupTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/webhook", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405 Method Not Allowed, got %d", rec.Code)
	}
}

func TestRootHandler(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	RootHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status 200 OK from RootHandler, got %d", rec.Code)
	}
}
