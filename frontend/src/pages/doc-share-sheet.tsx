import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/misc";
import {
  SheetBody,
  SheetContent,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet";
import { api } from "@/api";
import { queryKeys } from "@/lib/queryKeys";
import { ROLES, DEFAULT_ROLE } from "@/lib/constants";

export function ShareSheet({ docId }: { docId: string }) {
  const q = useQueryClient();
  const grants = useQuery({
    queryKey: queryKeys.grants(docId),
    queryFn: () => api.listGrants(docId),
  });
  const [address, setAddress] = useState("");
  const [role, setRole] = useState(DEFAULT_ROLE);
  const m = useMutation({
    mutationFn: () => api.createGrant(docId, address.trim(), role),
    onSuccess: () => {
      setAddress("");
      toast.success("Access granted.");
      q.invalidateQueries({ queryKey: queryKeys.grants(docId) });
    },
    onError: (e) =>
      toast.error(e instanceof Error ? e.message : "Couldn't share."),
  });
  const items = grants.data?.items || [];
  return (
    <SheetContent>
      <SheetHeader>
        <SheetTitle>Share document</SheetTitle>
      </SheetHeader>
      <SheetBody>
        <form
          className="space-y-3"
          onSubmit={(e) => {
            e.preventDefault();
            m.mutate();
          }}
        >
          <div>
            <Input
              placeholder="anurag@razorpay.com  or  n8n-prod@brave.otter.bot"
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              autoCapitalize="off"
              autoCorrect="off"
              spellCheck={false}
            />
            <p className="mt-1.5 text-[11px] text-[var(--color-fg-muted)]">
              Add a person by their email, or a bot by its address.
            </p>
          </div>
          <select
            value={role}
            onChange={(e) => setRole(e.target.value)}
            aria-label="Access role"
            className="h-9 w-full rounded-[10px] border border-[var(--color-border)] bg-[var(--color-surface)] px-3 text-sm"
          >
            {ROLES.map((r) => (
              <option key={r.value} value={r.value}>
                {r.label}
              </option>
            ))}
          </select>
          <Button
            className="w-full"
            type="submit"
            disabled={!address.trim() || m.isPending}
          >
            Share
          </Button>
        </form>
        <div className="mt-6">
          <h4 className="mb-2 text-xs font-medium uppercase tracking-wider text-[var(--color-fg-muted)]">
            People with access
          </h4>
          <div className="space-y-1">
            {grants.isLoading ? (
              <>
                <Skeleton className="h-9 w-full" />
                <Skeleton className="h-9 w-full" />
              </>
            ) : grants.error ? (
              <div className="text-xs text-[var(--color-danger)]">
                Could not load access list.
              </div>
            ) : items.length === 0 ? (
              <div className="text-xs text-[var(--color-fg-muted)]">
                No grants yet.
              </div>
            ) : (
              items.map((g) => (
                <div
                  key={g.id}
                  className="flex items-center justify-between rounded-md border border-[var(--color-border)] bg-[var(--color-surface)] px-3 py-2 text-sm"
                >
                  <span className="truncate">
                    {g.principal?.email || g.principal?.id}
                  </span>
                  <Badge variant="muted">{g.role}</Badge>
                </div>
              ))
            )}
          </div>
        </div>
      </SheetBody>
    </SheetContent>
  );
}
