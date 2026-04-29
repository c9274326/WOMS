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

func TestIngressAuthAcceptsValidToken(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	token := login(t, server, "sales", "demo")
	req := httptest.NewRequest(http.MethodGet, "/internal/auth/verify", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d body=%s", res.Code, res.Body.String())
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

func TestSchedulerCannotCreateScheduleJobWithoutPreview(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	token := login(t, server, "scheduler-a", "demo")
	body := bytes.NewBufferString(`{"lineId":"A","startDate":"2026-05-01"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/schedules/jobs", body)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestOnlyAdminCanAssignUsers(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	salesToken := login(t, server, "sales", "demo")
	body := bytes.NewBufferString(`{"username":"scheduler-a","role":"scheduler","lineId":"B"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/users", body)
	req.Header.Set("Authorization", "Bearer "+salesToken)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d body=%s", res.Code, res.Body.String())
	}

	adminToken := login(t, server, "admin", "demo")
	body = bytes.NewBufferString(`{"username":"scheduler-a","role":"scheduler","lineId":"B"}`)
	req = httptest.NewRequest(http.MethodPatch, "/api/users", body)
	req.Header.Set("Authorization", "Bearer "+adminToken)
	res = httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestOrderValidationRejectsInvalidQuantity(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	token := login(t, server, "sales", "demo")
	body := bytes.NewBufferString(`{"customer":"ACME","lineId":"A","quantity":10,"priority":"low","dueDate":"2026-05-03"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/orders", body)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestSchedulerSeesOnlyAssignedLineOrders(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	salesToken := login(t, server, "sales", "demo")
	createOrder(t, server, salesToken, "A")
	createOrder(t, server, salesToken, "B")

	schedulerA := login(t, server, "scheduler-a", "demo")
	req := httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	var payload struct {
		Orders []struct {
			LineID string `json:"lineId"`
		} `json:"orders"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode orders response: %v", err)
	}
	if len(payload.Orders) != 1 || payload.Orders[0].LineID != "A" {
		t.Fatalf("expected only line A order, got %+v", payload.Orders)
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

func TestScheduleJobPersistsAllocationsAndCalendar(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	salesToken := login(t, server, "sales", "demo")
	createOrder(t, server, salesToken, "A")

	schedulerA := login(t, server, "scheduler-a", "demo")
	jobID := createScheduleJob(t, server, schedulerA, "A")
	if jobID == "" {
		t.Fatal("expected schedule job id")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/schedules/calendar?lineId=A&month=2026-05", nil)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	var payload struct {
		Allocations []struct {
			OrderID string `json:"orderId"`
			Status  string `json:"status"`
		} `json:"allocations"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode calendar response: %v", err)
	}
	if len(payload.Allocations) != 1 {
		t.Fatalf("expected one allocation, got %+v", payload.Allocations)
	}
	if payload.Allocations[0].Status != string("已排程") {
		t.Fatalf("expected scheduled status, got %+v", payload.Allocations[0])
	}
}

func TestScheduleCalendarExcludesOtherMonths(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	salesToken := login(t, server, "sales", "demo")
	createOrder(t, server, salesToken, "A")
	schedulerA := login(t, server, "scheduler-a", "demo")
	createScheduleJob(t, server, schedulerA, "A")

	req := httptest.NewRequest(http.MethodGet, "/api/schedules/calendar?lineId=A&month=2026-06", nil)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	var payload struct {
		Allocations []any `json:"allocations"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode calendar response: %v", err)
	}
	if len(payload.Allocations) != 0 {
		t.Fatalf("expected no allocations in other month, got %+v", payload.Allocations)
	}
}

func TestSchedulerCannotReadAnotherLineCalendar(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	salesToken := login(t, server, "sales", "demo")
	createOrder(t, server, salesToken, "B")
	schedulerB := login(t, server, "scheduler-b", "demo")
	createScheduleJob(t, server, schedulerB, "B")

	schedulerA := login(t, server, "scheduler-a", "demo")
	req := httptest.NewRequest(http.MethodGet, "/api/schedules/calendar?lineId=B&month=2026-05", nil)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d body=%s", res.Code, res.Body.String())
	}
}

func TestSalesConfirmsDraftPreviewIntoPendingOrder(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	salesToken := login(t, server, "sales", "demo")
	body := bytes.NewBufferString(`{"lineId":"A","startDate":"2026-05-01","draftOrder":{"customer":"Draft Co","lineId":"A","quantity":2500,"priority":"low","dueDate":"2026-05-03"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/schedules/preview", body)
	req.Header.Set("Authorization", "Bearer "+salesToken)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("preview failed: %d %s", res.Code, res.Body.String())
	}
	var preview struct {
		PreviewID string `json:"previewId"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}

	body = bytes.NewBufferString(`{"previewId":"` + preview.PreviewID + `"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/orders/preview-confirm", body)
	req.Header.Set("Authorization", "Bearer "+salesToken)
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("confirm preview failed: %d %s", res.Code, res.Body.String())
	}
}

func TestDeleteOrdersRemovesScheduledAllocation(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	salesToken := login(t, server, "sales", "demo")
	createOrder(t, server, salesToken, "A")
	schedulerA := login(t, server, "scheduler-a", "demo")
	createScheduleJob(t, server, schedulerA, "A")

	body := bytes.NewBufferString(`{"orderIds":["ORD-1"]}`)
	req := httptest.NewRequest(http.MethodDelete, "/api/orders", body)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("delete failed: %d %s", res.Code, res.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/schedules/calendar?lineId=A&month=2026-05", nil)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("calendar failed: %d %s", res.Code, res.Body.String())
	}
	var payload struct {
		Allocations []any `json:"allocations"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode calendar response: %v", err)
	}
	if len(payload.Allocations) != 0 {
		t.Fatalf("expected allocation removed, got %+v", payload.Allocations)
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
	previewID := createSchedulePreview(t, server, token, lineID)
	body := bytes.NewBufferString(`{"lineId":"` + lineID + `","startDate":"2026-05-01","previewId":"` + previewID + `"}`)
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

func createSchedulePreview(t *testing.T, server *Server, token, lineID string) string {
	t.Helper()
	body := bytes.NewBufferString(`{"lineId":"` + lineID + `","startDate":"2026-05-01"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/schedules/preview", body)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("create schedule preview failed: %d %s", res.Code, res.Body.String())
	}
	var payload struct {
		PreviewID string `json:"previewId"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	return payload.PreviewID
}
