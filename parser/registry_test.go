package parser

import (
	"testing"

	"BS_backend_screening/models"
)

func TestParsePayment_Card(t *testing.T) {
	raw := []byte(`{
		"card": {
			"channel": "link",
			"card_number": "XXXXXXXXXXXX4738",
			"card_network": "visa",
			"card_type": "credit_card",
			"card_bank_name": "HDFC Bank"
		}
	}`)

	method, err := ParsePayment("credit_card", raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	card, ok := method.(*models.CardPayment)
	if !ok {
		t.Fatalf("expected *models.CardPayment, got %T", method)
	}
	if card.Card.BankName != "HDFC Bank" {
		t.Errorf("expected bank 'HDFC Bank', got '%s'", card.Card.BankName)
	}
}

func TestParsePayment_CustomRegistration(t *testing.T) {
	// Custom dummy struct implementing models.PaymentMethod
	type WalletPayment struct {
		Wallet struct {
			Provider string `json:"provider"`
		} `json:"wallet"`
	}

	// Register new method
	Register("wallet", func(data []byte) (models.PaymentMethod, error) {
		return &mockWallet{provider: "Paytm"}, nil
	})

	method, err := ParsePayment("wallet", []byte(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if method.GetPaymentGroup() != "wallet" {
		t.Errorf("expected group 'wallet', got '%s'", method.GetPaymentGroup())
	}
}

type mockWallet struct {
	provider string
}

func (m *mockWallet) GetPaymentGroup() string {
	return "wallet"
}
