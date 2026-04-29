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
	case r.Method == http.MethodPost && r.URL.Path == "/api/schedules/preview":
		s.handleSchedulePreview(w, r)
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
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
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
	mu          sync.Mutex
	nextOrderID int
	nextJobID   int
	nextAuditID int
	lines       map[string]domain.ProductionLine
	users       map[string]domain.User
	orders      map[string]domain.Order
	jobs        map[string]domain.ScheduleJob
	audits      []domain.AuditEntry
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		nextOrderID: 1,
		nextJobID:   1,
		nextAuditID: 1,
		lines: map[string]domain.ProductionLine{
			"A": {ID: "A", Name: "Line A", CapacityPerDay: 10000},
			"B": {ID: "B", Name: "Line B", CapacityPerDay: 10000},
			"C": {ID: "C", Name: "Line C", CapacityPerDay: 10000},
			"D": {ID: "D", Name: "Line D", CapacityPerDay: 10000},
		},
		users: map[string]domain.User{
			"sales":       {ID: "user-sales", Username: "sales", PasswordHash: "demo", Role: domain.RoleSales},
			"scheduler-a": {ID: "user-scheduler-a", Username: "scheduler-a", PasswordHash: "demo", Role: domain.RoleScheduler, LineID: "A"},
			"scheduler-b": {ID: "user-scheduler-b", Username: "scheduler-b", PasswordHash: "demo", Role: domain.RoleScheduler, LineID: "B"},
			"scheduler-c": {ID: "user-scheduler-c", Username: "scheduler-c", PasswordHash: "demo", Role: domain.RoleScheduler, LineID: "C"},
			"scheduler-d": {ID: "user-scheduler-d", Username: "scheduler-d", PasswordHash: "demo", Role: domain.RoleScheduler, LineID: "D"},
		},
		orders: map[string]domain.Order{},
		jobs:   map[string]domain.ScheduleJob{},
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

func (s *MemoryStore) CreateOrder(req createOrderRequest, actorID string) (domain.Order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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

type scheduleRequest struct {
	LineID      string   `json:"lineId"`
	StartDate   string   `json:"startDate"`
	OrderIDs    []string `json:"orderIds"`
	ManualForce bool     `json:"manualForce"`
	Reason      string   `json:"reason"`
}

func (s *MemoryStore) PreviewSchedule(req scheduleRequest, claims auth.Claims) (scheduler.Result, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.planLocked(req, claims)
}

func (s *MemoryStore) CreateScheduleJob(req scheduleRequest, claims auth.Claims) (domain.ScheduleJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

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
	if _, err := s.planLocked(req, claims); err != nil {
		job.Status = domain.JobFailed
		job.Message = err.Error()
		job.UpdatedAt = time.Now().UTC()
		s.jobs[id] = job
		return job, nil
	}

	job.Status = domain.JobCompleted
	job.Message = "schedule job accepted by foundation scheduler"
	job.UpdatedAt = time.Now().UTC()
	s.jobs[id] = job
	s.auditLocked(claims.Subject, "schedule.job.create", id, req.Reason)
	return job, nil
}

func (s *MemoryStore) GetScheduleJob(id string) (domain.ScheduleJob, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	return job, ok
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
	return scheduler.Plan(scheduler.Request{
		LineID:         lineID,
		CapacityPerDay: line.CapacityPerDay,
		StartDate:      startDate,
		Orders:         inputs,
		ManualForce:    req.ManualForce,
		ForceReason:    req.Reason,
	})
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
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Content-Security-Policy", "default-src 'self'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("X-Frame-Options", "DENY")
}
