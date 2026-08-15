package parser

import (
	"encoding/json"
	"fmt"
	"sync"

	"BS_backend_screening/models"
)

// ParserFunc defines the function blueprint for parsing raw JSON into a PaymentMethod.
type ParserFunc func([]byte) (models.PaymentMethod, error)

var (
	registryLock   sync.RWMutex
	methodRegistry = map[string]ParserFunc{
		"credit_card": func(data []byte) (models.PaymentMethod, error) {
			var c models.CardPayment
			err := json.Unmarshal(data, &c)
			return &c, err
		},
		"upi": func(data []byte) (models.PaymentMethod, error) {
			var u models.UPIPayment
			err := json.Unmarshal(data, &u)
			return &u, err
		},
		"net_banking": func(data []byte) (models.PaymentMethod, error) {
			var n models.NetBankingPayment
			err := json.Unmarshal(data, &n)
			return &n, err
		},
	}
)

// Register allows dynamic registration of new payment group parsers.
func Register(group string, parser ParserFunc) {
	registryLock.Lock()
	defer registryLock.Unlock()
	methodRegistry[group] = parser
}

// ParsePayment routes the raw JSON to the correct struct based on the payment group.
func ParsePayment(group string, rawData []byte) (models.PaymentMethod, error) {
	registryLock.RLock()
	parser, exists := methodRegistry[group]
	registryLock.RUnlock()

	if !exists {
		return nil, fmt.Errorf("unsupported payment group: %s", group)
	}
	return parser(rawData)
}
