// Package httpapi exposes billing's REST surface (docs/API_DESIGN.md).
package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"unital/backend/pkg/httpx"
	"unital/backend/pkg/jwtx"
	"unital/backend/services/billing/internal/domain"
	"unital/backend/services/billing/internal/usecase"
)

type API struct {
	billing *usecase.Billing
	signer  *jwtx.Signer
	base    string
}

func New(b *usecase.Billing, signer *jwtx.Signer) *API {
	return &API{billing: b, signer: signer, base: "billing"}
}

func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /buildings/{buildingID}/charge-templates", a.createTemplate)
	mux.HandleFunc("GET /buildings/{buildingID}/charge-templates", a.listTemplates)
	mux.HandleFunc("PATCH /charge-templates/{templateID}", a.patchTemplate)
	mux.HandleFunc("DELETE /charge-templates/{templateID}", a.deleteTemplate)
	mux.HandleFunc("POST /buildings/{buildingID}/charges:generate", a.generate)
	mux.HandleFunc("GET /buildings/{buildingID}/charges", a.listCharges)
	mux.HandleFunc("POST /charges/{chargeID}/payments", a.recordPayment)
	mux.HandleFunc("POST /transactions/{txID}/confirm", a.confirmTx)
	mux.HandleFunc("GET /buildings/{buildingID}/invoices", a.listInvoices)
	mux.HandleFunc("GET /buildings/{buildingID}/reports/financial", a.report)
	mux.HandleFunc("POST /internal/sweep-overdue", a.sweepOverdue)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { httpx.JSON(w, 200, map[string]string{"status": "ok"}) })
}

func (a *API) userID(r *http.Request) (string, bool) {
	if id := r.Header.Get("X-User-Id"); id != "" {
		return id, true
	}
	auth := r.Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return "", false
	}
	claims, err := a.signer.Parse(strings.TrimPrefix(auth, "Bearer "))
	if err != nil {
		return "", false
	}
	return claims.Sub, true
}

func (a *API) fail(w http.ResponseWriter, r *http.Request, err error) {
	p := httpx.NewProblem(a.base, "INTERNAL", "Internal server error", http.StatusInternalServerError)
	switch {
	case errors.Is(err, domain.ErrNotFound):
		p = httpx.NewProblem(a.base, "NOT_FOUND", "Resource not found", http.StatusNotFound)
	case errors.Is(err, domain.ErrForbidden):
		p = httpx.NewProblem(a.base, "FORBIDDEN", "Not allowed for this building", http.StatusForbidden)
	case errors.Is(err, domain.ErrAlreadySettled):
		p = httpx.NewProblem(a.base, "ALREADY_SETTLED", "Transaction already settled", http.StatusConflict)
	case errors.Is(err, domain.ErrOverpayment):
		p = httpx.NewProblem(a.base, "OVERPAYMENT", "Payment exceeds remaining amount", http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrDuplicate):
		p = httpx.NewProblem(a.base, "DUPLICATE", "Already exists", http.StatusConflict)
	}
	httpx.WriteError(w, r, p)
}

