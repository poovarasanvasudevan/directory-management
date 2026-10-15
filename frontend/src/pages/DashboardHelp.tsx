import { useNavigate } from "react-router-dom";
import {
  Fingerprint, KeyRound, LockKeyhole, Mail, MessageSquare, QrCode, ShieldCheck,
  Smartphone, BellRing, Radio, CircleHelp,
} from "lucide-react";
import { HelpList, HelpListItem, InfoPanel } from "@kiteapps/ui";
import type { ReactNode } from "react";

type HelpLink = { label: string; to: string; icon?: ReactNode };

const highlightLinks: HelpLink[] = [
  { label: "Password Reset/Unlock Account", to: "/policies", icon: <KeyRound /> },
  { label: "Self Update/Change Password", to: "/configuration?tab=directory-self-service", icon: <LockKeyhole /> },
  { label: "Password Expiry Notification", to: "/configuration?tab=password-expiration", icon: <BellRing /> },
  { label: "Password Sync/Single Sign On", to: "/configuration?tab=applications", icon: <ShieldCheck /> },
  { label: "Conditional Access", to: "/configuration?tab=conditional-access", icon: <ShieldCheck /> },
];

const mfaLinks: HelpLink[] = [
  { label: "Security Question & Answer", to: "/configuration?tab=multi-factor-authentication", icon: <CircleHelp /> },
  { label: "Email Verification", to: "/configuration?tab=multi-factor-authentication", icon: <Mail /> },
  { label: "SMS Verification", to: "/configuration?tab=multi-factor-authentication", icon: <MessageSquare /> },
  { label: "Google Authenticator", to: "/configuration?tab=multi-factor-authentication", icon: <Smartphone /> },
  { label: "Microsoft Authenticator", to: "/configuration?tab=multi-factor-authentication", icon: <Smartphone /> },
  { label: "Microsoft Entra ID MFA", to: "/configuration?tab=multi-factor-authentication", icon: <ShieldCheck /> },
  { label: "Duo Security", to: "/configuration?tab=multi-factor-authentication", icon: <ShieldCheck /> },
  { label: "RSA SecurID", to: "/configuration?tab=multi-factor-authentication", icon: <KeyRound /> },
  { label: "RADIUS Authentication", to: "/configuration?tab=multi-factor-authentication", icon: <Radio /> },
  { label: "Push Notification Authentication", to: "/configuration?tab=multi-factor-authentication", icon: <BellRing /> },
  { label: "Fingerprint Authentication", to: "/configuration?tab=multi-factor-authentication", icon: <Fingerprint /> },
  { label: "QR Code Based Authentication", to: "/configuration?tab=multi-factor-authentication", icon: <QrCode /> },
];

function HelpNavItem({ to, icon, children }: { to: string; icon?: ReactNode; children: ReactNode }) {
  const navigate = useNavigate();
  return (
    <HelpListItem
      href={to}
      icon={icon}
      onClick={(event) => {
        event.preventDefault();
        navigate(to);
      }}
    >
      {children}
    </HelpListItem>
  );
}

export function DashboardHelp() {
  return (
    <InfoPanel aria-label="Configuration shortlinks">
      <HelpList title="Highlights">
        {highlightLinks.map((item) => (
          <HelpNavItem key={item.label} to={item.to} icon={item.icon}>{item.label}</HelpNavItem>
        ))}
      </HelpList>
        <br/>
      <HelpList title="Multi-factor Authentication">
        {mfaLinks.map((item) => (
          <HelpNavItem key={item.label} to={item.to} icon={item.icon}>{item.label}</HelpNavItem>
        ))}
      </HelpList>
    </InfoPanel>
  );
}
