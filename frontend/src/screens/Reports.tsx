// Financial report — period collections dashboard.
import { useEffect, useState } from "react";
import { fetchFinancialReport, fetchChargeTemplates, fmtRialShort, fmtPeriod, session } from "../api";
import type { FinancialReport, ChargeTemplate } from "../api";
import { BackBtn, StatCard, ProgressBar, Toast } from "../components/ui";
import { IconChart, IconCreditCard } from "../components/icons";

export default function Reports() {
  const [report, setReport] = useState<FinancialReport | null>(null);
  const [templates, setTemplates] = useState<ChargeTemplate[]>([]);
  const [period, setPeriod] = useState(() => {
    const d = new Date(); const m = d.getMonth();
    return `${1404 + Math.floor((m + 9) / 12)}-${String(((m + 7) % 12) + 1).padStart(2, "0")}`;
  });
  const [loading, setLoading] = useState(true);
  const [toast, setToast] = useState("");

  function load(p: string) {
    setLoading(true);
    const bid = session.currentBuilding ?? "b1";
    Promise.all([fetchFinancialReport(bid, p), fetchChargeTemplates(bid)]).then(([r, t]) => {
      setReport(r); setTemplates(t); setLoading(false);
    });
  }

  useEffect(() => { load(period); }, []);

  if (!report && loading) {
    return <div className="view"><div className="card"><div className="skeleton" style={{ height: 160 }} /></div></div>;
  }

  const rate = report ? Math.round(report.collection_rate * 100) : 0;
  const rateColor = rate >= 80 ? "var(--c-success)" : rate >= 50 ? "var(--c-warning)" : "var(--c-destructive)";

  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 12 }}>
        <BackBtn onClick={() => window.history.back()} />
        <h1 className="page-title" style={{ margin: 0 }}>Financial report</h1>
        <div style={{ width: 38 }} />
      </div>

      <div className="field" style={{ marginBottom: 16 }}>
        <label style={{ marginBottom: 4 }}>Period</label>
        <div style={{ display: "flex", gap: 8 }}>
          <input
            type="month"
            value={period}
            onChange={(e) => { setPeriod(e.target.value.replace("-", "-")); load(e.target.value); }}
            dir="ltr"
            style={{ flex: 1 }}
          />
        </div>
      </div>

      {report && (
        <>
          <div className="grid-2" style={{ marginBottom: 16 }}>
            <StatCard label="Total billed" value={fmtRialShort(report.total_billed)} sub={`${report.units} units`} />
            <StatCard label="Collected" value={fmtRialShort(report.total_collected)} sub={`${rate}% collection rate`} accent={rate >= 80} />
            <StatCard label="Outstanding" value={fmtRialShort(report.outstanding)} sub={`${report.overdue_count} overdue`} accent={report.overdue_count > 0} />
            <StatCard label="Overdue" value={`${report.overdue_count}`} sub="units" />
          </div>

          <div className="card" style={{ marginBottom: 16 }}>
            <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 10 }}>
              <span style={{ fontWeight: 700, fontSize: 15 }}>Collection rate {fmtPeriod(period)}</span>
              <span style={{ fontWeight: 800, fontSize: 24, color: rateColor }}>{rate}%</span>
            </div>
            <ProgressBar value={rate} color={rateColor} />
            <div style={{ display: "flex", justifyContent: "space-between", marginTop: 6 }}>
              <span className="small secondary">{fmtRialShort(report.total_collected)} collected</span>
              <span className="small secondary">{fmtRialShort(report.total_billed)} total</span>
            </div>
          </div>

          <p className="eyebrow" style={{ marginBottom: 8 }}>Charge templates</p>
          <div className="stack">
            {templates.map((t) => (
              <div key={t.id} className="row" style={{ display: "flex", gap: 12, alignItems: "center" }}>
                <span className="row__icon row__icon--primary"><IconCreditCard /></span>
                <div className="row__body">
                  <div className="row__title">{t.name}</div>
                  <div className="row__sub">{t.type}</div>
                </div>
                <div className="row__end">
                  <span className="row__amount">{fmtRialShort(t.amount)}</span>
                  <span className={`badge ${t.is_active ? "badge--success" : "badge--muted"}`}>
                    {t.is_active ? "Active" : "Inactive"}
                  </span>
                </div>
              </div>
            ))}
          </div>
        </>
      )}

      {toast && <Toast msg={toast} onClose={() => setToast("")} />}
    </div>
  );
}
