// Package usecase implements billing's application services.
package usecase

import (
	"context"
	"time"

	"unital/backend/pkg/events"
	"unital/backend/pkg/ids"
	"unital/backend/services/billing/internal/domain"
)

type Billing struct {
	templates  domain.TemplateStore
	charges    domain.ChargeStore
	txs        domain.TxStore
	invoices   domain.InvoiceStore
	directory  domain.UnitDirectory
	membership domain.MembershipChecker
	pub        events.Publisher
	now        func() time.Time
}

func New(t domain.TemplateStore, c domain.ChargeStore, tx domain.TxStore,
	i domain.InvoiceStore, d domain.UnitDirectory, m domain.MembershipChecker, pub events.Publisher) *Billing {
	return &Billing{templates: t, charges: c, txs: tx, invoices: i, directory: d, membership: m, pub: pub, now: time.Now}
}

// --- templates ---

func (b *Billing) CreateTemplate(ctx context.Context, actorID string, t domain.Template) (*domain.Template, error) {
	if err := b.requireManager(ctx, actorID, t.BuildingID); err != nil {
		return nil, err
	}
	t.ID = ids.New()
	t.Active = true
	t.CreatedAt = b.now().UTC()
	if err := b.templates.Create(ctx, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (b *Billing) Templates(ctx context.Context, actorID, buildingID string) ([]domain.Template, error) {
	if err := b.requireMember(ctx, actorID, buildingID); err != nil {
		return nil, err
	}
	return b.templates.ListByBuilding(ctx, buildingID)
}

// UpdateTemplate applies a manager-authored patch (amount, active, …).
func (b *Billing) UpdateTemplate(ctx context.Context, actorID, templateID string, patch func(*domain.Template)) (*domain.Template, error) {
	t, err := b.templates.ByID(ctx, templateID)
	if err != nil {
		return nil, err
	}
	if err := b.requireManager(ctx, actorID, t.BuildingID); err != nil {
		return nil, err
	}
	patch(t)
	if err := b.templates.Update(ctx, t); err != nil {
		return nil, err
	}
	return t, nil
}

// DeleteTemplate removes a template (charges already issued keep their
// snapshot of the amount).
func (b *Billing) DeleteTemplate(ctx context.Context, actorID, templateID string) error {
	t, err := b.templates.ByID(ctx, templateID)
	if err != nil {
		return err
	}
	if err := b.requireManager(ctx, actorID, t.BuildingID); err != nil {
		return err
	}
	return b.templates.Delete(ctx, templateID)
}

// --- generation ---

// Generate creates charges for every unit × active template of the
// building for the given Jalali period. Idempotent: existing
// (unit, template, period) triples are skipped.
func (b *Billing) Generate(ctx context.Context, actorID, buildingID, period string, dueIn time.Duration) (int, error) {
	if err := b.requireManager(ctx, actorID, buildingID); err != nil {
		return 0, err
	}
	unitIDs, err := b.directory.UnitIDs(ctx, buildingID)
	if err != nil {
		return 0, err
	}
	tmpls, err := b.templates.ListByBuilding(ctx, buildingID)
	if err != nil {
		return 0, err
	}
	due := b.now().UTC().Add(dueIn)
	created := 0
	for _, unitID := range unitIDs {
		for _, tp := range tmpls {
			if !tp.Active {
				continue
			}
			now := b.now().UTC()
			c := &domain.Charge{
				ID: ids.New(), BuildingID: buildingID, UnitID: unitID,
				TemplateID: tp.ID, Period: period, Amount: tp.Amount,
				DueDate: due, Status: domain.ChargePending, Remaining: tp.Amount,
				CreatedAt: now, UpdatedAt: now,
			}
			made, err := b.charges.Upsert(ctx, c)
			if err != nil {
				return created, err
			}
			if made {
				created++
			}
		}
	}
	b.publish(ctx, "charges.generated", buildingID, map[string]any{"period": period, "created": created, "units": len(unitIDs)})
	return created, nil
}

// --- charges & payments ---

func (b *Billing) Charges(ctx context.Context, actorID string, f domain.ChargeFilter) ([]domain.Charge, error) {
	if f.BuildingID != "" {
		if err := b.requireMember(ctx, actorID, f.BuildingID); err != nil {
			return nil, err
		}
	}
	return b.charges.List(ctx, f)
}

// RecordPayment creates a pending transaction. Online payments await a
// PSP callback; manual methods (cash/transfer) await staff confirmation.
func (b *Billing) RecordPayment(ctx context.Context, actorID, chargeID string, amount int64, method string) (*domain.Transaction, error) {
	c, err := b.charges.ByID(ctx, chargeID)
	if err != nil {
		return nil, err
	}
	if err := b.requireMember(ctx, actorID, c.BuildingID); err != nil {
		return nil, err
	}
	switch method {
	case domain.MethodOnline, domain.MethodCash, domain.MethodBank, domain.MethodCheque, domain.MethodPOS:
	default:
		return nil, domain.ErrForbidden
	}
	tx := &domain.Transaction{
		ID: ids.New(), ChargeID: chargeID, Amount: amount, Method: method,
		Status: domain.TxPending, Reference: "TX-" + ids.New()[:12],
		CreatedAt: b.now().UTC(),
	}
	if err := b.txs.Create(ctx, tx); err != nil {
		return nil, err
	}
	return tx, nil
}

// ConfirmTransaction settles a pending transaction: applies the amount
// to the charge and recomputes the invoice. Idempotent per state.
func (b *Billing) ConfirmTransaction(ctx context.Context, actorID, txID string) error {
	tx, err := b.txs.ByID(ctx, txID)
	if err != nil {
		return err
	}
	c, err := b.charges.ByID(ctx, tx.ChargeID)
	if err != nil {
		return err
	}
	if err := b.requireManager(ctx, actorID, c.BuildingID); err != nil {
		return err
	}
	if tx.Status != domain.TxPending {
		return domain.ErrAlreadySettled
	}
	if c.Remaining < tx.Amount {
		return domain.ErrOverpayment
	}
	now := b.now().UTC()
	tx.Status = domain.TxCompleted
	tx.PaymentDate = &now
	if err := b.txs.Update(ctx, tx); err != nil {
		return err
	}
	c.Paid += tx.Amount
	c.Remaining -= tx.Amount
	switch {
	case c.Remaining == 0:
		c.Status = domain.ChargePaid
	default:
		c.Status = domain.ChargePartiallyPaid
	}
	c.UpdatedAt = now
	if err := b.charges.Update(ctx, c); err != nil {
		return err
	}
	b.refreshInvoice(ctx, c)
	b.publish(ctx, "payment.completed", tx.ID, map[string]any{
		"charge_id": c.ID, "unit_id": c.UnitID, "building_id": c.BuildingID,
		"amount": tx.Amount, "method": tx.Method,
	})
	return nil
}

func (b *Billing) refreshInvoice(ctx context.Context, c *domain.Charge) {
	inv, err := b.invoices.GetOrCreate(ctx, c.BuildingID, c.UnitID, c.Period)
	if err != nil {
		return
	}
	charges, err := b.charges.List(ctx, domain.ChargeFilter{BuildingID: c.BuildingID, UnitID: c.UnitID, Period: c.Period})
	if err != nil {
		return
	}
	var total, paid int64
	for _, ch := range charges {
		total += ch.Amount
		paid += ch.Paid
	}
	inv.Total, inv.Paid, inv.Remaining = total, paid, total-paid
	inv.IsPaid = total == paid
	inv.UpdatedAt = b.now().UTC()
	_ = b.invoices.Save(ctx, inv)
	if inv.IsPaid {
		b.publish(ctx, "invoice.settled", inv.ID, map[string]any{"unit_id": inv.UnitID, "period": inv.Period})
	}
}

// Invoices lists invoices, optionally scoped to a unit.
func (b *Billing) Invoices(ctx context.Context, actorID, buildingID string, unitID *string) ([]domain.Invoice, error) {
	if err := b.requireMember(ctx, actorID, buildingID); err != nil {
		return nil, err
	}
	return b.invoices.List(ctx, buildingID, unitID)
}

// Report summarizes collections for a building/period.
type Report struct {
	Period         string  `json:"period"`
	Units          int     `json:"units"`
	TotalBilled    int64   `json:"total_billed"`
	TotalCollected int64   `json:"total_collected"`
	Outstanding    int64   `json:"outstanding"`
	OverdueCount   int     `json:"overdue_count"`
	CollectionRate float64 `json:"collection_rate"` // 0..1
}

func (b *Billing) Report(ctx context.Context, actorID, buildingID, period string) (*Report, error) {
	if err := b.requireManager(ctx, actorID, buildingID); err != nil {
		return nil, err
	}
	charges, err := b.charges.List(ctx, domain.ChargeFilter{BuildingID: buildingID, Period: period})
	if err != nil {
		return nil, err
	}
	rep := &Report{Period: period}
	units := map[string]bool{}
	for _, c := range charges {
		units[c.UnitID] = true
		rep.TotalBilled += c.Amount
		rep.TotalCollected += c.Paid
		if c.Status == domain.ChargeOverdue {
			rep.OverdueCount++
		}
	}
	rep.Units = len(units)
	rep.Outstanding = rep.TotalBilled - rep.TotalCollected
	if rep.TotalBilled > 0 {
		rep.CollectionRate = float64(rep.TotalCollected) / float64(rep.TotalBilled)
	}
	return rep, nil
}

// SweepOverdue flips past-due pending/partial charges to overdue and
// emits charge.overdue per charge (cron entrypoint).
func (b *Billing) SweepOverdue(ctx context.Context) error {
	pending, err := b.charges.List(ctx, domain.ChargeFilter{Status: domain.ChargePending})
	if err != nil {
		return err
	}
	partial, err := b.charges.List(ctx, domain.ChargeFilter{Status: domain.ChargePartiallyPaid})
	if err != nil {
		return err
	}
	now := b.now().UTC()
	for _, c := range append(pending, partial...) {
		if c.DueDate.After(now) {
			continue
		}
		c.Status = domain.ChargeOverdue
		c.UpdatedAt = now
		if err := b.charges.Update(ctx, &c); err != nil {
			return err
		}
		b.publish(ctx, "charge.overdue", c.ID, map[string]any{
			"unit_id": c.UnitID, "building_id": c.BuildingID, "period": c.Period,
			"remaining": c.Remaining,
		})
	}
	return nil
}

func (b *Billing) requireManager(ctx context.Context, userID, buildingID string) error {
	ok, err := b.membership.AnyRole(ctx, userID, buildingID, "manager", "board_member")
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

func (b *Billing) requireMember(ctx context.Context, userID, buildingID string) error {
	ok, err := b.membership.HasRole(ctx, userID, buildingID, "*")
	if err != nil {
		return err
	}
	if !ok {
		return domain.ErrForbidden
	}
	return nil
}

func (b *Billing) publish(ctx context.Context, typ, subject string, data map[string]any) {
	_ = b.pub.Publish(ctx, events.New("billing", typ, subject, data))
}
