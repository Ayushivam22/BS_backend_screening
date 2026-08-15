package main

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultBaseURL = "http://localhost:8080"
	defaultSecret  = "dev_webhook_secret_key_change_in_prod"
)

type TestCase struct {
	Name            string
	Method          string
	Path            string
	Payload         []byte
	CustomHeaders   map[string]string
	OmitSignature   bool
	OmitTimestamp   bool
	CorruptSig      bool
	TamperBody      bool
	ExpectedStatus  int
	ValidateContent func(t *TestCase, body map[string]interface{}) error
}

func computeSignature(secret, ts string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(ts))
	mac.Write(body)
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func main() {
	baseURL := defaultBaseURL
	secret := defaultSecret
	if len(os.Args) > 1 && os.Args[1] != "" {
		baseURL = os.Args[1]
	}

	cardPayload, err := os.ReadFile("samples/payment_success_card.json")
	if err != nil {
		cardPayload, _ = os.ReadFile("../../samples/payment_success_card.json")
	}

	upiPayload, err := os.ReadFile("samples/payment_failed_upi.json")
	if err != nil {
		upiPayload, _ = os.ReadFile("../../samples/payment_failed_upi.json")
	}

	nbPayload, err := os.ReadFile("samples/payment_dropped_netbanking.json")
	if err != nil {
		nbPayload, _ = os.ReadFile("../../samples/payment_dropped_netbanking.json")
	}

	testCases := []TestCase{
		{
			Name:           "Feature 1: Health Check (GET /)",
			Method:         "GET",
			Path:           "/",
			ExpectedStatus: http.StatusOK,
			ValidateContent: func(t *TestCase, b map[string]interface{}) error {
				if b["status"] != "healthy" {
					return fmt.Errorf("expected status 'healthy', got %v", b["status"])
				}
				return nil
			},
		},
		{
			Name:           "Feature 1: Valid Credit Card Webhook (HMAC verified)",
			Method:         "POST",
			Path:           "/webhook",
			Payload:        cardPayload,
			ExpectedStatus: http.StatusOK,
			ValidateContent: func(t *TestCase, b map[string]interface{}) error {
				data, ok := b["data"].(map[string]interface{})
				if !ok || data["payment_group"] != "credit_card" {
					return fmt.Errorf("expected payment_group credit_card, got %v", b)
				}
				if data["order_id"] != "order_card_101" {
					return fmt.Errorf("expected order_id order_card_101, got %v", data["order_id"])
				}
				return nil
			},
		},
		{
			Name:           "Feature 2: Valid UPI Webhook (HMAC verified)",
			Method:         "POST",
			Path:           "/webhook",
			Payload:        upiPayload,
			ExpectedStatus: http.StatusOK,
			ValidateContent: func(t *TestCase, b map[string]interface{}) error {
				data, ok := b["data"].(map[string]interface{})
				if !ok || data["payment_group"] != "upi" {
					return fmt.Errorf("expected payment_group upi, got %v", b)
				}
				return nil
			},
		},
		{
			Name:           "Feature 2: Valid Net Banking Webhook (HMAC verified)",
			Method:         "POST",
			Path:           "/webhook",
			Payload:        nbPayload,
			ExpectedStatus: http.StatusOK,
			ValidateContent: func(t *TestCase, b map[string]interface{}) error {
				data, ok := b["data"].(map[string]interface{})
				if !ok || data["payment_group"] != "net_banking" {
					return fmt.Errorf("expected payment_group net_banking, got %v", b)
				}
				return nil
			},
		},
		{
			Name:           "Security: Missing Signature Header (Expect 401)",
			Method:         "POST",
			Path:           "/webhook",
			Payload:        cardPayload,
			OmitSignature:  true,
			ExpectedStatus: http.StatusUnauthorized,
		},
		{
			Name:           "Security: Missing Timestamp Header (Expect 401)",
			Method:         "POST",
			Path:           "/webhook",
			Payload:        cardPayload,
			OmitTimestamp:  true,
			ExpectedStatus: http.StatusUnauthorized,
		},
		{
			Name:           "Security: Forged Signature (Expect 401)",
			Method:         "POST",
			Path:           "/webhook",
			Payload:        cardPayload,
			CorruptSig:     true,
			ExpectedStatus: http.StatusUnauthorized,
		},
		{
			Name:           "Security: Tampered Body (Expect 401)",
			Method:         "POST",
			Path:           "/webhook",
			Payload:        cardPayload,
			TamperBody:     true,
			ExpectedStatus: http.StatusUnauthorized,
		},
		{
			Name:           "Pipeline: Unsupported Event Type (Expect 422)",
			Method:         "POST",
			Path:           "/webhook",
			Payload:        []byte(`{"type":"UNKNOWN_EVENT","data":{"payment":{"payment_group":"upi","payment_method":{"upi":{}}}}}`),
			ExpectedStatus: http.StatusUnprocessableEntity,
		},
		{
			Name:           "Pipeline: Unsupported Payment Group (Expect 422)",
			Method:         "POST",
			Path:           "/webhook",
			Payload:        []byte(`{"type":"PAYMENT_SUCCESS_WEBHOOK","data":{"payment":{"payment_group":"crypto_unsupported","payment_method":{}}}}`),
			ExpectedStatus: http.StatusUnprocessableEntity,
		},
		{
			Name:           "Pipeline: Empty Body (Expect 400)",
			Method:         "POST",
			Path:           "/webhook",
			Payload:        []byte(``),
			ExpectedStatus: http.StatusBadRequest,
		},
		{
			Name:           "Pipeline: Malformed JSON (Expect 400)",
			Method:         "POST",
			Path:           "/webhook",
			Payload:        []byte(`{malformed_json}`),
			ExpectedStatus: http.StatusBadRequest,
		},
		{
			Name:           "Transport: Method Not Allowed GET /webhook (Expect 405)",
			Method:         "GET",
			Path:           "/webhook",
			ExpectedStatus: http.StatusMethodNotAllowed,
		},
	}

	fmt.Println("╔═════════════════════════════════════════════════════════════════════════════╗")
	fmt.Println("║     CASHFREE WEBHOOK ENDPOINT TEST SUITE (HMAC-SHA256 VERIFIED)             ║")
	fmt.Printf("║     Target Server: %-56s ║\n", baseURL)
	fmt.Println("╚═════════════════════════════════════════════════════════════════════════════╝")
	fmt.Println()

	client := &http.Client{Timeout: 5 * time.Second}
	passed := 0
	failed := 0

	for i, tc := range testCases {
		ts := strconv.FormatInt(time.Now().Unix(), 10)
		bodyToSend := tc.Payload
		sig := computeSignature(secret, ts, bodyToSend)

		if tc.TamperBody {
			bodyToSend = []byte(string(tc.Payload) + " ")
		}

		req, err := http.NewRequest(tc.Method, baseURL+tc.Path, bytes.NewReader(bodyToSend))
		if err != nil {
			fmt.Printf("❌ [%02d] %-55s → Request Error: %v\n", i+1, tc.Name, err)
			failed++
			continue
		}

		req.Header.Set("Content-Type", "application/json")
		if !tc.OmitTimestamp {
			req.Header.Set("x-webhook-timestamp", ts)
		}
		if !tc.OmitSignature {
			if tc.CorruptSig {
				req.Header.Set("x-webhook-signature", "invalid_forged_hmac_signature_value")
			} else {
				req.Header.Set("x-webhook-signature", sig)
			}
		}

		resp, err := client.Do(req)
		if err != nil {
			fmt.Printf("❌ [%02d] %-55s → Connection Failed: %v\n", i+1, tc.Name, err)
			failed++
			continue
		}
		respBody, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode != tc.ExpectedStatus {
			fmt.Printf("❌ [%02d] %-55s → Expected HTTP %d, got %d. Resp: %s\n",
				i+1, tc.Name, tc.ExpectedStatus, resp.StatusCode, strings.TrimSpace(string(respBody)))
			failed++
			continue
		}

		var jsonResp map[string]interface{}
		_ = json.Unmarshal(respBody, &jsonResp)

		if tc.ValidateContent != nil {
			if err := tc.ValidateContent(&tc, jsonResp); err != nil {
				fmt.Printf("❌ [%02d] %-55s → Content Validation Error: %v\n", i+1, tc.Name, err)
				failed++
				continue
			}
		}

		fmt.Printf("✅ [%02d] %-55s → HTTP %d (Passed)\n", i+1, tc.Name, resp.StatusCode)
		passed++
	}

	fmt.Println("───────────────────────────────────────────────────────────────────────────────")
	fmt.Printf("  SUMMARY: %d Passed, %d Failed (Total %d Tests)\n", passed, failed, len(testCases))
	fmt.Println("───────────────────────────────────────────────────────────────────────────────")
}
