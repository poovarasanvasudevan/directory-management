import { useState } from "react";
import { NavLink, Outlet, useLocation } from "react-router-dom";
import { Bell, Menu } from "lucide-react";
import { AppLayout, Button, NotificationProvider, SearchBox, Sidebar, SidebarItem, TopBar } from "@kiteapps/ui";
import { navigation, navItemForPath } from "../navigation";
import { DashboardHelp } from "../pages/DashboardHelp";

function Shell() {
  const [menu, setMenu] = useState(false);
  const { pathname } = useLocation();
  const active = navItemForPath(pathname);
  const showDashboardHelp = pathname === "/";

  return (
    <AppLayout
      className="dm-shell"
      sidebar={
        <Sidebar open={menu} brand={<span className="dm-mark">D</span>} brandLabel="Directory Desk">
          {navigation.map((item) => (
            <SidebarItem
              as={NavLink}
              key={item.id}
              to={item.path}
              end={item.path === "/"}
              active={active.id === item.id}
              icon={<item.icon />}
              label={item.label}
              onClick={() => setMenu(false)}
            />
          ))}
        </Sidebar>
      }
      topbar={
        <TopBar
          className="dm-topbar"
          leading={
            <Button className="dm-menu" variant="ghost" size="sm" iconOnly aria-label="Toggle navigation" onClick={() => setMenu(!menu)}>
              <Menu />
            </Button>
          }
          title={active.label}
          actions={<SearchBox className="dm-global-search" placeholder="Search users or settings" shortcut="⌘K" />}
          trailing={
            <div className="dm-profile">
              <Button variant="ghost" size="sm" iconOnly aria-label="Notifications"><Bell /></Button>
              <span>RK</span>
              <strong>Ramesh</strong>
            </div>
          }
        />
      }
      aside={showDashboardHelp ? <DashboardHelp /> : undefined}
    >
      <div className="dm-mobile-nav" data-open={menu || undefined}>
        {navigation.map((item) => (
          <NavLink key={item.id} to={item.path} end={item.path === "/"} data-active={active.id === item.id || undefined} onClick={() => setMenu(false)}>
            {item.label}
          </NavLink>
        ))}
      </div>
      <Outlet />
    </AppLayout>
  );
}

export function AppShell() {
  return (
    <NotificationProvider placement="bottom-right" max={3}>
      <Shell />
    </NotificationProvider>
  );
}
