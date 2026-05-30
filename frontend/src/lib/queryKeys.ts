// React Query key factory. Keys share prefixes so related queries invalidate
// together.
export const queryKeys = {
  me: () => ["me"] as const,
  documents: () => ["documents"] as const,
  document: (id: string) => ["document", id] as const,
  versions: (doc: string) => ["versions", doc] as const,
  render: (version: string) => ["render", version] as const,
  comments: (doc: string) => ["comments", doc] as const,
  grants: (doc: string) => ["grants", doc] as const,
  serviceAccounts: () => ["service-accounts"] as const,
  serviceAccountKeys: (id: string) =>
    ["service-account-keys", id] as const,
};
