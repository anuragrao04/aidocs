import { Link, useNavigate } from "react-router-dom";
import { useMutation, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";
import {
  ArrowLeft,
  ExternalLink,
  MoreHorizontal,
  Share2,
  Trash2,
} from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Dropdown,
  DropdownContent,
  DropdownItem,
  DropdownSeparator,
  DropdownTrigger,
} from "@/components/ui/dropdown";
import { Sheet, SheetTrigger } from "@/components/ui/sheet";
import { DeleteDocumentDialog } from "@/components/delete-document-dialog";
import { api } from "@/api";
import { queryKeys } from "@/lib/queryKeys";
import { ShareSheet } from "./doc-share-sheet";
import { UploadVersionItem } from "./doc-upload-version";

export function DocBar({
  title,
  docId,
  current,
}: {
  title: string;
  docId: string;
  current: string;
}) {
  const nav = useNavigate();
  const q = useQueryClient();
  const del = useMutation({
    mutationFn: () => api.deleteDocument(docId),
    onSuccess: () => {
      toast.success("Document deleted.");
      q.invalidateQueries({ queryKey: queryKeys.documents() });
      nav("/app/documents");
    },
    onError: (e) =>
      toast.error(e instanceof Error ? e.message : "Delete failed"),
  });
  return (
    <div className="flex h-14 items-center gap-3 border-b border-[var(--color-border)] bg-[var(--color-surface)] px-4">
      <Button asChild variant="ghost" size="icon">
        <Link to="/app/documents" aria-label="Back to documents">
          <ArrowLeft className="h-4 w-4" />
        </Link>
      </Button>
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-semibold">{title}</div>
        <div className="truncate font-mono text-[11px] text-[var(--color-fg-muted)]">
          {docId}
        </div>
      </div>
      <Sheet>
        <SheetTrigger asChild>
          <Button variant="outline" size="sm">
            <Share2 className="h-4 w-4" /> Share
          </Button>
        </SheetTrigger>
        <ShareSheet docId={docId} />
      </Sheet>
      <Dropdown>
        <DropdownTrigger asChild>
          <Button variant="ghost" size="icon" aria-label="More">
            <MoreHorizontal className="h-4 w-4" />
          </Button>
        </DropdownTrigger>
        <DropdownContent>
          <DropdownItem
            onSelect={() =>
              window.open(`/v1/documents/${docId}/html`, "_blank")
            }
          >
            <ExternalLink className="h-4 w-4" /> Open raw HTML
          </DropdownItem>
          <UploadVersionItem docId={docId} baseVersion={current} />
          <DropdownSeparator />
          <DeleteDocumentDialog
            title={title}
            onConfirm={() => del.mutate()}
            trigger={
              <DropdownItem
                onSelect={(e) => e.preventDefault()}
                className="text-[var(--color-danger)] data-[highlighted]:bg-red-50 dark:data-[highlighted]:bg-red-950/40"
              >
                <Trash2 className="h-4 w-4" /> Delete document
              </DropdownItem>
            }
          />
        </DropdownContent>
      </Dropdown>
    </div>
  );
}
