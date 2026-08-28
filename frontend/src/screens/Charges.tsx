// Charges — period-grouped list with payment action.
import { useEffect, useMemo, useState } from "react";
import { fetchCharges, payCharge, fmtRial, fmtPeriod } from "../api";
import type { Charge } from "../api";
import { BackBtn, StatusPill, Empty, Toast } from "../components/ui";
import { IconCreditCard, IconChevronDown, IconChevronUp } from "../components/icons";

export default function Charges() {
  const [charges, setCharges] = useState<Charge[] | null>(null);
  const [paying, setPaying] = useState<string | null>(null);
  const [toastMsg, setToastMsg] = useState("");
  const [expandedPeriod, setExpandedPeriod] = useState<string | null>(null);

  useEffect(() => { fetchCharges().then((r) => setCharges(r.data)); }, []);

  const byPeriod = useMemo(() => {
    const map = new Map<string, Charge[]>();
    for (const c of charges ?? []) {
      if (!map.has(c.period)) map.set(c.period, []);
      map.get(c.period)!.push(c);
    }
    return [...map.entries()].sort((a, b) => b[0].localeCompare(a[0]));
  }, [charges]);

  async function pay(c: Charge) {
    setPaying(c.id);
    try {
      await payCharge(c.id, c.remaining, "online");
      // Refresh charges list to reflect new status
      const updated = await fetchCharges();
      setCharges(updated.data);
      setToastMsg(`Payment of ${fmtRial(c.remaining)} submitted successfully.`);
    } catch (err: unknown) {
      setToastMsg(err instanceof Error ? err.message : "Payment failed. Please try again.");
    } finally {
      setPaying(null);
    }
  }

  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 12 }}>
        <BackBtn onClick={() => window.history.back()} />
        <h1 className="page-title" style={{ margin: 0 }}>Charges</h1>
        <div style={{ width: 38 }} />
      </div>

      {charges === null && <div className="card"><div className="skeleton" style={{ height: 80 }} /></div>}
      {charges !== null && charges.length === 0 && <Empty text="No charges recorded yet." icon={<IconCreditCard />} />}

      <div className="stack">
        {byPeriod.map(([period, list]) => {
          const total = list.reduce((s, c) => s + c.amount, 0);
          const paid = list.reduce((s, c) => s + c.paid, 0);
          const remaining = list.reduce((s, c) => s + c.remaining, 0);
          const isOpen = expandedPeriod === period;
          const allPaid = remaining === 0;

          return (
            <div key={period} className="card" style={{ padding: 0, overflow: "hidden" }}>
              <button
                className="card"
                style={{ border: 0, borderRadius: 0, width: "100%", textAlign: "start", cursor: "pointer", borderBottom: isOpen ? "1px solid var(--c-border)" : 0 }}
                onClick={() => setExpandedPeriod(isOpen ? null : period)}
              >
                <div className="row-between">
                  <div>
                    <div style={{ fontWeight: 700, fontSize: 15 }}>{fmtPeriod(period)}</div>
                    <div className="small secondary" style={{ marginTop: 2 }}>
                      {list.length} items · {fmtRial(total)}
                    </div>
                  </div>
                  <div style={{ display: "flex", alignItems: "center", gap: 12 }}>
                    {allPaid
                      ? <span className="badge badge--success">Paid</span>
                      : <span className="row__amount">{fmtRial(remaining)}</span>
                    }
                    {isOpen ? <IconChevronUp size={16} /> : <IconChevronDown size={16} />}
                  </div>
                </div>
                {!allPaid && (
                  <div className="progress" style={{ marginTop: 10 }}>
                    <div className="progress__fill" style={{ width: `${(paid / total) * 100}%` }} />
                  </div>
                )}
              </button>

              {isOpen && (
                <div style={{ padding: 12 }}>
                  {list.map((c) => (
                    <div key={c.id} style={{ padding: "10px 0", borderBottom: "1px solid var(--c-border)" }}>
                      <div className="row-between">
                        <div>
                          <div style={{ fontWeight: 600, fontSize: 14 }}>{c.template_name || c.template_id}</div>
                          <div className="tiny secondary">Due {new Intl.DateTimeFormat("en-US", { day: "numeric", month: "long" }).format(new Date(c.due_date))}</div>
                        </div>
                        <div className="row__end">
                          <span className="row__amount">{fmtRial(c.remaining || c.amount)}</span>
                          <StatusPill status={c.status} />
                        </div>
                      </div>
                      {c.paid > 0 && c.remaining > 0 && (
                        <div className="progress">
                          <div className="progress__fill" style={{ width: `${(c.paid / c.amount) * 100}%` }} />
                        </div>
                      )}
                      {c.remaining > 0 && (
                        <button
                          className="btn btn-primary btn-block btn-sm"
                          style={{ marginTop: 8 }}
                          disabled={paying === c.id}
                          onClick={() => pay(c)}
                        >
                          {paying === c.id ? "Processing…" : `Pay ${fmtRial(c.remaining)}`}
                        </button>
                      )}
                    </div>
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {toastMsg && <Toast msg={toastMsg} onClose={() => setToastMsg("")} />}
    </div>
  );
}
