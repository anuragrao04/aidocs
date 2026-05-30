import { Link, useNavigate } from "react-router-dom";
import {
  Check,
  LogOut,
  Monitor,
  Moon,
  Rocket,
  Sparkles,
  Sun,
  Terminal,
  User as UserIcon,
} from "lucide-react";
import { GetStartedPill } from "./get-started-pill";
import { useOnboarding } from "@/lib/onboarding";
import {
  Dropdown,
  DropdownContent,
  DropdownItem,
  DropdownLabel,
  DropdownSeparator,
  DropdownTrigger,
} from "@/components/ui/dropdown";
import { Avatar } from "@/components/ui/misc";
import { useTheme } from "@/lib/theme";
import type { Principal } from "@/api";

export function TopBar({
  user,
}: {
  user: Principal;
}) {
  const nav = useNavigate();
  const { theme, setTheme } = useTheme();
  const { reset } = useOnboarding();
  return (
    <header className="sticky top-0 z-30 flex h-14 items-center justify-between border-b border-[var(--color-border)] bg-[var(--color-surface)]/80 px-5 backdrop-blur">
      <Link to="/app/documents" className="flex items-center gap-2 font-semibold">
        <Sparkles className="h-5 w-5 text-[var(--color-accent)]" />
        aidocs
      </Link>
      <div className="flex items-center gap-2">
        <GetStartedPill />
        <Dropdown>
          <DropdownTrigger asChild>
            <button
              className="rounded-full outline-none focus-visible:ring-2 focus-visible:ring-[var(--color-accent)]"
              aria-label="Account menu"
            >
              <Avatar
                src={user.picture_url}
                name={user.name || user.email || user.id}
              />
            </button>
          </DropdownTrigger>
          <DropdownContent>
            <div className="px-2 py-2">
              <div className="text-sm font-medium">
                {user.name || "Signed in"}
              </div>
              <div className="text-xs text-[var(--color-fg-muted)]">
                {user.email || user.id}
              </div>
            </div>
            <DropdownSeparator />
            <DropdownItem
              onSelect={() => {
                reset();
                nav("/app/start");
              }}
            >
              <Rocket className="h-4 w-4" /> Setup guide
            </DropdownItem>
            <DropdownItem onSelect={() => nav("/app/settings/profile")}>
              <UserIcon className="h-4 w-4" /> Profile
            </DropdownItem>
            <DropdownItem onSelect={() => nav("/app/developers")}>
              <Terminal className="h-4 w-4" /> Developers
            </DropdownItem>
            <DropdownSeparator />
            <DropdownLabel>Theme</DropdownLabel>
            {(
              [
                { value: "light", label: "Light", icon: Sun },
                { value: "dark", label: "Dark", icon: Moon },
                { value: "system", label: "System", icon: Monitor },
              ] as const
            ).map(({ value, label, icon: Icon }) => (
              <DropdownItem
                key={value}
                onSelect={(e) => {
                  e.preventDefault();
                  setTheme(value);
                }}
              >
                <Icon className="h-4 w-4" /> {label}
                {theme === value && <Check className="ml-auto h-4 w-4" />}
              </DropdownItem>
            ))}
            <DropdownSeparator />
            <DropdownItem
              onSelect={() => {
                window.location.href = "/v1/auth/logout";
              }}
            >
              <LogOut className="h-4 w-4" /> Sign out
            </DropdownItem>
          </DropdownContent>
        </Dropdown>
      </div>
    </header>
  );
}
