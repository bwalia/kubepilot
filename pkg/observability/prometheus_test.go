package observability

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPrometheusClient_IsConfigured(t *testing.T) {
	client := NewPrometheusClient(PrometheusConfig{URL: ""})
	if client.IsConfigured() {
		t.Error("expected IsConfigured=false for empty URL")
	}

	client = NewPrometheusClient(PrometheusConfig{URL: "http://localhost:9090"})
	if !client.IsConfigured() {
		t.Error("expected IsConfigured=true for non-empty URL")
	}
}

func TestPrometheusClient_QuerySuccess(t *testing.T) {
	mockResp := QueryResult{
		Status: "success",
		Data: QueryResultData{
			ResultType: "vector",
			Result:     []json.RawMessage{},
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query" {
			t.Errorf("expected path /api/v1/query, got %s", r.URL.Path)
		}
		q := r.URL.Query().Get("query")
		if q == "" {
			t.Error("expected non-empty query parameter")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResp)
	}))
	defer server.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: server.URL, Timeout: 5 * time.Second})
	result, err := client.Query(context.Background(), "up")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Status != "success" {
		t.Errorf("expected status 'success', got %q", result.Status)
	}
}

func TestPrometheusClient_QueryFailedStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "error"})
	}))
	defer server.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: server.URL})
	_, err := client.Query(context.Background(), "up")
	if err == nil {
		t.Error("expected error for failed prometheus status")
	}
}

func TestPrometheusClient_QueryNoURL(t *testing.T) {
	client := NewPrometheusClient(PrometheusConfig{URL: ""})
	_, err := client.Query(context.Background(), "up")
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestPrometheusClient_QueryRangeNoURL(t *testing.T) {
	client := NewPrometheusClient(PrometheusConfig{URL: ""})
	_, err := client.QueryRange(context.Background(), "up", time.Now().Add(-1*time.Hour), time.Now(), time.Minute)
	if err == nil {
		t.Error("expected error for empty URL")
	}
}

func TestPrometheusClient_QueryRangeSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/query_range" {
			t.Errorf("expected path /api/v1/query_range, got %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(QueryResult{
			Status: "success",
			Data:   QueryResultData{ResultType: "matrix"},
		})
	}))
	defer server.Close()

	client := NewPrometheusClient(PrometheusConfig{URL: server.URL})
	result, err := client.QueryRange(context.Background(), "up", time.Now().Add(-1*time.Hour), time.Now(), time.Minute)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Data.ResultType != "matrix" {
		t.Errorf("expected resultType 'matrix', got %q", result.Data.ResultType)
	}
}

func TestPrometheusClient_DefaultTimeout(t *testing.T) {
	client := NewPrometheusClient(PrometheusConfig{URL: "http://localhost:9090"})
	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected default timeout 30s, got %v", client.httpClient.Timeout)
	}
}

func TestPromQLQueryConstants(t *testing.T) {
	queries := []struct {
		name  string
		query string
	}{
		{"PodCPUUsageQuery", PodCPUUsageQuery},
		{"PodMemoryUsageQuery", PodMemoryUsageQuery},
		{"PodRestartRateQuery", PodRestartRateQuery},
		{"NodeCPUUsageQuery", NodeCPUUsageQuery},
		{"NodeMemoryUsageQuery", NodeMemoryUsageQuery},
		{"HTTPErrorRateQuery", HTTPErrorRateQuery},
	}

	for _, tc := range queries {
		if tc.query == "" {
			t.Errorf("%s is empty", tc.name)
		}
	}
}