func (a *API) createTemplate(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	var req struct {
		Name        string `json:"name"`
		Type        string `json:"type"`
		Amount      int64  `json:"amount"`
		Description string `json:"description"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.Name == "" || req.Amount <= 0 {
		httpx.WriteError(w, r, httpx.Invalid(a.base,
			httpx.Validation{Field: "name", Message: "required"},
			httpx.Validation{Field: "amount", Message: "must be positive"}))
		return
	}
	t, err := a.billing.CreateTemplate(r.Context(), uid, domain.Template{
		BuildingID: r.PathValue("buildingID"), Name: req.Name,
		Type: req.Type, Amount: req.Amount, Description: req.Description,
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"id": t.ID, "building_id": t.BuildingID, "name": t.Name, "type": t.Type,
		"amount": t.Amount, "is_active": t.Active,
	})
}

func (a *API) patchTemplate(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	var req struct {
		Name        string `json:"name"`
		Amount      int64  `json:"amount"`
		Description string `json:"description"`
		IsActive    *bool  `json:"is_active"`
	}
	if err := httpx.Decode(w, r, &req); err != nil {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "body", Message: "malformed JSON"}))
		return
	}
	t, err := a.billing.UpdateTemplate(r.Context(), uid, r.PathValue("templateID"), func(t *domain.Template) {
		if req.Name != "" {
			t.Name = req.Name
		}
		if req.Amount > 0 {
			t.Amount = req.Amount
		}
		if req.Description != "" {
			t.Description = req.Description
		}
		if req.IsActive != nil {
			t.Active = *req.IsActive
		}
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{
		"id": t.ID, "building_id": t.BuildingID, "name": t.Name, "type": t.Type,
		"amount": t.Amount, "is_active": t.Active,
	})
}

func (a *API) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	if err := a.billing.DeleteTemplate(r.Context(), uid, r.PathValue("templateID")); err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) listTemplates(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	ts, err := a.billing.Templates(r.Context(), uid, r.PathValue("buildingID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(ts))
	for i := range ts {
		t := &ts[i]
		out = append(out, map[string]any{"id": t.ID, "name": t.Name, "type": t.Type, "amount": t.Amount, "is_active": t.Active})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *API) generate(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	var req struct {
		Period    string `json:"period"`      // Jalali YYYY-MM
		DueInDays *int   `json:"due_in_days"` // days until due; default 14
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.Period == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "period", Message: "required"}))
		return
	}
	dueDays := 14
	if req.DueInDays != nil {
		dueDays = *req.DueInDays
	}
	dueIn := time.Duration(dueDays) * 24 * time.Hour
	n, err := a.billing.Generate(r.Context(), uid, r.PathValue("buildingID"), req.Period, dueIn)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"created": n, "period": req.Period})
}

func (a *API) listCharges(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	q := r.URL.Query()
	cs, err := a.billing.Charges(r.Context(), uid, domain.ChargeFilter{
		BuildingID: r.PathValue("buildingID"),
		UnitID:     q.Get("unit_id"),
		Period:     q.Get("period"),
		Status:     q.Get("status"),
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(cs))
	for i := range cs {
		c := &cs[i]
		out = append(out, map[string]any{
			"id": c.ID, "unit_id": c.UnitID, "template_id": c.TemplateID,
			"period": c.Period, "amount": c.Amount, "paid": c.Paid,
			"remaining": c.Remaining, "status": c.Status, "due_date": c.DueDate,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *API) recordPayment(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	var req struct {
		Amount int64  `json:"amount"`
		Method string `json:"method"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.Amount <= 0 || req.Method == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base,
			httpx.Validation{Field: "amount", Message: "must be positive"},
			httpx.Validation{Field: "method", Message: "required"}))
		return
	}
	tx, err := a.billing.RecordPayment(r.Context(), uid, r.PathValue("chargeID"), req.Amount, req.Method)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, map[string]any{
		"id": tx.ID, "charge_id": tx.ChargeID, "amount": tx.Amount,
		"method": tx.Method, "status": tx.Status, "reference": tx.Reference,
	})
}

func (a *API) confirmTx(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	if err := a.billing.ConfirmTransaction(r.Context(), uid, r.PathValue("txID")); err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "completed"})
}

func (a *API) listInvoices(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	var unitID *string
	if v := r.URL.Query().Get("unit_id"); v != "" {
		unitID = &v
	}
	invs, err := a.billing.Invoices(r.Context(), uid, r.PathValue("buildingID"), unitID)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(invs))
	for i := range invs {
		inv := &invs[i]
		out = append(out, map[string]any{
			"id": inv.ID, "unit_id": inv.UnitID, "period": inv.Period,
			"total": inv.Total, "paid": inv.Paid, "remaining": inv.Remaining,
			"is_paid": inv.IsPaid,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *API) report(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	rep, err := a.billing.Report(r.Context(), uid, r.PathValue("buildingID"), r.URL.Query().Get("period"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, rep)
}

func (a *API) sweepOverdue(w http.ResponseWriter, r *http.Request) {
	if err := a.billing.SweepOverdue(r.Context()); err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "swept"})
}

// NewServer wires middleware (used by main and tests).
func NewServer(api *API) http.Handler {
	mux := http.NewServeMux()
	api.Routes(mux)
	return httpx.Chain(mux, httpx.RequestID, httpx.AccessLog, httpx.Recover)
}
