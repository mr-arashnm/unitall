// Package sql provides a PostgreSQL adapter for property's domain ports.
// Migrations are applied on startup by the parent service's main.go.
package sql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"unital/backend/services/property/internal/domain"
)

// ── Helpers ───────────────────────────────────────────────────

func strPtr(s string) *string { return &s }
func tsPtr(t time.Time) *time.Time { return &t }
func boolPtr(b bool) *bool { return &b }

func scanTime(scanner interface{ Scan(...any) error }, dest *time.Time) error {
	var t sql.NullTime
	if err := scanner.Scan(&t); err != nil {
		return err
	}
	if t.Valid {
		*dest = t.Time
	}
	return nil
}

// ── BuildingStore ─────────────────────────────────────────────

type BuildingStore struct {
	db *sql.DB
}

func NewBuildingStore(db *sql.DB) *BuildingStore { return &BuildingStore{db: db} }

func (s *BuildingStore) Create(ctx context.Context, b *domain.Building) error {
	features, _ := json.Marshal(b.Features)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO buildings (id,name,code,type,address,floors,features,created_by,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT (code) DO NOTHING`,
		b.ID, b.Name, b.Code, b.Type, b.Address, b.Floors, features, b.CreatedBy, b.CreatedAt, b.UpdatedAt,
	)
	if err != nil {
		return err
	}
	// Check if it was actually inserted (ON CONFLICT DO NOTHING).
	var exists string
	_ = s.db.QueryRowContext(ctx, `SELECT id FROM buildings WHERE code=$1`, b.Code).Scan(&exists)
	if exists == "" {
		return domain.ErrCodeTaken
	}
	return nil
}

func (s *BuildingStore) ByID(ctx context.Context, id string) (*domain.Building, error) {
	var b domain.Building
	var features []byte
	err := s.db.QueryRowContext(ctx, `
		SELECT id,name,code,type,address,floors,features,created_by,created_at,updated_at
		FROM buildings WHERE id=$1`,
		id,
	).Scan(&b.ID, &b.Name, &b.Code, &b.Type, &b.Address, &b.Floors, &features, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(features, &b.Features); err != nil {
		b.Features = nil
	}
	return &b, nil
}

func (s *BuildingStore) Update(ctx context.Context, b *domain.Building) error {
	features, _ := json.Marshal(b.Features)
	b.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE buildings SET name=$2,type=$3,address=$4,floors=$5,features=$6,updated_at=$7 WHERE id=$1`,
		b.ID, b.Name, b.Type, b.Address, b.Floors, features, b.UpdatedAt,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *BuildingStore) ListByUser(ctx context.Context, userID string) ([]domain.Building, error) {
	// Buildings where user is creator OR holds a unit-party row.
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT b.id,b.name,b.code,b.type,b.address,b.floors,b.features,b.created_by,b.created_at,b.updated_at
		FROM buildings b
		LEFT JOIN units u ON u.building_id = b.id
		LEFT JOIN unit_parties up ON up.unit_id = u.id AND up.to_at IS NULL
		WHERE b.created_by = $1 OR up.user_id = $1
		ORDER BY b.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Building
	for rows.Next() {
		var b domain.Building
		var features []byte
		if err := rows.Scan(&b.ID, &b.Name, &b.Code, &b.Type, &b.Address, &b.Floors, &features, &b.CreatedBy, &b.CreatedAt, &b.UpdatedAt); err != nil {
			return nil, err
		}
		json.Unmarshal(features, &b.Features)
		out = append(out, b)
	}
	return out, rows.Err()
}

// ── UnitStore ─────────────────────────────────────────────────

type UnitStore struct {
	db *sql.DB
}

func NewUnitStore(db *sql.DB) *UnitStore { return &UnitStore{db: db} }

func (s *UnitStore) Create(ctx context.Context, u *domain.Unit) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO units (id,building_id,floor,number,area_m2,rooms,status,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)
		ON CONFLICT (building_id,number) DO NOTHING`,
		u.ID, u.BuildingID, u.Floor, u.Number, u.AreaM2, u.Rooms, u.Status, u.CreatedAt, u.UpdatedAt,
	)
	if err != nil {
		return err
	}
	var exists string
	_ = s.db.QueryRowContext(ctx, `SELECT id FROM units WHERE building_id=$1 AND number=$2`, u.BuildingID, u.Number).Scan(&exists)
	if exists == "" {
		return domain.ErrCodeTaken
	}
	return nil
}

func (s *UnitStore) ByID(ctx context.Context, id string) (*domain.Unit, error) {
	var u domain.Unit
	err := s.db.QueryRowContext(ctx, `
		SELECT id,building_id,floor,number,area_m2,rooms,status,created_at,updated_at
		FROM units WHERE id=$1`, id,
	).Scan(&u.ID, &u.BuildingID, &u.Floor, &u.Number, &u.AreaM2, &u.Rooms, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	return &u, err
}

func (s *UnitStore) Update(ctx context.Context, u *domain.Unit) error {
	u.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE units SET floor=$2,number=$3,area_m2=$4,rooms=$5,status=$6,updated_at=$7 WHERE id=$1`,
		u.ID, u.Floor, u.Number, u.AreaM2, u.Rooms, u.Status, u.UpdatedAt,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *UnitStore) ListByBuilding(ctx context.Context, buildingID string) ([]domain.Unit, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,building_id,floor,number,area_m2,rooms,status,created_at,updated_at
		FROM units WHERE building_id=$1 ORDER BY floor,number`,
		buildingID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Unit
	for rows.Next() {
		var u domain.Unit
		if err := rows.Scan(&u.ID, &u.BuildingID, &u.Floor, &u.Number, &u.AreaM2, &u.Rooms, &u.Status, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *UnitStore) CountByBuilding(ctx context.Context, buildingID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM units WHERE building_id=$1`, buildingID).Scan(&n)
	return n, err
}

