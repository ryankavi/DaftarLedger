# Deployment

Target architecture for hosting PayClone on AWS. This captures decisions made
during design; nothing here is automated yet (no IaC / scripts).

## Architecture

One CloudFront distribution fronts **two origins**, routed by path. The browser
only ever talks to the CloudFront domain, so the app is **same-origin** — there
is deliberately **no CORS** in the backend (a single-distribution path-routing
setup is what let us remove it).

```
                          ┌────────────────────┐
        Browser  ───────► │     CloudFront     │  one distribution, HTTPS
                          └─────────┬──────────┘
                    /api/*  ────────┤────────  /* (default)
                          │                   │
                   ┌──────▼──────┐     ┌───────▼────────┐
                   │     EC2     │     │       S3       │
                   │  Go binary  │     │  React build   │  (static assets)
                   │  (port 8080)│     │   (dist/)      │
                   └──────┬──────┘     └────────────────┘
                          │
                   ┌──────▼──────┐
                   │  Postgres   │   Docker on the EC2 box, or RDS
                   └─────────────┘
```

- **Frontend** — `vite build` → upload `dist/` to an **S3** bucket. Served as
  the CloudFront default (`/*`) behavior, with normal CDN caching.
- **Backend** — the Go binary on a single **EC2** instance, listening on `:8080`.
  Served by CloudFront's `/api/*` behavior. The server already mounts every
  route under `/api` (`s.mux.Handle("/api/", http.StripPrefix("/api", api))`),
  so CloudFront forwards `/api/*` unchanged and the handlers see clean paths.
- **Database** — Postgres. Cheapest is a Docker container on the same EC2 box
  (mirrors local dev); RDS is the managed alternative. **Decision open.**

## CloudFront `/api/*` behavior — required non-default settings

CloudFront is a CDN first, so its defaults assume cacheable reads and **will
silently break a write API** unless overridden on the `/api/*` behavior:

| Setting | Default (breaks it) | Required | Symptom if wrong |
|---|---|---|---|
| Allowed HTTP methods | GET, HEAD | **All** (incl. POST) | every POST → `403` at CloudFront |
| Origin request policy | minimal headers | **AllViewer** (forwards `Authorization`) | protected routes → `401` |
| Cache policy | caches responses | **CachingDisabled** | per-user/API responses served to the wrong caller |

The default `/*` behavior (S3) keeps normal caching — you *want* the static
frontend cached.

## TLS / HTTPS

CloudFront serves the frontend over **HTTPS**, and a browser on an HTTPS page
**refuses to call an HTTP API** (mixed content). So the backend must also be
HTTPS — there is no clean free HTTPS on a bare EC2 IP, which means a **domain +
certificate** is required. Two ways:

- **Caddy (or nginx) on the EC2 box** + a domain → auto Let's Encrypt certs.
  Leanest for a single box; **recommended** for this setup.
- **ALB + ACM certificate** in front of EC2 → managed TLS, but ~$16/mo and only
  worth it if scaling to more than one instance later.

Either way CloudFront's `/api/*` origin should point at the HTTPS endpoint.

## Production environment variables

Set these on the EC2 host (e.g. a systemd `EnvironmentFile`, or exported in the
service unit) — real env vars take precedence over `.env`, which is gitignored.
Keys are documented in `.env.example`.

| Var | Notes |
|---|---|
| `JWT_SECRET` | **Generate a real secret** (e.g. `openssl rand -base64 48`) — NOT the dev value. Server fails fast if unset. |
| `DATABASE_URL` | Points at the Postgres location (local container or RDS endpoint). |
| `BOOTSTRAP_ADMIN_EMAIL` / `BOOTSTRAP_ADMIN_PASSWORD` | Set both; ensures an admin idempotently on boot. The only way an admin is created. |

## Rough deploy flow (manual, for now)

1. **DB**: start Postgres (Docker on the box) or provision RDS; set `DATABASE_URL`.
2. **Backend**: build (`go build -o payclone ./main.go`), copy to EC2, run under
   systemd with the env vars above; front with Caddy (TLS + a domain).
3. **Frontend**: `vite build`; sync `dist/` to the S3 bucket.
4. **CloudFront**: one distribution, two origins (S3 + EC2); default `/*`
   behavior → S3, `/api/*` behavior → EC2 with the three settings above.
5. Set `BOOTSTRAP_ADMIN_*`, boot the server (admin is ensured), then
   `POST /api/login` to get an admin token and drive the demo.

## Open decisions

- **Postgres**: Docker-on-box (cheapest) vs RDS (managed).
- **TLS**: Caddy-on-box (recommended, ~$0) vs ALB + ACM (~$16/mo).
- **Domain**: required for HTTPS; not yet chosen.

## Cost note

For an always-on demo, a single small EC2 (`t4g.micro`/`nano`, possibly
free-tier) + Postgres-in-Docker + Caddy + S3/CloudFront is ~$3–6/mo. Fargate was
considered and rejected: not cheaper for always-on once you add an ALB
(~$16/mo) and possibly a NAT gateway (~$32/mo).
