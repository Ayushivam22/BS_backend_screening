package pipeline

import (
	"context"
	"encoding/json"
	"testing"

	"BS_backend_screening/engine"
	"BS_backend_screening/models"
	"BS_backend_screening/parser"
)

// =====================================================================
// EXTENSIBILITY TEST: Adding a new Payment Group & External Template
// Demonstrates that new payment groups can be introduced purely via runtime
// registration and template files without modifying any existing pipeline code.
// =====================================================================

// Step 1: Define the new concrete PaymentMethod struct
type CryptoPayment struct {
	Crypto struct {
		WalletAddress string `json:"wallet_address"`
		TxHash        string `json:"tx_hash"`
		Network       string `json:"network"`
	} `json:"crypto"`
}

func (c *CryptoPayment) GetPaymentGroup() string {
	return "crypto_wallet"
}

func TestExtensibility_AddNewPaymentMethodWithoutCoreChanges(t *testing.T) {
	// Step 2: Register the new payment group parser in the parser registry
	parser.Register("crypto_wallet", func(data []byte) (models.PaymentMethod, error) {
		var c CryptoPayment
		err := json.Unmarshal(data, &c)
		return &c, err
	})

	// Step 3: Register a new external template definition in the template engine
	templateEngine := engine.NewEmptyEngine()
	templateEngine.RegisterTemplate(engine.TemplateDefinition{
		TemplateID:   "crypto_wallet_template",
		PaymentGroup: "crypto_wallet",
		EventType:    "*",
		Mappings: map[string]string{
			"event_type":     "type",
			"order_id":       "data.order.order_id",
			"payment_id":     "data.payment.cf_payment_id",
			"payment_group":  "data.payment.payment_group",
			"amount":         "data.payment.payment_amount",
			"currency":       "data.payment.payment_currency",
			"wallet_address": "data.payment.payment_method.crypto.wallet_address",
			"tx_hash":        "data.payment.payment_method.crypto.tx_hash",
			"crypto_network": "data.payment.payment_method.crypto.network",
		},
	})

	// Step 4: Initialize pipeline with the template engine
	pipe, err := NewPipeline(templateEngine)
	if err != nil {
		t.Fatalf("failed to create pipeline: %v", err)
	}

	// Step 5: Process a live payload for the newly registered method
	cryptoPayload := []byte(`{
		"type": "PAYMENT_SUCCESS_WEBHOOK",
		"event_time": "2023-08-14T10:15:00+05:30",
		"data": {
			"order": {
				"order_id": "order_crypto_999",
				"order_amount": 0.05,
				"order_currency": "ETH"
			},
			"payment": {
				"cf_payment_id": "cf_crypto_001",
				"payment_status": "SUCCESS",
				"payment_amount": 0.05,
				"payment_currency": "ETH",
				"payment_group": "crypto_wallet",
				"payment_method": {
					"crypto": {
						"wallet_address": "0x71C...3a9",
						"tx_hash": "0xabc123456789",
						"network": "ethereum"
					}
				}
			}
		}
	}`)

	norm, err := pipe.Process(context.Background(), cryptoPayload)
	if err != nil {
		t.Fatalf("extensible pipeline processing failed: %v", err)
	}

	// Step 6: Verify normalized event output
	if norm.PaymentGroup != "crypto_wallet" {
		t.Errorf("expected payment_group 'crypto_wallet', got '%s'", norm.PaymentGroup)
	}
	if norm.RawMappedFields["wallet_address"] != "0x71C...3a9" {
		t.Errorf("expected wallet_address '0x71C...3a9', got '%v'", norm.RawMappedFields["wallet_address"])
	}
	if norm.RawMappedFields["tx_hash"] != "0xabc123456789" {
		t.Errorf("expected tx_hash '0xabc123456789', got '%v'", norm.RawMappedFields["tx_hash"])
	}
	if norm.RawMappedFields["crypto_network"] != "ethereum" {
		t.Errorf("expected crypto_network 'ethereum', got '%v'", norm.RawMappedFields["crypto_network"])
	}

	// Verify typed payment details polymorphism
	cryptoDetails, ok := norm.PaymentDetails.(*CryptoPayment)
	if !ok {
		t.Fatalf("expected *CryptoPayment type, got %T", norm.PaymentDetails)
	}
	if cryptoDetails.Crypto.WalletAddress != "0x71C...3a9" {
		t.Errorf("expected struct wallet address '0x71C...3a9', got '%s'", cryptoDetails.Crypto.WalletAddress)
	}
}
