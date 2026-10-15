import { useEffect, useMemo, useState } from "react";
import {
  ArrowDown, ArrowUp, Copy, KeyRound, LockKeyhole, Pencil, Plus, Search, ShieldCheck, SlidersHorizontal, Trash2, UserCog,
} from "lucide-react";
import {
  Alert, Button, Card, CardBody, CardHeader, Checkbox, Dialog, DialogFooter, MiniButton,
  PageHeader, SearchBox, Select, StatusIndicator, Table, TBody, TD, TH, THead, TextField, TR,
} from "@kiteapps/ui";
import { adminFetch } from "../lib/adminFetch";

type Policy = {
  id: number;
  name: string;
  scope: string;
  domain: string;
  permissions: string[];
  status: "Active" | "Paused" | "Review";
  priority: number;
  ouDNs: string[];
  groupDNs: string[];
  modifiedBy: string;
  modifiedAt: string;
};

type OU = { name: string; distinguishedName: string };
type Group = { name: string; distinguishedName: string; samAccountName?: string };

const permissionOptions = ["Reset Password", "Unlock Account", "Self Update", "Change Password"];
const emptyPolicy: Policy = {
  id: 0, name: "", scope: "All employees", domain: "ADSELFSERVICE",
  permissions: ["Reset Password", "Unlock Account"], status: "Active", priority: 0,
  ouDNs: [], groupDNs: [], modifiedBy: "", modifiedAt: "",
};

