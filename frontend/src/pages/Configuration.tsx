import { useEffect, useMemo, useState } from "react";
import { Network, RefreshCw, ShieldCheck, UserCheck } from "lucide-react";
import {
  Accordion, AccordionItem, AccordionPanel, AccordionTrigger, Alert, Button, Card, CardBody,
  CardHeader, PageHeader, StatusIndicator, TextField,
} from "@kiteapps/ui";
import { ConfigurationModule } from "../ConfigurationModule";
import { DIRECTORY_CONNECTION_ID } from "../data/configurationModules";
import { useQueryState } from "../hooks/useQueryState";
import { adminFetch } from "../lib/adminFetch";

type DirectoryConfig = {
  directory: {
    name: string;
    host: string;
    port: number;
    security: string;
    baseDN: string;
    bindDN: string;
    userSearchBase: string;
    groupSearchBase: string;
  };
  credentialsConfigured: boolean;
  tlsVerification: boolean;
};

type ConfigModule = {
  id: string;
  group: string;
  title: string;
  description: string;
};

function slugGroup(group: string) {
  return group.toLowerCase().replace(/[^a-z0-9]+/g, "-");
}

export function DirectoryConfiguration() {
  const [moduleID, setModuleID] = useQueryState("tab", DIRECTORY_CONNECTION_ID);
  const [modules, setModules] = useState<ConfigModule[]>([]);
  const [modulesError, setModulesError] = useState("");

  const knownModuleIDs = useMemo(
    () => new Set([DIRECTORY_CONNECTION_ID, ...modules.map((item) => item.id)]),
    [modules],
  );
  const activeModuleID = knownModuleIDs.has(moduleID) ? moduleID : DIRECTORY_CONNECTION_ID;
  const configurationGroups = useMemo(() => {
    const order: string[] = [];
    const map = new Map<string, ConfigModule[]>();
    for (const item of modules) {
      if (!map.has(item.group)) {
        order.push(item.group);
        map.set(item.group, []);
      }
      map.get(item.group)!.push(item);
    }
    return order.map((group) => ({
      id: slugGroup(group),
      group,
      items: map.get(group)!,
    }));
  }, [modules]);

  const [config, setConfig] = useState<DirectoryConfig | null>(null);
  const [message, setMessage] = useState<{
    tone: "info" | "success" | "danger" | "warning";
    title: string;
    detail: string;
  }>({
    tone: "info",
    title: "Connection not tested",
    detail: "Test the configured service account before enabling directory workflows.",
  });
  const [testing, setTesting] = useState(false);
  const [units, setUnits] = useState(0);

  const loadConfig = () =>
    adminFetch("/api/v1/admin/directories/config")
      .then(async (response) => {
        if (!response.ok) throw new Error("Configuration API is unavailable");
        setConfig(await response.json());
      })
      .catch((error) => setMessage({ tone: "danger", title: "Configuration unavailable", detail: error.message }));

  const loadModules = () =>
    adminFetch("/api/v1/admin/configuration/")
      .then(async (response) => {
        if (!response.ok) throw new Error("Module catalog is unavailable");
        const body = await response.json();
        setModules(body.modules || []);
        setModulesError("");
      })
      .catch((error) => setModulesError(error instanceof Error ? error.message : "Module catalog is unavailable"));

  useEffect(() => {
    void loadConfig();
    void loadModules();
  }, []);

  const testConnection = async () => {
    setTesting(true);
    setUnits(0);
    try {
      const response = await adminFetch("/api/v1/admin/directories/test", { method: "POST" });
      const result = await response.json();
      if (!response.ok) throw new Error(result.message || "Connection failed");
      setMessage({ tone: "success", title: "Active Directory connected", detail: `${result.message} (${result.latencyMs} ms)` });
      const ouResponse = await adminFetch("/api/v1/admin/directories/organizational-units");
      if (ouResponse.ok) {
        const body = await ouResponse.json();
        setUnits(body.organizationalUnits.length);
      }
    } catch (error) {
      setMessage({
        tone: "danger",
        title: "Connection failed",
        detail: error instanceof Error ? error.message : "Unknown directory error",
      });
    } finally {
      setTesting(false);
    }
  };

  const ad = config?.directory;
  const openSections = ["active-directory", ...configurationGroups.map((group) => group.id)];

  return (
    <main className="dm-page">
      <PageHeader
        title="Configuration"
        description="Connect Active Directory from config.yaml and manage self-service, administrative, and security modules."
        actions={
          <Button variant="primary" size="sm" leftIcon={<RefreshCw />} disabled={testing || !config} onClick={testConnection}>
            {testing ? "Testing…" : "Test connection"}
          </Button>
        }
      />
      <section className="dm-config-layout">
        <nav aria-label="Configuration modules">
          <Accordion
            className="dm-module-nav"
            type="multiple"
            defaultValue={openSections}
            key={openSections.join("|")}
          >
            <AccordionItem value="active-directory">
              <AccordionTrigger>
                <span className="dm-module-heading">Active Directory<Network /></span>
              </AccordionTrigger>
              <AccordionPanel>
                <button
                  className="dm-module-link"
                  type="button"
                  aria-current={activeModuleID === DIRECTORY_CONNECTION_ID ? "page" : undefined}
                  onClick={() => setModuleID(DIRECTORY_CONNECTION_ID)}
                >
                  Directory Connection
                </button>
              </AccordionPanel>
            </AccordionItem>
            {configurationGroups.map((group) => (
              <AccordionItem value={group.id} key={group.id}>
                <AccordionTrigger>{group.group}</AccordionTrigger>
                <AccordionPanel>
                  {group.items.map((item) => (
                    <button
                      className="dm-module-link"
                      type="button"
                      key={item.id}
                      aria-current={activeModuleID === item.id ? "page" : undefined}
                      onClick={() => setModuleID(item.id)}
                    >
                      {item.title}
                    </button>
                  ))}
                </AccordionPanel>
              </AccordionItem>
            ))}
          </Accordion>
          {modulesError && <Alert tone="warning" title="Module catalog unavailable">{modulesError}</Alert>}
        </nav>
        <div className="dm-config-content">
          {activeModuleID !== DIRECTORY_CONNECTION_ID && <ConfigurationModule moduleID={activeModuleID} />}
          {activeModuleID === DIRECTORY_CONNECTION_ID && (
            <>
              <Alert tone={message.tone} title={message.title}>{message.detail}</Alert>
              <Card className="dm-card">
                <CardHeader>
                  <div>
                    <h2>Directory connection</h2>
                    <p>Connection and credential settings are loaded from config.yaml.</p>
                  </div>
                  <StatusIndicator
                    tone={config?.credentialsConfigured ? "success" : "warning"}
                    label={config?.credentialsConfigured ? "Credentials loaded" : "Password missing"}
                  />
                </CardHeader>
                <CardBody className="dm-directory-form">
                  <TextField label="Directory name" value={ad?.name || ""} readOnly />
                  <TextField label="Domain controller" value={ad?.host || ""} readOnly />
                  <TextField label="Port" value={ad?.port || ""} readOnly />
                  <TextField label="Transport security" value={ad?.security?.toUpperCase() || ""} readOnly />
                  <TextField className="dm-field-wide" label="Base DN" value={ad?.baseDN || ""} readOnly />
                  <TextField className="dm-field-wide" label="Bind DN" value={ad?.bindDN || ""} readOnly />
                  <TextField className="dm-field-wide" label="User search base" value={ad?.userSearchBase || ""} readOnly />
                  <TextField className="dm-field-wide" label="Group search base" value={ad?.groupSearchBase || ""} readOnly />
                </CardBody>
              </Card>
              <Card className="dm-card dm-directory-readiness">
                <CardHeader>
                  <div>
                    <h2>Integration readiness</h2>
                    <p>Capabilities unlocked by a verified directory connection.</p>
                  </div>
                </CardHeader>
                <CardBody>
                  <div>
                    <Network />
                    <span>
                      <strong>Organizational units</strong>
                      <small>{units ? `${units} discovered during the latest test` : "Discovered after a successful test"}</small>
                    </span>
                  </div>
                  <div>
                    <UserCheck />
                    <span>
                      <strong>User and group lookup</strong>
                      <small>Search bases configured for policy targeting</small>
                    </span>
                  </div>
                  <div>
                    <ShieldCheck />
                    <span>
                      <strong>Secure bind</strong>
                      <small>{config?.tlsVerification ? "Certificate verification enabled" : "Certificate verification disabled"}</small>
                    </span>
                  </div>
                </CardBody>
              </Card>
            </>
          )}
        </div>
      </section>
    </main>
  );
}
