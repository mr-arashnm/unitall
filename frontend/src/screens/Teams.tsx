// Teams — operational teams in a building.
import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { fetchTeams, fetchTasks, session } from "../api";
import type { Team, Task } from "../api";
import { BackBtn, Empty, TaskStatusBadge, PriorityBadge } from "../components/ui";
import { IconUsers, IconWrench } from "../components/icons";

export default function Teams() {
  const nav = useNavigate();
  const [teams, setTeams] = useState<Team[]>([]);
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    const bid = session.currentBuilding;
    if (bid) {
      Promise.all([fetchTeams(bid), fetchTasks(bid)]).then(([t, tk]) => {
        setTeams(t); setTasks(tk); setLoading(false);
      });
    } else { setLoading(false); }
  }, []);

  const byTeam = teams.map((team) => ({
    team,
    tasks: tasks.filter((t) => t.team_id === team.id),
  }));

  const unassigned = tasks.filter((t) => !byTeam.find((b) => b.team.id === t.team_id));

  return (
    <div className="view">
      <div className="row-between" style={{ marginBottom: 12 }}>
        <BackBtn onClick={() => window.history.back()} />
        <h1 className="page-title" style={{ margin: 0 }}>Teams & tasks</h1>
        <div style={{ width: 38 }} />
      </div>

      {loading && <div className="card"><div className="skeleton" style={{ height: 80 }} /></div>}

      <div className="stack">
        {byTeam.map(({ team, tasks: teamTasks }) => (
          <div key={team.id} className="card" style={{ display: "grid", gap: 10 }}>
            <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
              <span className="row__icon row__icon--warning"><IconUsers /></span>
              <div style={{ flex: 1 }}>
                <div style={{ fontWeight: 700, fontSize: 15 }}>{team.name}</div>
                <div className="small secondary">{team.members.length} members</div>
              </div>
              <span className="badge badge--muted">{teamTasks.length} tasks</span>
            </div>
            {teamTasks.slice(0, 3).map((t) => (
              <div key={t.id} className="card card--inset" style={{ padding: "10px 12px", display: "flex", gap: 10, alignItems: "center" }}>
                <div style={{ flex: 1, minWidth: 0 }}>
                  <div style={{ fontWeight: 600, fontSize: 13.5, overflow: "hidden", textOverflow: "ellipsis", whiteSpace: "nowrap" }}>{t.title}</div>
                  <div className="tiny secondary">{t.priority === "urgent" || t.priority === "high" ? <PriorityBadge priority={t.priority} /> : t.status}</div>
                </div>
                <TaskStatusBadge status={t.status} />
              </div>
            ))}
            {teamTasks.length > 3 && (
              <button className="btn btn-ghost btn-sm" style={{ width: "100%", marginTop: 4 }} onClick={() => nav(`/teams/${team.id}/tasks`)}>
                All {teamTasks.length} tasks →
              </button>
            )}
          </div>
        ))}

        {unassigned.length > 0 && (
          <div className="card">
            <div className="eyebrow" style={{ marginBottom: 10 }}>Unassigned</div>
            {unassigned.map((t) => (
              <div key={t.id} className="card card--inset" style={{ padding: "10px 12px", marginBottom: 6 }}>
                <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                  <div style={{ fontWeight: 600, fontSize: 13.5 }}>{t.title}</div>
                  <TaskStatusBadge status={t.status} />
                </div>
              </div>
            ))}
          </div>
        )}

        {teams.length === 0 && !loading && (
          <Empty text="No teams defined" icon={<IconUsers />} />
        )}
      </div>
    </div>
  );
}
