import { useEffect, useState } from "react";
import { Activity, Clock3, KeyRound, ShieldCheck, UserCheck } from "lucide-react";
import {
  Alert, Badge, Button, Card, CardBody, CardHeader, PageHeader,
  StatsCard, StatsCardGroup,
} from "@kiteapps/ui";
import { adminFetch } from "../lib/adminFetch";

type DashboardSummary = {
  generatedAt: string;
  directory: { name: string; mode: string; host: string; writeEnabled: boolean; error?: string };
  tiles: {
    lockedUsers: number;
    expiredPasswords: number;
    disabledUsers: number;
    sampledUsers: number;
    activePolicies: number;
    totalPolicies: number;
    recentPasswordResets: number;
    recentUnlocks: number;
  };
  enrollment: { status: string; message: string };
  attention: { title: string; detail: string }[];
  activity: { label: string; value: number }[];
};

export function Dashboard() {
  const [summary, setSummary] = useState<DashboardSummary | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    void adminFetch("/api/v1/admin/dashboard/")
      .then(async (response) => {
        if (!response.ok) {
          const body = await response.json().catch(() => ({}));
          throw new Error(body.message || "Dashboard aggregates are unavailable.");
        }
        setSummary(await response.json());
      })
      .catch((cause) => setError(cause instanceof Error ? cause.message : "Dashboard unavailable."));
  }, []);

  const tiles = summary?.tiles;
  const maxActivity = Math.max(1, ...(summary?.activity.map((item) => item.value) || [1]));

  return (
    <main className="dm-page">
      <PageHeader
        title="Dashboard"
        description="Monitor directory health, self-service policies, and audited recovery activity."
        actions={<Button variant="secondary" size="sm" leftIcon={<Activity />}>Audit events</Button>}
      />
      {error && <Alert tone="warning" title="Dashboard unavailable">{error}</Alert>}
      {summary?.directory.error && <Alert tone="danger" title="Active Directory unavailable">{summary.directory.error}</Alert>}
      {summary && (
        <div className="dm-demo">
          <Badge tone="accent">{summary.directory.mode.toUpperCase()}</Badge>
          <span>{summary.directory.name} · {summary.directory.host} · writes {summary.directory.writeEnabled ? "enabled" : "read-only"}</span>
        </div>
      )}
      <StatsCardGroup className="dm-stats">
        <StatsCard title="Locked users" value={String(tiles?.lockedUsers ?? "—")} description="From live directory sample" icon={<Clock3 />} iconTone="warning" />
        <StatsCard title="Expired passwords" value={String(tiles?.expiredPasswords ?? "—")} description="pwdLastSet indicates expired" icon={<KeyRound />} iconTone="danger" />
        <StatsCard title="Active policies" value={tiles ? `${tiles.activePolicies} / ${tiles.totalPolicies}` : "—"} description="Self-service policy coverage" icon={<ShieldCheck />} iconTone="accent" />
        <StatsCard title="Enrollment coverage" value="—" description={summary?.enrollment.message || "MFA enrollment later"} icon={<UserCheck />} iconTone="neutral" />
      </StatsCardGroup>
      <section className="dm-dashboard-grid">
        <Card className="dm-card">
          <CardHeader>
            <div>
              <h2>Directory sample</h2>
              <p>{tiles ? `${tiles.sampledUsers} users returned from the configured search base.` : "Loading directory sample…"}</p>
            </div>
          </CardHeader>
          <CardBody>
            <div className="dm-enrollment">
              <div className="dm-donut">
                <strong>{tiles?.sampledUsers ?? "—"}</strong>
                <span>sampled</span>
              </div>
              <div className="dm-methods">
                {[
                  ["Locked accounts", tiles?.lockedUsers ?? 0],
                  ["Expired passwords", tiles?.expiredPasswords ?? 0],
                  ["Disabled accounts", tiles?.disabledUsers ?? 0],
                  ["Recent unlocks", tiles?.recentUnlocks ?? 0],
                ].map(([name, count]) => (
                  <div className="dm-method" key={name as string}>
                    <div><span>{name}</span><strong>{count}</strong></div>
                    <i><b style={{ width: `${Math.min(100, Number(count) * 8)}%` }} /></i>
                  </div>
                ))}
              </div>
            </div>
          </CardBody>
        </Card>
        <Card className="dm-card">
          <CardHeader>
            <div>
              <h2>Needs attention</h2>
              <p>Checks derived from Active Directory and policy state.</p>
            </div>
            <Badge tone="warning">{summary?.attention.length || 0} OPEN</Badge>
          </CardHeader>
          <CardBody className="dm-attention">
            {(summary?.attention || []).map((item, i) => (
              <button type="button" key={item.title}>
                <em>{i + 1}</em>
                <span><strong>{item.title}</strong><small>{item.detail}</small></span>
              </button>
            ))}
          </CardBody>
        </Card>
      </section>
      <Card className="dm-card dm-activity">
        <CardHeader>
          <div>
            <h2>Audited recovery activity</h2>
            <p>Counts from durable audit events (password reset and unlock).</p>
          </div>
        </CardHeader>
        <CardBody>
          <div className="dm-bars">
            {(summary?.activity || [
              { label: "Reset password", value: 0 },
              { label: "Unlock account", value: 0 },
            ]).map((item) => (
              <div key={item.label}>
                <span>{item.label}</span>
                <i><b style={{ width: `${Math.round((item.value / maxActivity) * 100)}%` }} /></i>
                <strong>{item.value}</strong>
              </div>
            ))}
          </div>
        </CardBody>
      </Card>
    </main>
  );
}
