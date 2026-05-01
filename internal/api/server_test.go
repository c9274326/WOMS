package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/c9274326/woms/internal/domain"
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

func TestScheduleCalendarIncludesVisibleAdjacentMonthDays(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	salesToken := login(t, server, "sales", "demo")
	createOrder(t, server, salesToken, "A")
	schedulerA := login(t, server, "scheduler-a", "demo")
	createScheduleJob(t, server, schedulerA, "A")

	req := httptest.NewRequest(http.MethodGet, "/api/schedules/calendar?lineId=A&month=2026-04", nil)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res := httptest.NewRecorder()

	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", res.Code, res.Body.String())
	}
	var payload struct {
		Allocations []struct {
			OrderID string `json:"orderId"`
		} `json:"allocations"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode calendar response: %v", err)
	}
	if len(payload.Allocations) != 1 || payload.Allocations[0].OrderID != "ORD-1" {
		t.Fatalf("expected May 1 allocation on April calendar page, got %+v", payload.Allocations)
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

func TestSchedulePreviewRespectsRequestedFutureStart(t *testing.T) {
	store := NewMemoryStore()
	server := NewServer("secret", store)
	salesToken := login(t, server, "sales", "demo")
	body := bytes.NewBufferString(`{"customer":"ACME","lineId":"A","quantity":2500,"priority":"low","dueDate":"2026-05-01"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/orders", body)
	req.Header.Set("Authorization", "Bearer "+salesToken)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("create order failed: %d %s", res.Code, res.Body.String())
	}

	store.mu.Lock()
	store.allocations = append(store.allocations, domain.ScheduleAllocation{
		OrderID:  "EXISTING-APR30",
		LineID:   "A",
		Date:     mustAPIDate(t, "2026-04-30"),
		Quantity: 7710,
		Priority: domain.PriorityLow,
	})
	store.mu.Unlock()

	schedulerA := login(t, server, "scheduler-a", "demo")
	body = bytes.NewBufferString(`{"lineId":"A","startDate":"2026-05-01","currentDate":"2026-04-30","orderIds":["ORD-1"]}`)
	req = httptest.NewRequest(http.MethodPost, "/api/schedules/preview", body)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("preview failed: %d %s", res.Code, res.Body.String())
	}
	var payload struct {
		Allocations []struct {
			Date     time.Time `json:"date"`
			Quantity int       `json:"quantity"`
		} `json:"allocations"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	if len(payload.Allocations) != 1 {
		t.Fatalf("expected one allocation on requested future start, got %+v", payload.Allocations)
	}
	if payload.Allocations[0].Date.Format(dateLayout) != "2026-05-01" || payload.Allocations[0].Quantity != 2500 {
		t.Fatalf("expected full allocation on 2026-05-01, got %+v", payload.Allocations[0])
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

func TestSalesDraftPreviewDoesNotScheduleOtherPendingOrders(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	salesToken := login(t, server, "sales", "demo")
	createOrder(t, server, salesToken, "A")

	body := bytes.NewBufferString(`{"lineId":"A","startDate":"2026-05-01","draftOrder":{"customer":"Draft Co","lineId":"A","quantity":2500,"priority":"low","dueDate":"2026-05-03"}}`)
	req := httptest.NewRequest(http.MethodPost, "/api/schedules/preview", body)
	req.Header.Set("Authorization", "Bearer "+salesToken)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("preview failed: %d %s", res.Code, res.Body.String())
	}
	var payload struct {
		Allocations []struct {
			OrderID string `json:"orderId"`
		} `json:"allocations"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	if len(payload.Allocations) == 0 {
		t.Fatal("expected draft allocation")
	}
	for _, allocation := range payload.Allocations {
		if allocation.OrderID != "PREVIEW-DRAFT" {
			t.Fatalf("draft preview should not include existing pending orders, got %+v", payload.Allocations)
		}
	}
}

