package handlers

import "BS_backend_screening/models"

// Response represents the standard JSON API response structure.
type Response struct {
	Status  string                  `json:"status"`
	Message string                  `json:"message,omitempty"`
	Data    *models.NormalizedEvent `json:"data,omitempty"`
	Error   string                  `json:"error,omitempty"`
}
