import { Bot, ExternalLink, KeyRound, Rocket } from "lucide-react";
import { Link } from "react-router-dom";
import { Button } from "@/components/ui/button";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card";
import { CodeBlock } from "@/components/ui/misc";

export function DevelopersPage() {
  return (
    <div className="mx-auto max-w-4xl px-6 py-10">
      <div className="mb-6">
        <h1 className="text-2xl font-semibold tracking-tight">Developers</h1>
        <p className="mt-1 text-sm text-[var(--color-fg-muted)]">
          API reference and authentication model for building against
          aidocs directly.
        </p>
      </div>

      <div className="space-y-4">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Rocket className="h-4 w-4" /> Looking to set up your agent?
            </CardTitle>
            <CardDescription>
              The CLI install and agent skill setup live in the setup
              guide, not here.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Button asChild>
              <Link to="/app/start">
                Open setup guide <ExternalLink className="h-3.5 w-3.5" />
              </Link>
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <KeyRound className="h-4 w-4" /> Authentication model
            </CardTitle>
            <CardDescription>
              Two ways to authenticate against the API.
            </CardDescription>
          </CardHeader>
          <CardContent className="space-y-4">
            <div>
              <div className="text-sm font-medium">User session</div>
              <p className="mt-1 text-xs text-[var(--color-fg-muted)]">
                Google OAuth via the browser. The CLI uses this when you
                run <code className="font-mono">aidocs auth login</code>.
                Best for interactive use and any agent running locally on
                your machine.
              </p>
            </div>
            <div>
              <div className="text-sm font-medium">Service account token</div>
              <p className="mt-1 text-xs text-[var(--color-fg-muted)]">
                Bearer token for headless agents that can't run OAuth —
                n8n, scheduled jobs, CI. Mint a key under{" "}
                <Link
                  className="font-medium text-[var(--color-fg)] underline-offset-2 hover:underline"
                  to="/app/settings/service-accounts"
                >
                  Service accounts
                </Link>{" "}
                and pass it as{" "}
                <code className="font-mono">Authorization: Bearer …</code>.
              </p>
            </div>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <Bot className="h-4 w-4" /> Service accounts
            </CardTitle>
            <CardDescription>
              Headless identities for agents that can’t run OAuth (n8n,
              CI, scheduled jobs). Mint bearer tokens here. Most users
              should ignore this — the CLI handles auth automatically
              when an agent runs it on your machine.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <Button asChild variant="outline">
              <Link to="/app/settings/service-accounts">
                Manage service accounts
              </Link>
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>API reference</CardTitle>
            <CardDescription>OpenAPI spec and Swagger UI.</CardDescription>
          </CardHeader>
          <CardContent className="flex gap-2">
            <Button asChild variant="outline">
              <a href="/api-docs" target="_blank" rel="noreferrer">
                <ExternalLink className="h-4 w-4" /> Swagger UI
              </a>
            </Button>
            <Button asChild variant="outline">
              <a href="/openapi.json" target="_blank" rel="noreferrer">
                <ExternalLink className="h-4 w-4" /> openapi.json
              </a>
            </Button>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>Example: raw HTTP</CardTitle>
            <CardDescription>
              List documents using a service-account bearer token.
            </CardDescription>
          </CardHeader>
          <CardContent>
            <CodeBlock>{`curl -H "Authorization: Bearer <YOUR_KEY>" \\
  ${typeof window !== "undefined" ? window.location.origin : "https://your-host"}/v1/documents`}</CodeBlock>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
