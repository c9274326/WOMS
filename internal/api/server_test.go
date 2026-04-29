package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIngressAuthRejectsMissingToken(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	req := httptest.NewRequest(http.MethodGet, "/internal/auth/verify", nil)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", res.Code)
	}
}

func TestSalesCannotCreateScheduleJob(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	token := login(t, server, "sales", "demo")
	body := bytes.NewBufferString(`{"lineId":"A","startDate":"2026-05-01"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/schedules/jobs", body)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestSchedulerCannotReadAnotherLineJob(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	salesToken := login(t, server, "sales", "demo")
	createOrder(t, server, salesToken, "B")

	schedulerB := login(t, server, "scheduler-b", "demo")
	jobID := createScheduleJob(t, server, schedulerB, "B")

	schedulerA := login(t, server, "scheduler-a", "demo")
	req := httptest.NewRequest(http.MethodGet, "/api/schedules/jobs/"+jobID, nil)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", res.Code, res.Body.String())
	}
}

func login(t *testing.T, server *Server, username, password string) string {
	t.Helper()
	body := bytes.NewBufferString(`{"username":"` + username + `","password":"` + password + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", res.Code, res.Body.String())
	}
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode login response: %v", err)
	}
	return payload.Token
}

func createOrder(t *testing.T, server *Server, token, lineID string) {
	t.Helper()
	body := bytes.NewBufferString(`{"customer":"ACME","lineId":"` + lineID + `","quantity":2500,"priority":"low","dueDate":"2026-05-03"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/orders", body)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("create order failed: %d %s", res.Code, res.Body.String())
	}
}

func createScheduleJob(t *testing.T, server *Server, token, lineID string) string {
	t.Helper()
	body := bytes.NewBufferString(`{"lineId":"` + lineID + `","startDate":"2026-05-01"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/schedules/jobs", body)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("create schedule job failed: %d %s", res.Code, res.Body.String())
	}
	var payload struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode job response: %v", err)
	}
	return payload.ID
}
