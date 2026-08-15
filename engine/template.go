package engine

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"BS_backend_screening/models"
)

// TemplateDefinition defines an external mapping schema from raw payload fields to a normalized event.
type TemplateDefinition struct {
	TemplateID   string            `json:"template_id"`
	PaymentGroup string            `json:"payment_group"`
	EventType    string            `json:"event_type"`
	Mappings     map[string]string `json:"mappings"`
}

// Validate validates the structural and semantic integrity of the template definition.
// Operator Safety: Enforces required fields at startup to fail-fast before serving live traffic.
func (t *TemplateDefinition) Validate() error {
	if strings.TrimSpace(t.TemplateID) == "" {
		return errors.New("template_id cannot be empty")
	}
	if strings.TrimSpace(t.PaymentGroup) == "" {
		return fmt.Errorf("template '%s': payment_group cannot be empty", t.TemplateID)
	}
	if len(t.Mappings) == 0 {
		return fmt.Errorf("template '%s': mappings cannot be empty", t.TemplateID)
	}
	for target, src := range t.Mappings {
		if strings.TrimSpace(target) == "" || strings.TrimSpace(src) == "" {
			return fmt.Errorf("template '%s': mapping cannot contain empty key or path (target: '%s', source: '%s')", t.TemplateID, target, src)
		}
	}
	return nil
}

// TemplateEngine manages loaded transformation templates and applies them to payloads.
type TemplateEngine struct {
	mu        sync.RWMutex
	templates map[string]TemplateDefinition // key: paymentGroup:eventType
	fallback  *TemplateDefinition
}

// NewEngine creates and initializes a TemplateEngine from the specified directory.
// Operator Safety: Reads all templates at startup and strictly rejects missing/malformed templates.
func NewEngine(templateDir string) (*TemplateEngine, error) {
	engine := &TemplateEngine{
		templates: make(map[string]TemplateDefinition),
	}

	info, err := os.Stat(templateDir)
	if err != nil {
		return nil, fmt.Errorf("template directory error: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("template path '%s' is not a directory", templateDir)
	}

	loadedCount := 0
	err = filepath.WalkDir(templateDir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(strings.ToLower(d.Name()), ".json") {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read template file '%s': %w", path, err)
		}

		var tmpl TemplateDefinition
		if err := json.Unmarshal(data, &tmpl); err != nil {
			return fmt.Errorf("malformed JSON in template file '%s': %w", path, err)
		}

		if err := tmpl.Validate(); err != nil {
			return fmt.Errorf("invalid template in '%s': %w", path, err)
		}

		engine.RegisterTemplate(tmpl)
		loadedCount++
		return nil
	})

	if err != nil {
		return nil, err
	}

	if loadedCount == 0 {
		return nil, fmt.Errorf("no valid JSON templates found in '%s'", templateDir)
	}

	return engine, nil
}

// NewEmptyEngine creates an empty TemplateEngine (useful for unit tests with dynamic registrations).
func NewEmptyEngine() *TemplateEngine {
	return &TemplateEngine{
		templates: make(map[string]TemplateDefinition),
	}
}

// RegisterTemplate registers a template definition dynamically.
// Extensibility: Enables adding new templates at runtime or in tests.
func (e *TemplateEngine) RegisterTemplate(tmpl TemplateDefinition) {
	e.mu.Lock()
	defer e.mu.Unlock()

	key := makeKey(tmpl.PaymentGroup, tmpl.EventType)
	e.templates[key] = tmpl

	if tmpl.PaymentGroup == "*" || tmpl.TemplateID == "default_fallback_template" {
		e.fallback = &tmpl
	}
}

// GetTemplate finds the best matching template for the given payment group and event type.
func (e *TemplateEngine) GetTemplate(paymentGroup, eventType string) (*TemplateDefinition, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	// 1. Exact match: group + eventType
	if tmpl, ok := e.templates[makeKey(paymentGroup, eventType)]; ok {
		return &tmpl, nil
	}

	// 2. Group match with wildcard event: group + *
	if tmpl, ok := e.templates[makeKey(paymentGroup, "*")]; ok {
		return &tmpl, nil
	}

	// 3. Wildcard group with event: * + eventType
	if tmpl, ok := e.templates[makeKey("*", eventType)]; ok {
		return &tmpl, nil
	}

	// 4. Global fallback
	if e.fallback != nil {
		return e.fallback, nil
	}

	return nil, fmt.Errorf("no matching template found for group '%s' and event '%s'", paymentGroup, eventType)
}

// Transform executes the external template mapping against the raw JSON payload.
// Defensive Programming:
// 1. Unmarshals payload into a generic map for flexible dot-notation traversal.
// 2. Resolves nested fields safely without panicking if fields are missing or null.
// 3. Normalizes known fields while retaining all raw mapped fields for consumer flexibility.
func (e *TemplateEngine) Transform(rawPayload []byte, paymentGroup, eventType string, parsedMethod models.PaymentMethod) (*models.NormalizedEvent, error) {
	tmpl, err := e.GetTemplate(paymentGroup, eventType)
	if err != nil {
		return nil, err
	}

	var rootMap map[string]interface{}
	if err := json.Unmarshal(rawPayload, &rootMap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal payload for template transform: %w", err)
	}

	mappedFields := make(map[string]interface{})
	for targetKey, srcPath := range tmpl.Mappings {
		val := resolvePath(rootMap, srcPath)
		if val != nil {
			mappedFields[targetKey] = val
		}
	}

	// Construct standardized NormalizedEvent
	norm := &models.NormalizedEvent{
		EventType:       getString(mappedFields, "event_type", eventType),
		EventTime:       getString(mappedFields, "event_time", ""),
		OrderID:         getString(mappedFields, "order_id", ""),
		PaymentID:       getString(mappedFields, "payment_id", ""),
		PaymentGroup:    getString(mappedFields, "payment_group", paymentGroup),
		PaymentStatus:   getString(mappedFields, "payment_status", ""),
		Amount:          getFloat64(mappedFields, "amount"),
		Currency:        getString(mappedFields, "currency", ""),
		CustomerEmail:   getString(mappedFields, "customer_email", ""),
		PaymentDetails:  parsedMethod,
		RawMappedFields: mappedFields,
	}

	return norm, nil
}

// resolvePath traverses a nested map[string]interface{} safely using dot-notation (e.g. data.payment.cf_payment_id).
// Defensive Programming: Safely handles non-map intermediaries, missing keys, and nil slices without panicking.
func resolvePath(data map[string]interface{}, path string) interface{} {
	if data == nil || path == "" {
		return nil
	}

	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok || m == nil {
			return nil
		}
		val, exists := m[part]
		if !exists || val == nil {
			return nil
		}
		current = val
	}

	return current
}

func makeKey(group, eventType string) string {
	if eventType == "" {
		eventType = "*"
	}
	return strings.ToLower(strings.TrimSpace(group)) + ":" + strings.ToUpper(strings.TrimSpace(eventType))
}

func getString(m map[string]interface{}, key string, defaultVal string) string {
	if val, ok := m[key]; ok && val != nil {
		switch v := val.(type) {
		case string:
			return v
		default:
			return fmt.Sprintf("%v", v)
		}
	}
	return defaultVal
}

func getFloat64(m map[string]interface{}, key string) float64 {
	if val, ok := m[key]; ok && val != nil {
		switch v := val.(type) {
		case float64:
			return v
		case float32:
			return float64(v)
		case int:
			return float64(v)
		case int64:
			return float64(v)
		case string:
			if f, err := strconv.ParseFloat(v, 64); err == nil {
				return f
			}
		}
	}
	return 0.0
}
