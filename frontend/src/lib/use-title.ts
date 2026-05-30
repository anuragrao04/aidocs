import { useEffect } from "react";

const BASE = "aidocs";

// Sets the browser tab title to "<title> · aidocs" (or just "aidocs" when no
// page title is given), restoring the previous title on unmount so tabs stay
// distinguishable as the user navigates.
export function useTitle(title?: string) {
  useEffect(() => {
    const prev = document.title;
    document.title = title ? `${title} · ${BASE}` : BASE;
    return () => {
      document.title = prev;
    };
  }, [title]);
}
