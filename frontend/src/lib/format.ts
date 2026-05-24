import { formatDistanceToNowStrict } from "date-fns";

export function relTime(iso?: string) {
  if (!iso) return "—";
  try {
    return formatDistanceToNowStrict(new Date(iso), { addSuffix: true });
  } catch {
    return "—";
  }
}

export function shortID(id: string, n = 8) {
  if (!id) return "";
  return id.length > n + 4 ? id.slice(0, n) + "…" : id;
}
