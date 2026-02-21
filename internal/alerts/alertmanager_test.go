package alerts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestFetchAlerts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v2/alerts" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("active") != "true" {
			t.Error("expected active=true query param")
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{
				"labels": {
					"alertname": "DependencyDown",
					"job": "svc-go",
					"dependency": "postgres",
					"severity": "critical",
					"type": "postgres"
				},
				"annotations": {
					"summary": "Dependency postgres is down"
				},
				"startsAt": "2026-02-08T10:00:00Z",
				"status": {"state": "active"}
			},
			{
				"labels": {
					"alertname": "DependencyHighLatency",
					"job": "svc-python",
					"dependency": "grpc-stub",
					"severity": "warning",
					"type": "grpc"
				},
				"annotations": {
					"summary": "High latency on grpc-stub"
				},
				"startsAt": "2026-02-08T10:05:00Z",
				"status": {"state": "active"}
			},
			{
				"labels": {
					"alertname": "SomeUnrelatedAlert",
					"severity": "info"
				},
				"annotations": {},
				"startsAt": "2026-02-08T10:10:00Z",
				"status": {"state": "active"}
			}
		]`))
	}))
	defer srv.Close()

	c := NewClient(Config{URL: srv.URL})
	alerts, err := c.FetchAlerts(context.Background())
	if err != nil {
		t.Fatalf("FetchAlerts() error: %v", err)
	}

	// Only 2 alerts should be mapped (the third has no job/dependency labels).
	if len(alerts) != 2 {
		t.Fatalf("got %d alerts, want 2", len(alerts))
	}

	a := alerts[0]
	if a.AlertName != "DependencyDown" {
		t.Errorf("AlertName = %q, want %q", a.AlertName, "DependencyDown")
	}
	if a.Service != "svc-go" {
		t.Errorf("Service = %q, want %q", a.Service, "svc-go")
	}
	if a.Dependency != "postgres" {
		t.Errorf("Dependency = %q, want %q", a.Dependency, "postgres")
	}
	if a.Severity != "critical" {
		t.Errorf("Severity = %q, want %q", a.Severity, "critical")
	}
	if a.State != "firing" {
		t.Errorf("State = %q, want %q", a.State, "firing")
	}
	if a.Summary != "Dependency postgres is down" {
		t.Errorf("Summary = %q", a.Summary)
	}
}

func TestFetchAlertsEmptyURL(t *testing.T) {
	c := NewClient(Config{URL: ""})
	alerts, err := c.FetchAlerts(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if alerts != nil {
		t.Errorf("expected nil alerts, got %d", len(alerts))
	}
}

func TestFetchAlertsBasicAuth(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	c := NewClient(Config{URL: srv.URL, Username: "admin", Password: "secret"})
	_, err := c.FetchAlerts(context.Background())
	if err != nil {
		t.Fatalf("FetchAlerts() error: %v", err)
	}
}

func TestFetchAlertsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`error`))
	}))
	defer srv.Close()

	c := NewClient(Config{URL: srv.URL})
	_, err := c.FetchAlerts(context.Background())
	if err == nil {
		t.Error("expected error for 500 response")
	}
}
