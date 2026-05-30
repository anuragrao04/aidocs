// Shared domain constants so statuses, roles, and visibility values are not
// re-typed as magic strings across the app (web-16, web-18).
export const COMMENT_STATUS = {
  open: "open",
  resolved: "resolved",
} as const;
export type CommentStatus =
  (typeof COMMENT_STATUS)[keyof typeof COMMENT_STATUS];

export const COMMENT_FILTERS = ["all", "open", "resolved"] as const;
export type CommentFilter = (typeof COMMENT_FILTERS)[number];

export const ROLES = [
  { value: "viewer", label: "Viewer" },
  { value: "commenter", label: "Commenter" },
  { value: "editor", label: "Editor" },
] as const;

export const VISIBILITIES = [
  { value: "private", label: "Private" },
  { value: "org", label: "Org visible" },
  { value: "link", label: "Anyone with link" },
] as const;

// Mirrors the server-side rule that bot addresses end in `.bot`. The server is
// authoritative; this is only used for inline client hints (web-18).
export const BOT_DOMAIN_SUFFIX = ".bot";
