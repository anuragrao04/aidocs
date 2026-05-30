// Shared domain constants for comment statuses, roles, and visibility values.
export const COMMENT_STATUS = {
  open: "open",
  resolved: "resolved",
} as const;
export type CommentStatus =
  (typeof COMMENT_STATUS)[keyof typeof COMMENT_STATUS];

export const COMMENT_FILTERS = ["all", "open", "resolved"] as const;
export type CommentFilter = (typeof COMMENT_FILTERS)[number];
export const COMMENT_FILTER_ALL: CommentFilter = "all";
export const DEFAULT_COMMENT_FILTER: CommentFilter = "open";

// Placement status of a comment relative to the rendered version.
export const PLACEMENT_STATUS = {
  attached: "attached",
  orphaned: "orphaned",
} as const;

export const ROLES = [
  { value: "viewer", label: "Viewer" },
  { value: "commenter", label: "Commenter" },
  { value: "editor", label: "Editor" },
] as const;
export const DEFAULT_ROLE = "commenter";

export const VISIBILITIES = [
  { value: "private", label: "Private" },
  { value: "org", label: "Org visible" },
  { value: "link", label: "Anyone with link" },
] as const;
export const DEFAULT_VISIBILITY = "private";

// Bot addresses end in `.bot`. The server enforces this; the client uses it
// only for inline hints.
export const BOT_DOMAIN_SUFFIX = ".bot";
