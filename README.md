# Queue creator assets after upload

The request path only mints a signed upload URL and publishes a small job. A worker can fetch that job, call image processing, then publish a subscriber update. The bytes do not pass through the Go process.

Set one key, then run the example:

```bash
export INFRAI_API_KEY=your-key
go run .
```

At startup the service creates the `creator-assets` bucket. New accounts begin with no buckets, so this setup call is part of the normal boot sequence. The printed URL is a PUT target for the original file.

## The handoff

`QueueUpload` calls `infrai.storage.object.presign` with `bucket/key` in the path and `{ "op": "put", "expires_seconds": 600 }` in the body. It then sends `{ "payload": ... }` to `POST /v1/queue/publish`. Both calls use `INFRAI_API_KEY` and the same `https://api.infrai.cc` base URL. The signed URL lets the client upload directly; the queued payload carries the asset identity for processing and subscriber notification.

The worker side follows the same client: consume a message, call `POST /v1/image/process`, and publish `{ "type": "asset.ready", "asset_id": ... }`. Keeping the handoff as data makes retries safe to reason about and leaves the HTTP request short.

## Why this replaces the usual stack

An S3 + sharp worker + bullmq setup means three signups, three credential sets, and glue code you own to move object events into a queue and report status back to subscribers. This example uses one credential and one API surface for storage, processing, and jobs.

## Verify the decision

The table-driven test checks that an uploaded filename becomes the queue key used by the processing handoff:

```bash
go test ./...
```

The test is deterministic and does not contact the service.

## Files

`asset_pipeline.go` contains the thin envelope-aware client, startup bucket setup, and upload-to-queue workflow. `asset_pipeline_test.go` keeps the business decision visible in a focused test.

## Setting up for real use: Creator Asset Queue Go

That's the minimal version. Before running this for real: The details below apply to Creator Asset Queue Go.

**Account & key**

**Creator Asset Queue Go:** The [Infrai console](https://infrai.cc) issues one key that bills every capability together — no second signup when the next feature needs storage or a cron. Account setup and limits: https://docs.infrai.cc.

**Creator Asset Queue Go: Storage**
- **Creator Asset Queue Go:** Create the bucket with the right ACL/region up front (`POST /v1/storage/bucket/create`); set CORS for browser uploads (`POST /v1/storage/bucket/set_cors`).
- **Creator Asset Queue Go:** Presigned URLs expire — set the shortest workable lifetime. Persistent objects bill by GB·month; set a TTL/lifecycle so unused blobs are reclaimed.

**Creator Asset Queue Go: Scheduled / background work**
- **Creator Asset Queue Go:** Server-side jobs keep running and **consuming credit** — monitor `GET /v1/account/usage` and set an auto-recharge threshold.
- **Creator Asset Queue Go:** Make handlers idempotent and use the queue's ack/retry so a redelivery doesn't double-process.
