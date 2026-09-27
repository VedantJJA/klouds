package api

import (
	"net/http"

	"github.com/vedant/klouds/internal/metrics"
)

// MetricsHandler serves system metrics to the admin dashboard.
type MetricsHandler struct {
	collector *metrics.Collector
}

// NewMetricsHandler creates a new metrics handler.
func NewMetricsHandler() *MetricsHandler {
	return &MetricsHandler{
		collector: metrics.NewCollector(),
	}
}

// GetSystemMetrics handles GET /api/admin/metrics/system
func (h *MetricsHandler) GetSystemMetrics(w http.ResponseWriter, r *http.Request) {
	m := h.collector.Collect()
	writeJSON(w, http.StatusOK, m)
}
