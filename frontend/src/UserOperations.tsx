import { useCallback, useEffect, useMemo, useState } from "react";
import { KeyRound, LockKeyholeOpen, RefreshCw, Search, ShieldAlert, UserRoundSearch } from "lucide-react";
import {
  Alert, Badge, Button, Card, CardBody, CardHeader, Checkbox, Dialog, DialogFooter,
  MiniButton, PageHeader, SearchBox, Select, StatusIndicator, Table, TBody, TD, TH,
  THead, TextField, TR,
} from "@kiteapps/ui";

type DirectoryUser = {
  id: string;
  displayName: string;
  email: string;
  distinguishedName: string;
  organizationalUnit: string;
  enabled: boolean;
  locked: boolean;
  passwordExpired: boolean;
  lastLogon?: string;
};

type AuditEvent = {
  id: number;
  actor: string;
  action: string;
  targetType: string;
  targetId: string;
  outcome: string;
  requestId?: string;
  sourceIp?: string;
  occurredAt: string;
};

function authorizedFetch(path: string, init: RequestInit = {}) {
  return fetch(path, init);
}

async function responseMessage(response: Response, fallback: string) {
  try {
    const body = await response.json() as { message?: string };
    return body.message || fallback;
  } catch {
    return fallback;
  }
}

function formatAction(action: string) {
  return action.split(".").map(part => part.replace(/-/g, " ")).join(" · ");
}

function UserStatus({ user }: { user: DirectoryUser }) {
  if (!user.enabled) return <StatusIndicator tone="neutral" label="Disabled" />;
  if (user.locked) return <StatusIndicator tone="danger" label="Locked" />;
  if (user.passwordExpired) return <StatusIndicator tone="warning" label="Password expired" />;
  return <StatusIndicator tone="success" label="Active" />;
}

