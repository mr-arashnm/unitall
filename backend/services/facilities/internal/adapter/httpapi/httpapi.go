// Package httpapi exposes the facilities REST surface.
package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"unital/backend/pkg/httpx"
	"unital/backend/pkg/jwtx"
	"unital/backend/services/facilities/internal/domain"
	"unital/backend/services/facilities/internal/usecase"
)

type API struct {
	fac    *usecase.Facilities
	signer *jwtx.Signer
	base   string
}

func New(f *usecase.Facilities, signer *jwtx.Signer) *API {
	return &API{fac: f, signer: signer, base: "facilities"}
}

func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /facilities", a.createFacility)
	mux.HandleFunc("GET /buildings/{buildingID}/facilities", a.listFacilities)
	mux.HandleFunc("GET /facilities/{facilityID}", a.getFacility)
	mux.HandleFunc("PATCH /facilities/{facilityID}", a.patchFacility)
	mux.HandleFunc("DELETE /facilities/{facilityID}", a.deleteFacility)
	mux.HandleFunc("GET /facilities/{facilityID}/availability", a.availability)
	mux.HandleFunc("POST /facilities/{facilityID}/bookings", a.book)
	mux.HandleFunc("GET /bookings", a.listBookings)
	mux.HandleFunc("GET /bookings/{bookingID}", a.getBooking)
	mux.HandleFunc("POST /bookings/{bookingID}/approve", a.approve)
	mux.HandleFunc("POST /bookings/{bookingID}/reject", a.reject)
	mux.HandleFunc("POST /bookings/{bookingID}/cancel", a.cancel)
	mux.HandleFunc("POST /facilities/{facilityID}/maintenance-windows", a.scheduleMaintenance)
	mux.HandleFunc("POST /maintenance-windows/{maintenanceID}/start", a.maintAction("start"))
	mux.HandleFunc("POST /maintenance-windows/{maintenanceID}/complete", a.maintAction("complete"))
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
	case errors.Is(err, domain.ErrConflict):
		p = httpx.NewProblem(a.base, "SLOT_CONFLICT", "Slot conflicts with a booking or maintenance", http.StatusConflict)
	case errors.Is(err, domain.ErrClosed):
		p = httpx.NewProblem(a.base, "FACILITY_CLOSED", "Outside facility opening hours", http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrTooSoon):
		p = httpx.NewProblem(a.base, "TOO_SOON", "Inside minimum advance window", http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrTooFar):
		p = httpx.NewProblem(a.base, "TOO_FAR", "Outside advance-booking window", http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrOverCapacity):
		p = httpx.NewProblem(a.base, "OVER_CAPACITY", "Participants exceed capacity", http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrUnderMaintenance):
		p = httpx.NewProblem(a.base, "UNDER_MAINTENANCE", "Facility inactive or under maintenance", http.StatusConflict)
	case errors.Is(err, domain.ErrInvalidState):
		p = httpx.NewProblem(a.base, "INVALID_STATE", "Invalid state or input", http.StatusUnprocessableEntity)
	}
	httpx.WriteError(w, r, p)
}

func (a *API) createFacility(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	var f domain.Facility
	if err := httpx.Decode(w, r, &f); err != nil || f.Name == "" || f.BuildingID == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base,
			httpx.Validation{Field: "name", Message: "required"},
			httpx.Validation{Field: "building_id", Message: "required"}))
		return
	}
	out, err := a.fac.CreateFacility(r.Context(), uid, &f)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (a *API) listFacilities(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	fs, err := a.fac.List(r.Context(), uid, r.PathValue("buildingID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": fs})
}

