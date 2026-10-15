import { useEffect, useState } from "react";
import { History, Save, ShieldCheck } from "lucide-react";
import {
  Alert, Badge, Button, Card, CardBody, CardHeader, Checkbox, Select,
  StatusIndicator, TextArea, TextField,
} from "@kiteapps/ui";

type Option = { value: string; label: string };
type Field = {
  key: string;
  label: string;
  description?: string;
  type: "boolean" | "text" | "number" | "select" | "textarea";
  required?: boolean;
  min?: number;
  max?: number;
  options?: Option[];
};
type Section = { id: string; title: string; description?: string; fields: Field[] };
type Definition = { id: string; group: string; title: string; description: string; sections: Section[] };
type Payload = { definition: Definition; values: Record<string, boolean | number | string>; version: number; updatedAt?: string };
type Revision = { version: number; modifiedBy: string; createdAt: string };

function request(path: string, init: RequestInit = {}) {
  return fetch(path, {
    ...init,
    headers: { "Content-Type": "application/json", ...init.headers },
  });
}

export function ConfigurationModule({ moduleID }: { moduleID: string }) {
  const [payload, setPayload] = useState<Payload | null>(null);
  const [state, setState] = useState<"loading" | "ready" | "saving" | "saved" | "error">("loading");
  const [message, setMessage] = useState("");
  const [retry, setRetry] = useState(0);
  const [baseline, setBaseline] = useState("");
  const [history, setHistory] = useState<Revision[] | null>(null);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [restoreTarget, setRestoreTarget] = useState<number | null>(null);
  const [conflict, setConflict] = useState(false);

  useEffect(() => {
    let current = true;
    setPayload(null);
    setState("loading");
    setMessage("");
    setHistory(null);
    setRestoreTarget(null);
    setConflict(false);
    void request(`/api/v1/admin/configuration/${moduleID}`).then(async response => {
      const body = await response.json();
      if (!response.ok) throw new Error(body.message || "Configuration could not be loaded.");
      if (current) { setPayload(body); setBaseline(JSON.stringify(body.values)); setState("ready"); }
    }).catch(error => {
      if (current) { setMessage(error instanceof Error ? error.message : "Configuration could not be loaded."); setState("error"); }
    });
    return () => { current = false; };
  }, [moduleID, retry]);

  const setValue = (key: string, value: boolean | number | string) => {
    setPayload(current => current ? { ...current, values: { ...current.values, [key]: value } } : current);
    if (state === "saved") setState("ready");
  };

  const save = async () => {
    if (!payload) return;
    setState("saving");
    setMessage("");
    try {
      const response = await request(`/api/v1/admin/configuration/${moduleID}`, {
        method: "PUT",
        body: JSON.stringify({ values: payload.values, version: payload.version }),
      });
      const body = await response.json();
      if (response.status === 409) setConflict(true);
      if (!response.ok) throw new Error(body.message || "The configuration was rejected.");
      setPayload(current => current ? { ...current, values: body.values, version: body.version, updatedAt: body.updatedAt } : current);
      setBaseline(JSON.stringify(body.values));
      setHistory(null);
      setState("saved");
      setMessage("Configuration validated and saved.");
    } catch (error) {
      setState("error");
      setMessage(error instanceof Error ? error.message : "Configuration could not be saved.");
    }
  };

  const restore = async () => {
    if (!payload || restoreTarget === null) return;
    setState("saving");
    setMessage("");
    try {
      const response = await request(`/api/v1/admin/configuration/${moduleID}/restore`, {
        method: "POST",
        body: JSON.stringify({ targetVersion: restoreTarget, currentVersion: payload.version }),
      });
      const body = await response.json();
      if (response.status === 409) setConflict(true);
      if (!response.ok) throw new Error(body.message || "The revision could not be restored.");
      setPayload(current => current ? { ...current, values: body.values, version: body.version, updatedAt: body.updatedAt } : current);
      setBaseline(JSON.stringify(body.values));
      setRestoreTarget(null);
      setHistory(null);
      setState("saved");
      setMessage(`Revision ${body.restoredFrom} restored as revision ${body.version}.`);
    } catch (error) {
      setState("error");
      setMessage(error instanceof Error ? error.message : "The revision could not be restored.");
    }
  };

  const loadHistory = async () => {
    if (history) { setHistory(null); return; }
    setHistoryLoading(true);
    try {
      const response = await request(`/api/v1/admin/configuration/${moduleID}/history?limit=8`);
      const body = await response.json();
      if (!response.ok) throw new Error(body.message || "Revision history could not be loaded.");
      setHistory(body.revisions || []);
    } catch (error) {
      setMessage(error instanceof Error ? error.message : "Revision history could not be loaded.");
      setState("error");
    } finally {
      setHistoryLoading(false);
    }
  };

  if (!payload) {
    return <Card className="dm-card dm-live-config"><CardBody>
      {state === "error" ? <Alert tone="danger" title="Configuration unavailable">{message}</Alert> : <div className="dm-config-loading"><StatusIndicator tone="neutral" label="Loading configuration" /></div>}
    </CardBody></Card>;
  }

  const definition = payload.definition;
  const dirty = JSON.stringify(payload.values) !== baseline;
  return <Card className="dm-card dm-live-config">
    <CardHeader><div><h2>{definition.title}</h2><p>{definition.description}</p></div>
      <StatusIndicator tone={state === "saved" ? "success" : state === "error" ? "danger" : "neutral"} label={state === "saving" ? "Saving" : state === "saved" ? "Saved" : state === "error" ? "Needs attention" : "Configured"} />
    </CardHeader>
    <CardBody>
      {message && <Alert tone={state === "error" ? "danger" : "success"} title={state === "error" ? "Unable to save configuration" : "Configuration saved"}>{message}</Alert>}
      <div className="dm-config-summary"><ShieldCheck /><span><strong>{definition.group}</strong><small>{definition.sections.length} configuration sections · server-validated fields</small></span><Badge tone="accent">REV {payload.version || "—"}</Badge></div>
      {definition.sections.map(section => <fieldset className="dm-config-section" disabled={state === "saving"} key={section.id}>
        <legend><strong>{section.title}</strong>{section.description && <small>{section.description}</small>}</legend>
        <div className="dm-config-fields">{section.fields.map(field => {
          const value = payload.values[field.key];
          if (field.type === "boolean") return <Checkbox className="dm-config-toggle" key={field.key} label={field.label} description={field.description} checked={Boolean(value)} onChange={event => setValue(field.key, event.target.checked)} />;
          if (field.type === "select") return <Select key={field.key} label={field.label} hint={field.description} value={String(value ?? "")} options={field.options || []} onValueChange={next => setValue(field.key, next)} />;
          if (field.type === "textarea") return <TextArea className="dm-config-wide" key={field.key} label={field.label} hint={field.description} required={field.required} value={String(value ?? "")} onChange={event => setValue(field.key, event.target.value)} />;
          return <TextField key={field.key} type={field.type === "number" ? "number" : "text"} label={field.label} hint={field.description} required={field.required} min={field.min} max={field.max} value={field.type === "number" ? Number(value ?? 0) : String(value ?? "")} onChange={event => setValue(field.key, field.type === "number" ? Number(event.target.value) : event.target.value)} />;
        })}</div>
      </fieldset>)}
      {history && <section className="dm-config-history" aria-label="Configuration revision history">
        <h3>Revision history</h3>
        {history.length === 0 ? <p>No saved revisions yet.</p> : <ol>{history.map(item => <li key={item.version}><span><strong>Revision {item.version}</strong><small>{item.modifiedBy} · {new Date(item.createdAt).toLocaleString()}</small></span>{item.version !== payload.version && <Button size="sm" variant="ghost" onClick={()=>setRestoreTarget(item.version)}>Restore</Button>}</li>)}</ol>}
      </section>}
      {restoreTarget !== null && <div className="dm-restore-confirm" role="alert"><span><strong>Restore revision {restoreTarget}?</strong><small>The current settings remain in history and this creates a new revision.</small></span><div><Button size="sm" variant="ghost" onClick={()=>setRestoreTarget(null)}>Cancel</Button><Button size="sm" variant="primary" onClick={()=>void restore()}>Confirm restore</Button></div></div>}
      <footer className="dm-config-footer">
        <span>{dirty ? "Unsaved changes" : payload.updatedAt ? `Last saved ${new Date(payload.updatedAt).toLocaleString()}` : "Using audited secure defaults"}</span>
        <div>{conflict && <Button variant="ghost" onClick={()=>setRetry(current=>current+1)}>Reload current</Button>}<Button variant="ghost" leftIcon={<History />} disabled={historyLoading} onClick={()=>void loadHistory()}>{historyLoading ? "Loading…" : history ? "Hide history" : "View history"}</Button><Button variant="primary" leftIcon={<Save />} disabled={state === "saving" || !dirty} onClick={()=>void save()}>{state === "saving" ? "Saving…" : "Save configuration"}</Button></div>
      </footer>
    </CardBody>
  </Card>;
}
