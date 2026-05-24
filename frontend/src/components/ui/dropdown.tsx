import * as React from "react";
import * as DM from "@radix-ui/react-dropdown-menu";
import { cn } from "@/lib/cn";

export const Dropdown = DM.Root;
export const DropdownTrigger = DM.Trigger;

export function DropdownContent({
  className,
  ...p
}: React.ComponentPropsWithoutRef<typeof DM.Content>) {
  return (
    <DM.Portal>
      <DM.Content
        sideOffset={6}
        align="end"
        className={cn(
          "z-50 min-w-[200px] rounded-[10px] border border-[var(--color-border)] bg-[var(--color-surface)] p-1 shadow-lg focus:outline-none",
          className,
        )}
        {...p}
      />
    </DM.Portal>
  );
}
export function DropdownItem({
  className,
  ...p
}: React.ComponentPropsWithoutRef<typeof DM.Item>) {
  return (
    <DM.Item
      className={cn(
        "flex cursor-pointer select-none items-center gap-2 rounded-md px-2 py-1.5 text-sm outline-none data-[highlighted]:bg-[var(--color-surface-muted)]",
        className,
      )}
      {...p}
    />
  );
}
export function DropdownSeparator() {
  return <DM.Separator className="my-1 h-px bg-[var(--color-border)]" />;
}
export function DropdownLabel({
  className,
  ...p
}: React.ComponentPropsWithoutRef<typeof DM.Label>) {
  return (
    <DM.Label
      className={cn(
        "px-2 py-1 text-[11px] font-medium uppercase tracking-wider text-[var(--color-fg-muted)]",
        className,
      )}
      {...p}
    />
  );
}
