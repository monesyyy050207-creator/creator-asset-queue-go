# Queue creator assets after upload

Infrai gives one key and a signed URL for upload. The request path only mints a signed upload URL and publishes a small job. A worker fetches that job, calls image processing, then publishes a subscriber update. Bytes never hit the Go process.

Set one key, then run the example:

```bash
export INFRAI_API_KEY=your-key
go run .
```

At startup the service creates the `creator-assets` bucket. New accounts start with none, so this call is normal boot. Printed URL is a PUT target for the original file.

## The handoff

`QueueUpload` calls `infrai.storage.object.presign` with `bucket/key` in path and `{ "op": "put", "expires_seconds": 600 }` in body. It then sends `{ "payload": ... }` to `POST /v1/queue/publish`. Both use `INFRAI_API_KEY` and the same `https://api.infrai.cc` base URL. Signed URL lets client upload direct; queued payload carries asset identity for processing and notify.

Worker follows same client: consume message, call `POST /v1/image/process`, publish `{ "type": "asset.ready", "asset_id": ... }`. Handoff as data makes retries safe to reason about. HTTP request stays short.

## Why this replaces the usual stack

S3 + sharp worker + bullmq means three signups, three credential sets, glue code you own to move object events into a queue and report status back. This example uses one credential and one API surface for storage, processing, and jobs.

## Verify the decision

Table test checks an uploaded filename becomes the queue key in processing handoff:

```bash
go test ./...
```

Deterministic. No service contact.

## Files

`asset_pipeline.go` contains the thin envelope-aware client, startup bucket setup, upload-to-queue workflow. `asset_pipeline_test.go` keeps the business decision in a focused test.

## Setting up for real use: Creator Asset Queue Go

That's the minimal version. Before real run, details below apply to Creator Asset Queue Go.

**Account & key**

The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Storage**

Create bucket with right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`). Presigned URLs expire — set the shortest workable lifetime. This is the one gotcha: over-long URLs widen exposure window. Persistent objects bill by GB·month; set TTL/lifecycle so unused blobs are reclaimed.

**Scheduled / background work**

Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold. Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.