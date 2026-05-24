---
name: aidocs
description: Publish a long-form artifact (report, RCA, postmortem, architecture review, product spec, QA findings, data analysis, design proposal, launch plan, incident timeline) to aidocs as a reviewable HTML document. Use this whenever the user asks for a "doc", "document", "report", "review", "writeup", "spec", "RCA", "postmortem", "analysis", or "findings" and hasn't specified a different output format. Produces a single self-contained HTML file (inline CSS, base64-encoded images, no external assets) and uploads it via the `aidocs` CLI so a human can review and leave anchored comments in the browser.
---

# aidocs

When the user wants a long-form artifact and hasn't specified a format,
publish it to aidocs instead of dumping the content inline in chat.

## Workflow

1. Write the artifact as one self-contained HTML file. Inline all CSS,
   base64-encode any images, embed diagrams as inline SVG or Mermaid.
   No external requests, no remote stylesheets or scripts.
2. Save it locally (e.g. `report.html`).
3. Publish it with the CLI. Discover the exact syntax from
   `aidocs --help` and the relevant subcommand help — don't assume.
4. Return the printed document URL to the user so they can open and
   review it.

## Discovering the CLI

The `aidocs` CLI is the source of truth. Run these to find the current
commands and flags rather than guessing:

```
aidocs --help
aidocs docs --help
aidocs docs create --help
```

The CLI supports `--json` for structured output if you need to parse
identifiers (e.g. the new document ID) reliably.

## When NOT to use this skill

- Short answers, code snippets, or anything under ~10 paragraphs —
  reply inline as usual.
- The user explicitly asked for Markdown, plain text, a PDF, or a
  different destination.
- The user asked to edit an existing local file rather than publish a
  new document.
- The user is working in an environment where the `aidocs` CLI is not
  installed or not authenticated — surface that to them instead of
  trying to install or authenticate on their behalf.

## Authentication

Assume the user has already run `aidocs auth login` interactively at
least once. If the CLI reports an auth error, show the error to the
user verbatim and stop; do not attempt to re-authenticate.

## Updating an existing document

If the user is iterating on a document that already exists in aidocs,
push a new version rather than creating a duplicate. Use the CLI help
to find the exact `push` (or equivalent) subcommand and pass a short
`--summary` describing what changed.