// ── AssetStore ────────────────────────────────────────────────

type AssetStore struct {
	db *sql.DB
}

func NewAssetStore(db *sql.DB) *AssetStore { return &AssetStore{db: db} }

func (s *AssetStore) Create(ctx context.Context, a *domain.Asset) error {
	kind := string(a.Kind)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO assets (id,kind,building_id,code,floor,area_m2,unit_id)
		VALUES ($1,$2,$3,$4,$5,$6,NULL)
		ON CONFLICT (kind,building_id,code) DO NOTHING`,
		a.ID, kind, a.BuildingID, a.Code, a.Floor, a.AreaM2,
	)
	if err != nil {
		return err
	}
	var exists string
	_ = s.db.QueryRowContext(ctx, `SELECT id FROM assets WHERE kind=$1 AND building_id=$2 AND code=$3`, kind, a.BuildingID, a.Code).Scan(&exists)
	if exists == "" {
		return domain.ErrCodeTaken
	}
	return nil
}

func (s *AssetStore) ByID(ctx context.Context, id string) (*domain.Asset, error) {
	var a domain.Asset
	var kind string
	var unitID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id,kind,building_id,code,floor,area_m2,unit_id FROM assets WHERE id=$1`, id,
	).Scan(&a.ID, &kind, &a.BuildingID, &a.Code, &a.Floor, &a.AreaM2, &unitID)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.Kind = domain.AssetKind(kind)
	if unitID.Valid {
		a.UnitID = unitID.String
	}
	return &a, nil
}

