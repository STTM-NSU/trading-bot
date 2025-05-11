package model

import "time"

type STTMResponse struct {
	Indexes []float64 `json:"indexes"`
}

type STTMErrorResponse struct {
	Message    string        `json:"message"`
	RetryAfter time.Duration `json:"retry_after"`
}
