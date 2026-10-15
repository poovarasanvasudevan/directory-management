import { useEffect, useState } from "react";
import { Save, ShieldCheck } from "lucide-react";
import { Alert, Badge, Button, Card, CardBody, CardHeader, Checkbox, StatusIndicator } from "@kiteapps/ui";

type SettingDefinition = { key: string; label: string; description: string; defaultValue?: boolean };

export type ModuleDefinition = {
  id: string;
  title: string;
  description: string;
  settings: SettingDefinition[];
};

function request(path: string, init: RequestInit = {}) {
  return fetch(path, { ...init, headers: { "Content-Type": "application/json", ...init.headers } });
}

export function SettingsModule({ module }: { module: ModuleDefinition }) {
  const defaults = Object.fromEntries(module.settings.map(item => [item.key, item.defaultValue ?? false])) as Record<string, boolean>;
  const [values, setValues] = useState<Record<string, boolean>>(defaults);
  const [state, setState] = useState<"loading" | "ready" | "saving" | "saved" | "error">("loading");
  const [message, setMessage] = useState("");

  useEffect(() => {
    setState("loading");
    void request(`/api/v1/admin/settings/${module.id}`).then(async response => {
      if (response.status === 404) { setValues(defaults); setState("ready"); return; }
      if (!response.ok) throw new Error("Settings could not be loaded.");
      const body = await response.json();
      setValues({ ...defaults, ...body.settings });
      setState("ready");
    }).catch(error => { setMessage(error instanceof Error ? error.message : "Settings are unavailable."); setState("error"); });
  }, [module.id]);

  const save = async () => {
    setState("saving"); setMessage("");
    try {
      const response = await request(`/api/v1/admin/settings/${module.id}`, { method: "PUT", body: JSON.stringify({ settings: values }) });
      if (!response.ok) throw new Error("The server rejected these settings.");
      setState("saved"); setMessage("Configuration saved to the protected server-side settings store.");
    } catch (error) { setState("error"); setMessage(error instanceof Error ? error.message : "Settings could not be saved."); }
  };

  return <Card className="dm-card dm-module-settings">
    <CardHeader><div><h2>{module.title}</h2><p>{module.description}</p></div>
      <StatusIndicator tone={state === "saved" ? "success" : state === "error" ? "danger" : "neutral"} label={state === "loading" ? "Loading" : state === "saving" ? "Saving" : state === "saved" ? "Saved" : state === "error" ? "Unavailable" : "Configured"} /></CardHeader>
    <CardBody>
      {message && <Alert tone={state === "error" ? "danger" : "success"} title={state === "error" ? "Unable to save" : "Settings saved"}>{message}</Alert>}
      <div className="dm-module-summary"><ShieldCheck /><span><strong>Policy-backed configuration</strong><small>Changes are authenticated, validated as JSON, and retained by the running Go service.</small></span><Badge>{module.settings.length} CONTROLS</Badge></div>
      <fieldset disabled={state === "loading" || state === "saving"}>{module.settings.map(item => <Checkbox key={item.key} checked={Boolean(values[item.key])} onChange={event => setValues(current => ({ ...current, [item.key]: event.target.checked }))} label={item.label} description={item.description} />)}</fieldset>
      <footer><Button variant="primary" leftIcon={<Save />} disabled={state === "loading" || state === "saving"} onClick={save}>{state === "saving" ? "Saving…" : "Save configuration"}</Button></footer>
    </CardBody>
  </Card>;
}