// Update modifies editable fields on an asset. The kind and unit_id are
// not patchable here — assignment is a separate flow.
func (s *AssetStore) Update(ctx context.Context, a *domain.Asset) error {
	res, err := s.db.ExecContext(ctx, `
		UPDATE assets SET code=$2, floor=$3, area_m2=$4 WHERE id=$1`,
		a.ID, a.Code, a.Floor, a.AreaM2,
	)
	if err != nil {
		// Detect unique-constraint violation as a friendly error.
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return domain.ErrCodeTaken
		}
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *AssetStore) ByCode(ctx context.Context, kind domain.AssetKind, buildingID, code string) (*domain.Asset, error) {
	var a domain.Asset
	var unitID sql.NullString
	err := s.db.QueryRowContext(ctx, `
		SELECT id,kind,building_id,code,floor,area_m2,unit_id
		FROM assets WHERE kind=$1 AND building_id=$2 AND code=$3`,
		string(kind), buildingID, code,
	).Scan(&a.ID, &kind, &a.BuildingID, &a.Code, &a.Floor, &a.AreaM2, &unitID)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	a.Kind = domain.AssetKind(kind)
	if unitID.Valid {
		a.UnitID = unitID.String
	}
	return &a, nil
}

func (s *AssetStore) ListByBuilding(ctx context.Context, buildingID string, kind domain.AssetKind) ([]domain.Asset, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,kind,building_id,code,floor,area_m2,unit_id
		FROM assets WHERE building_id=$1 AND kind=$2 ORDER BY code`,
		buildingID, string(kind),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Asset
	for rows.Next() {
		var a domain.Asset
		var k string
		var unitID sql.NullString
		if err := rows.Scan(&a.ID, &k, &a.BuildingID, &a.Code, &a.Floor, &a.AreaM2, &unitID); err != nil {
			return nil, err
		}
		a.Kind = domain.AssetKind(k)
		if unitID.Valid {
			a.UnitID = unitID.String
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (s *AssetStore) Assign(ctx context.Context, assetID, unitID string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE assets SET unit_id=$2 WHERE id=$1`, assetID, unitID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	// Check asset wasn't already assigned to a different unit.
	var current string
	_ = s.db.QueryRowContext(ctx, `SELECT unit_id FROM assets WHERE id=$1`, assetID).Scan(&current)
	if current != "" && current != unitID {
		return domain.ErrAssetOccupied
	}
	return nil
}

