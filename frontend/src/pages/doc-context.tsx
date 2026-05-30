import { createContext, useContext } from "react";
import type { Anchor, Comment } from "@/api";

// A text selection captured from the render bridge that a new comment will
// anchor to. `anchor` is only present when the bridge supplies real offsets
// (web-12); otherwise createComment falls back to a placeholder anchor.
export type PendingSelection = {
  quote: string;
  anchor?: Partial<Anchor>;
};

export type DocContextValue = {
  docId: string;
  version: string;
  comments: Comment[];
  commentsLoading: boolean;
  commentsError: boolean;
  selection: PendingSelection | null;
  setSelection: (s: PendingSelection | null) => void;
  activeComment?: string;
  setActiveComment: (id?: string) => void;
};

const DocContext = createContext<DocContextValue | null>(null);

export const DocProvider = DocContext.Provider;

export function useDoc(): DocContextValue {
  const v = useContext(DocContext);
  if (!v) throw new Error("useDoc must be used within a DocProvider");
  return v;
}
