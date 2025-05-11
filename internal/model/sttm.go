package model

import "time"

type STTMResponse struct {
	Index float64 `json:"index"`
}

type STTMErrorResponse struct {
	Message    string        `json:"message"`
	RetryAfter time.Duration `json:"retry_after"`
}