func (s *AssetStore) Release(ctx context.Context, assetID string) error {
	res, err := s.db.ExecContext(ctx, `UPDATE assets SET unit_id=NULL WHERE id=$1`, assetID)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ── PartyStore ────────────────────────────────────────────────

type PartyStore struct {
	db *sql.DB
}

func NewPartyStore(db *sql.DB) *PartyStore { return &PartyStore{db: db} }

// Current returns the current (open) party row for a unit+role, or ErrNotFound.
func (s *PartyStore) Current(ctx context.Context, unitID string, role domain.PartyRole) (*domain.UnitParty, error) {
	var p domain.UnitParty
	err := s.db.QueryRowContext(ctx, `
		SELECT id,unit_id,role,user_id,from_at,to_at
		FROM unit_parties WHERE unit_id=$1 AND role=$2 AND to_at IS NULL`,
		unitID, string(role),
	).Scan(&p.ID, &p.UnitID, &p.Role, &p.UserID, &p.From, &p.To)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	return &p, err
}

// Append closes any current row and inserts a new one.
func (s *PartyStore) Append(ctx context.Context, p *domain.UnitParty) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Close any existing current row for this unit+role.
	_, _ = tx.ExecContext(ctx, `
		UPDATE unit_parties SET to_at=$3 WHERE unit_id=$1 AND role=$2 AND to_at IS NULL`,
		p.UnitID, string(p.Role), p.From,
	)

	_, err = tx.ExecContext(ctx, `
		INSERT INTO unit_parties (id,unit_id,role,user_id,from_at) VALUES ($1,$2,$3,$4,$5)`,
		p.ID, p.UnitID, string(p.Role), p.UserID, p.From,
	)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// History returns all party rows for a unit (including closed ones).
func (s *PartyStore) History(ctx context.Context, unitID string) ([]domain.UnitParty, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,unit_id,role,user_id,from_at,to_at
		FROM unit_parties WHERE unit_id=$1 ORDER BY from_at`,
		unitID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.UnitParty
	for rows.Next() {
		var p domain.UnitParty
		var role string
		var toAt sql.NullTime
		if err := rows.Scan(&p.ID, &p.UnitID, &role, &p.UserID, &p.From, &toAt); err != nil {
			return nil, err
		}
		p.Role = domain.PartyRole(role)
		if toAt.Valid {
			p.To = &toAt.Time
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// RecordTransfer inserts an audit record.
func (s *PartyStore) RecordTransfer(ctx context.Context, rec *domain.TransferRecord) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO transfer_records (id,unit_id,role,previous_user_id,new_user_id,effective_date,contract_number,recorded_by,description)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
		rec.ID, rec.UnitID, string(rec.Role), rec.PreviousUserID, rec.NewUserID,
		rec.EffectiveDate, rec.ContractNumber, rec.RecordedBy, rec.Description,
	)
	return err
}

// Transfers returns all transfer records for a unit.
func (s *PartyStore) Transfers(ctx context.Context, unitID string) ([]domain.TransferRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,unit_id,role,previous_user_id,new_user_id,effective_date,contract_number,recorded_by,description,created_at
		FROM transfer_records WHERE unit_id=$1 ORDER BY effective_date`,
		unitID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.TransferRecord
	for rows.Next() {
		var r domain.TransferRecord
		var role string
		if err := rows.Scan(&r.ID, &r.UnitID, &role, &r.PreviousUserID, &r.NewUserID, &r.EffectiveDate, &r.ContractNumber, &r.RecordedBy, &r.Description, &r.CreatedAt); err != nil {
			return nil, err
		}
		r.Role = domain.PartyRole(role)
		out = append(out, r)
	}
	return out, rows.Err()
}

// UnitIDsByUser returns distinct unit IDs for a user.
func (s *PartyStore) UnitIDsByUser(ctx context.Context, userID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT unit_id FROM unit_parties WHERE user_id=$1 AND to_at IS NULL`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

// ── ContractStore ─────────────────────────────────────────────

type ContractStore struct {
	db *sql.DB
}

func NewContractStore(db *sql.DB) *ContractStore { return &ContractStore{db: db} }

func (s *ContractStore) Create(ctx context.Context, c *domain.Contract) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO contracts (id,number,type,unit_id,first_party_id,second_party_id,title,
		                      amount,deposit_amount,start_date,end_date,duration_months,status,
		                      first_signed,second_signed,signed_date,created_by,created_at,updated_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19)`,
		c.ID, c.Number, c.Type, c.UnitID, c.FirstPartyID, c.SecondPartyID, c.Title,
		c.Amount, c.DepositAmount, c.StartDate, c.EndDate, c.DurationMonths, c.Status,
		c.FirstSigned, c.SecondSigned, c.SignedDate, c.CreatedBy, c.CreatedAt, c.UpdatedAt,
	)
	if err != nil {
		// Detect duplicate number at DB level.
		if strings.Contains(err.Error(), "unique") || strings.Contains(err.Error(), "duplicate") {
			return domain.ErrCodeTaken
		}
		return err
	}
	return nil
}

