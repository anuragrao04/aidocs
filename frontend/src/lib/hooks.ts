import { useQuery } from "@tanstack/react-query";
import { api, APIError } from "@/api";
import { queryKeys } from "@/lib/queryKeys";

export function useMe() {
  const q = useQuery({ queryKey: queryKeys.me(), queryFn: api.me, retry: false });
  const unauth = q.error instanceof APIError && q.error.status === 401;
  return { ...q, unauth };
}