export function Policies() {
  const [query, setQuery] = useState("");
  const [policies, setPolicies] = useState<Policy[]>([]);
  const [ous, setOUs] = useState<OU[]>([]);
  const [groups, setGroups] = useState<Group[]>([]);
  const [editing, setEditing] = useState<Policy | null>(null);
  const [dialog, setDialog] = useState(false);
  const [error, setError] = useState("");
  const rows = useMemo(
    () => policies.filter((row) => `${row.name} ${row.scope}`.toLowerCase().includes(query.toLowerCase())),
    [policies, query],
  );

  const load = async () => {
    const response = await adminFetch("/api/v1/admin/policies/");
    if (!response.ok) throw new Error("Enter the administrator token in Configuration to manage policies.");
    const body = await response.json();
    setPolicies(body.policies);
  };

  const loadDirectoryTargets = async () => {
    const [ouResponse, groupResponse] = await Promise.all([
      adminFetch("/api/v1/admin/directories/organizational-units"),
      adminFetch("/api/v1/admin/directories/groups"),
    ]);
    if (ouResponse.ok) {
      const body = await ouResponse.json();
      setOUs(body.organizationalUnits || []);
    }
    if (groupResponse.ok) {
      const body = await groupResponse.json();
      setGroups(body.groups || []);
    }
  };

  useEffect(() => {
    void load().catch((cause) => setError(cause instanceof Error ? cause.message : "Policies unavailable."));
    void loadDirectoryTargets();
  }, []);

  const openCreate = () => { setEditing({ ...emptyPolicy }); setDialog(true); };
  const openEdit = (policy: Policy) => {
    setEditing({ ...policy, permissions: [...policy.permissions], ouDNs: [...(policy.ouDNs || [])], groupDNs: [...(policy.groupDNs || [])] });
    setDialog(true);
  };

  const save = async () => {
    if (!editing) return;
    setError("");
    const response = await adminFetch(
      editing.id ? `/api/v1/admin/policies/${editing.id}` : "/api/v1/admin/policies/",
      {
        method: editing.id ? "PUT" : "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(editing),
      },
    );
    if (!response.ok) {
      const body = await response.json();
      setError(body.message || "Policy could not be saved.");
      return;
    }
    setDialog(false);
    setEditing(null);
    await load();
  };

  const clone = async (id: number) => {
    const response = await adminFetch(`/api/v1/admin/policies/${id}/clone`, { method: "POST" });
    if (!response.ok) { setError("Policy could not be cloned."); return; }
    await load();
  };

  const remove = async (policy: Policy) => {
    if (!window.confirm(`Delete ${policy.name}? This removes the policy from the durable store.`)) return;
    const response = await adminFetch(`/api/v1/admin/policies/${policy.id}`, { method: "DELETE" });
    if (!response.ok) { setError("Policy could not be deleted."); return; }
    await load();
  };

  const move = async (index: number, direction: -1 | 1) => {
    const next = index + direction;
    if (next < 0 || next >= policies.length) return;
    const ordered = [...policies];
    const item = ordered[index];
    if (!item) return;
    ordered.splice(index, 1);
    ordered.splice(next, 0, item);
    const response = await adminFetch("/api/v1/admin/policies/reorder", {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ orderedIds: ordered.map((policy) => policy.id) }),
    });
    if (!response.ok) { setError("Policy order could not be updated."); return; }
    const body = await response.json();
    setPolicies(body.policies);
  };

  const togglePermission = (permission: string, checked: boolean) =>
    setEditing((current) => current ? {
      ...current,
      permissions: checked ? [...current.permissions, permission] : current.permissions.filter((item) => item !== permission),
    } : current);

  const toggleDN = (field: "ouDNs" | "groupDNs", dn: string, checked: boolean) =>
    setEditing((current) => current ? {
      ...current,
      [field]: checked ? [...current[field], dn] : current[field].filter((item) => item !== dn),
    } : current);

  return (
    <main className="dm-page">
      <PageHeader
        title="Policy Configuration"
        description="Assign self-service features to Active Directory OUs and groups. Higher list position wins."
        actions={<Button variant="primary" size="sm" leftIcon={<Plus />} onClick={openCreate}>New policy</Button>}
      />
      {error && <Alert tone="danger" title="Policy operation failed">{error}</Alert>}
      <section className="dm-features">
        {([
          { icon: KeyRound, label: "Reset password" },
          { icon: LockKeyhole, label: "Unlock account" },
          { icon: ShieldCheck, label: "Change password" },
          { icon: UserCog, label: "Update profile" },
        ]).map(({ icon: Icon, label }) => (
          <div key={label}>
            <span><Icon /></span>
            <strong>{label}</strong>
            <StatusIndicator tone="success" label="Policy-gated" />
          </div>
        ))}
      </section>
      <Card className="dm-card">
        <CardHeader>
          <div>
            <h2>Available policies</h2>
            <p>{rows.length} policies · first match by priority applies</p>
          </div>
          <div className="dm-tools">
            <SearchBox value={query} onChange={(e) => setQuery(e.target.value)} placeholder="Filter policies" aria-label="Filter policies" />
            <Button variant="secondary" size="sm" iconOnly aria-label="Filters"><SlidersHorizontal /></Button>
          </div>
        </CardHeader>
        <CardBody className="dm-table">
          <Table compact>
            <THead>
              <TR>
                <TH>Order</TH><TH>Policy</TH><TH>Scope</TH><TH>Targets</TH><TH>Permissions</TH><TH>Status</TH><TH>Modified</TH><TH>Actions</TH>
              </TR>
            </THead>
            <TBody>
              {rows.map((row, index) => (
                <TR key={row.id}>
                  <TD>
                    <div className="dm-actions">
                      <MiniButton aria-label={`Raise ${row.name}`} onClick={() => void move(index, -1)}><ArrowUp /></MiniButton>
                      <MiniButton aria-label={`Lower ${row.name}`} onClick={() => void move(index, 1)}><ArrowDown /></MiniButton>
                    </div>
                  </TD>
                  <TD>
                    <strong>{row.name}</strong>
                    <small className="dm-cell-note">{row.domain}</small>
                  </TD>
                  <TD>{row.scope}</TD>
                  <TD>
                    <small className="dm-cell-note">
                      {(row.ouDNs?.length || 0) + (row.groupDNs?.length || 0) === 0
                        ? "Entire domain"
                        : `${row.ouDNs?.length || 0} OU · ${row.groupDNs?.length || 0} group`}
                    </small>
                  </TD>
                  <TD>{row.permissions.join(", ")}</TD>
                  <TD>
                    <StatusIndicator
                      tone={row.status === "Active" ? "success" : row.status === "Review" ? "warning" : "neutral"}
                      label={row.status}
                    />
                  </TD>
                  <TD>{row.modifiedAt ? new Date(row.modifiedAt).toLocaleString() : "—"}</TD>
                  <TD>
                    <div className="dm-actions">
                      <MiniButton aria-label={`Edit ${row.name}`} onClick={() => openEdit(row)}><Pencil /></MiniButton>
                      <MiniButton aria-label={`Copy ${row.name}`} onClick={() => void clone(row.id)}><Copy /></MiniButton>
                      <MiniButton aria-label={`Delete ${row.name}`} onClick={() => void remove(row)}><Trash2 /></MiniButton>
                    </div>
                  </TD>
                </TR>
              ))}
            </TBody>
          </Table>
          {rows.length === 0 && (
            <div className="dm-empty">
              <Search />
              <strong>No matching policies</strong>
              <span>Create a policy or try another name.</span>
            </div>
          )}
        </CardBody>
      </Card>
      <Dialog
        open={dialog}
        onOpenChange={setDialog}
        title={editing?.id ? "Edit policy" : "Create policy"}
        description="Policies are stored in MySQL and evaluated by OU/group precedence."
        size="lg"
        footer={
          <DialogFooter>
            <Button variant="ghost" onClick={() => setDialog(false)}>Cancel</Button>
            <Button variant="primary" onClick={() => void save()}>Save policy</Button>
          </DialogFooter>
        }
      >
        {editing && (
          <div className="dm-policy-form">
            <TextField label="Policy name" value={editing.name} onChange={(event) => setEditing({ ...editing, name: event.target.value })} />
            <TextField label="Target scope label" value={editing.scope} onChange={(event) => setEditing({ ...editing, scope: event.target.value })} />
            <TextField label="Directory" value={editing.domain} onChange={(event) => setEditing({ ...editing, domain: event.target.value })} />
            <Select
              aria-label="Policy status"
              value={editing.status}
              onValueChange={(value) => setEditing({ ...editing, status: value as Policy["status"] })}
              options={[
                { value: "Active", label: "Active" },
                { value: "Paused", label: "Paused" },
                { value: "Review", label: "Needs review" },
              ]}
            />
            <fieldset>
              <legend>Self-service permissions</legend>
              {permissionOptions.map((permission) => (
                <Checkbox
                  key={permission}
                  label={permission}
                  checked={editing.permissions.includes(permission)}
                  onChange={(event) => togglePermission(permission, event.target.checked)}
                />
              ))}
            </fieldset>
            <fieldset>
              <legend>Organizational units</legend>
              {ous.length === 0 && <p className="dm-cell-note">No OUs loaded. Test the directory connection first.</p>}
              {ous.map((ou) => (
                <Checkbox
                  key={ou.distinguishedName}
                  label={ou.name}
                  description={ou.distinguishedName}
                  checked={editing.ouDNs.includes(ou.distinguishedName)}
                  onChange={(event) => toggleDN("ouDNs", ou.distinguishedName, event.target.checked)}
                />
              ))}
            </fieldset>
            <fieldset>
              <legend>Groups</legend>
              {groups.length === 0 && <p className="dm-cell-note">No groups loaded from groupSearchBase.</p>}
              {groups.map((group) => (
                <Checkbox
                  key={group.distinguishedName}
                  label={group.name}
                  description={group.distinguishedName}
                  checked={editing.groupDNs.includes(group.distinguishedName)}
                  onChange={(event) => toggleDN("groupDNs", group.distinguishedName, event.target.checked)}
                />
              ))}
            </fieldset>
          </div>
        )}
      </Dialog>
    </main>
  );
}
