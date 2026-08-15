package models

import "encoding/json"

// Supported event types from Cashfree webhook specifications.
const (
	EventPaymentSuccess     = "PAYMENT_SUCCESS_WEBHOOK"
	EventPaymentFailed      = "PAYMENT_FAILED_WEBHOOK"
	EventPaymentUserDropped = "PAYMENT_USER_DROPPED_WEBHOOK"
)

// WebhookEvent represents the comprehensive Cashfree webhook payload envelope.
// Operator Safety: JSON tags strictly map to Cashfree's payload specification.
type WebhookEvent struct {
	Type      string `json:"type"`
	EventTime string `json:"event_time"`
	Data      struct {
		Order struct {
			OrderID       string      `json:"order_id"`
			OrderAmount   float64     `json:"order_amount"`
			OrderCurrency string      `json:"order_currency"`
			OrderTags     interface{} `json:"order_tags,omitempty"`
		} `json:"order"`
		Payment struct {
			CfPaymentID    interface{}     `json:"cf_payment_id"`
			PaymentStatus  string          `json:"payment_status"`
			PaymentAmount  float64         `json:"payment_amount"`
			PaymentCurrency string         `json:"payment_currency"`
			PaymentMessage string          `json:"payment_message"`
			PaymentTime    string          `json:"payment_time"`
			BankReference  string          `json:"bank_reference"`
			PaymentGroup   string          `json:"payment_group"`
			// json.RawMessage defers parsing so polymorphic parser registry can inspect payment_group
			PaymentMethod  json.RawMessage `json:"payment_method"`
		} `json:"payment"`
		CustomerDetails struct {
			CustomerID    string `json:"customer_id"`
			CustomerName  string `json:"customer_name"`
			CustomerEmail string `json:"customer_email"`
			CustomerPhone string `json:"customer_phone"`
		} `json:"customer_details"`
		ErrorDetails struct {
			ErrorCode        string `json:"error_code,omitempty"`
			ErrorDescription  string `json:"error_description,omitempty"`
			ErrorReason      string `json:"error_reason,omitempty"`
			ErrorSource      string `json:"error_source,omitempty"`
		} `json:"error_details"`
	} `json:"data"`
}

// IsSupportedEvent checks whether the given event type is handled by this service.
func IsSupportedEvent(eventType string) bool {
	switch eventType {
	case EventPaymentSuccess, EventPaymentFailed, EventPaymentUserDropped:
		return true
	default:
		return false
	}
}
