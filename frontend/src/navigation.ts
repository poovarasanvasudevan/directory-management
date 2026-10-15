import {
  BarChart3, Blocks, FolderTree, History, LayoutDashboard, ServerCog, Settings, ShieldCheck, UsersRound,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

export type AppRoute =
  | "dashboard"
  | "workspace"
  | "directory"
  | "users"
  | "configuration"
  | "policies"
  | "reports"
  | "audit"
  | "admin";

export type NavItem = {
  id: AppRoute;
  path: string;
  label: string;
  icon: LucideIcon;
};

export const navigation: NavItem[] = [
  { id: "dashboard", path: "/", label: "Dashboard", icon: LayoutDashboard },
  { id: "configuration", path: "/configuration", label: "Configuration", icon: ServerCog },
  { id: "policies", path: "/policies", label: "Policy Configuration", icon: ShieldCheck },
  { id: "directory", path: "/directory", label: "Directory Browser", icon: FolderTree },
  { id: "users", path: "/users", label: "Directory Users", icon: UsersRound },
  { id: "workspace", path: "/workspace", label: "Operations Catalog", icon: Blocks },
  { id: "reports", path: "/reports", label: "Reports", icon: BarChart3 },
  { id: "audit", path: "/audit", label: "Audit", icon: History },
  { id: "admin", path: "/admin", label: "Admin", icon: Settings },
];

export function navItemForPath(pathname: string): NavItem {
  const exact = navigation.find((item) => item.path === pathname);
  if (exact) return exact;
  const nested = navigation.find((item) => item.path !== "/" && pathname.startsWith(item.path));
  return nested ?? navigation[0]!;
}
