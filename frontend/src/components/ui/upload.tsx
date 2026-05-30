import * as React from "react";
import { useState } from "react";

// Shared HTML-file picking primitives so the backup-document and
// new-version upload forms don't duplicate file state + markup (web-09).
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
