package pipeline

import (
	"context"
	"path/filepath"
	"testing"

	"BS_backend_screening/engine"
	"BS_backend_screening/models"
)

func setupTestPipeline(t *testing.T) *Pipeline {
	t.Helper()
	templateDir := filepath.Join("..", "templates")
	eng, err := engine.NewEngine(templateDir)
	if err != nil {
		t.Fatalf("failed to init template engine: %v", err)
	}
	pipe, err := NewPipeline(eng)
	if err != nil {
		t.Fatalf("failed to init pipeline: %v", err)
	}
	return pipe
}

func TestPipeline_PaymentSuccess_Card(t *testing.T) {
	pipe := setupTestPipeline(t)

	payload := []byte(`{
		"type": "PAYMENT_SUCCESS_WEBHOOK",
		"event_time": "2023-08-14T10:00:05+05:30",
		"data": {
			"order": {
				"order_id": "order_123456",
				"order_amount": 1499.00,
				"order_currency": "INR"
			},
			"payment": {
				"cf_payment_id": "cf_pay_998877",
				"payment_status": "SUCCESS",
				"payment_amount": 1499.00,
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
				"customer_email": "john.doe@example.com",
				"customer_name": "John Doe"
			}
		}
	}`)

	norm, err := pipe.Process(context.Background(), payload)
	if err != nil {
		t.Fatalf("pipeline processing failed: %v", err)
	}

	if norm.EventType != models.EventPaymentSuccess {
		t.Errorf("expected event_type '%s', got '%s'", models.EventPaymentSuccess, norm.EventType)
	}
	if norm.PaymentGroup != "credit_card" {
		t.Errorf("expected payment_group 'credit_card', got '%s'", norm.PaymentGroup)
	}
	if norm.OrderID != "order_123456" {
		t.Errorf("expected order_id 'order_123456', got '%s'", norm.OrderID)
	}
	if norm.PaymentID != "cf_pay_998877" {
		t.Errorf("expected payment_id 'cf_pay_998877', got '%s'", norm.PaymentID)
	}
	if norm.CustomerEmail != "john.doe@example.com" {
		t.Errorf("expected email 'john.doe@example.com', got '%s'", norm.CustomerEmail)
	}
	if norm.Amount != 1499.00 {
		t.Errorf("expected amount 1499.00, got %f", norm.Amount)
	}

	card, ok := norm.PaymentDetails.(*models.CardPayment)
	if !ok {
		t.Fatalf("expected PaymentDetails to be *models.CardPayment, got %T", norm.PaymentDetails)
	}
	if card.Card.BankName != "HDFC Bank" {
		t.Errorf("expected bank 'HDFC Bank', got '%s'", card.Card.BankName)
	}
}

func TestPipeline_PaymentFailed_UPI(t *testing.T) {
	pipe := setupTestPipeline(t)

	payload := []byte(`{
		"type": "PAYMENT_FAILED_WEBHOOK",
		"event_time": "2023-08-14T10:05:00+05:30",
		"data": {
			"order": {
				"order_id": "order_789012",
				"order_amount": 500.00,
				"order_currency": "INR"
			},
			"payment": {
				"cf_payment_id": "cf_pay_334455",
				"payment_status": "FAILED",
				"payment_amount": 500.00,
				"payment_currency": "INR",
				"payment_group": "upi",
				"payment_method": {
					"upi": {
						"channel": "collect",
						"upi_id": "user@okicici"
					}
				}
			},
			"customer_details": {
				"customer_email": "jane@example.com"
			}
		}
	}`)

	norm, err := pipe.Process(context.Background(), payload)
	if err != nil {
		t.Fatalf("pipeline processing failed: %v", err)
	}

	if norm.EventType != models.EventPaymentFailed {
		t.Errorf("expected event_type '%s', got '%s'", models.EventPaymentFailed, norm.EventType)
	}
	if norm.PaymentGroup != "upi" {
		t.Errorf("expected payment_group 'upi', got '%s'", norm.PaymentGroup)
	}
	if norm.PaymentStatus != "FAILED" {
		t.Errorf("expected payment_status 'FAILED', got '%s'", norm.PaymentStatus)
	}
	if norm.RawMappedFields["upi_id"] != "user@okicici" {
		t.Errorf("expected upi_id 'user@okicici', got '%v'", norm.RawMappedFields["upi_id"])
	}
}

func TestPipeline_PaymentUserDropped_NetBanking(t *testing.T) {
	pipe := setupTestPipeline(t)

	payload := []byte(`{
		"type": "PAYMENT_USER_DROPPED_WEBHOOK",
		"event_time": "2023-08-14T10:10:00+05:30",
		"data": {
			"order": {
				"order_id": "order_556677",
				"order_amount": 2999.00,
				"order_currency": "INR"
			},
			"payment": {
				"cf_payment_id": "cf_pay_112233",
				"payment_status": "USER_DROPPED",
				"payment_amount": 2999.00,
				"payment_currency": "INR",
				"payment_group": "net_banking",
				"payment_method": {
					"netbanking": {
						"channel": "nb",
						"netbanking_bank_name": "State Bank of India"
					}
				}
			},
			"customer_details": {
				"customer_email": "user@sbi.com"
			}
		}
	}`)

	norm, err := pipe.Process(context.Background(), payload)
	if err != nil {
		t.Fatalf("pipeline processing failed: %v", err)
	}

	if norm.EventType != models.EventPaymentUserDropped {
		t.Errorf("expected event_type '%s', got '%s'", models.EventPaymentUserDropped, norm.EventType)
	}
	if norm.PaymentGroup != "net_banking" {
		t.Errorf("expected payment_group 'net_banking', got '%s'", norm.PaymentGroup)
	}
}

func TestPipeline_UnsupportedEvent(t *testing.T) {
	pipe := setupTestPipeline(t)

	payload := []byte(`{
		"type": "SUBSCRIPTION_STATUS_CHANGED",
		"data": {
			"payment": {
				"payment_group": "upi",
				"payment_method": {"upi": {}}
			}
		}
	}`)

	_, err := pipe.Process(context.Background(), payload)
	if err == nil {
		t.Fatal("expected error on unsupported event type, got nil")
	}
}

func TestPipeline_UnsupportedPaymentGroup(t *testing.T) {
	pipe := setupTestPipeline(t)

	payload := []byte(`{
		"type": "PAYMENT_SUCCESS_WEBHOOK",
		"data": {
			"payment": {
				"payment_group": "crypto_unsupported",
				"payment_method": {}
			}
		}
	}`)

	_, err := pipe.Process(context.Background(), payload)
	if err == nil {
		t.Fatal("expected error on unsupported payment group, got nil")
	}
}