func (s *ContractStore) ByID(ctx context.Context, id string) (*domain.Contract, error) {
	var c domain.Contract
	var endDate, signedDate sql.NullTime
	err := s.db.QueryRowContext(ctx, `
		SELECT id,number,type,unit_id,first_party_id,second_party_id,title,
		       amount,deposit_amount,start_date,end_date,duration_months,status,
		       first_signed,second_signed,signed_date,created_by,created_at,updated_at
		FROM contracts WHERE id=$1`,
		id,
	).Scan(
		&c.ID, &c.Number, &c.Type, &c.UnitID, &c.FirstPartyID, &c.SecondPartyID, &c.Title,
		&c.Amount, &c.DepositAmount, &c.StartDate, &endDate, &c.DurationMonths, &c.Status,
		&c.FirstSigned, &c.SecondSigned, &signedDate, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	if endDate.Valid {
		c.EndDate = &endDate.Time
	}
	if signedDate.Valid {
		c.SignedDate = &signedDate.Time
	}
	return &c, nil
}

func (s *ContractStore) Update(ctx context.Context, c *domain.Contract) error {
	c.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx, `
		UPDATE contracts SET
			type=$2,unit_id=$3,first_party_id=$4,second_party_id=$5,title=$6,
			amount=$7,deposit_amount=$8,start_date=$9,end_date=$10,duration_months=$11,
			status=$12,first_signed=$13,second_signed=$14,signed_date=$15,
			updated_at=$16
		WHERE id=$1`,
		c.ID, c.Type, c.UnitID, c.FirstPartyID, c.SecondPartyID, c.Title,
		c.Amount, c.DepositAmount, c.StartDate, c.EndDate, c.DurationMonths,
		c.Status, c.FirstSigned, c.SecondSigned, c.SignedDate, c.UpdatedAt,
	)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *ContractStore) ListByUnit(ctx context.Context, unitID string) ([]domain.Contract, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id,number,type,unit_id,first_party_id,second_party_id,title,
		       amount,deposit_amount,start_date,end_date,duration_months,status,
		       first_signed,second_signed,signed_date,created_by,created_at,updated_at
		FROM contracts WHERE unit_id=$1 ORDER BY created_at DESC`,
		unitID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.Contract
	for rows.Next() {
		var c domain.Contract
		var endDate, signedDate sql.NullTime
		if err := rows.Scan(
			&c.ID, &c.Number, &c.Type, &c.UnitID, &c.FirstPartyID, &c.SecondPartyID, &c.Title,
			&c.Amount, &c.DepositAmount, &c.StartDate, &endDate, &c.DurationMonths, &c.Status,
			&c.FirstSigned, &c.SecondSigned, &signedDate, &c.CreatedBy, &c.CreatedAt, &c.UpdatedAt,
		); err != nil {
			return nil, err
		}
		if endDate.Valid {
			c.EndDate = &endDate.Time
		}
		if signedDate.Valid {
			c.SignedDate = &signedDate.Time
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// NextSequence atomically increments and returns the next sequence number for
// a given dateKey (YYYYMMDD format).
func (s *ContractStore) NextSequence(ctx context.Context, dateKey string) (int, error) {
	var seq int
	err := s.db.QueryRowContext(ctx, `
		INSERT INTO contract_sequences (date_key, last_seq)
		VALUES ($1, 1)
		ON CONFLICT (date_key) DO UPDATE SET last_seq = contract_sequences.last_seq + 1
		RETURNING last_seq`,
		dateKey,
	).Scan(&seq)
	if err != nil {
		return 0, fmt.Errorf("next sequence: %w", err)
	}
	return seq, nil
}

// ── MembershipTable (in-memory subset for DB-backed services) ──
// For the property service, membership checks are forwarded to identity
// via HTTPBootstrap. We implement a local table for DEV_TRUST_ALL bypass.

type MembershipTable struct {
	// keyed by "userID|buildingID|role"
	m map[string]bool
}

func NewMembershipTable() *MembershipTable {
	return &MembershipTable{m: map[string]bool{}}
}

func (m *MembershipTable) HasRole(ctx context.Context, userID, buildingID, role string) (bool, error) {
	if role == "*" {
		// Check if user has any role in the building.
		for k := range m.m {
			if strings.HasPrefix(k, userID+"|"+buildingID+"|") {
				return true, nil
			}
		}
		return false, nil
	}
	return m.m[userID+"|"+buildingID+"|"+role], nil
}

func (m *MembershipTable) AnyRole(ctx context.Context, userID, buildingID string, roles ...string) (bool, error) {
	for _, r := range roles {
		if r == "*" {
			if ok, _ := m.HasRole(ctx, userID, buildingID, "*"); ok {
				return true, nil
			}
			continue
		}
		if m.m[userID+"|"+buildingID+"|"+r] {
			return true, nil
		}
	}
	return false, nil
}

var _ domain.MembershipChecker = (*MembershipTable)(nil)

// SafeIsNotFound returns true if err is sql.ErrNoRows.
func SafeIsNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
