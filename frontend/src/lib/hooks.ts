import { useQuery } from "@tanstack/react-query";
import { api, APIError } from "@/api";

export function useMe() {
  const q = useQuery({ queryKey: ["me"], queryFn: api.me, retry: false });
  const unauth = q.error instanceof APIError && q.error.status === 401;
  return { ...q, unauth };
}
