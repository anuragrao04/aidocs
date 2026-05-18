# aidocs Grafana dashboard

This directory contains a portable Prometheus/Grafana dashboard for aidocs app and business metrics.

- `aidocs-dashboard.libsonnet` is the Grafonnet source of truth.
- `aidocs-dashboard.json` is the rendered dashboard for direct Grafana import or provisioning.
- `jsonnetfile.json` and `jsonnetfile.lock.json` pin Grafonnet dependencies.
- `vendor/` is intentionally ignored; regenerate it with `jb install`.

Render locally:

```bash
make dashboards
```

Verify the committed JSON is up to date:

```bash
make check-dashboards
```

CI runs `make check-dashboards` and fails if the rendered JSON differs from the committed JSON.

The dashboard uses a datasource variable named `DS_PROMETHEUS`, so it should work across internal, hosted, and self-hosted deployments after selecting the right Prometheus datasource in Grafana.
