import { useCallback, useEffect, useMemo, useRef } from "react";
import { useQuery } from "@tanstack/react-query";
import { api, type Anchor } from "@/api";
import { queryKeys } from "@/lib/queryKeys";
import { Center } from "@/components/ui/misc";
import { useDoc } from "./doc-context";

// Renders the sandboxed document iframe and exchanges selection and paint
// messages with the in-iframe bridge. Message origins are validated on both
// ends, and painting is driven by the bridge's ready signal.
export function RenderFrame() {
  const { version, comments, activeComment, setSelection } = useDoc();
  const ref = useRef<HTMLIFrameElement>(null);
  const readyRef = useRef(false);

  const token = useQuery({
    queryKey: queryKeys.render(version),
    queryFn: () => api.renderToken(version),
    staleTime: 4 * 60_000,
    enabled: !!version,
  });

  const paintPayload = useMemo(
    () =>
      comments.map((c) => ({
        id: c.id,
        quote: c.selected_text || c.anchor?.quote,
      })),
    [comments],
  );

  const frameOrigin = useMemo(() => {
    if (!token.data?.url) return null;
    try {
      return new URL(token.data.url, window.location.href).origin;
    } catch {
      return null;
    }
  }, [token.data]);

  // Reset readiness whenever we load a different frame.
  useEffect(() => {
    readyRef.current = false;
  }, [frameOrigin, token.data?.url]);

  const paint = useCallback(() => {
    if (!readyRef.current || !frameOrigin) return;
    ref.current?.contentWindow?.postMessage(
      { type: "aidocs:paint", comments: paintPayload, active: activeComment },
      frameOrigin,
    );
  }, [paintPayload, activeComment, frameOrigin]);

  useEffect(() => {
    const onMsg = (e: MessageEvent) => {
      if (!frameOrigin || e.origin !== frameOrigin) return;
      if (e.data?.type === "aidocs:selection") {
        const anchor = e.data.anchor as Partial<Anchor> | undefined;
        const quote = anchor?.quote ?? e.data.quote;
        if (quote) setSelection({ quote, anchor });
      }
      if (e.data?.type === "aidocs:ready") {
        readyRef.current = true;
        paint();
      }
    };
    window.addEventListener("message", onMsg);
    return () => window.removeEventListener("message", onMsg);
  }, [frameOrigin, paint, setSelection]);

  // Push fresh paint data once the frame is ready and whenever it changes.
  useEffect(() => {
    paint();
  }, [paint]);

  if (!version) return <Center>No version available.</Center>;
  if (token.isLoading) return <Center>Preparing secure render…</Center>;
  if (token.error) return <Center>Could not create render token.</Center>;
  return (
    <iframe
      ref={ref}
      title="Rendered document"
      className="h-full w-full bg-white"
      src={token.data!.url}
      sandbox="allow-scripts allow-same-origin"
    />
  );
}
