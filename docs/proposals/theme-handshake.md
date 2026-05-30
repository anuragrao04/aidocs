# Feature request: expose the aidocs web theme to the user document

## Problem

Today the aidocs web app has a light/dark theme. So do many user HTML
documents — anything authored with a real stylesheet typically defines its
own palette and often a toggle. These two themes are completely **decoupled**:

- Toggling theme in the aidocs chrome (top bar, side panels, comment rail)
  does not affect the rendered document in the iframe.
- A theme toggle inside the user's HTML does not affect the aidocs chrome.
- The user document has no way to know what theme the reader picked in the
  aidocs UI, so it can't even make a one-time decision on load.

Result: most documents render in their own default (commonly dark) regardless
of the reader's app-level preference, and the seam between chrome and content
is visually jarring — a light-themed app frame around a dark document, or
vice versa. Authors who care end up shipping their own toggle, which
duplicates a control the host already provides and confuses readers (now
there are two toggles that don't agree).

## What I want as an author

I want to write a document that automatically adopts the reader's chosen
aidocs theme, and updates if the reader changes it, without writing any
postMessage plumbing of my own.

In practice I'd like one of:

- A CSS hook I can target — e.g. `:root[data-aidocs-theme="light"]`
  / `[data-aidocs-theme="dark"]` set on `<html>` inside the iframe by the
  render bridge, updated live.
- And/or a CSS custom-media or `prefers-color-scheme` simulation forwarded
  to the iframe, so existing `@media (prefers-color-scheme: ...)` rules
  start working.
- And/or a documented event I can listen to:
  `window.addEventListener('aidocs:theme', e => ...)` carrying
  `{ theme: 'light' | 'dark' | 'auto' }`.

The first option is the simplest and covers ~all real authoring cases.

## What I want as a reader

- One toggle. The aidocs app theme button is the single source of truth.
- Documents that explicitly opt into theming follow it.
- Documents that don't opt in keep their hard-coded look (no surprise
  re-styling of unrelated docs).

## Proposed contract — minimal version

Inside the render iframe, the aidocs render bridge sets and keeps in sync:

```html
<html data-aidocs-theme="light"  data-aidocs-color-scheme="light">
<html data-aidocs-theme="dark"   data-aidocs-color-scheme="dark">
```

Update rules:

1. On iframe load, the bridge reads the parent's current theme and writes
   the attribute before the document's `DOMContentLoaded` fires (so initial
   paint can be theme-aware, no flash).
2. When the parent app theme changes, the bridge updates the attribute and
   fires a `aidocs:theme` `CustomEvent` on the iframe `window`.
3. If the reader has the parent set to "auto" / system, the bridge resolves
   that to a concrete `light` / `dark` before writing the attribute, and
   re-resolves on `matchMedia('(prefers-color-scheme: dark)').change`.

That's the whole API surface from the user-HTML side.

## Author-side usage example

A document opts in with two lines of CSS:

```css
:root[data-aidocs-theme="light"] { --bg: #fff; --ink: #111; }
:root[data-aidocs-theme="dark"]  { --bg: #0b0d12; --ink: #e7ecf3; }
body { background: var(--bg); color: var(--ink); }
```

A document with a `prefers-color-scheme` stylesheet keeps working as-is, as
long as the bridge also forwards the resolved scheme to the iframe's
`prefers-color-scheme` (option B). For the minimal version above, authors
who want both can do:

```css
@media (prefers-color-scheme: dark) { :root { color-scheme: dark; } }
:root[data-aidocs-theme="dark"]     { color-scheme: dark; }
```

A document that wants to **ignore** the host theme just doesn't reference
`data-aidocs-theme` — backward compatible by default.

A document that wants its **own** in-doc toggle to also drive the host can
post back to the parent:

```js
parent.postMessage({ type: 'aidocs:set-theme', theme: 'light' },
                   /* set by render bridge */ window.__AIDOCS_APP_ORIGIN__);
```

The app may accept or ignore this (probably a setting — "let documents
change my theme: never / ask / always"). MVP can omit this direction.

## Where it lives in the code

The render bridge already exists at
`api/internal/server/server.go` (`renderBridgeJS` constant + the `<script>`
appended after the iframe). All the new behaviour fits inside that bridge:

1. Read the app theme on bridge boot — either via a query parameter on the
   render URL set by the React app, or via an initial `postMessage` the
   React app already sends, or via a `prefers-color-scheme` snapshot if
   nothing else is plumbed yet.
2. Set `data-aidocs-theme` on `<html>` inside the iframe.
3. Add a `message` listener for `aidocs:set-theme` and `aidocs:theme`
   notifications from the parent (the React app emits one whenever its
   own theme toggles).
4. Dispatch the `aidocs:theme` event on the iframe `window` for authors
   who want JS notifications.

Server-side change is small; the React side needs to broadcast its theme
on mount and on change.

## Non-goals

- Forcing a theme onto documents that haven't opted in. The data attribute
  is silently ignored by un-themed docs.
- A general "design token" handoff. Just the theme axis, MVP.
- Cross-document persistence of an in-document theme override. The host
  toggle stays canonical.

## Tests to add alongside the change

- Bridge sets `data-aidocs-theme` to the value provided by the parent
  before any content scripts run (no flash of wrong theme).
- Bridge updates the attribute and fires the event when the parent posts
  a new theme.
- `auto` resolves to `light` / `dark` and re-resolves on system change.
- Documents that don't reference the attribute are byte-identical to
  today's render (backward compatibility).

## Why now

The first long-form documents shipped on aidocs already had to bake in
their own theme toggle (see e.g. the agentic-observability writeup) which
ends up sitting awkwardly next to the host's own toggle. A reader who
opens the doc in light-mode aidocs still lands on a dark document and has
to discover and click a second button. Wiring this once at the platform
keeps every future doc honest with no per-author work beyond two CSS
selectors.
