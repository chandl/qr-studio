# qrcode

A lightweight, self-hostable QR code generation service in Go. Generates
SVG, PNG, and JPEG QR codes, usable as a public HTTP API or embedded
directly in an `<img src>` tag, with CDN-friendly cache headers.

Rendering is built on [`yeqown/go-qrcode/v2`](https://github.com/yeqown/go-qrcode)
(MIT licensed) for matrix generation, encoding, masking, and raster (PNG/JPEG)
drawing — including custom module shapes. There is no SVG
writer in that library, so SVG output is produced by a small hand-rolled
writer in [`internal/qr/svg.go`](internal/qr/svg.go) that walks the same
QR matrix.

## Quick start

```bash
go run ./cmd/server
# or
docker compose up --build
```

Then open http://localhost:8080 for the demo UI, or:

```bash
curl "http://localhost:8080/qr?data=https://example.com&shape=rounded" -o qr.png
```

## API

### `GET /qr`

The primary endpoint — cache-friendly and meant to be used directly as an
`<img src="">` target.

| Param     | Default | Description |
|-----------|---------|-------------|
| `data`    | *(required)* | URL-encoded content to encode. Capped at 2000 characters on GET (a practical URL-length ceiling) — use `POST /qr` for longer payloads. |
| `format`  | `png`   | `svg` \| `png` \| `jpeg` |
| `size`    | ~auto   | Approximate output width/height in pixels (square). Capped at 4096. |
| `ecl`     | `M`     | Error correction level: `L` (7%) \| `M` (15%) \| `Q` (25%) \| `H` (30%) |
| `color`   | `#000000` | Foreground hex color |
| `bgcolor` | `#ffffff` | Background hex color |
| `shape`   | `square` | Module shape: `square` \| `rounded` \| `circle` \| `diamond` |
| `margin`  | `2`     | Quiet zone (border) width in modules, `0`–`10`. The QR spec recommends 4; smaller values look tighter but may scan less reliably. |

Returns raw image bytes with the correct `Content-Type` and
`Cache-Control: public, max-age=31536000, immutable` — output is fully
deterministic per parameter set, so it's safe to cache forever at the edge.
If `data` exceeds the QR code's capacity for the chosen error correction
level, the request fails with `400` and a clear message.

If the requested foreground/background colors are low-contrast enough to
risk scan reliability, the response still succeeds but carries an
`X-QR-Contrast-Warning` header.

### `POST /qr`

Escape hatch for long payloads. Same fields as the `GET` query params, as a
JSON body:

```json
{
  "data": "…very long payload…",
  "format": "png",
  "size": 512,
  "ecl": "M",
  "color": "#000000",
  "bgcolor": "#ffffff",
  "shape": "circle",
  "margin": 2
}
```

Response is the **raw image bytes** (not JSON/base64), same as `GET`. POST
responses are not cache-controlled — POST isn't cacheable, and long,
one-off payloads are unlikely to repeat.

### `GET /healthz`

Returns `200 ok`. Used by the Docker healthcheck.

## Non-functional behavior

- **Quiet zone**: a quiet zone (2 modules by default, configurable via `margin`) is enforced on every
  output (raster and SVG) regardless of requested size, for scan
  reliability.
- **Contrast check**: `GET`/`POST /qr` responses carry `X-QR-Contrast-Warning`
  when the requested fg/bg colors are too close (WCAG relative-luminance
  ratio below 3.0).
- **No app-level caching**: the service does not cache generated images
  itself — `GET /qr` is deterministic and sets long-lived, immutable cache
  headers so a CDN in front of it can cache aggressively.

## Project layout

```
cmd/server/          entrypoint, HTTP server wiring
internal/qr/          QR generation/rendering (library wrapper, shapes, SVG writer, contrast check)
internal/httpapi/      HTTP handlers for GET/POST /qr, param validation
web/                   embedded single-page demo UI (go:embed)
```

The demo UI at `/` calls `GET /qr` directly with no separate backend logic —
it's a working example of consuming the public API.

## Testing

```bash
go test ./...
```

Covers QR generation across all format/shape/ECL combinations, parameter
validation (bad colors, oversized data, invalid enums), contrast-warning
detection, and the HTTP handlers (including body-size
limits).

## Deployment

Ships as a single static binary (no CGO, no external state — there's no
database in this MVP). The included `Dockerfile` builds a small Alpine
image running as a non-root user with a healthcheck:

```bash
docker compose up --build -d
```

Configure the listen address with the `QR_ADDR` environment variable
(default `:8080`).

## Scope notes

This build intentionally omits the async batch-job endpoints
(`POST /batch`, `GET /jobs/{id}`) and their SQLite-backed job store from the
original spec, to keep the MVP a single stateless binary. They can be added
later as a separate worker + store without touching `internal/qr` or
`internal/httpapi`.