func TestManualForceConflictCanCreateScheduleJobWithAudit(t *testing.T) {
	store := NewMemoryStore()
	server := NewServer("secret", store)
	salesToken := login(t, server, "sales", "demo")
	createOrderWithPriority(t, server, salesToken, "A", "low")
	schedulerA := login(t, server, "scheduler-a", "demo")
	createScheduleJob(t, server, schedulerA, "A")
	createOrderWithPriority(t, server, salesToken, "A", "high")

	body := bytes.NewBufferString(`{"lineId":"A","startDate":"2026-05-01","orderIds":["ORD-2"],"manualForce":true,"reason":"customer escalation approved"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/schedules/preview", body)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("manual preview failed: %d %s", res.Code, res.Body.String())
	}
	var preview struct {
		PreviewID string `json:"previewId"`
		Conflicts []struct {
			AffectedOrderIDs []string `json:"affectedOrderIds"`
		} `json:"conflicts"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &preview); err != nil {
		t.Fatalf("decode preview response: %v", err)
	}
	if len(preview.Conflicts) == 0 || len(preview.Conflicts[0].AffectedOrderIDs) == 0 {
		t.Fatalf("expected manual conflict with affected orders, got %+v", preview.Conflicts)
	}

	body = bytes.NewBufferString(`{"lineId":"A","startDate":"2026-05-01","orderIds":["ORD-2"],"manualForce":true,"reason":"customer escalation approved","previewId":"` + preview.PreviewID + `"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/schedules/jobs", body)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("manual job failed: %d %s", res.Code, res.Body.String())
	}
	var job domain.ScheduleJob
	if err := json.Unmarshal(res.Body.Bytes(), &job); err != nil {
		t.Fatalf("decode job response: %v", err)
	}
	if job.Status != domain.JobCompleted {
		t.Fatalf("expected completed manual job, got %+v", job)
	}
	foundAudit := false
	for _, audit := range store.audits {
		if audit.Action == "schedule.job.manual_force" && audit.Reason == "customer escalation approved" {
			foundAudit = true
		}
	}
	if !foundAudit {
		t.Fatalf("expected manual force audit, got %+v", store.audits)
	}
}

func TestConflictSolutionCanMoveScheduledLowPriorityOrder(t *testing.T) {
	store := NewMemoryStore()
	server := NewServer("secret", store)
	salesToken := login(t, server, "sales", "demo")
	for index := 0; index < 4; index++ {
		createOrderWithPriorityAndDue(t, server, salesToken, "A", "low", "2026-05-01")
	}
	schedulerA := login(t, server, "scheduler-a", "demo")
	createScheduleJob(t, server, schedulerA, "A")
	newOrderID := createOrderWithPriorityAndDue(t, server, salesToken, "A", "high", "2026-05-01")

	body := bytes.NewBufferString(`{"lineId":"A","startDate":"2026-05-01","orderIds":["` + newOrderID + `"]}`)
	req := httptest.NewRequest(http.MethodPost, "/api/schedules/preview", body)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("conflict preview failed: %d %s", res.Code, res.Body.String())
	}
	var conflictPreview struct {
		Conflicts []struct {
			AffectedOrderIDs []string `json:"affectedOrderIds"`
		} `json:"conflicts"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &conflictPreview); err != nil {
		t.Fatalf("decode conflict preview: %v", err)
	}
	if len(conflictPreview.Conflicts) != 1 || len(conflictPreview.Conflicts[0].AffectedOrderIDs) == 0 {
		t.Fatalf("expected affected movable scheduled orders, got %+v", conflictPreview.Conflicts)
	}
	movableOrderID := conflictPreview.Conflicts[0].AffectedOrderIDs[0]

	body = bytes.NewBufferString(`{"lineId":"A","startDate":"2026-05-01","orderIds":["` + newOrderID + `"],"resolutionOrderIds":["` + movableOrderID + `"],"allowLateCompletion":true}`)
	req = httptest.NewRequest(http.MethodPost, "/api/schedules/preview", body)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("solution preview failed: %d %s", res.Code, res.Body.String())
	}
	var solutionPreview struct {
		PreviewID   string `json:"previewId"`
		Conflicts   []any  `json:"conflicts"`
		Allocations []struct {
			OrderID string `json:"orderId"`
			Date    string `json:"date"`
		} `json:"allocations"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &solutionPreview); err != nil {
		t.Fatalf("decode solution preview: %v", err)
	}
	if len(solutionPreview.Conflicts) != 0 {
		t.Fatalf("expected conflict-free solution preview, got %+v", solutionPreview.Conflicts)
	}
	if !hasAllocationOnDate(solutionPreview.Allocations, newOrderID, "2026-05-01") || !hasAllocationOnDate(solutionPreview.Allocations, movableOrderID, "2026-05-02") {
		t.Fatalf("expected high priority order on due date and moved low priority order on next day, got %+v", solutionPreview.Allocations)
	}

	body = bytes.NewBufferString(`{"lineId":"A","startDate":"2026-05-01","orderIds":["` + newOrderID + `"],"resolutionOrderIds":["` + movableOrderID + `"],"allowLateCompletion":true,"previewId":"` + solutionPreview.PreviewID + `"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/schedules/jobs", body)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusAccepted {
		t.Fatalf("solution job failed: %d %s", res.Code, res.Body.String())
	}
	var job domain.ScheduleJob
	if err := json.Unmarshal(res.Body.Bytes(), &job); err != nil {
		t.Fatalf("decode solution job: %v", err)
	}
	if job.Status != domain.JobCompleted {
		t.Fatalf("expected completed solution job, got %+v", job)
	}
	if allocationCountForOrder(store.allocations, movableOrderID) != 1 {
		t.Fatalf("expected moved order to have one replacement allocation, got %+v", store.allocations)
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

func TestSchedulerCanUpdatePendingOrderDueDate(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	salesToken := login(t, server, "sales", "demo")
	createOrder(t, server, salesToken, "A")
	schedulerA := login(t, server, "scheduler-a", "demo")

	body := bytes.NewBufferString(`{"dueDate":"2026-05-06"}`)
	req := httptest.NewRequest(http.MethodPatch, "/api/orders/ORD-1", body)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("update due date failed: %d %s", res.Code, res.Body.String())
	}
	var order domain.Order
	if err := json.Unmarshal(res.Body.Bytes(), &order); err != nil {
		t.Fatalf("decode order response: %v", err)
	}
	if order.DueDate.Format("2006-01-02") != "2026-05-06" {
		t.Fatalf("expected updated due date, got %s", order.DueDate)
	}
}

func TestOrderNoteCannotBeUpdatedAfterCreate(t *testing.T) {
	store := NewMemoryStore()
	server := NewServer("secret", store)
	salesToken := login(t, server, "sales", "demo")
	body := bytes.NewBufferString(`{"customer":"ACME","lineId":"A","quantity":2500,"priority":"low","dueDate":"2026-05-03","note":"original sales note"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/orders", body)
	req.Header.Set("Authorization", "Bearer "+salesToken)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("create order failed: %d %s", res.Code, res.Body.String())
	}

	schedulerA := login(t, server, "scheduler-a", "demo")
	body = bytes.NewBufferString(`{"dueDate":"2026-05-06","note":"scheduler changed note"}`)
	req = httptest.NewRequest(http.MethodPatch, "/api/orders/ORD-1", body)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected note update rejection, got %d body=%s", res.Code, res.Body.String())
	}
	if store.orders["ORD-1"].Note != "original sales note" || store.orders["ORD-1"].DueDate.Format("2006-01-02") != "2026-05-03" {
		t.Fatalf("order should remain unchanged, got %+v", store.orders["ORD-1"])
	}
}

func TestStartProductionLocksScheduledAllocations(t *testing.T) {
	store := NewMemoryStore()
	server := NewServer("secret", store)
	salesToken := login(t, server, "sales", "demo")
	createOrder(t, server, salesToken, "A")
	schedulerA := login(t, server, "scheduler-a", "demo")
	createScheduleJob(t, server, schedulerA, "A")

	body := bytes.NewBufferString(`{"orderId":"ORD-1"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/production/start", body)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("start production failed: %d %s", res.Code, res.Body.String())
	}
	if store.orders["ORD-1"].Status != domain.StatusInProgress {
		t.Fatalf("expected in-progress status, got %+v", store.orders["ORD-1"])
	}
	if len(store.allocations) != 1 || !store.allocations[0].Locked {
		t.Fatalf("expected locked allocation, got %+v", store.allocations)
	}
}

func TestPartialProductionReturnsRemainderToPendingQueue(t *testing.T) {
	store := NewMemoryStore()
	server := NewServer("secret", store)
	salesToken := login(t, server, "sales", "demo")
	createOrder(t, server, salesToken, "A")
	schedulerA := login(t, server, "scheduler-a", "demo")
	createScheduleJob(t, server, schedulerA, "A")

	store.mu.Lock()
	store.allocations = []domain.ScheduleAllocation{
		{
			OrderID:  "ORD-1",
			LineID:   "A",
			Date:     mustAPIDate(t, "2026-05-01"),
			Quantity: 900,
			Priority: domain.PriorityLow,
		},
		{
			OrderID:  "ORD-1",
			LineID:   "A",
			Date:     mustAPIDate(t, "2026-05-02"),
			Quantity: 1600,
			Priority: domain.PriorityLow,
		},
	}
	store.mu.Unlock()

	startProduction(t, server, schedulerA, "ORD-1")

	body := bytes.NewBufferString(`{"orderId":"ORD-1","productionDate":"2026-05-01","producedQuantity":800}`)
	req := httptest.NewRequest(http.MethodPost, "/api/production/confirm", body)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("confirm production failed: %d %s", res.Code, res.Body.String())
	}
	var payload productionConfirmResponse
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode production response: %v", err)
	}
	if payload.Order.ID != "ORD-1" || payload.Order.Status != domain.StatusPending || payload.Order.Quantity != 1700 {
		t.Fatalf("expected original order to return pending with remaining quantity, got %+v", payload.Order)
	}
	if payload.Remainder == nil || payload.Remainder.ID != "ORD-1" || payload.Remainder.Quantity != 1700 || payload.Remainder.Status != domain.StatusPending {
		t.Fatalf("unexpected remainder: %+v", payload.Remainder)
	}
	if len(store.allocations) != 1 {
		t.Fatalf("expected partial production to keep one completed allocation, got %+v", store.allocations)
	}
	if store.allocations[0].OrderID != "ORD-1" || store.allocations[0].Quantity != 800 || store.allocations[0].Status != domain.StatusCompleted || !store.allocations[0].Date.Equal(mustAPIDate(t, "2026-05-01")) {
		t.Fatalf("expected completed May 1 allocation for produced quantity, got %+v", store.allocations[0])
	}
}

