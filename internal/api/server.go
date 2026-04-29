package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/c9274326/woms/internal/auth"
	"github.com/c9274326/woms/internal/domain"
	"github.com/c9274326/woms/internal/scheduler"
)

const dateLayout = "2006-01-02"

type Server struct {
	jwtSecret string
	store     *MemoryStore
}

func NewServer(jwtSecret string, store *MemoryStore) *Server {
	if store == nil {
		store = NewMemoryStore()
	}
	return &Server{jwtSecret: jwtSecret, store: store}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w)
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/healthz":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	case r.Method == http.MethodGet && r.URL.Path == "/readyz":
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	case r.Method == http.MethodPost && r.URL.Path == "/api/auth/login":
		s.handleLogin(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/internal/auth/verify":
		s.handleIngressAuth(w, r)
	case r.URL.Path == "/api/orders":
		s.handleOrders(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/orders/preview-confirm":
		s.handleConfirmPreviewOrder(w, r)
	case r.URL.Path == "/api/users":
		s.handleUsers(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/demo/conflict-orders":
		s.handleDemoConflictOrders(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/schedules/preview":
		s.handleSchedulePreview(w, r)
	case r.Method == http.MethodGet && r.URL.Path == "/api/schedules/calendar":
		s.handleScheduleCalendar(w, r)
	case r.URL.Path == "/api/schedules/jobs":
		s.handleScheduleJobs(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, "/api/schedules/jobs/"):
		s.handleGetScheduleJob(w, r)
	case r.Method == http.MethodPost && r.URL.Path == "/api/production/confirm":
		s.handleProductionConfirm(w, r)
	default:
		writeError(w, http.StatusNotFound, "route not found")
	}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	user, ok := s.store.Authenticate(req.Username, req.Password)
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	token, err := auth.CreateToken(s.jwtSecret, auth.Claims{
		Subject: user.ID,
		Role:    user.Role,
		LineID:  user.LineID,
	}, 8*time.Hour)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"token": token,
		"user":  user,
	})
}

func (s *Server) handleIngressAuth(w http.ResponseWriter, r *http.Request) {
	if _, err := s.claimsFromRequest(r); err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleOrders(w http.ResponseWriter, r *http.Request) {
	claims, err := s.claimsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	switch r.Method {
	case http.MethodGet:
		orders := s.store.ListOrders(claims)
		writeJSON(w, http.StatusOK, map[string]any{"orders": orders})
	case http.MethodPost:
		if claims.Role != domain.RoleSales {
			writeError(w, http.StatusForbidden, "only sales can create orders")
			return
		}
		var req createOrderRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		order, err := s.store.CreateOrder(req, claims.Subject)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, order)
	case http.MethodDelete:
		var req deleteOrdersRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result, err := s.store.DeleteOrders(req, claims)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, result)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleConfirmPreviewOrder(w http.ResponseWriter, r *http.Request) {
	claims, err := s.claimsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if claims.Role != domain.RoleSales {
		writeError(w, http.StatusForbidden, "only sales can confirm preview orders")
		return
	}
	var req confirmPreviewRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	order, err := s.store.ConfirmPreviewOrder(req.PreviewID, claims)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, order)
}

func (s *Server) handleUsers(w http.ResponseWriter, r *http.Request) {
	claims, err := s.claimsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if claims.Role != domain.RoleAdmin {
		writeError(w, http.StatusForbidden, "only admin can manage accounts")
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]any{"users": s.store.ListUsers()})
	case http.MethodPatch:
		var req assignUserRequest
		if err := readJSON(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		user, err := s.store.AssignUser(req, claims.Subject)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, user)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (s *Server) handleDemoConflictOrders(w http.ResponseWriter, r *http.Request) {
	claims, err := s.claimsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if claims.Role != domain.RoleAdmin && claims.Role != domain.RoleScheduler {
		writeError(w, http.StatusForbidden, "only admin or schedulers can create demo conflict orders")
		return
	}
	var req demoConflictRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	orders, err := s.store.CreateDemoConflictOrders(req, claims)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"orders": orders})
}

