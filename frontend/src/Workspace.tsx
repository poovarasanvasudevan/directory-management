import { useEffect, useMemo, useState } from "react";
import { AlertTriangle, CheckCircle2, CircleDot, Search, ShieldCheck } from "lucide-react";
import {
  Accordion, AccordionItem, AccordionPanel, AccordionTrigger, Alert, Badge, Card, CardBody,
  CardHeader, PageHeader, SearchBox, StatusIndicator,
} from "@kiteapps/ui";
import { useQueryState } from "./hooks/useQueryState";

export type Capability = {
  id: string;
  group: string;
  name: string;
  description: string;
  status: "available" | "configured" | "monitored" | "review" | "planned";
  features: string[];
};

type CatalogResponse = { source: string; version: string; capabilities: Capability[] };

const statusCopy: Record<Capability["status"], { label: string; tone: "success" | "warning" | "neutral" }> = {
  available: { label: "Available", tone: "success" },
  configured: { label: "Configured", tone: "success" },
  monitored: { label: "Monitored", tone: "neutral" },
  review: { label: "Needs review", tone: "warning" },
  planned: { label: "Backend required", tone: "neutral" },
};

function slugGroup(group: string) {
  return group.toLowerCase().replace(/[^a-z0-9]+/g, "-");
}

export function OperationsWorkspace() {
  const [catalog, setCatalog] = useState<CatalogResponse | null>(null);
  const [selectedID, setSelectedID] = useQueryState("tab", "policy-configuration");
  const [query, setQuery] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    void fetch("/api/v1/admin/capabilities/").then(async (response) => {
      if (!response.ok) throw new Error("The module catalog is unavailable.");
      setCatalog(await response.json());
    }).catch((cause) => setError(cause instanceof Error ? cause.message : "The module catalog is unavailable."));
  }, []);

  const visible = useMemo(() => (catalog?.capabilities || []).filter((item) =>
    `${item.name} ${item.description} ${item.group} ${item.features.join(" ")}`.toLowerCase().includes(query.toLowerCase())
  ), [catalog, query]);

  const grouped = useMemo(() => {
    const order: string[] = [];
    const map = new Map<string, Capability[]>();
    for (const item of visible) {
      if (!map.has(item.group)) {
        order.push(item.group);
        map.set(item.group, []);
      }
      map.get(item.group)!.push(item);
    }
    return order.map((group) => ({ group, id: slugGroup(group), items: map.get(group)! }));
  }, [visible]);

  const selected = catalog?.capabilities.find((item) => item.id === selectedID)
    || visible.find((item) => item.id === selectedID)
    || visible[0];
  const ready = catalog?.capabilities.filter((item) => item.status !== "planned").length || 0;
  const openSections = grouped.map((item) => item.id);

  return (
    <main className="dm-page">
      <PageHeader
        title="Operations catalog"
        description="Browse identity-security capabilities by group, the same way Configuration modules are organized."
        actions={<Badge tone="accent">SPEC {catalog?.version || "—"}</Badge>}
      />
      {error && <Alert tone="warning" title="Protected catalog unavailable">{error}</Alert>}
      {catalog && <>
        <div className="dm-catalog-tools">
          <SearchBox
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Search modules and capabilities"
            aria-label="Search modules and capabilities"
          />
        </div>
        <section className="dm-workspace-layout">
          <nav aria-label="Product modules">
            {grouped.length > 0 ? (
              <Accordion className="dm-module-nav" type="multiple" defaultValue={openSections} key={openSections.join("|")}>
                {grouped.map((section) => (
                  <AccordionItem value={section.id} key={section.id}>
                    <AccordionTrigger>{section.group}</AccordionTrigger>
                    <AccordionPanel>
                      {section.items.map((item) => (
                        <button
                          className="dm-module-link"
                          type="button"
                          key={item.id}
                          aria-current={selected?.id === item.id ? "page" : undefined}
                          onClick={() => setSelectedID(item.id)}
                        >
                          {item.name}
                        </button>
                      ))}
                    </AccordionPanel>
                  </AccordionItem>
                ))}
              </Accordion>
            ) : (
              <div className="dm-empty dm-catalog-empty">
                <Search />
                <strong>No matching modules</strong>
                <span>Try another feature or group.</span>
              </div>
            )}
          </nav>
          {selected && (
            <Card className="dm-card dm-capability-detail">
              <CardHeader>
                <div>
                  <span className="dm-section-label">{selected.group}</span>
                  <h2>{selected.name}</h2>
                  <p>{selected.description}</p>
                </div>
                <StatusIndicator tone={statusCopy[selected.status].tone} label={statusCopy[selected.status].label} />
              </CardHeader>
              <CardBody>
                <div className="dm-delivery-note">
                  {selected.status === "planned" ? <AlertTriangle /> : <CheckCircle2 />}
                  <span>
                    <strong>
                      {selected.status === "planned"
                        ? "Secure implementation pending"
                        : selected.status === "configured"
                          ? "Administrator configuration saved"
                          : "Admin configuration available"}
                    </strong>
                    <small>
                      {selected.status === "planned"
                        ? "The workflow is visible for planning, but no state-changing action will report success until its API, authorization, persistence, and audit path exist."
                        : selected.status === "configured"
                          ? "Settings for this module are persisted in MySQL. Runtime behaviors such as live MFA or mail delivery remain out of scope until their engines exist."
                          : "Open Configuration to edit durable admin settings. Catalog status becomes Configured after the first successful save."}
                    </small>
                  </span>
                </div>
                <h3>Required capabilities</h3>
                <ul className="dm-requirement-list">
                  {selected.features.map((feature) => (
                    <li key={feature}>
                      <CircleDot />
                      <span>{feature}</span>
                      <Badge>{selected.status === "planned" ? "PLANNED" : "TRACKED"}</Badge>
                    </li>
                  ))}
                </ul>
              </CardBody>
            </Card>
          )}
        </section>
      </>}
    </main>
  );
}