func TestProductionConfirmRejectsQuantityAboveOrderTotal(t *testing.T) {
	store := NewMemoryStore()
	server := NewServer("secret", store)
	salesToken := login(t, server, "sales", "demo")
	createOrder(t, server, salesToken, "A")
	schedulerA := login(t, server, "scheduler-a", "demo")
	createScheduleJob(t, server, schedulerA, "A")
	startProduction(t, server, schedulerA, "ORD-1")

	body := bytes.NewBufferString(`{"orderId":"ORD-1","productionDate":"2026-05-01","producedQuantity":2501}`)
	req := httptest.NewRequest(http.MethodPost, "/api/production/confirm", body)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected bad request, got %d %s", res.Code, res.Body.String())
	}
	if !strings.Contains(res.Body.String(), "producedQuantity cannot exceed scheduled allocation quantity") {
		t.Fatalf("expected clear quantity error, got %s", res.Body.String())
	}
}

func TestSchedulerRejectsPendingOrdersAndSalesCanResubmit(t *testing.T) {
	store := NewMemoryStore()
	server := NewServer("secret", store)
	salesToken := login(t, server, "sales", "demo")

	body := bytes.NewBufferString(`{"customer":"ACME","lineId":"A","quantity":2500,"priority":"low","dueDate":"2026-05-03","note":"customer can accept split delivery"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/orders", body)
	req.Header.Set("Authorization", "Bearer "+salesToken)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("create order failed: %d %s", res.Code, res.Body.String())
	}

	schedulerA := login(t, server, "scheduler-a", "demo")
	body = bytes.NewBufferString(`{"orderIds":["ORD-1"],"reason":"capacity unavailable before due date"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/orders/reject", body)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("reject failed: %d %s", res.Code, res.Body.String())
	}
	if store.orders["ORD-1"].Status != domain.StatusRejected || store.orders["ORD-1"].RejectionReason == "" {
		t.Fatalf("expected rejected order with reason, got %+v", store.orders["ORD-1"])
	}

	req = httptest.NewRequest(http.MethodGet, "/api/orders", nil)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if strings.Contains(res.Body.String(), "ORD-1") {
		t.Fatalf("rejected order should be hidden from scheduler pending queue: %s", res.Body.String())
	}

	body = bytes.NewBufferString(`{"orderId":"ORD-1","dueDate":"2026-05-05","quantity":2000}`)
	req = httptest.NewRequest(http.MethodPost, "/api/orders/resubmit", body)
	req.Header.Set("Authorization", "Bearer "+salesToken)
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("resubmit failed: %d %s", res.Code, res.Body.String())
	}
	if store.orders["ORD-1"].Status != domain.StatusPending || store.orders["ORD-1"].RejectionReason != "" {
		t.Fatalf("expected resubmitted pending order, got %+v", store.orders["ORD-1"])
	}
	if store.orders["ORD-1"].Quantity != 2000 || store.orders["ORD-1"].Note != "customer can accept split delivery" {
		t.Fatalf("expected sales edits to persist, got %+v", store.orders["ORD-1"])
	}
}