func (s *Server) handleSchedulePreview(w http.ResponseWriter, r *http.Request) {
	claims, err := s.claimsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	var req scheduleRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.store.PreviewSchedule(req, claims)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleScheduleCalendar(w http.ResponseWriter, r *http.Request) {
	claims, err := s.claimsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	lineID := r.URL.Query().Get("lineId")
	month := r.URL.Query().Get("month")
	result, err := s.store.ScheduleCalendar(lineID, month, claims)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) handleScheduleJobs(w http.ResponseWriter, r *http.Request) {
	claims, err := s.claimsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if claims.Role != domain.RoleScheduler {
		writeError(w, http.StatusForbidden, "only schedulers can create schedule jobs")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req scheduleRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	job, err := s.store.CreateScheduleJob(req, claims)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (s *Server) handleGetScheduleJob(w http.ResponseWriter, r *http.Request) {
	claims, err := s.claimsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	id := strings.TrimPrefix(r.URL.Path, "/api/schedules/jobs/")
	job, ok := s.store.GetScheduleJob(id)
	if !ok {
		writeError(w, http.StatusNotFound, "schedule job not found")
		return
	}
	if claims.Role == domain.RoleScheduler && claims.LineID != job.LineID {
		writeError(w, http.StatusForbidden, "cannot access another production line")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (s *Server) handleProductionConfirm(w http.ResponseWriter, r *http.Request) {
	claims, err := s.claimsFromRequest(r)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	if claims.Role != domain.RoleScheduler {
		writeError(w, http.StatusForbidden, "only schedulers can confirm production")
		return
	}
	var req productionConfirmRequest
	if err := readJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := s.store.ConfirmProduction(req, claims)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) claimsFromRequest(r *http.Request) (auth.Claims, error) {
	token, err := auth.BearerToken(r.Header.Get("Authorization"))
	if err != nil {
		return auth.Claims{}, err
	}
	return auth.VerifyToken(s.jwtSecret, token)
}

type MemoryStore struct {
	mu            sync.Mutex
	nextOrderID   int
	nextJobID     int
	nextAuditID   int
	nextPreviewID int
	lines         map[string]domain.ProductionLine
	users         map[string]domain.User
	orders        map[string]domain.Order
	jobs          map[string]domain.ScheduleJob
	allocations   []domain.ScheduleAllocation
	previews      map[string]previewRecord
	audits        []domain.AuditEntry
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nextOrderID:   1,
		nextJobID:     1,
		nextAuditID:   1,
		nextPreviewID: 1,
		lines: map[string]domain.ProductionLine{
			"A": {ID: "A", Name: "Line A", CapacityPerDay: 10000},
			"B": {ID: "B", Name: "Line B", CapacityPerDay: 10000},
			"C": {ID: "C", Name: "Line C", CapacityPerDay: 10000},
			"D": {ID: "D", Name: "Line D", CapacityPerDay: 10000},
		},
		users: map[string]domain.User{
			"admin":       {ID: "user-admin", Username: "admin", PasswordHash: "demo", Role: domain.RoleAdmin},
			"sales":       {ID: "user-sales", Username: "sales", PasswordHash: "demo", Role: domain.RoleSales},
			"scheduler-a": {ID: "user-scheduler-a", Username: "scheduler-a", PasswordHash: "demo", Role: domain.RoleScheduler, LineID: "A"},
			"scheduler-b": {ID: "user-scheduler-b", Username: "scheduler-b", PasswordHash: "demo", Role: domain.RoleScheduler, LineID: "B"},
			"scheduler-c": {ID: "user-scheduler-c", Username: "scheduler-c", PasswordHash: "demo", Role: domain.RoleScheduler, LineID: "C"},
			"scheduler-d": {ID: "user-scheduler-d", Username: "scheduler-d", PasswordHash: "demo", Role: domain.RoleScheduler, LineID: "D"},
		},
		orders:      map[string]domain.Order{},
		jobs:        map[string]domain.ScheduleJob{},
		allocations: []domain.ScheduleAllocation{},
		previews:    map[string]previewRecord{},
	}
}

func (s *MemoryStore) Authenticate(username, password string) (domain.User, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	user, ok := s.users[username]
	if !ok || password == "" || password != user.PasswordHash {
		return domain.User{}, false
	}
	return user, true
}

type createOrderRequest struct {
	Customer string          `json:"customer"`
	LineID   string          `json:"lineId"`
	Quantity int             `json:"quantity"`
	Priority domain.Priority `json:"priority"`
	DueDate  string          `json:"dueDate"`
}

type deleteOrdersRequest struct {
	OrderIDs []string `json:"orderIds"`
}

type deleteOrdersResponse struct {
	DeletedOrderIDs []string `json:"deletedOrderIds"`
	SkippedOrderIDs []string `json:"skippedOrderIds,omitempty"`
}

type assignUserRequest struct {
	Username string      `json:"username"`
	Role     domain.Role `json:"role"`
	LineID   string      `json:"lineId"`
}

type confirmPreviewRequest struct {
	PreviewID string `json:"previewId"`
}

type demoConflictRequest struct {
	LineID  string `json:"lineId"`
	DueDate string `json:"dueDate"`
	Count   int    `json:"count"`
}

type previewRecord struct {
	ActorID    string
	ActorRole  domain.Role
	Request    scheduleRequest
	DraftOrder *createOrderRequest
	CreatedAt  time.Time
}

type schedulePreviewResponse struct {
	PreviewID   string                 `json:"previewId"`
	Allocations []scheduler.Allocation `json:"allocations"`
	Conflicts   []scheduler.Conflict   `json:"conflicts"`
	FinishDate  time.Time              `json:"finishDate"`
	DraftOrder  *createOrderRequest    `json:"draftOrder,omitempty"`
}

func (s *MemoryStore) CreateOrder(req createOrderRequest, actorID string) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.createOrderLocked(req, actorID)
}

func (s *MemoryStore) createOrderLocked(req createOrderRequest, actorID string) (domain.Order, error) {
	if req.Customer == "" || req.Quantity < 25 || req.Quantity > 2500 {
		return domain.Order{}, errors.New("customer is required and quantity must be between 25 and 2500")
	}
	if _, ok := s.lines[req.LineID]; !ok {
		return domain.Order{}, errors.New("production line does not exist")
	}
	if req.Priority == "" {
		req.Priority = domain.PriorityLow
	}
	if req.Priority != domain.PriorityLow && req.Priority != domain.PriorityHigh {
		return domain.Order{}, errors.New("priority must be low or high")
	}
	dueDate, err := time.Parse(dateLayout, req.DueDate)
	if err != nil {
		return domain.Order{}, errors.New("dueDate must use YYYY-MM-DD")
	}

	now := time.Now().UTC()
	id := "ORD-" + strconv.Itoa(s.nextOrderID)
	s.nextOrderID++
	order := domain.Order{
		ID:        id,
		Customer:  req.Customer,
		LineID:    req.LineID,
		Quantity:  req.Quantity,
		Priority:  req.Priority,
		Status:    domain.StatusPending,
		DueDate:   dueDate,
		CreatedBy: actorID,
		CreatedAt: now,
		UpdatedAt: now,
	}
	s.orders[id] = order
	s.auditLocked(actorID, "order.create", id, "")
	return order, nil
}

func (s *MemoryStore) ListOrders(claims auth.Claims) []domain.Order {
	s.mu.Lock()
	defer s.mu.Unlock()

	orders := make([]domain.Order, 0, len(s.orders))
	for _, order := range s.orders {
		if claims.Role == domain.RoleScheduler && order.LineID != claims.LineID {
			continue
		}
		orders = append(orders, order)
	}
	sort.Slice(orders, func(i, j int) bool {
		return orders[i].ID < orders[j].ID
	})
	return orders
}

func (s *MemoryStore) DeleteOrders(req deleteOrdersRequest, claims auth.Claims) (deleteOrdersResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(req.OrderIDs) == 0 {
		return deleteOrdersResponse{}, errors.New("orderIds is required")
	}
	result := deleteOrdersResponse{}
	for _, id := range req.OrderIDs {
		order, ok := s.orders[id]
		if !ok {
			result.SkippedOrderIDs = append(result.SkippedOrderIDs, id)
			continue
		}
		if claims.Role == domain.RoleSales && order.CreatedBy != claims.Subject {
			return deleteOrdersResponse{}, errors.New("sales can delete only their own orders")
		}
		if claims.Role == domain.RoleScheduler && order.LineID != claims.LineID {
			return deleteOrdersResponse{}, errors.New("cannot delete another production line")
		}
		if claims.Role != domain.RoleAdmin && claims.Role != domain.RoleSales && claims.Role != domain.RoleScheduler {
			return deleteOrdersResponse{}, errors.New("role cannot delete orders")
		}
		if order.Status == domain.StatusInProgress || order.Status == domain.StatusCompleted {
			return deleteOrdersResponse{}, errors.New("cannot delete in-progress or completed orders")
		}
		delete(s.orders, id)
		s.removeAllocationsLocked(id)
		s.auditLocked(claims.Subject, "order.delete", id, "")
		result.DeletedOrderIDs = append(result.DeletedOrderIDs, id)
	}
	return result, nil
}

func (s *MemoryStore) ListUsers() []domain.User {
	s.mu.Lock()
	defer s.mu.Unlock()

	users := make([]domain.User, 0, len(s.users))
	for _, user := range s.users {
		users = append(users, user)
	}
	sort.Slice(users, func(i, j int) bool {
		return users[i].Username < users[j].Username
	})
	return users
}

func (s *MemoryStore) AssignUser(req assignUserRequest, actorID string) (domain.User, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	user, ok := s.users[req.Username]
	if !ok {
		return domain.User{}, errors.New("user not found")
	}
	if req.Role != domain.RoleAdmin && req.Role != domain.RoleSales && req.Role != domain.RoleScheduler {
		return domain.User{}, errors.New("role must be admin, sales, or scheduler")
	}
	if req.Role == domain.RoleScheduler {
		if _, ok := s.lines[req.LineID]; !ok {
			return domain.User{}, errors.New("scheduler lineId must be A, B, C, or D")
		}
	} else {
		req.LineID = ""
	}
	user.Role = req.Role
	user.LineID = req.LineID
	s.users[user.Username] = user
	s.auditLocked(actorID, "user.assign", user.ID, string(req.Role)+" "+req.LineID)
	return user, nil
}

type scheduleRequest struct {
	LineID      string              `json:"lineId"`
	StartDate   string              `json:"startDate"`
	OrderIDs    []string            `json:"orderIds"`
	ManualForce bool                `json:"manualForce"`
	Reason      string              `json:"reason"`
	PreviewID   string              `json:"previewId"`
	DraftOrder  *createOrderRequest `json:"draftOrder,omitempty"`
}

func (s *MemoryStore) PreviewSchedule(req scheduleRequest, claims auth.Claims) (schedulePreviewResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result, err := s.planLocked(req, claims)
	if err != nil {
		return schedulePreviewResponse{}, err
	}
	id := "PREVIEW-" + strconv.Itoa(s.nextPreviewID)
	s.nextPreviewID++
	s.previews[id] = previewRecord{
		ActorID:    claims.Subject,
		ActorRole:  claims.Role,
		Request:    normalizedPreviewRequest(req),
		DraftOrder: req.DraftOrder,
		CreatedAt:  time.Now().UTC(),
	}
	return schedulePreviewResponse{
		PreviewID:   id,
		Allocations: result.Allocations,
		Conflicts:   result.Conflicts,
		FinishDate:  result.FinishDate,
		DraftOrder:  req.DraftOrder,
	}, nil
}

func (s *MemoryStore) CreateScheduleJob(req scheduleRequest, claims auth.Claims) (domain.ScheduleJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if req.PreviewID == "" {
		return domain.ScheduleJob{}, errors.New("previewId is required before creating a schedule job")
	}
	preview, ok := s.previews[req.PreviewID]
	if !ok {
		return domain.ScheduleJob{}, errors.New("preview result expired or not found")
	}
	if preview.ActorID != claims.Subject || preview.ActorRole != claims.Role {
		return domain.ScheduleJob{}, errors.New("preview result belongs to another user")
	}
	if !sameScheduleRequest(preview.Request, normalizedPreviewRequest(req)) {
		return domain.ScheduleJob{}, errors.New("schedule request changed after preview")
	}

	jobLine := req.LineID
	if jobLine == "" {
		jobLine = claims.LineID
	}
	if claims.LineID != jobLine {
		return domain.ScheduleJob{}, errors.New("cannot schedule another production line")
	}

	now := time.Now().UTC()
	id := "JOB-" + strconv.Itoa(s.nextJobID)
	s.nextJobID++
	job := domain.ScheduleJob{ID: id, LineID: jobLine, Status: domain.JobQueued, CreatedAt: now, UpdatedAt: now}
	s.jobs[id] = job

	job.Status = domain.JobRunning
	job.UpdatedAt = time.Now().UTC()
	s.jobs[id] = job
	result, err := s.planLocked(req, claims)
	if err != nil {
		job.Status = domain.JobFailed
		job.Message = err.Error()
		job.UpdatedAt = time.Now().UTC()
		s.jobs[id] = job
		return job, nil
	}
	if len(result.Conflicts) > 0 {
		job.Status = domain.JobFailed
		job.Message = "schedule conflicts require review"
		job.UpdatedAt = time.Now().UTC()
		s.jobs[id] = job
		return job, nil
	}

	job.Status = domain.JobCompleted
	job.Message = "schedule job accepted by foundation scheduler"
	job.UpdatedAt = time.Now().UTC()
	s.jobs[id] = job
	s.persistAllocationsLocked(result.Allocations)
	delete(s.previews, req.PreviewID)
	s.auditLocked(claims.Subject, "schedule.job.create", id, req.Reason)
	return job, nil
}

func (s *MemoryStore) GetScheduleJob(id string) (domain.ScheduleJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	return job, ok
}

type calendarAllocation struct {
	OrderID  string             `json:"orderId"`
	Customer string             `json:"customer"`
	LineID   string             `json:"lineId"`
	Date     time.Time          `json:"date"`
	Quantity int                `json:"quantity"`
	Priority domain.Priority    `json:"priority"`
	Status   domain.OrderStatus `json:"status"`
	Locked   bool               `json:"locked"`
	DueDate  time.Time          `json:"dueDate"`
}

type calendarResponse struct {
	LineID      string               `json:"lineId"`
	Month       string               `json:"month"`
	Allocations []calendarAllocation `json:"allocations"`
}

func (s *MemoryStore) ScheduleCalendar(lineID, month string, claims auth.Claims) (calendarResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if lineID == "" && claims.Role == domain.RoleScheduler {
		lineID = claims.LineID
	}
	if lineID == "" {
		return calendarResponse{}, errors.New("lineId is required")
	}
	if claims.Role == domain.RoleScheduler && claims.LineID != lineID {
		return calendarResponse{}, errors.New("cannot access another production line")
	}
	if _, ok := s.lines[lineID]; !ok {
		return calendarResponse{}, errors.New("production line does not exist")
	}
	if month == "" {
		month = time.Now().UTC().Format("2006-01")
	}
	monthStart, err := time.Parse("2006-01", month)
	if err != nil {
		return calendarResponse{}, errors.New("month must use YYYY-MM")
	}
	monthEnd := monthStart.AddDate(0, 1, 0)

	allocations := []calendarAllocation{}
	for _, allocation := range s.allocations {
		if allocation.LineID != lineID {
			continue
		}
		allocationDate := truncateDate(allocation.Date)
		if allocationDate.Before(monthStart) || !allocationDate.Before(monthEnd) {
			continue
		}
		order := s.orders[allocation.OrderID]
		allocations = append(allocations, calendarAllocation{
			OrderID:  allocation.OrderID,
			Customer: order.Customer,
			LineID:   allocation.LineID,
			Date:     allocationDate,
			Quantity: allocation.Quantity,
			Priority: allocation.Priority,
			Status:   order.Status,
			Locked:   allocation.Locked,
			DueDate:  order.DueDate,
		})
	}
	sort.Slice(allocations, func(i, j int) bool {
		if !allocations[i].Date.Equal(allocations[j].Date) {
			return allocations[i].Date.Before(allocations[j].Date)
		}
		return allocations[i].OrderID < allocations[j].OrderID
	})

	return calendarResponse{LineID: lineID, Month: month, Allocations: allocations}, nil
}

type productionConfirmRequest struct {
	OrderID          string `json:"orderId"`
	ProducedQuantity int    `json:"producedQuantity"`
}

type productionConfirmResponse struct {
	Order     domain.Order  `json:"order"`
	Remainder *domain.Order `json:"remainder,omitempty"`
}

func (s *MemoryStore) ConfirmProduction(req productionConfirmRequest, claims auth.Claims) (productionConfirmResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	order, ok := s.orders[req.OrderID]
	if !ok {
		return productionConfirmResponse{}, errors.New("order not found")
	}
	if order.LineID != claims.LineID {
		return productionConfirmResponse{}, errors.New("cannot confirm another production line")
	}
	result, err := scheduler.ConfirmProduction(order, req.ProducedQuantity, time.Now().UTC())
	if err != nil {
		return productionConfirmResponse{}, err
	}
	if result.Completed {
		order.Status = domain.StatusCompleted
		order.UpdatedAt = time.Now().UTC()
		s.orders[order.ID] = order
		s.auditLocked(claims.Subject, "production.confirm.complete", order.ID, "")
		return productionConfirmResponse{Order: order}, nil
	}

	order.Status = domain.StatusInProgress
	order.UpdatedAt = time.Now().UTC()
	s.orders[order.ID] = order

	remainder := *result.Remainder
	remainder.ID = "ORD-" + strconv.Itoa(s.nextOrderID)
	s.nextOrderID++
	s.orders[remainder.ID] = remainder
	s.auditLocked(claims.Subject, "production.confirm.partial", order.ID, "created remainder "+remainder.ID)
	return productionConfirmResponse{Order: order, Remainder: &remainder}, nil
}

func (s *MemoryStore) planLocked(req scheduleRequest, claims auth.Claims) (scheduler.Result, error) {
	lineID := req.LineID
	if lineID == "" && claims.Role == domain.RoleScheduler {
		lineID = claims.LineID
	}
	if lineID == "" {
		return scheduler.Result{}, errors.New("lineId is required")
	}
	if req.ManualForce && strings.TrimSpace(req.Reason) == "" {
		return scheduler.Result{}, errors.New("manual force requires a reason")
	}
	if claims.Role == domain.RoleScheduler && claims.LineID != lineID {
		return scheduler.Result{}, errors.New("cannot access another production line")
	}
	line, ok := s.lines[lineID]
	if !ok {
		return scheduler.Result{}, errors.New("production line does not exist")
	}
	startDate := time.Now().UTC()
	if req.StartDate != "" {
		parsed, err := time.Parse(dateLayout, req.StartDate)
		if err != nil {
			return scheduler.Result{}, errors.New("startDate must use YYYY-MM-DD")
		}
		startDate = parsed
	}

	selected := map[string]bool{}
	for _, id := range req.OrderIDs {
		selected[id] = true
	}
	inputs := []scheduler.OrderInput{}
	if req.DraftOrder != nil {
		if claims.Role != domain.RoleSales {
			return scheduler.Result{}, errors.New("only sales can preview draft orders")
		}
		draft := *req.DraftOrder
		if draft.LineID == "" {
			draft.LineID = lineID
		}
		if draft.LineID != lineID {
			return scheduler.Result{}, errors.New("draft order line must match preview line")
		}
		if draft.Priority == "" {
			draft.Priority = domain.PriorityLow
		}
		dueDate, err := validateOrderRequest(draft, s.lines)
		if err != nil {
			return scheduler.Result{}, err
		}
		inputs = append(inputs, scheduler.OrderInput{
			ID:       "PREVIEW-DRAFT",
			LineID:   draft.LineID,
			Quantity: draft.Quantity,
			Priority: draft.Priority,
			DueDate:  dueDate,
		})
	}
	for _, order := range s.orders {
		if order.LineID != lineID || order.Status != domain.StatusPending {
			continue
		}
		if len(selected) > 0 && !selected[order.ID] {
			continue
		}
		inputs = append(inputs, scheduler.OrderInput{
			ID:       order.ID,
			LineID:   order.LineID,
			Quantity: order.Quantity,
			Priority: order.Priority,
			DueDate:  order.DueDate,
		})
	}
	existingAllocations := []scheduler.ExistingAllocation{}
	for _, allocation := range s.allocations {
		existingAllocations = append(existingAllocations, scheduler.ExistingAllocation{
			OrderID:  allocation.OrderID,
			LineID:   allocation.LineID,
			Date:     allocation.Date,
			Quantity: allocation.Quantity,
			Priority: allocation.Priority,
			Locked:   allocation.Locked,
		})
	}

	return scheduler.Plan(scheduler.Request{
		LineID:              lineID,
		CapacityPerDay:      line.CapacityPerDay,
		StartDate:           startDate,
		Orders:              inputs,
		ExistingAllocations: existingAllocations,
		ManualForce:         req.ManualForce,
		ForceReason:         req.Reason,
	})
}

func (s *MemoryStore) persistAllocationsLocked(allocations []scheduler.Allocation) {
	for _, allocation := range allocations {
		s.allocations = append(s.allocations, domain.ScheduleAllocation{
			OrderID:  allocation.OrderID,
			LineID:   allocation.LineID,
			Date:     truncateDate(allocation.Date),
			Quantity: allocation.Quantity,
			Priority: allocation.Priority,
			Locked:   allocation.Locked,
		})
		order, ok := s.orders[allocation.OrderID]
		if ok && order.Status == domain.StatusPending {
			order.Status = domain.StatusScheduled
			order.UpdatedAt = time.Now().UTC()
			s.orders[order.ID] = order
		}
	}
}

func (s *MemoryStore) ConfirmPreviewOrder(previewID string, claims auth.Claims) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	preview, ok := s.previews[previewID]
	if !ok {
		return domain.Order{}, errors.New("preview result expired or not found")
	}
	if preview.ActorID != claims.Subject || preview.ActorRole != claims.Role {
		return domain.Order{}, errors.New("preview result belongs to another user")
	}
	if preview.DraftOrder == nil {
		return domain.Order{}, errors.New("preview does not contain a draft order")
	}
	order, err := s.createOrderLocked(*preview.DraftOrder, claims.Subject)
	if err != nil {
		return domain.Order{}, err
	}
	delete(s.previews, previewID)
	return order, nil
}

func (s *MemoryStore) CreateDemoConflictOrders(req demoConflictRequest, claims auth.Claims) ([]domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	lineID := req.LineID
	if lineID == "" && claims.Role == domain.RoleScheduler {
		lineID = claims.LineID
	}
	if claims.Role == domain.RoleScheduler && lineID != claims.LineID {
		return nil, errors.New("cannot create demo orders for another production line")
	}
	if _, ok := s.lines[lineID]; !ok {
		return nil, errors.New("production line does not exist")
	}
	if req.Count == 0 {
		req.Count = 6
	}
	if req.Count < 5 || req.Count > 20 {
		return nil, errors.New("count must be between 5 and 20")
	}
	if req.DueDate == "" {
		req.DueDate = time.Now().UTC().AddDate(0, 0, 1).Format(dateLayout)
	}

	orders := make([]domain.Order, 0, req.Count)
	for index := 1; index <= req.Count; index++ {
		order, err := s.createOrderLocked(createOrderRequest{
			Customer: "Conflict Demo " + strconv.Itoa(index),
			LineID:   lineID,
			Quantity: 2500,
			Priority: domain.PriorityLow,
			DueDate:  req.DueDate,
		}, claims.Subject)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, nil
}

func (s *MemoryStore) removeAllocationsLocked(orderID string) {
	kept := s.allocations[:0]
	for _, allocation := range s.allocations {
		if allocation.OrderID != orderID {
			kept = append(kept, allocation)
		}
	}
	s.allocations = kept
}

func validateOrderRequest(req createOrderRequest, lines map[string]domain.ProductionLine) (time.Time, error) {
	if req.Customer == "" || req.Quantity < 25 || req.Quantity > 2500 {
		return time.Time{}, errors.New("customer is required and quantity must be between 25 and 2500")
	}
	if _, ok := lines[req.LineID]; !ok {
		return time.Time{}, errors.New("production line does not exist")
	}
	if req.Priority == "" {
		req.Priority = domain.PriorityLow
	}
	if req.Priority != domain.PriorityLow && req.Priority != domain.PriorityHigh {
		return time.Time{}, errors.New("priority must be low or high")
	}
	dueDate, err := time.Parse(dateLayout, req.DueDate)
	if err != nil {
		return time.Time{}, errors.New("dueDate must use YYYY-MM-DD")
	}
	return dueDate, nil
}

func normalizedPreviewRequest(req scheduleRequest) scheduleRequest {
	normalized := req
	normalized.PreviewID = ""
	normalized.DraftOrder = nil
	normalized.OrderIDs = append([]string(nil), req.OrderIDs...)
	sort.Strings(normalized.OrderIDs)
	return normalized
}

func sameScheduleRequest(a, b scheduleRequest) bool {
	if a.LineID != b.LineID || a.StartDate != b.StartDate || a.ManualForce != b.ManualForce || a.Reason != b.Reason {
		return false
	}
	if len(a.OrderIDs) != len(b.OrderIDs) {
		return false
	}
	for index := range a.OrderIDs {
		if a.OrderIDs[index] != b.OrderIDs[index] {
			return false
		}
	}
	return true
}

func truncateDate(value time.Time) time.Time {
	year, month, day := value.Date()
	return time.Date(year, month, day, 0, 0, 0, 0, time.UTC)
}

func (s *MemoryStore) auditLocked(actorID, action, resource, reason string) {
	id := "AUD-" + strconv.Itoa(s.nextAuditID)
	s.nextAuditID++
	s.audits = append(s.audits, domain.AuditEntry{
		ID:        id,
		ActorID:   actorID,
		Action:    action,
		Resource:  resource,
		Reason:    reason,
		CreatedAt: time.Now().UTC(),
	})
}

func readJSON(r *http.Request, target any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func setSecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
	w.Header().Set("Content-Security-Policy", "default-src 'self'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
}
