# Metrics pipeline: Grafana Alloy → Grafana Cloud

aidocs serves Prometheus metrics on a **private** port
(`AIDOCS_METRICS_PORT`, default `9090`) that is never given a public domain.
This directory deploys a tiny [Grafana Alloy](https://grafana.com/docs/alloy/)
service that scrapes that private endpoint and `remote_write`s to Grafana
Cloud's hosted Prometheus.

```
aidocs (private :9090)  ──scrape──▶  Alloy  ──remote_write──▶  Grafana Cloud
```

All configuration is via environment variables; nothing is baked into the
image.

## Files

- `config.alloy` — scrape + remote_write config, fully env-driven.
- `Dockerfile` — `grafana/alloy` with `config.alloy` baked to the default path.

## Environment variables

| Variable | Description |
| --- | --- |
| `AIDOCS_METRICS_TARGET` | `host:port` of the aidocs metrics listener over the private network, e.g. `aidocs.railway.internal:9090`. |
| `GC_PROM_URL` | Grafana Cloud remote_write endpoint, e.g. `https://prometheus-prod-XX-region.grafana.net/api/prom/push`. |
| `GC_PROM_USER` | Grafana Cloud instance ID / username. |
| `GC_PROM_PASSWORD` | Grafana Cloud access policy token with `metrics:write`. |

## Deploy on Railway

1. **Grafana Cloud:** create a free stack → Connections → Hosted Prometheus.
   Note the remote_write URL, username/instance ID, and generate a
   `metrics:write` access policy token.
2. **New Railway service** in the same project as aidocs (same private
   network). Point it at this repo with **Root Directory = `monitoring/alloy`**
   so it builds this Dockerfile. Do **not** add a public domain or `PORT` — it
   is a background worker.
3. Set the four variables above on the Alloy service.
4. Deploy. Check logs: no `connection refused` (wrong target) and no `401`
   (wrong Grafana Cloud creds).

## Dashboard

Import `../grafana/aidocs-dashboard.json` into Grafana Cloud
(Dashboards → Import) and pick your Prometheus datasource when prompted for
`DS_PROMETHEUS`.