func (a *API) getFacility(w http.ResponseWriter, r *http.Request) {
	f, err := a.fac.Facility(r.Context(), r.PathValue("facilityID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, f)
}

func (a *API) patchFacility(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	var patch struct {
		Name, Description, OpeningTime, ClosingTime string
		Capacity, MinAdvanceH, MaxAdvanceH          int
		HourlyRate                                  int64
		IsActive                                    *bool
		Rules, Images                               []string
	}
	if err := httpx.Decode(w, r, &patch); err != nil {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "body", Message: "malformed JSON"}))
		return
	}
	out, err := a.fac.UpdateFacility(r.Context(), uid, r.PathValue("facilityID"), func(f *domain.Facility) {
		if patch.Name != "" {
			f.Name = patch.Name
		}
		if patch.Description != "" {
			f.Description = patch.Description
		}
		if patch.OpeningTime != "" {
			f.OpeningTime = patch.OpeningTime
		}
		if patch.ClosingTime != "" {
			f.ClosingTime = patch.ClosingTime
		}
		if patch.Capacity > 0 {
			f.Capacity = patch.Capacity
		}
		if patch.MinAdvanceH > 0 {
			f.MinAdvanceH = patch.MinAdvanceH
		}
		if patch.MaxAdvanceH > 0 {
			f.MaxAdvanceH = patch.MaxAdvanceH
		}
		if patch.HourlyRate >= 0 && patch.HourlyRate != 0 {
			f.HourlyRate = patch.HourlyRate
		}
		if patch.IsActive != nil {
			f.IsActive = *patch.IsActive
		}
		if patch.Rules != nil {
			f.Rules = patch.Rules
		}
		if patch.Images != nil {
			f.Images = patch.Images
		}
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, out)
}

func (a *API) deleteFacility(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	if err := a.fac.DeleteFacility(r.Context(), uid, r.PathValue("facilityID")); err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (a *API) availability(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.userID(r); !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	day := time.Now().UTC()
	if d := r.URL.Query().Get("date"); d != "" {
		if t, err := time.Parse("2006-01-02", d); err == nil {
			day = t
		}
	}
	hours, err := a.fac.Availability(r.Context(), r.PathValue("facilityID"), day)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	if hours == nil {
		hours = []int{}
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"date": day.Format("2006-01-02"), "free_hours": hours})
}

func (a *API) book(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	var req struct {
		Start        string `json:"start"`
		End          string `json:"end"`
		Purpose      string `json:"purpose"`
		Participants int    `json:"participants"`
	}
	if err := httpx.Decode(w, r, &req); err != nil || req.Start == "" || req.End == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base,
			httpx.Validation{Field: "start", Message: "required (RFC3339)"},
			httpx.Validation{Field: "end", Message: "required (RFC3339)"}))
		return
	}
	start, err1 := time.Parse(time.RFC3339, req.Start)
	end, err2 := time.Parse(time.RFC3339, req.End)
	if err1 != nil || err2 != nil {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "start/end", Message: "must be RFC3339"}))
		return
	}
	b, err := a.fac.Book(r.Context(), usecase.BookingInput{
		FacilityID: r.PathValue("facilityID"), UserID: uid,
		Start: start, End: end, Purpose: req.Purpose, Participants: req.Participants,
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, b)
}

func (a *API) listBookings(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	if r.URL.Query().Get("mine") == "true" {
		bs, err := a.fac.MyBookings(r.Context(), uid)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, map[string]any{"data": bs})
		return
	}
	httpx.WriteError(w, r, httpx.NewProblem(a.base, "NOT_FOUND", "Use ?mine=true or a building-scoped route", http.StatusNotFound))
}

func (a *API) getBooking(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.userID(r); !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	b, err := a.fac.Booking(r.Context(), r.PathValue("bookingID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, b)
}

func (a *API) approve(w http.ResponseWriter, r *http.Request) { a.decide(w, r, "approve") }
func (a *API) reject(w http.ResponseWriter, r *http.Request)  { a.decide(w, r, "reject") }
func (a *API) cancel(w http.ResponseWriter, r *http.Request)  { a.decide(w, r, "cancel") }

func (a *API) decide(w http.ResponseWriter, r *http.Request, action string) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	b, err := a.fac.Decide(r.Context(), uid, r.PathValue("bookingID"), action)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, b)
}

func (a *API) scheduleMaintenance(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	var m domain.Maintenance
	if err := httpx.Decode(w, r, &m); err != nil || m.Title == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "title", Message: "required"}))
		return
	}
	m.FacilityID = r.PathValue("facilityID")
	out, err := a.fac.ScheduleMaintenance(r.Context(), uid, &m)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, out)
}

func (a *API) maintAction(action string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		uid, ok := a.userID(r)
		if !ok {
			httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
			return
		}
		m, err := a.fac.MaintenanceAction(r.Context(), uid, r.PathValue("maintenanceID"), action)
		if err != nil {
			a.fail(w, r, err)
			return
		}
		httpx.JSON(w, http.StatusOK, m)
	}
}

// NewServer wires middleware (used by main and tests).
func NewServer(api *API) http.Handler {
	mux := http.NewServeMux()
	api.Routes(mux)
	return httpx.Chain(mux, httpx.RequestID, httpx.AccessLog, httpx.Recover)
}
