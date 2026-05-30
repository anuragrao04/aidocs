import { LogOut, Monitor, Moon, Sun } from "lucide-react";
import { Avatar, Center, Skeleton } from "@/components/ui/misc";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { useMe } from "@/lib/hooks";
import { useTheme } from "@/lib/theme";
import { cn } from "@/lib/cn";

export function ProfilePage() {
  const me = useMe();
  const { theme, setTheme } = useTheme();
  const user = me.data?.user || me.data?.principal;

  if (me.isLoading) {
    return (
      <div className="mx-auto max-w-2xl px-6 py-10">
        <Skeleton className="mb-6 h-8 w-40" />
        <Skeleton className="h-28 w-full" />
      </div>
    );
  }
  if (me.error || !user) {
    return <Center>Could not load your profile.</Center>;
  }
  return (
    <div className="mx-auto max-w-2xl px-6 py-10">
      <div className="mb-6">
        <h1 className="text-2xl font-semibold tracking-tight">Profile</h1>
      </div>
      <div className="space-y-4">
        <Card>
          <CardContent className="flex items-center gap-4 pt-5">
            <Avatar
              size={56}
              src={user.picture_url}
              name={user.name || user.email}
            />
            <div className="min-w-0">
              <div className="font-medium">{user.name || "Signed-in user"}</div>
              <div className="text-sm text-[var(--color-fg-muted)]">
                {user.email || user.id}
              </div>
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className="text-sm">Appearance</CardTitle>
            <CardDescription>Choose how aidocs looks to you.</CardDescription>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-3 gap-2">
              {(
                [
                  ["light", "Light", Sun],
                  ["dark", "Dark", Moon],
                  ["system", "System", Monitor],
                ] as const
              ).map(([v, label, Icon]) => (
                <button
                  key={v}
                  onClick={() => setTheme(v)}
                  className={cn(
                    "flex flex-col items-center gap-2 rounded-md border p-4 text-xs transition-colors",
                    theme === v
                      ? "border-[var(--color-accent)] bg-[var(--color-accent-muted)]"
                      : "border-[var(--color-border)] hover:bg-[var(--color-surface-muted)]",
                  )}
                >
                  <Icon className="h-4 w-4" /> {label}
                </button>
              ))}
            </div>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="pt-5">
            <Button
              variant="outline"
              onClick={() => (window.location.href = "/v1/auth/logout")}
            >
              <LogOut className="h-4 w-4" /> Sign out
            </Button>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
