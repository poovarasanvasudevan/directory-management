import { useEffect, useState } from "react";
import { ChevronDown, Download, Search } from "lucide-react";
import {
  Alert, Badge, Button, Card, CardBody, CardHeader, PageHeader, SearchBox, Table, TBody, TD, TH, THead, TR,
} from "@kiteapps/ui";
import { useQueryState } from "../hooks/useQueryState";
import { adminFetch } from "../lib/adminFetch";

type ReportDefinition = { id: string; category: string; name: string; description: string };
type ReportResult = { report: ReportDefinition; source: string; columns: string[]; rows: string[][] };

export function Reports() {
  const [selected, setSelected] = useQueryState("tab", "locked-users");
  const [catalog, setCatalog] = useState<ReportDefinition[]>([]);
  const [result, setResult] = useState<ReportResult | null>(null);
  const [query, setQuery] = useState("");
  const [error, setError] = useState("");

  const loadCatalog = async () => {
    const response = await adminFetch("/api/v1/admin/reports/");
    if (!response.ok) throw new Error("Enter the administrator token in Configuration before opening reports.");
    const body = await response.json();
    setCatalog(body.reports);
  };

  const loadReport = async (id = selected, search = query) => {
    setError("");
    const response = await adminFetch(`/api/v1/admin/reports/${id}?q=${encodeURIComponent(search)}`);
    if (!response.ok) {
      setError("Report data is unavailable. Verify administrator access.");
      return;
    }
    setResult(await response.json());
  };

  useEffect(() => {
    void loadCatalog().catch((cause) => setError(cause.message));
  }, []);

  useEffect(() => {
    if (!catalog.length) return;
    void loadReport(selected, query);
  }, [catalog, selected]);

  const choose = (id: string) => {
    setQuery("");
    setSelected(id);
  };

  const exportCSV = async () => {
    const response = await adminFetch(`/api/v1/admin/reports/${selected}/export?q=${encodeURIComponent(query)}`);
    if (!response.ok) {
      setError("CSV export failed.");
      return;
    }
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `${selected}.csv`;
    anchor.click();
    URL.revokeObjectURL(url);
  };

  const categories = [...new Set(catalog.map((item) => item.category))];

  return (
    <main className="dm-page">
      <PageHeader
        title="Reports"
        description="Investigate directory, self-service, MFA, endpoint, and security activity."
        actions={
          <Button variant="secondary" size="sm" leftIcon={<Download />} disabled={!result} onClick={exportCSV}>
            Export CSV
          </Button>
        }
      />
      {error && <Alert tone="warning" title="Reports unavailable">{error}</Alert>}
      <section className="dm-reports-layout">
        <nav className="dm-report-nav" aria-label="Report catalog">
          {categories.map((category) => (
            <div key={category}>
              <strong>{category}<ChevronDown /></strong>
              {catalog
                .filter((item) => item.category === category)
                .map((item) => (
                  <button type="button" key={item.id} data-active={item.id === selected || undefined} onClick={() => choose(item.id)}>
                    {item.name}
                  </button>
                ))}
            </div>
          ))}
        </nav>
        <div className="dm-report-content">
          <Card className="dm-card">
            <CardHeader>
              <div>
                <h2>{result?.report.name || "Select a report"}</h2>
                <p>{result?.report.description || "Choose a report from the catalog."}</p>
              </div>
              {result && <Badge tone="accent">{result.source.toUpperCase()} DATA</Badge>}
            </CardHeader>
            <CardBody className="dm-report-body">
              <div className="dm-report-tools">
                <SearchBox
                  value={query}
                  onChange={(event) => setQuery(event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === "Enter") void loadReport();
                  }}
                  placeholder="Filter report rows"
                  aria-label="Filter report rows"
                />
                <Button variant="secondary" size="sm" onClick={() => loadReport()}>Apply filter</Button>
              </div>
              {result && (
                <Table compact>
                  <THead>
                    <TR>{result.columns.map((column) => <TH key={column}>{column}</TH>)}</TR>
                  </THead>
                  <TBody>
                    {result.rows.map((row, index) => (
                      <TR key={index}>{row.map((value, cell) => <TD key={cell}>{value}</TD>)}</TR>
                    ))}
                  </TBody>
                </Table>
              )}
              {result?.rows.length === 0 && (
                <div className="dm-empty">
                  <Search />
                  <strong style={{ fontSize: 12}}>No matching report rows</strong>
                  <span style={{ fontSize: 10}}>Change the filter and try again.</span>
                </div>
              )}
            </CardBody>
          </Card>
        </div>
      </section>
    </main>
  );
}
