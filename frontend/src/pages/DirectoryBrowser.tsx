import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";
import {
  ChevronRight, FolderTree, Plus, Save, UserPlus, Users, Folder, User, Shield,
} from "lucide-react";
import {
  Alert, Badge, Button, Card, CardBody, CardHeader, Checkbox, Dialog, PageHeader,
  SearchBox, TextField, StatusIndicator,
} from "@kiteapps/ui";
import { adminFetch } from "../lib/adminFetch";

type NodeKind = "container" | "ou" | "user" | "group" | "other";
type DirectoryNode = {
  name: string;
  distinguishedName: string;
  kind: NodeKind;
  hasChildren: boolean;
};
type DirectoryEntry = {
  distinguishedName: string;
  kind: NodeKind;
  name: string;
  attributes: Record<string, string>;
  enabled?: boolean;
};
type MemberRef = { distinguishedName: string; name: string; kind?: string };

const ATTR_FIELDS = [
  "displayName", "mail", "givenName", "sn", "telephoneNumber", "mobile",
  "title", "department", "description", "sAMAccountName", "uid", "cn",
] as const;

function kindIcon(kind: NodeKind) {
  if (kind === "user") return <User />;
  if (kind === "group") return <Users />;
  if (kind === "ou" || kind === "container") return <Folder />;
  return <Shield />;
}

