// Package httpapi exposes property's REST surface (docs/API_DESIGN.md).
package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"unital/backend/pkg/httpx"
	"unital/backend/pkg/jwtx"
	"unital/backend/services/property/internal/domain"
	"unital/backend/services/property/internal/usecase"
)

type API struct {
	prop   *usecase.Property
	signer *jwtx.Signer
	base   string
}

func New(prop *usecase.Property, signer *jwtx.Signer) *API {
	return &API{prop: prop, signer: signer, base: "property"}
}

func (a *API) Routes(mux *http.ServeMux) {
	mux.HandleFunc("POST /buildings", a.createBuilding)
	mux.HandleFunc("GET /buildings", a.listBuildings)
	mux.HandleFunc("GET /buildings/{buildingID}", a.getBuilding)
	mux.HandleFunc("PATCH /buildings/{buildingID}", a.patchBuilding)
	mux.HandleFunc("POST /buildings/{buildingID}/units", a.createUnit)
	mux.HandleFunc("GET /buildings/{buildingID}/units", a.listUnits)
	mux.HandleFunc("GET /units/{unitID}", a.getUnit)
	mux.HandleFunc("PATCH /units/{unitID}", a.patchUnit)
	mux.HandleFunc("GET /units/{unitID}/parties", a.unitParties)
	mux.HandleFunc("GET /units/{unitID}/transfer-history", a.unitTransfers)
	mux.HandleFunc("POST /units/{unitID}/ownership-changes", a.changeOwnership)
	mux.HandleFunc("POST /units/{unitID}/residency-changes", a.changeResidency)
	mux.HandleFunc("POST /buildings/{buildingID}/assets", a.createAsset)
	mux.HandleFunc("GET /buildings/{buildingID}/assets", a.listAssets)
	mux.HandleFunc("GET /assets/{assetID}", a.getAsset)
	mux.HandleFunc("PATCH /assets/{assetID}", a.patchAsset)
	mux.HandleFunc("POST /units/{unitID}/assets", a.assignAsset)
	mux.HandleFunc("DELETE /units/{unitID}/assets/{assetCode}", a.releaseAsset)
	mux.HandleFunc("POST /contracts", a.createContract)
	mux.HandleFunc("GET /contracts/{contractID}", a.getContract)
	mux.HandleFunc("GET /units/{unitID}/contracts", a.unitContracts)
	mux.HandleFunc("POST /contracts/{contractID}/sign", a.signContract)
	mux.HandleFunc("POST /contracts/{contractID}/activate", a.activateContract)
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
	case errors.Is(err, domain.ErrCodeTaken):
		p = httpx.NewProblem(a.base, "CODE_TAKEN", "Code already exists", http.StatusConflict)
	case errors.Is(err, domain.ErrAssetOccupied):
		p = httpx.NewProblem(a.base, "ASSET_OCCUPIED", "Asset is already assigned", http.StatusConflict)
	case errors.Is(err, domain.ErrAssetFree):
		p = httpx.NewProblem(a.base, "ASSET_NOT_ASSIGNED", "Asset is not assigned to this unit", http.StatusConflict)
	case errors.Is(err, domain.ErrInvalidState):
		p = httpx.NewProblem(a.base, "INVALID_STATE", "Invalid state or input", http.StatusUnprocessableEntity)
	case errors.Is(err, domain.ErrMissingSignatures):
		p = httpx.NewProblem(a.base, "MISSING_SIGNATURES", "Contract not fully signed", http.StatusConflict)
	}
	httpx.WriteError(w, r, p)
}

func decode[T any](a *API, w http.ResponseWriter, r *http.Request) (T, bool) {
	var v T
	if err := httpx.Decode(w, r, &v); err != nil {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "body", Message: "malformed JSON"}))
		return v, false
	}
	return v, true
}

