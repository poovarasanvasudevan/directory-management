/** Admin tabs reuse ConfigurationModule against catalog-aligned configuration IDs. */
export type AdminModule = {
  id: string;
  title: string;
  description: string;
};

export const administrationGroups: { group: string; items: AdminModule[] }[] = [
  {
    group: "Customize",
    items: [
      {
        id: "login-settings",
        title: "Login Settings",
        description: "Control administrator and directory-user access to the portal.",
      },
      {
        id: "personalization",
        title: "Personalization",
        description: "Configure portal branding, language, date formats, and timezone.",
      },
    ],
  },
  {
    group: "Enterprise Essentials",
    items: [
      {
        id: "resilience",
        title: "Failover and Reverse Proxy",
        description: "Operate primary/secondary nodes, secure gateway, and reverse proxy.",
      },
    ],
  },
  {
    group: "Product Settings",
    items: [
      {
        id: "mail-sms",
        title: "Mail / SMS Settings",
        description: "Configure secure notification delivery providers.",
      },
      {
        id: "schedulers",
        title: "Schedulers and Backups",
        description: "Configure synchronizers, report jobs, and encrypted backups.",
      },
      {
        id: "integrations",
        title: "Integration Settings",
        description: "Connect ITSM, SIEM, password-breach, and identity products.",
      },
      {
        id: "privacy-licensing",
        title: "Security, Privacy, and Licensing",
        description: "Control protected exports, telemetry, entitlements, and restricted users.",
      },
    ],
  },
];

export const allAdministrationModules = administrationGroups.flatMap((group) => group.items);
export const DEFAULT_ADMIN_TAB = allAdministrationModules[0]?.id ?? "login-settings";