export function DirectoryBrowser() {
  const [baseDN, setBaseDN] = useState("DC=example,DC=com");
  const [writeEnabled, setWriteEnabled] = useState(false);
  const [expanded, setExpanded] = useState<Record<string, DirectoryNode[]>>({});
  const [openNodes, setOpenNodes] = useState<Record<string, boolean>>({});
  const [selectedDN, setSelectedDN] = useState("");
  const [entry, setEntry] = useState<DirectoryEntry | null>(null);
  const [attrs, setAttrs] = useState<Record<string, string>>({});
  const [members, setMembers] = useState<MemberRef[]>([]);
  const [memberOf, setMemberOf] = useState<MemberRef[]>([]);
  const [filter, setFilter] = useState("");
  const [message, setMessage] = useState<{ tone: "success" | "danger" | "warning" | "info"; title: string; detail: string } | null>(null);
  const [loading, setLoading] = useState(true);
  const [createUserOpen, setCreateUserOpen] = useState(false);
  const [createGroupOpen, setCreateGroupOpen] = useState(false);
  const [assignOpen, setAssignOpen] = useState(false);
  const [userForm, setUserForm] = useState({ name: "", mail: "", password: "" });
  const [groupForm, setGroupForm] = useState({ name: "", description: "" });
  const [assignQuery, setAssignQuery] = useState("");
  const [assignCandidates, setAssignCandidates] = useState<{ id: string; displayName: string; distinguishedName: string }[]>([]);
  const [groupPickQuery, setGroupPickQuery] = useState("");
  const [groupCandidates, setGroupCandidates] = useState<{ name: string; distinguishedName: string }[]>([]);

  const loadChildren = useCallback(async (parent: string) => {
    const response = await adminFetch(`/api/v1/admin/directories/browse?parent=${encodeURIComponent(parent)}`);
    const body = await response.json();
    if (!response.ok) throw new Error(body.message || "Browse failed");
    if (body.baseDN) setBaseDN(body.baseDN);
    setWriteEnabled(Boolean(body.writeEnabled));
    setExpanded((current) => ({ ...current, [parent]: body.nodes || [] }));
    return body.nodes as DirectoryNode[];
  }, []);

  const selectEntry = useCallback(async (dn: string) => {
    setSelectedDN(dn);
    setMessage(null);
    const response = await adminFetch(`/api/v1/admin/directories/entry?dn=${encodeURIComponent(dn)}`);
    const body = await response.json();
    if (!response.ok) throw new Error(body.message || "Entry not found");
    const next = body.entry as DirectoryEntry;
    setEntry(next);
    setAttrs({ ...next.attributes });
    setWriteEnabled(Boolean(body.writeEnabled));
    if (next.kind === "group") {
      const membersResponse = await adminFetch(`/api/v1/admin/directories/group/members?dn=${encodeURIComponent(dn)}`);
      const membersBody = await membersResponse.json();
      setMembers(membersBody.members || []);
      setMemberOf([]);
    } else if (next.kind === "user") {
      const memberOfResponse = await adminFetch(`/api/v1/admin/directories/entry/member-of?dn=${encodeURIComponent(dn)}`);
      const memberOfBody = await memberOfResponse.json();
      setMemberOf(memberOfBody.groups || []);
      setMembers([]);
    } else {
      setMembers([]);
      setMemberOf([]);
    }
  }, []);

  useEffect(() => {
    void (async () => {
      try {
        const configResponse = await adminFetch("/api/v1/admin/directories/config");
        if (configResponse.ok) {
          const config = await configResponse.json();
          const root = config.directory?.baseDN || "DC=example,DC=com";
          setBaseDN(root);
          setWriteEnabled(Boolean(config.writeEnabled));
          await loadChildren(root);
          setOpenNodes({ [root]: true });
        } else {
          await loadChildren(baseDN);
          setOpenNodes({ [baseDN]: true });
        }
      } catch (error) {
        setMessage({
          tone: "danger",
          title: "Directory browser unavailable",
          detail: error instanceof Error ? error.message : "Unable to load directory tree",
        });
      } finally {
        setLoading(false);
      }
    })();
  }, [loadChildren]);

  const toggleNode = async (node: DirectoryNode) => {
    const isOpen = openNodes[node.distinguishedName];
    if (!isOpen && node.hasChildren && !expanded[node.distinguishedName]) {
      try {
        await loadChildren(node.distinguishedName);
      } catch (error) {
        setMessage({
          tone: "danger",
          title: "Unable to expand",
          detail: error instanceof Error ? error.message : "Browse failed",
        });
        return;
      }
    }
    setOpenNodes((current) => ({ ...current, [node.distinguishedName]: !isOpen }));
    void selectEntry(node.distinguishedName).catch((error) => setMessage({
      tone: "danger",
      title: "Unable to open entry",
      detail: error instanceof Error ? error.message : "Entry load failed",
    }));
  };

  const rootNodes = expanded[baseDN] || [];
  const visibleRoots = useMemo(() => {
    const q = filter.toLowerCase().trim();
    if (!q) return rootNodes;
    return rootNodes.filter((node) => node.name.toLowerCase().includes(q) || node.distinguishedName.toLowerCase().includes(q));
  }, [filter, rootNodes]);

  const saveAttributes = async () => {
    if (!entry) return;
    try {
      const response = await adminFetch(`/api/v1/admin/directories/entry/attributes?dn=${encodeURIComponent(entry.distinguishedName)}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ attributes: attrs }),
      });
      const body = await response.json();
      if (!response.ok) throw new Error(body.message || "Save failed");
      setEntry(body.entry);
      setAttrs({ ...body.entry.attributes });
      setMessage({ tone: "success", title: "Attributes saved", detail: "Allowlisted attributes were updated in the directory." });
    } catch (error) {
      setMessage({ tone: "danger", title: "Save failed", detail: error instanceof Error ? error.message : "Unable to save attributes" });
    }
  };

  const toggleEnabled = async (enabled: boolean) => {
    if (!entry) return;
    try {
      const response = await adminFetch(`/api/v1/admin/directories/entry/enabled?dn=${encodeURIComponent(entry.distinguishedName)}`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ enabled }),
      });
      const body = await response.json();
      if (!response.ok) throw new Error(body.message || "Update failed");
      await selectEntry(entry.distinguishedName);
      setMessage({ tone: "success", title: enabled ? "Account enabled" : "Account disabled", detail: entry.distinguishedName });
    } catch (error) {
      setMessage({ tone: "danger", title: "Enablement failed", detail: error instanceof Error ? error.message : "Unable to update account" });
    }
  };

  const createUser = async () => {
    const parentDN = entry && (entry.kind === "ou" || entry.kind === "container") ? entry.distinguishedName : selectedDN || baseDN;
    try {
      const response = await adminFetch("/api/v1/admin/directories/users", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          parentDN,
          name: userForm.name,
          password: userForm.password || undefined,
          attributes: { mail: userForm.mail, displayName: userForm.name },
        }),
      });
      const body = await response.json();
      if (!response.ok) throw new Error(body.message || "Create failed");
      setCreateUserOpen(false);
      setUserForm({ name: "", mail: "", password: "" });
      await loadChildren(parentDN);
      setOpenNodes((current) => ({ ...current, [parentDN]: true }));
      await selectEntry(body.entry.distinguishedName);
      setMessage({ tone: "success", title: "User created", detail: body.entry.distinguishedName });
    } catch (error) {
      setMessage({ tone: "danger", title: "Create user failed", detail: error instanceof Error ? error.message : "Unable to create user" });
    }
  };

  const createGroup = async () => {
    const parentDN = entry && (entry.kind === "ou" || entry.kind === "container") ? entry.distinguishedName : selectedDN || baseDN;
    try {
      const response = await adminFetch("/api/v1/admin/directories/groups", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ parentDN, name: groupForm.name, description: groupForm.description }),
      });
      const body = await response.json();
      if (!response.ok) throw new Error(body.message || "Create failed");
      setCreateGroupOpen(false);
      setGroupForm({ name: "", description: "" });
      await loadChildren(parentDN);
      setOpenNodes((current) => ({ ...current, [parentDN]: true }));
      await selectEntry(body.entry.distinguishedName);
      setMessage({ tone: "success", title: "Group created", detail: body.entry.distinguishedName });
    } catch (error) {
      setMessage({ tone: "danger", title: "Create group failed", detail: error instanceof Error ? error.message : "Unable to create group" });
    }
  };

  const searchUsersForAssign = async (query: string) => {
    setAssignQuery(query);
    const response = await adminFetch(`/api/v1/admin/users/?q=${encodeURIComponent(query)}&limit=20`);
    if (!response.ok) return;
    const body = await response.json();
    setAssignCandidates(body.users || []);
  };

  const searchGroupsForAssign = async (query: string) => {
    setGroupPickQuery(query);
    const response = await adminFetch(`/api/v1/admin/directories/groups?q=${encodeURIComponent(query)}`);
    if (!response.ok) return;
    const body = await response.json();
    setGroupCandidates(body.groups || []);
  };

  const addMember = async (memberDN: string) => {
    if (!entry || entry.kind !== "group") return;
    const response = await adminFetch(`/api/v1/admin/directories/group/members?dn=${encodeURIComponent(entry.distinguishedName)}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ memberDN }),
    });
    const body = await response.json();
    if (!response.ok) {
      setMessage({ tone: "danger", title: "Add member failed", detail: body.message || "Unable to add member" });
      return;
    }
    setAssignOpen(false);
    await selectEntry(entry.distinguishedName);
    setMessage({ tone: "success", title: "Member added", detail: memberDN });
  };

  const removeMember = async (memberDN: string) => {
    if (!entry || entry.kind !== "group") return;
    const response = await adminFetch(`/api/v1/admin/directories/group/members?dn=${encodeURIComponent(entry.distinguishedName)}`, {
      method: "DELETE",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ memberDN }),
    });
    const body = await response.json();
    if (!response.ok) {
      setMessage({ tone: "danger", title: "Remove member failed", detail: body.message || "Unable to remove member" });
      return;
    }
    await selectEntry(entry.distinguishedName);
  };

  const assignUserToGroup = async (groupDN: string) => {
    if (!entry || entry.kind !== "user") return;
    const response = await adminFetch(`/api/v1/admin/directories/group/members?dn=${encodeURIComponent(groupDN)}`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ memberDN: entry.distinguishedName }),
    });
    const body = await response.json();
    if (!response.ok) {
      setMessage({ tone: "danger", title: "Group assignment failed", detail: body.message || "Unable to assign group" });
      return;
    }
    setAssignOpen(false);
    await selectEntry(entry.distinguishedName);
    setMessage({ tone: "success", title: "Assigned to group", detail: groupDN });
  };

  const renderTree = (nodes: DirectoryNode[], depth = 0): ReactNode => nodes.map((node) => {
    const open = Boolean(openNodes[node.distinguishedName]);
    const children = expanded[node.distinguishedName] || [];
    return (
      <div key={node.distinguishedName} className="dm-dir-node" style={{ paddingLeft: depth * 12 }}>
        <button
          type="button"
          className="dm-dir-node__btn"
          data-active={selectedDN === node.distinguishedName || undefined}
          onClick={() => void toggleNode(node)}
        >
          <ChevronRight data-open={open || undefined} />
          {kindIcon(node.kind)}
          <span>{node.name}</span>
          <small>{node.kind}</small>
        </button>
        {open && children.length > 0 && renderTree(children, depth + 1)}
      </div>
    );
  });

  const containerSelected = entry?.kind === "ou" || entry?.kind === "container";

  return (
    <main className="dm-page">
      <PageHeader
        title="Directory Browser"
        description="Browse organizational units, create users and groups, edit allowlisted attributes, and manage membership."
        actions={<Badge tone={writeEnabled ? "success" : "warning"}>{writeEnabled ? "WRITES ENABLED" : "READ ONLY"}</Badge>}
      />
      {message && <Alert tone={message.tone} title={message.title}>{message.detail}</Alert>}
      <section className="dm-dir-layout">
        <Card className="dm-card dm-dir-tree">
          <CardHeader>
            <div>
              <h2>Directory tree</h2>
              <p>{baseDN}</p>
            </div>
          </CardHeader>
          <CardBody>
            <SearchBox value={filter} onChange={(event) => setFilter(event.target.value)} placeholder="Filter current level" aria-label="Filter directory nodes" />
            {loading ? <StatusIndicator tone="neutral" label="Loading directory" /> : (
              <div className="dm-dir-tree__list" aria-label="Directory nodes">
                <button type="button" className="dm-dir-node__btn" data-active={selectedDN === baseDN || undefined} onClick={() => {
                  setOpenNodes((current) => ({ ...current, [baseDN]: true }));
                  void loadChildren(baseDN);
                  setSelectedDN(baseDN);
                  setEntry({ distinguishedName: baseDN, kind: "container", name: baseDN, attributes: {} });
                }}>
                  <FolderTree /><span>Root</span><small>base</small>
                </button>
                {renderTree(visibleRoots)}
              </div>
            )}
          </CardBody>
        </Card>

        <div className="dm-dir-detail">
          {!entry && <Card className="dm-card"><CardBody className="dm-empty"><FolderTree /><strong>Select a directory object</strong><span>Choose an OU, user, or group from the tree.</span></CardBody></Card>}
          {entry && (
            <Card className="dm-card">
              <CardHeader>
                <div>
                  <span className="dm-section-label">{entry.kind}</span>
                  <h2>{entry.name}</h2>
                  <p>{entry.distinguishedName}</p>
                </div>
                <div className="dm-dir-actions">
                  {containerSelected && writeEnabled && (
                    <>
                      <Button size="sm" variant="secondary" leftIcon={<UserPlus />} onClick={() => setCreateUserOpen(true)}>Create user</Button>
                      <Button size="sm" variant="secondary" leftIcon={<Plus />} onClick={() => setCreateGroupOpen(true)}>Create group</Button>
                    </>
                  )}
                  {entry.kind === "group" && writeEnabled && (
                    <Button size="sm" variant="secondary" leftIcon={<UserPlus />} onClick={() => { setAssignOpen(true); void searchUsersForAssign(""); }}>Add member</Button>
                  )}
                  {entry.kind === "user" && writeEnabled && (
                    <Button size="sm" variant="secondary" leftIcon={<Users />} onClick={() => { setAssignOpen(true); void searchGroupsForAssign(""); }}>Assign group</Button>
                  )}
                </div>
              </CardHeader>
              <CardBody>
                {(entry.kind === "user" || entry.kind === "group") && (
                  <>
                    {entry.kind === "user" && (
                      <div className="dm-dir-enable">
                        <Checkbox
                          label="Account enabled"
                          description="Maps to Active Directory userAccountControl when available."
                          checked={Boolean(entry.enabled)}
                          disabled={!writeEnabled}
                          onChange={(event) => void toggleEnabled(event.target.checked)}
                        />
                      </div>
                    )}
                    <fieldset className="dm-config-section" disabled={!writeEnabled}>
                      <legend><strong>Attributes</strong><small>Allowlisted directory attributes only.</small></legend>
                      <div className="dm-config-fields">
                        {ATTR_FIELDS.map((key) => (
                          <TextField
                            key={key}
                            label={key}
                            value={attrs[key] || ""}
                            onChange={(event) => setAttrs((current) => ({ ...current, [key]: event.target.value }))}
                          />
                        ))}
                      </div>
                    </fieldset>
                    {writeEnabled && (
                      <footer className="dm-config-footer">
                        <span>Changes write directly to the directory when enabled.</span>
                        <Button variant="primary" leftIcon={<Save />} onClick={() => void saveAttributes()}>Save attributes</Button>
                      </footer>
                    )}
                  </>
                )}

                {entry.kind === "group" && (
                  <section className="dm-dir-members">
                    <h3>Members</h3>
                    {members.length === 0 ? <p>No members listed.</p> : (
                      <ul>
                        {members.map((member) => (
                          <li key={member.distinguishedName}>
                            <span><strong>{member.name}</strong><small>{member.distinguishedName}</small></span>
                            {writeEnabled && <Button size="sm" variant="ghost" onClick={() => void removeMember(member.distinguishedName)}>Remove</Button>}
                          </li>
                        ))}
                      </ul>
                    )}
                  </section>
                )}

                {entry.kind === "user" && (
                  <section className="dm-dir-members">
                    <h3>Member of</h3>
                    {memberOf.length === 0 ? <p>No group memberships found.</p> : (
                      <ul>
                        {memberOf.map((group) => (
                          <li key={group.distinguishedName}>
                            <span><strong>{group.name}</strong><small>{group.distinguishedName}</small></span>
                          </li>
                        ))}
                      </ul>
                    )}
                  </section>
                )}

                {containerSelected && (
                  <p className="dm-dir-hint">Use Create user / Create group to add objects under this container.</p>
                )}
              </CardBody>
            </Card>
          )}
        </div>
      </section>

      <Dialog open={createUserOpen} onOpenChange={setCreateUserOpen} title="Create user" description="Creates a user under the selected organizational unit.">
        <div className="dm-dir-form">
          <TextField label="Display name" value={userForm.name} onChange={(event) => setUserForm((current) => ({ ...current, name: event.target.value }))} required />
          <TextField label="Email" value={userForm.mail} onChange={(event) => setUserForm((current) => ({ ...current, mail: event.target.value }))} />
          <TextField label="Temporary password" type="password" value={userForm.password} onChange={(event) => setUserForm((current) => ({ ...current, password: event.target.value }))} hint="Optional. AD requires LDAPS/StartTLS to set unicodePwd." />
          <Button variant="primary" onClick={() => void createUser()} disabled={!userForm.name.trim()}>Create user</Button>
        </div>
      </Dialog>

      <Dialog open={createGroupOpen} onOpenChange={setCreateGroupOpen} title="Create group" description="Creates a security/groupOfNames group under the selected container.">
        <div className="dm-dir-form">
          <TextField label="Group name" value={groupForm.name} onChange={(event) => setGroupForm((current) => ({ ...current, name: event.target.value }))} required />
          <TextField label="Description" value={groupForm.description} onChange={(event) => setGroupForm((current) => ({ ...current, description: event.target.value }))} />
          <Button variant="primary" onClick={() => void createGroup()} disabled={!groupForm.name.trim()}>Create group</Button>
        </div>
      </Dialog>

      <Dialog
        open={assignOpen}
        onOpenChange={setAssignOpen}
        title={entry?.kind === "group" ? "Add group member" : "Assign user to group"}
        description={entry?.kind === "group" ? "Search directory users and add them to this group." : "Search groups and assign the selected user."}
      >
        <div className="dm-dir-form">
          {entry?.kind === "group" ? (
            <>
              <SearchBox value={assignQuery} onChange={(event) => void searchUsersForAssign(event.target.value)} placeholder="Search users" aria-label="Search users" />
              <ul className="dm-dir-picklist">
                {assignCandidates.map((user) => (
                  <li key={user.distinguishedName}>
                    <span><strong>{user.displayName}</strong><small>{user.distinguishedName}</small></span>
                    <Button size="sm" variant="secondary" onClick={() => void addMember(user.distinguishedName)}>Add</Button>
                  </li>
                ))}
              </ul>
            </>
          ) : (
            <>
              <SearchBox value={groupPickQuery} onChange={(event) => void searchGroupsForAssign(event.target.value)} placeholder="Search groups" aria-label="Search groups" />
              <ul className="dm-dir-picklist">
                {groupCandidates.map((group) => (
                  <li key={group.distinguishedName}>
                    <span><strong>{group.name}</strong><small>{group.distinguishedName}</small></span>
                    <Button size="sm" variant="secondary" onClick={() => void assignUserToGroup(group.distinguishedName)}>Assign</Button>
                  </li>
                ))}
              </ul>
            </>
          )}
        </div>
      </Dialog>
    </main>
  );
}
