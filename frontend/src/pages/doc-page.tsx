import { useMemo, useState } from "react";
import { useParams } from "react-router-dom";
import { useQuery } from "@tanstack/react-query";
import {
  api,
  currentVersionID,
  docTitle,
  versionID,
} from "@/api";
import { queryKeys } from "@/lib/queryKeys";
import { Center, Skeleton } from "@/components/ui/misc";
import { DocProvider, type PendingSelection } from "./doc-context";
import { DocBar } from "./doc-bar";
import { RenderFrame } from "./doc-render-frame";
import { Comments } from "./doc-comments";

export function DocumentPage() {
  const { id = "" } = useParams();
  const doc = useQuery({
    queryKey: queryKeys.document(id),
    queryFn: () => api.getDocument(id),
  });
  const versions = useQuery({
    queryKey: queryKeys.versions(id),
    queryFn: () => api.listVersions(id),
  });
  // One shared comments query for both the paint overlay and the side panel
  // so the two can never desync (web-07).
  const comments = useQuery({
    queryKey: queryKeys.comments(id),
    queryFn: () => api.listComments(id),
    enabled: !!id,
  });
  const [selection, setSelection] = useState<PendingSelection | null>(null);
  const [activeComment, setActiveComment] = useState<string>();

  const current =
    currentVersionID(doc.data || {}) ||
    versionID(versions.data?.items?.[0] || {});

  const ctx = useMemo(
    () => ({
      docId: id,
      version: current,
      comments: comments.data?.items || [],
      commentsLoading: comments.isLoading,
      commentsError: !!comments.error,
      selection,
      setSelection,
      activeComment,
      setActiveComment,
    }),
    [
      id,
      current,
      comments.data,
      comments.isLoading,
      comments.error,
      selection,
      activeComment,
    ],
  );

  if (doc.isLoading) {
    return (
      <div className="p-6">
        <Skeleton className="mb-3 h-7 w-72" />
        <Skeleton className="h-[60vh] w-full" />
      </div>
    );
  }
  if (doc.error) return <Center>Could not load document.</Center>;
  if (!doc.data) return null;

  return (
    <DocProvider value={ctx}>
      <div className="flex h-full flex-col">
        <DocBar title={docTitle(doc.data)} docId={id} current={current} />
        <div className="flex flex-1 overflow-hidden">
          <section className="relative flex-1 bg-[var(--color-surface-muted)]">
            {current ? <RenderFrame /> : <Center>No version available.</Center>}
          </section>
          <aside className="hidden w-[380px] shrink-0 flex-col border-l border-[var(--color-border)] bg-[var(--color-surface)] md:flex">
            <Comments />
          </aside>
        </div>
      </div>
    </DocProvider>
  );
}