export function UserManagement() {
  const [query, setQuery] = useState("");
  const [users, setUsers] = useState<DirectoryUser[]>([]);
  const [writeEnabled, setWriteEnabled] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [resetUser, setResetUser] = useState<DirectoryUser | null>(null);
  const [unlockUser, setUnlockUser] = useState<DirectoryUser | null>(null);
  const [password, setPassword] = useState("");
  const [forceChange, setForceChange] = useState(true);
  const [saving, setSaving] = useState(false);

  const load = useCallback(async (search = query) => {
    setLoading(true);
    setError("");
    try {
      const response = await authorizedFetch(`/api/v1/admin/users/?q=${encodeURIComponent(search)}&limit=100`);
      if (!response.ok) throw new Error(await responseMessage(response, response.status === 401 ? "Administrator authentication is required. Enter the API token in Configuration → Directory Connection." : "Directory users could not be loaded."));
      const body = await response.json() as { users: DirectoryUser[]; writeEnabled: boolean };
      setUsers(body.users || []);
      setWriteEnabled(Boolean(body.writeEnabled));
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Directory users could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, [query]);

  useEffect(() => { void load(""); }, []); // The first load intentionally does not wait for a search term.

  const submitSearch = (event: React.FormEvent) => {
    event.preventDefault();
    void load();
  };

  const resetPassword = async () => {
    if (!resetUser) return;
    setSaving(true);
    setError("");
    setNotice("");
    try {
      const response = await authorizedFetch(`/api/v1/admin/users/${encodeURIComponent(resetUser.id)}/password-reset`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ password, forceChange }),
      });
      if (!response.ok) throw new Error(await responseMessage(response, "The password could not be reset."));
      setNotice(`Password reset completed for ${resetUser.displayName}.`);
      setResetUser(null);
      setPassword("");
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "The password could not be reset.");
    } finally {
      setSaving(false);
    }
  };

  const unlock = async () => {
    if (!unlockUser) return;
    setSaving(true);
    setError("");
    setNotice("");
    try {
      const response = await authorizedFetch(`/api/v1/admin/users/${encodeURIComponent(unlockUser.id)}/unlock`, { method: "POST" });
      if (!response.ok) throw new Error(await responseMessage(response, "The account could not be unlocked."));
      setNotice(`${unlockUser.displayName} is unlocked.`);
      setUnlockUser(null);
      await load();
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "The account could not be unlocked.");
    } finally {
      setSaving(false);
    }
  };

  return <main className="dm-page">
    <PageHeader title="Directory users" description="Find Active Directory users and perform audited recovery operations."
      actions={<Button variant="secondary" size="sm" leftIcon={<RefreshCw />} onClick={() => void load()} disabled={loading}>Refresh</Button>} />
    {error && <Alert tone="danger" title="Directory operation failed">{error}</Alert>}
    {notice && <Alert tone="success" title="Operation completed">{notice}</Alert>}
    {!writeEnabled && !loading && !error && <Alert tone="warning" title="Directory is read-only">Enable DIRECTORY_WRITE_ENABLED only after granting the service account least-privilege password reset and unlock permissions.</Alert>}
    <Card className="dm-card">
      <CardHeader><div><h2>User directory</h2><p>{loading ? "Loading users…" : `${users.length} users found`}</p></div>
        <form className="dm-user-tools" onSubmit={submitSearch}>
          <SearchBox value={query} onChange={event => setQuery(event.target.value)} placeholder="Name, account, or email" aria-label="Search directory users" />
          <Button type="submit" variant="secondary" size="sm" leftIcon={<Search />}>Search</Button>
        </form>
      </CardHeader>
      <CardBody className="dm-table"><Table compact><THead><TR><TH>User</TH><TH>Account</TH><TH>Organization</TH><TH>Status</TH><TH>Last logon</TH><TH>Actions</TH></TR></THead>
        <TBody>{users.map(user => <TR key={user.id}>
          <TD><span className="dm-user-identity"><strong>{user.displayName || user.id}</strong><small>{user.email || "No email address"}</small></span></TD>
          <TD>{user.id}</TD><TD>{user.organizationalUnit || "—"}</TD><TD><UserStatus user={user} /></TD>
          <TD>{user.lastLogon ? new Date(user.lastLogon).toLocaleString() : "Never"}</TD>
          <TD><div className="dm-actions">
            <MiniButton aria-label={`Reset password for ${user.displayName}`} disabled={!writeEnabled || !user.enabled} onClick={() => { setPassword(""); setResetUser(user); }}><KeyRound /></MiniButton>
            <MiniButton aria-label={`Unlock ${user.displayName}`} disabled={!writeEnabled || !user.locked} onClick={() => setUnlockUser(user)}><LockKeyholeOpen /></MiniButton>
          </div></TD>
        </TR>)}</TBody>
      </Table>{!loading && users.length === 0 && <div className="dm-empty"><UserRoundSearch /><strong>No users found</strong><span>Try a broader name, account, or email search.</span></div>}</CardBody>
    </Card>
    <Dialog open={Boolean(resetUser)} onOpenChange={open => { if (!open) { setResetUser(null); setPassword(""); } }} title="Reset password" description={`Set a temporary password for ${resetUser?.displayName || "this user"}. The password is sent only to the directory and is never written to the audit log.`} footer={<DialogFooter><Button variant="ghost" onClick={() => setResetUser(null)}>Cancel</Button><Button variant="primary" disabled={saving || password.length < 12} onClick={() => void resetPassword()}>{saving ? "Resetting…" : "Reset password"}</Button></DialogFooter>}>
      <div className="dm-operation-form"><TextField type="password" label="Temporary password" hint="Use 12–128 characters and follow the domain password policy." value={password} minLength={12} maxLength={128} autoComplete="new-password" onChange={event => setPassword(event.target.value)} /><Checkbox label="Require password change at next sign-in" checked={forceChange} onChange={event => setForceChange(event.target.checked)} /></div>
    </Dialog>
    <Dialog open={Boolean(unlockUser)} onOpenChange={open => { if (!open) setUnlockUser(null); }} title="Unlock account" description={`Restore sign-in access for ${unlockUser?.displayName || "this user"}? This operation is recorded in the audit log.`} footer={<DialogFooter><Button variant="ghost" onClick={() => setUnlockUser(null)}>Cancel</Button><Button variant="primary" disabled={saving} onClick={() => void unlock()}>{saving ? "Unlocking…" : "Unlock account"}</Button></DialogFooter>}>
      <Alert tone="warning" title="Confirm directory write">Only unlock the account after verifying the user and reviewing the lockout source.</Alert>
    </Dialog>
  </main>;
}

