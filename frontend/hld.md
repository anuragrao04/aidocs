# Web — High-Level Design

## What aidocs is

A review layer for AI-generated HTML documents. One document is one
self-contained HTML file. Humans review through a browser; AI agents
read and write documents and comments through a CLI/API.

The system has three components:

```
+-----------+        +-----------+        +-----------+
| frontend  | <----> |    api    | <----> |   cli     |
| (browser) |        | (server)  |        | (agent)   |
+-----------+        +-----------+        +-----------+
                         |
                   +-----------+
                   | postgres  |
                   |  + blob   |
                   +-----------+
```

`frontend` is the React SPA. It is **not a separate runtime**: at
build time `npm run build` produces a static `dist/` directory which
the `aidocs-server` Go binary embeds via `//go:embed`. At runtime
the Gin server in `api` serves these assets directly, alongside
`/v1/*`. There is no Node process in production. The folder exists
because the frontend is a meaningful body of code with its own
tooling, dependencies, and concerns, even though it ships inside the
backend binary.

## Responsibilities

The frontend is the browser-side application. It is the only place
humans interact with aidocs directly.

It is responsible for:

- Google OAuth sign-in flow (handing the user off to `api` for the
  actual token exchange).
- Listing documents the signed-in user can access, including the
  caller's role on each document.
- Uploading a single HTML file as a new document or a new version of
  an existing document. New document creation is human-only in V1;
  service accounts cannot own documents. New version upload requires
  `editor` or `owner` access.
- Rendering an uploaded HTML document inside a sandboxed iframe
  served from a separate render origin.
- Injecting the annotation bridge inside that iframe so users can
  select text and create anchored comments. The bridge does not
  invent its own anchoring algorithm; it uses
  [`@apache-annotator/dom`](https://github.com/apache/incubator-annotator)
  to capture and resolve W3C Web Annotation selectors
  (`TextQuoteSelector` + `TextPositionSelector`), and `mark.js` to
  paint highlights once a range has been resolved.
- Showing the comments UI: popup on selection, list of all comments,
  status (open / resolved / stale / orphaned), and a "copy for agent"
  view. Comment creation requires `commenter` or above; resolving or
  reopening requires comment author, document `editor`, or document
  `owner`.
- Calling `api` for all persistent state. The frontend owns no
  storage of its own beyond short-lived session cookies and local
  UI state.
- Managing document grants for owners: grant users or service accounts
  `viewer`, `commenter`, or `editor` access. Service-account ownership
  alone never grants document access.

It is not responsible for:

- Storing documents, versions, or comments.
- Verifying Google identity tokens.
- Running AI agents.
- Editing HTML on the server side.
- The CLI experience.

## Where it fits

The user opens the app in a browser, signs in with Google, uploads or
opens a document, and either reads it or comments on it. The frontend
is a thin client over `api` served from the same origin. The render
origin is logically part of the frontend's delivery path but uses a
separate subdomain so untrusted HTML cannot touch the main app's
session.

```
user --HTTPS--> frontend (app.aidocs)
                 |
                 +--iframe--> render origin (doc.aidocs)
                 |
                 +--fetch----> api (api.aidocs)
```

## Anchoring stack

The shape of `anchor_json` defined in `api/interface.md` is literally a
serialized pair of W3C Web Annotation selectors. The frontend is the
component that produces and consumes them in the browser.

- **Capture** (on selection): `describeTextQuote` and
  `describeTextPosition` from `@apache-annotator/dom`, run against the
  current `Range`, scoped to the document body. The two selectors
  together populate `quote`, `prefix`, `suffix`, `start_offset`,
  `end_offset`. `dom_path` is added as a cheap structural hint.
- **Resolve** (on load / new version): try the
  `TextPositionSelector` matcher first (exact, cheap), fall back to
  the `TextQuoteSelector` matcher (robust to DOM edits via
  prefix/suffix), and finally fuzzy-match. The first successful tier
  wins and sets `current_placement.confidence`.
- **Paint**: `mark.js` wraps the resolved range so the comment
  sidebar can scroll/highlight in sync.
- **Fallbacks**: `rangy` is used only where native
  `Selection`/`Range` APIs misbehave (older Safari, weird
  `contenteditable` cases).

Server-side reattachment, when added, runs the same
`@apache-annotator/dom` matchers on top of `linkedom` so the
algorithm stays identical across browser and server.

## Implementation stack

- **Framework:** Vite + React + TypeScript. No SSR. No Next.js.
  Everything is behind auth, SEO is irrelevant, and the rendered
  document lives in a sandboxed iframe rather than the React tree.
- **Routing:** `react-router` with client-side routes; the Go server
  falls back to `index.html` for unknown non-API paths so deep links
  work.
- **State / data:** `@tanstack/react-query` over the `api` REST
  endpoints, using same-origin `fetch` with credentials. No GraphQL,
  no custom data layer.
- **Styling:** Tailwind CSS, with a small component primitive set
  (Radix UI or shadcn/ui) for menus, dialogs, popovers.
- **Annotation:** `@apache-annotator/dom` for capture/resolve,
  `mark.js` for painting, `rangy` as a fallback (see Anchoring stack).
- **Build output:** `npm run build` produces a static `dist/`
  directory that the Go build embeds. The frontend has no awareness
  of which deployment it is in — it talks to `/v1/*` on its own
  origin, so the same build runs on `aidocs.anuragrao.dev` and
  `aidocs.razorpay.com` unchanged.
- **Render origin:** rendered HTML is loaded from a different
  hostname (`RENDER_ORIGIN`) into a sandboxed iframe. The SPA
  obtains a short-lived signed token from `api` and uses it to
  request the wrapper page from the render origin. The SPA itself
  never receives the raw user HTML.

## V1 scope

- Google OAuth login.
- Upload single-file HTML, max ~10 MB.
- Document list, document view, version history.
- Text-selection comments with popup composer.
- Comments sidebar/modal with statuses.
- Document sharing/grants UI (private/org/link visibility plus
  explicit user/service-account grants).
- No real-time collaboration. No WYSIWYG editing.