func (a *API) createBuilding(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	req, ok := decode[struct {
		Name     string   `json:"name"`
		Code     string   `json:"code"`
		Type     string   `json:"type"`
		Address  string   `json:"address"`
		Floors   int      `json:"floors"`
		Features []string `json:"features"`
	}](a, w, r)
	if !ok {
		return
	}
	b, err := a.prop.CreateBuilding(r.Context(), usecase.BuildingInput{
		Name: req.Name, Code: req.Code, BuildingType: req.Type,
		Address: req.Address, Floors: req.Floors, Features: req.Features,
		CreatedBy: uid,
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, dtoBuilding(b))
}

func (a *API) listBuildings(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	bs, err := a.prop.BuildingsForUser(r.Context(), uid)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(bs))
	for i := range bs {
		out = append(out, dtoBuilding(&bs[i]))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *API) getBuilding(w http.ResponseWriter, r *http.Request) {
	b, err := a.prop.Building(r.Context(), r.PathValue("buildingID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dtoBuilding(b))
}

func (a *API) patchBuilding(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	req, ok := decode[struct {
		Name     string   `json:"name"`
		Type     string   `json:"type"`
		Address  string   `json:"address"`
		Floors   int      `json:"floors"`
		Features []string `json:"features"`
	}](a, w, r)
	if !ok {
		return
	}
	b, err := a.prop.UpdateBuilding(r.Context(), uid, r.PathValue("buildingID"), func(b *domain.Building) {
		if req.Name != "" {
			b.Name = req.Name
		}
		if req.Type != "" {
			b.Type = req.Type
		}
		if req.Address != "" {
			b.Address = req.Address
		}
		if req.Floors > 0 {
			b.Floors = req.Floors
		}
		if req.Features != nil {
			b.Features = req.Features
		}
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dtoBuilding(b))
}

func (a *API) createUnit(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	req, ok := decode[struct {
		Floor  int     `json:"floor"`
		Number string  `json:"number"`
		AreaM2 float64 `json:"area_m2"`
		Rooms  int     `json:"rooms"`
	}](a, w, r)
	if !ok {
		return
	}
	u, err := a.prop.CreateUnit(r.Context(), uid, usecase.UnitInput{
		BuildingID: r.PathValue("buildingID"), Floor: req.Floor,
		Number: req.Number, AreaM2: req.AreaM2, Rooms: req.Rooms,
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, dtoUnit(u))
}

func (a *API) listUnits(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	us, err := a.prop.Units(r.Context(), uid, r.PathValue("buildingID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(us))
	for i := range us {
		out = append(out, dtoUnit(&us[i]))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *API) getUnit(w http.ResponseWriter, r *http.Request) {
	u, err := a.prop.Unit(r.Context(), r.PathValue("unitID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dtoUnit(u))
}

func (a *API) patchUnit(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	req, ok := decode[struct {
		Status string  `json:"status"`
		AreaM2 float64 `json:"area_m2"`
		Rooms  int     `json:"rooms"`
	}](a, w, r)
	if !ok {
		return
	}
	u, err := a.prop.UpdateUnit(r.Context(), uid, r.PathValue("unitID"), func(u *domain.Unit) {
		if req.Status != "" {
			u.Status = req.Status
		}
		if req.AreaM2 > 0 {
			u.AreaM2 = req.AreaM2
		}
		if req.Rooms > 0 {
			u.Rooms = req.Rooms
		}
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dtoUnit(u))
}

func (a *API) unitParties(w http.ResponseWriter, r *http.Request) {
	parties, err := a.prop.CurrentParties(r.Context(), r.PathValue("unitID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	out := map[string]any{}
	for role, p := range parties {
		out[string(role)] = map[string]any{"user_id": p.UserID, "from": p.From}
	}
	hist, _ := a.prop.PartyHistory(r.Context(), r.PathValue("unitID"))
	rows := make([]map[string]any, 0, len(hist))
	for _, h := range hist {
		rows = append(rows, map[string]any{"role": string(h.Role), "user_id": h.UserID, "from": h.From, "to": h.To})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"current": out, "history": rows})
}

func (a *API) unitTransfers(w http.ResponseWriter, r *http.Request) {
	recs, err := a.prop.TransferHistory(r.Context(), r.PathValue("unitID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	rows := make([]map[string]any, 0, len(recs))
	for _, rec := range recs {
		rows = append(rows, map[string]any{
			"id": rec.ID, "role": string(rec.Role), "previous_user_id": rec.PreviousUserID,
			"new_user_id": rec.NewUserID, "effective_date": rec.EffectiveDate,
			"contract_number": rec.ContractNumber, "recorded_by": rec.RecordedBy,
			"description": rec.Description, "created_at": rec.CreatedAt,
		})
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": rows})
}

func (a *API) changeOwnership(w http.ResponseWriter, r *http.Request) {
	a.changeParty(w, r, domain.PartyOwner)
}

func (a *API) changeResidency(w http.ResponseWriter, r *http.Request) {
	a.changeParty(w, r, domain.PartyResident)
}

func (a *API) changeParty(w http.ResponseWriter, r *http.Request, role domain.PartyRole) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	req, ok := decode[struct {
		NewUserID      string `json:"new_user_id"`
		EffectiveDate  string `json:"effective_date"` // YYYY-MM-DD
		ContractNumber string `json:"contract_number"`
		Description    string `json:"description"`
	}](a, w, r)
	if !ok || req.NewUserID == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "new_user_id", Message: "required"}))
		return
	}
	eff := time.Now().UTC()
	if req.EffectiveDate != "" {
		if t, err := time.Parse("2006-01-02", req.EffectiveDate); err == nil {
			eff = t
		}
	}
	if err := a.prop.ChangeParty(r.Context(), uid, r.PathValue("unitID"), role, req.NewUserID, eff, req.ContractNumber, req.Description); err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": string(role) + "_changed"})
}

func (a *API) createAsset(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	req, ok := decode[struct {
		Kind   string  `json:"kind"` // parking | warehouse
		Code   string  `json:"code"`
		Floor  int     `json:"floor"`
		AreaM2 float64 `json:"area_m2"`
	}](a, w, r)
	if !ok || req.Code == "" || req.Kind == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base,
			httpx.Validation{Field: "kind", Message: "required"},
			httpx.Validation{Field: "code", Message: "required"}))
		return
	}
	asset, err := a.prop.CreateAsset(r.Context(), uid, usecase.AssetInput{
		BuildingID: r.PathValue("buildingID"), Kind: domain.AssetKind(req.Kind),
		Code: req.Code, Floor: req.Floor, AreaM2: req.AreaM2,
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, dtoAsset(asset))
}

func (a *API) listAssets(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	kind := domain.AssetKind(r.URL.Query().Get("kind"))
	if kind != domain.AssetParking && kind != domain.AssetWarehouse {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "kind", Message: "must be parking or warehouse"}))
		return
	}
	availableOnly := r.URL.Query().Get("status") == "available"
	list, err := a.prop.Assets(r.Context(), uid, r.PathValue("buildingID"), kind, availableOnly)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(list))
	for i := range list {
		out = append(out, dtoAsset(&list[i]))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *API) getAsset(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	asset, err := a.prop.AssetByID(r.Context(), uid, r.PathValue("assetID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dtoAsset(asset))
}

func (a *API) patchAsset(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	req, ok := decode[struct {
		Code   string  `json:"code"`
		Floor  int     `json:"floor"`
		AreaM2 float64 `json:"area_m2"`
	}](a, w, r)
	if !ok {
		return
	}
	asset, err := a.prop.UpdateAsset(r.Context(), uid, r.PathValue("assetID"), func(a *domain.Asset) {
		if req.Code != "" {
			a.Code = req.Code
		}
		if req.Floor != 0 {
			a.Floor = req.Floor
		}
		if req.AreaM2 != 0 {
			a.AreaM2 = req.AreaM2
		}
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dtoAsset(asset))
}

func (a *API) assignAsset(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	req, ok := decode[struct {
		Kind string `json:"kind"`
		Code string `json:"code"`
	}](a, w, r)
	if !ok || req.Code == "" || req.Kind == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base,
			httpx.Validation{Field: "kind", Message: "required"},
			httpx.Validation{Field: "code", Message: "required"}))
		return
	}
	asset, err := a.prop.AssignAsset(r.Context(), uid, domain.AssetKind(req.Kind), "", req.Code, r.PathValue("unitID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dtoAsset(asset))
}

func (a *API) releaseAsset(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	kind := domain.AssetKind(r.URL.Query().Get("kind"))
	if kind != domain.AssetParking && kind != domain.AssetWarehouse {
		httpx.WriteError(w, r, httpx.Invalid(a.base, httpx.Validation{Field: "kind", Message: "must be parking or warehouse"}))
		return
	}
	if err := a.prop.ReleaseAsset(r.Context(), uid, kind, r.PathValue("assetCode"), r.PathValue("unitID")); err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "released"})
}

func (a *API) createContract(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	req, ok := decode[struct {
		Type           string `json:"type"`
		UnitID         string `json:"unit_id"`
		FirstPartyID   string `json:"first_party_id"`
		SecondPartyID  string `json:"second_party_id"`
		Title          string `json:"title"`
		Amount         int64  `json:"amount"`
		DepositAmount  int64  `json:"deposit_amount"`
		StartDate      string `json:"start_date"`
		DurationMonths int    `json:"duration_months"`
	}](a, w, r)
	if !ok || req.UnitID == "" || req.Type == "" {
		httpx.WriteError(w, r, httpx.Invalid(a.base,
			httpx.Validation{Field: "type", Message: "required"},
			httpx.Validation{Field: "unit_id", Message: "required"}))
		return
	}
	start := time.Now().UTC()
	if req.StartDate != "" {
		if t, err := time.Parse("2006-01-02", req.StartDate); err == nil {
			start = t
		}
	}
	c, err := a.prop.CreateContract(r.Context(), usecase.ContractInput{
		Type: req.Type, UnitID: req.UnitID, FirstPartyID: req.FirstPartyID,
		SecondPartyID: req.SecondPartyID, Title: req.Title, Amount: req.Amount,
		DepositAmount: req.DepositAmount, StartDate: start,
		DurationMonths: req.DurationMonths, CreatedBy: uid,
	})
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusCreated, dtoContract(c))
}

func (a *API) getContract(w http.ResponseWriter, r *http.Request) {
	c, err := a.prop.Contract(r.Context(), r.PathValue("contractID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dtoContract(c))
}

func (a *API) unitContracts(w http.ResponseWriter, r *http.Request) {
	cs, err := a.prop.ContractsByUnit(r.Context(), r.PathValue("unitID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	out := make([]map[string]any, 0, len(cs))
	for i := range cs {
		out = append(out, dtoContract(&cs[i]))
	}
	httpx.JSON(w, http.StatusOK, map[string]any{"data": out})
}

func (a *API) signContract(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	if err := a.prop.Sign(r.Context(), r.PathValue("contractID"), uid); err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, map[string]string{"status": "signed"})
}

func (a *API) activateContract(w http.ResponseWriter, r *http.Request) {
	uid, ok := a.userID(r)
	if !ok {
		httpx.WriteError(w, r, httpx.NewProblem(a.base, "UNAUTHORIZED", "Authentication required", http.StatusUnauthorized))
		return
	}
	c, err := a.prop.Activate(r.Context(), uid, r.PathValue("contractID"))
	if err != nil {
		a.fail(w, r, err)
		return
	}
	httpx.JSON(w, http.StatusOK, dtoContract(c))
}

func dtoBuilding(b *domain.Building) map[string]any {
	return map[string]any{
		"id": b.ID, "name": b.Name, "code": b.Code, "type": b.Type,
		"address": b.Address, "floors": b.Floors, "features": b.Features,
		"created_by": b.CreatedBy, "created_at": b.CreatedAt,
	}
}

func dtoUnit(u *domain.Unit) map[string]any {
	return map[string]any{
		"id": u.ID, "building_id": u.BuildingID, "floor": u.Floor,
		"number": u.Number, "area_m2": u.AreaM2, "rooms": u.Rooms,
		"status": u.Status, "created_at": u.CreatedAt,
	}
}

func dtoAsset(a *domain.Asset) map[string]any {
	return map[string]any{
		"id": a.ID, "kind": string(a.Kind), "building_id": a.BuildingID,
		"code": a.Code, "floor": a.Floor, "area_m2": a.AreaM2,
		"unit_id": a.UnitID, "available": a.UnitID == "",
	}
}

func dtoContract(c *domain.Contract) map[string]any {
	return map[string]any{
		"id": c.ID, "number": c.Number, "type": c.Type, "unit_id": c.UnitID,
		"first_party_id": c.FirstPartyID, "second_party_id": c.SecondPartyID,
		"title": c.Title, "amount": c.Amount, "deposit_amount": c.DepositAmount,
		"start_date": c.StartDate, "duration_months": c.DurationMonths,
		"status": c.Status, "first_signed": c.FirstSigned,
		"second_signed": c.SecondSigned, "signed_date": c.SignedDate,
	}
}

// NewServer wires middleware (used by main and tests).
func NewServer(api *API) http.Handler {
	mux := http.NewServeMux()
	api.Routes(mux)
	return httpx.Chain(mux, httpx.RequestID, httpx.AccessLog, httpx.Recover)
}