export function AuditEvents() {
  const [query, setQuery] = useState("");
  const [outcome, setOutcome] = useState("");
  const [events, setEvents] = useState<AuditEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const params = new URLSearchParams({ limit: "200" });
      if (query) params.set("q", query);
      if (outcome) params.set("outcome", outcome);
      const response = await authorizedFetch(`/api/v1/admin/audit-events/?${params}`);
      if (!response.ok) throw new Error(await responseMessage(response, response.status === 401 ? "Administrator authentication is required. Enter the API token in Configuration → Directory Connection." : "Audit events could not be loaded."));
      const body = await response.json() as { events: AuditEvent[] };
      setEvents(body.events || []);
    } catch (cause) {
      setError(cause instanceof Error ? cause.message : "Audit events could not be loaded.");
    } finally {
      setLoading(false);
    }
  }, [outcome, query]);

  useEffect(() => { void load(); }, [outcome]);
  const filteredSummary = useMemo(() => outcome ? `${events.length} ${outcome} events` : `${events.length} recent events`, [events.length, outcome]);

  return <main className="dm-page">
    <PageHeader title="Audit events" description="Review administrative, policy, directory, and notification activity."
      actions={<Button variant="secondary" size="sm" leftIcon={<RefreshCw />} onClick={() => void load()} disabled={loading}>Refresh</Button>} />
    {error && <Alert tone="danger" title="Audit log unavailable">{error}</Alert>}
    <Card className="dm-card">
      <CardHeader><div><h2>Administrative activity</h2><p>{loading ? "Loading events…" : filteredSummary}</p></div>
        <form className="dm-audit-tools" onSubmit={event => { event.preventDefault(); void load(); }}>
          <SearchBox value={query} onChange={event => setQuery(event.target.value)} placeholder="Actor, action, or target" aria-label="Search audit events" />
          <Select aria-label="Filter audit outcome" value={outcome} onValueChange={setOutcome} options={[{ value: "", label: "All outcomes" }, { value: "success", label: "Successful" }, { value: "failure", label: "Failed" }]} />
          <Button type="submit" variant="secondary" size="sm">Apply</Button>
        </form>
      </CardHeader>
      <CardBody className="dm-table"><Table compact><THead><TR><TH>Time</TH><TH>Actor</TH><TH>Action</TH><TH>Target</TH><TH>Outcome</TH><TH>Request</TH></TR></THead>
        <TBody>{events.map(event => <TR key={event.id}><TD>{new Date(event.occurredAt).toLocaleString()}</TD><TD>{event.actor}</TD><TD><strong>{formatAction(event.action)}</strong></TD><TD><span className="dm-user-identity"><strong>{event.targetId}</strong><small>{event.targetType}</small></span></TD><TD><StatusIndicator tone={event.outcome === "success" ? "success" : "danger"} label={event.outcome} /></TD><TD>{event.requestId || "—"}</TD></TR>)}</TBody>
      </Table>{!loading && events.length === 0 && <div className="dm-empty"><ShieldAlert /><strong>No matching events</strong><span>Clear the filters or wait for new administrative activity.</span></div>}</CardBody>
    </Card>
  </main>;
}
