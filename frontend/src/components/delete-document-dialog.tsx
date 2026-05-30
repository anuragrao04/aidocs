import * as React from "react";
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog";

// Shared delete-document confirmation, used from the documents table row and
// the document toolbar dropdown (web-08).
export function DeleteDocumentDialog({
  title,
  trigger,
  onConfirm,
}: {
  title: string;
  trigger: React.ReactNode;
  onConfirm: () => void;
}) {
  return (
    <AlertDialog>
      <AlertDialogTrigger asChild>{trigger}</AlertDialogTrigger>
      <AlertDialogContent>
        <AlertDialogTitle>Delete “{title}”?</AlertDialogTitle>
        <AlertDialogDescription>
          This permanently removes the document, all versions, and all
          comments. This cannot be undone.
        </AlertDialogDescription>
        <AlertDialogFooter>
          <AlertDialogCancel>Cancel</AlertDialogCancel>
          <AlertDialogAction variant="danger" onClick={onConfirm}>
            Delete
          </AlertDialogAction>
        </AlertDialogFooter>
      </AlertDialogContent>
    </AlertDialog>
  );
}
