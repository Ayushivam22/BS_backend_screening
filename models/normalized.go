package models

// NormalizedEvent represents the standardized, flat output event produced by the pipeline.
// Extensibility: Standardized fields provide a consistent contract for downstream consumers,
// while RawMappedFields preserves arbitrary template-mapped key-value pairs.
type NormalizedEvent struct {
	EventID         string                 `json:"event_id,omitempty"`
	EventType       string                 `json:"event_type"`
	EventTime       string                 `json:"event_time,omitempty"`
	OrderID         string                 `json:"order_id,omitempty"`
	PaymentID       string                 `json:"payment_id,omitempty"`
	PaymentGroup    string                 `json:"payment_group"`
	PaymentStatus   string                 `json:"payment_status,omitempty"`
	Amount          float64                `json:"amount,omitempty"`
	Currency        string                 `json:"currency,omitempty"`
	CustomerEmail   string                 `json:"customer_email,omitempty"`
	PaymentDetails  interface{}            `json:"payment_details,omitempty"`
	RawMappedFields map[string]interface{} `json:"raw_mapped_fields,omitempty"`
}
