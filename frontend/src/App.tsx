import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AppShell } from "./layout/AppShell";
import { Admin } from "./pages/Admin";
import { DirectoryConfiguration } from "./pages/Configuration";
import { Dashboard } from "./pages/Dashboard";
import { DirectoryBrowser } from "./pages/DirectoryBrowser";
import { Policies } from "./pages/Policies";
import { Reports } from "./pages/Reports";
import { AuditEvents, UserManagement } from "./UserOperations";
import { OperationsWorkspace } from "./Workspace";

export function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<AppShell />}>
          <Route index element={<Dashboard />} />
          <Route path="workspace" element={<OperationsWorkspace />} />
          <Route path="directory" element={<DirectoryBrowser />} />
          <Route path="users" element={<UserManagement />} />
          <Route path="configuration" element={<DirectoryConfiguration />} />
          <Route path="policies" element={<Policies />} />
          <Route path="reports" element={<Reports />} />
          <Route path="audit" element={<AuditEvents />} />
          <Route path="admin" element={<Admin />} />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Route>
      </Routes>
    </BrowserRouter>
  );
}

export default App;
