package engine

import (
	"os"
	"path/filepath"
	"testing"

	"BS_backend_screening/models"
)

func TestNewEngine_ValidDirectory(t *testing.T) {
	// Points to the project's real templates directory
	templateDir := filepath.Join("..", "templates")
	engine, err := NewEngine(templateDir)
	if err != nil {
		t.Fatalf("failed to initialize template engine from '%s': %v", templateDir, err)
	}

	tmpl, err := engine.GetTemplate("credit_card", "PAYMENT_SUCCESS_WEBHOOK")
	if err != nil {
		t.Fatalf("expected credit_card template to be loaded: %v", err)
	}
	if tmpl.TemplateID != "credit_card_template" {
		t.Errorf("expected template_id 'credit_card_template', got '%s'", tmpl.TemplateID)
	}
}

func TestNewEngine_NonExistentDirectory(t *testing.T) {
	_, err := NewEngine("non_existent_directory_xyz")
	if err == nil {
		t.Fatal("expected error for non-existent directory, got nil")
	}
}

func TestNewEngine_MalformedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	badFile := filepath.Join(tmpDir, "bad.json")
	if err := os.WriteFile(badFile, []byte("{ malformed json"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, err := NewEngine(tmpDir)
	if err == nil {
		t.Fatal("expected startup error on malformed JSON template, got nil")
	}
}

func TestNewEngine_InvalidTemplateSchema(t *testing.T) {
	tmpDir := t.TempDir()
	invalidFile := filepath.Join(tmpDir, "invalid.json")
	// Missing mappings
	invalidJSON := `{"template_id": "test", "payment_group": "upi", "mappings": {}}`
	if err := os.WriteFile(invalidFile, []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	_, err := NewEngine(tmpDir)
	if err == nil {
		t.Fatal("expected startup error on empty mappings, got nil")
	}
}

func TestTemplateEngine_TransformWithMissingFields(t *testing.T) {
	engine := NewEmptyEngine()
	engine.RegisterTemplate(TemplateDefinition{
		TemplateID:   "test_card",
		PaymentGroup: "credit_card",
		EventType:    "*",
		Mappings: map[string]string{
			"order_id":       "data.order.order_id",
			"missing_field":  "data.non_existent.nested.field",
			"card_network":   "data.payment.payment_method.card.card_network",
			"customer_email": "data.customer_details.customer_email",
		},
	})

	// Payload with missing order and customer_details
	partialPayload := []byte(`{
		"type": "PAYMENT_SUCCESS_WEBHOOK",
		"data": {
			"payment": {
				"payment_group": "credit_card",
				"payment_method": {
					"card": {
						"card_network": "visa"
					}
				}
			}
		}
	}`)

	card := &models.CardPayment{}
	card.Card.CardNetwork = "visa"

	norm, err := engine.Transform(partialPayload, "credit_card", "PAYMENT_SUCCESS_WEBHOOK", card)
	if err != nil {
		t.Fatalf("unexpected transform error: %v", err)
	}

	if norm == nil {
		t.Fatal("expected non-nil normalized event")
	}

	// Missing field should not be present in mapped fields and should not panic
	if _, exists := norm.RawMappedFields["missing_field"]; exists {
		t.Error("expected missing_field to be omitted")
	}
	if norm.RawMappedFields["card_network"] != "visa" {
		t.Errorf("expected card_network 'visa', got '%v'", norm.RawMappedFields["card_network"])
	}
}

func TestResolvePath(t *testing.T) {
	nestedData := map[string]interface{}{
		"a": map[string]interface{}{
			"b": map[string]interface{}{
				"c": "deep_value",
				"d": 42.0,
			},
		},
	}

	// Valid path
	if val := resolvePath(nestedData, "a.b.c"); val != "deep_value" {
		t.Errorf("expected 'deep_value', got '%v'", val)
	}

	// Non-existent intermediate
	if val := resolvePath(nestedData, "a.missing.c"); val != nil {
		t.Errorf("expected nil for missing key, got '%v'", val)
	}

	// Empty path
	if val := resolvePath(nestedData, ""); val != nil {
		t.Errorf("expected nil for empty path, got '%v'", val)
	}

	// Nil data
	if val := resolvePath(nil, "a.b"); val != nil {
		t.Errorf("expected nil for nil data, got '%v'", val)
	}
}
