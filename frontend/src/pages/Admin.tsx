import { PageHeader, Tab, TabList, TabPanel, Tabs } from "@kiteapps/ui";
import { ConfigurationModule } from "../ConfigurationModule";
import {
  administrationGroups,
  allAdministrationModules,
  DEFAULT_ADMIN_TAB,
} from "../data/administrationModules";
import { useQueryState } from "../hooks/useQueryState";

const knownTabs = new Set(allAdministrationModules.map((item) => item.id));

export function Admin() {
  const [tab, setTab] = useQueryState("tab", DEFAULT_ADMIN_TAB);
  const moduleID = knownTabs.has(tab) ? tab : DEFAULT_ADMIN_TAB;

  return (
    <main className="dm-page">
      <PageHeader
        title="Administration"
        description="Configure portal access, resilience, product services, privacy, and licensing."
      />
      <Tabs
        className="dm-settings"
        orientation="vertical"
        defaultValue={DEFAULT_ADMIN_TAB}
        value={moduleID}
        onValueChange={setTab}
      >
        <TabList className="dm-admin-tablist" aria-label="Administration settings">
          {administrationGroups.map((group) => (
            <div className="dm-admin-tab-group" role="presentation" key={group.group}>
              <span className="dm-admin-tab-group-label" role="presentation">{group.group}</span>
              {group.items.map((item) => (
                <Tab value={item.id} key={item.id}>{item.title}</Tab>
              ))}
            </div>
          ))}
        </TabList>
        {allAdministrationModules.map((item) => (
          <TabPanel className="dm-admin-tabpanel" value={item.id} key={item.id}>
            <ConfigurationModule moduleID={item.id} />
          </TabPanel>
        ))}
      </Tabs>
    </main>
  );
}