func TestSalesCannotChangeNoteDuringResubmit(t *testing.T) {
	store := NewMemoryStore()
	server := NewServer("secret", store)
	salesToken := login(t, server, "sales", "demo")
	body := bytes.NewBufferString(`{"customer":"ACME","lineId":"A","quantity":2500,"priority":"low","dueDate":"2026-05-03","note":"original sales note"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/orders", body)
	req.Header.Set("Authorization", "Bearer "+salesToken)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("create order failed: %d %s", res.Code, res.Body.String())
	}

	schedulerA := login(t, server, "scheduler-a", "demo")
	body = bytes.NewBufferString(`{"orderIds":["ORD-1"],"reason":"capacity unavailable before due date"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/orders/reject", body)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("reject failed: %d %s", res.Code, res.Body.String())
	}

	body = bytes.NewBufferString(`{"orderId":"ORD-1","dueDate":"2026-05-05","quantity":2000,"note":"changed note"}`)
	req = httptest.NewRequest(http.MethodPost, "/api/orders/resubmit", body)
	req.Header.Set("Authorization", "Bearer "+salesToken)
	res = httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusBadRequest {
		t.Fatalf("expected note update rejection, got %d body=%s", res.Code, res.Body.String())
	}
	if store.orders["ORD-1"].Note != "original sales note" || store.orders["ORD-1"].Status != domain.StatusRejected {
		t.Fatalf("order should remain rejected with original note, got %+v", store.orders["ORD-1"])
	}
}

func TestScheduleHistoryReturnsWorkflowAuditsForSchedulerLine(t *testing.T) {
	server := NewServer("secret", NewMemoryStore())
	salesToken := login(t, server, "sales", "demo")
	createOrder(t, server, salesToken, "A")
	createOrder(t, server, salesToken, "B")

	schedulerA := login(t, server, "scheduler-a", "demo")
	createScheduleJob(t, server, schedulerA, "A")
	startProduction(t, server, schedulerA, "ORD-1")

	schedulerB := login(t, server, "scheduler-b", "demo")
	createScheduleJob(t, server, schedulerB, "B")

	req := httptest.NewRequest(http.MethodGet, "/api/schedules/history", nil)
	req.Header.Set("Authorization", "Bearer "+schedulerA)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)

	if res.Code != http.StatusOK {
		t.Fatalf("history failed: %d %s", res.Code, res.Body.String())
	}
	var payload struct {
		History []domain.AuditEntry `json:"history"`
	}
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode history response: %v", err)
	}
	actions := []string{}
	for _, entry := range payload.History {
		actions = append(actions, entry.Action)
		if entry.Resource == "JOB-2" {
			t.Fatalf("scheduler A should not see line B job history: %+v", payload.History)
		}
		if entry.Action == "order.create" {
			t.Fatalf("history should exclude non-workflow audits: %+v", payload.History)
		}
	}
	if !contains(actions, "schedule.job.create") || !contains(actions, "production.start") {
		t.Fatalf("expected scheduler workflow actions, got %+v", actions)
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
	createOrderWithPriority(t, server, token, lineID, "low")
}

