import { Outlet, useLocation, useNavigate } from "react-router-dom";
import { useEffect } from "react";
import { TopBar } from "./topbar";
import { useMe } from "@/lib/hooks";
import { Center } from "@/components/ui/misc";

export function AppShell() {
  const me = useMe();
  const nav = useNavigate();
  const loc = useLocation();
  useEffect(() => {
    if (me.unauth) {
      const next = encodeURIComponent(loc.pathname + loc.search);
      nav(`/login?next=${next}`, { replace: true });
    }
  }, [me.unauth, loc.pathname, loc.search, nav]);
  if (me.isLoading) return <Center>Loading workspace…</Center>;
  if (!me.data) return null;
  const user = me.data.user || me.data.principal;
  return (
    <div className="flex h-full flex-col">
      <TopBar user={user} />
      <main className="flex-1 overflow-auto">
        <Outlet />
      </main>
    </div>
  );
}
