import * as React from "react";
import { useState } from "react";

// HTML-file picking primitives shared by the backup-document and new-version
// upload forms.
export function useStagedFile() {
  const [file, setFile] = useState<File | null>(null);
  return { file, setFile, reset: () => setFile(null) };
}

export function HtmlFileInput({
  onFile,
  className,
  ...rest
}: Omit<React.InputHTMLAttributes<HTMLInputElement>, "type" | "onChange"> & {
  onFile: (f: File | null) => void;
}) {
  return (
    <input
      type="file"
      accept=".html,text/html"
      onChange={(e) => onFile(e.target.files?.[0] || null)}
      className={className ?? "text-sm"}
      {...rest}
    />
  );
}
