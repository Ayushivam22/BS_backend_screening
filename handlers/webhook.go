package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"BS_backend_screening/pipeline"
	"BS_backend_screening/security"
)

// WebhookHandler handles incoming Cashfree webhook HTTP requests.
// Agnostic Design: Contains ZERO payment-method-specific branching.
type WebhookHandler struct {
	processor pipeline.EventProcessor
}

// NewWebhookHandler initializes a new WebhookHandler with the provided pipeline processor.
func NewWebhookHandler(processor pipeline.EventProcessor) *WebhookHandler {
	return &WebhookHandler{
		processor: processor,
	}
}

// ServeHTTP processes incoming webhook requests over HTTP.
// Defensive Programming:
// 1. Strictly enforces HTTP POST method.
// 2. Extracts raw body verified by upstream security middleware.
// 3. Delegates all processing to the generic pipeline.
// 4. Returns appropriate HTTP error codes without panicking.
func (h *WebhookHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(Response{
			Status: "error",
			Error:  fmt.Sprintf("method %s not allowed, use POST", r.Method),
		})
		return
	}

	// Retrieve raw request body (from context if placed by security middleware, or r.Body)
	rawBody, err := security.GetRawBody(r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(Response{
			Status: "error",
			Error:  "failed to read request body",
		})
		return
	}

	// Delegate event processing to generic pipeline
	normalizedEvent, err := h.processor.Process(r.Context(), rawBody)
	if err != nil {
		statusCode := http.StatusBadRequest
		if errors.Is(err, pipeline.ErrUnsupportedEvent) || strings.Contains(err.Error(), "unsupported payment group") {
			statusCode = http.StatusUnprocessableEntity
		}

		w.WriteHeader(statusCode)
		json.NewEncoder(w).Encode(Response{
			Status: "error",
			Error:  err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Status:  "success",
		Message: fmt.Sprintf("Webhook %s processed successfully", normalizedEvent.EventType),
		Data:    normalizedEvent,
	})
}

// RootHandler provides service status and diagnostic information.
func RootHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"service": "Cashfree Webhook Consumer",
		"status":  "healthy",
		"message": "Send signed POST requests to /webhook",
	})
}
