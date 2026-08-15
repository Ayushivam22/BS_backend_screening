package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"BS_backend_screening/engine"
	"BS_backend_screening/models"
	"BS_backend_screening/parser"
)

// Pipeline error definitions.
var (
	ErrEmptyPayload         = errors.New("webhook payload cannot be empty")
	ErrInvalidJSON          = errors.New("malformed webhook JSON payload")
	ErrMissingEventType     = errors.New("missing event type in webhook payload")
	ErrUnsupportedEvent     = errors.New("unsupported event type")
	ErrMissingPaymentGroup  = errors.New("missing payment_group in webhook payload")
)

// EventProcessor defines the contract for processing raw webhook payloads into normalized events.
// Extensibility: Decouples transport layers (HTTP handlers, CLI, async consumers) from business processing.
type EventProcessor interface {
	Process(ctx context.Context, rawPayload []byte) (*models.NormalizedEvent, error)
}

// Pipeline orchestrates the linear webhook processing lifecycle.
type Pipeline struct {
	templateEngine *engine.TemplateEngine
}

// NewPipeline creates a new generic Pipeline with the provided template engine.
// Operator Safety: Fails fast if the template engine is nil.
func NewPipeline(templateEngine *engine.TemplateEngine) (*Pipeline, error) {
	if templateEngine == nil {
		return nil, errors.New("templateEngine cannot be nil")
	}
	return &Pipeline{
		templateEngine: templateEngine,
	}, nil
}

// Process executes the linear processing pipeline:
// Step 1: Validate payload presence
// Step 2: Parse outer envelope & Identify Event Type (Success, Failed, User Dropped)
// Step 3: Identify Payment Group (credit_card, upi, net_banking, etc.)
// Step 4: Interpret Polymorphic Payment Method via Parser Registry
// Step 5: Produce Flat Normalized Event via External Template Engine
//
// Defensive Programming & Extensibility:
// - Zero hardcoded payment-method logic: completely agnostic to specific payment groups.
// - Returns typed errors for clean HTTP status code mapping upstream.
func (p *Pipeline) Process(ctx context.Context, rawPayload []byte) (*models.NormalizedEvent, error) {
	if len(rawPayload) == 0 {
		return nil, ErrEmptyPayload
	}

	// Step 2: Identify Event and Outer Envelope
	var webhook models.WebhookEvent
	if err := json.Unmarshal(rawPayload, &webhook); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidJSON, err)
	}

	eventType := strings.TrimSpace(webhook.Type)
	if eventType == "" {
		return nil, ErrMissingEventType
	}

	if !models.IsSupportedEvent(eventType) {
		return nil, fmt.Errorf("%w: '%s'", ErrUnsupportedEvent, eventType)
	}

	// Step 3: Identify Payment Group
	paymentGroup := strings.TrimSpace(webhook.Data.Payment.PaymentGroup)
	if paymentGroup == "" {
		return nil, ErrMissingPaymentGroup
	}

	// Step 4: Interpret Polymorphic Payment Method via Registry
	rawMethodBytes := webhook.Data.Payment.PaymentMethod
	parsedMethod, err := parser.ParsePayment(paymentGroup, rawMethodBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to interpret payment method: %w", err)
	}

	// Step 5: Produce Normalized Event via Template Engine
	normalizedEvent, err := p.templateEngine.Transform(rawPayload, paymentGroup, eventType, parsedMethod)
	if err != nil {
		return nil, fmt.Errorf("template transformation failed: %w", err)
	}

	return normalizedEvent, nil
}