func createOrderWithPriority(t *testing.T, server *Server, token, lineID, priority string) string {
	t.Helper()
	return createOrderWithPriorityAndDue(t, server, token, lineID, priority, "2026-05-03")
}

func createOrderWithPriorityAndDue(t *testing.T, server *Server, token, lineID, priority, dueDate string) string {
	t.Helper()
	body := bytes.NewBufferString(`{"customer":"ACME","lineId":"` + lineID + `","quantity":2500,"priority":"` + priority + `","dueDate":"` + dueDate + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/orders", body)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusCreated {
		t.Fatalf("create order failed: %d %s", res.Code, res.Body.String())
	}
	var payload domain.Order
	if err := json.Unmarshal(res.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode order response: %v", err)
	}
	return payload.ID
}

func startProduction(t *testing.T, server *Server, token, orderID string) {
	t.Helper()
	body := bytes.NewBufferString(`{"orderId":"` + orderID + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/production/start", body)
	req.Header.Set("Authorization", "Bearer "+token)
	res := httptest.NewRecorder()
	server.ServeHTTP(res, req)
	if res.Code != http.StatusOK {
		t.Fatalf("start production failed: %d %s", res.Code, res.Body.String())
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

func hasAllocationOnDate(allocations []struct {
	OrderID string `json:"orderId"`
	Date    string `json:"date"`
}, orderID, date string) bool {
	for _, allocation := range allocations {
		if allocation.OrderID == orderID && strings.HasPrefix(allocation.Date, date) {
			return true
		}
	}
	return false
}

func allocationCountForOrder(allocations []domain.ScheduleAllocation, orderID string) int {
	count := 0
	for _, allocation := range allocations {
		if allocation.OrderID == orderID && allocation.Status != domain.StatusCompleted {
			count++
		}
	}
	return count
}

func mustAPIDate(t *testing.T, value string) time.Time {
	t.Helper()
	parsed, err := time.Parse(dateLayout, value)
	if err != nil {
		t.Fatalf("parse date: %v", err)
	}
	return parsed
}

func contains(values []string, expected string) bool {
	for _, value := range values {
		if value == expected {
			return true
		}
	}
	return false
}
