# Vault

Personal data vault on pure AWS. Drop any file and get it back almost instantly by
free-form phrase. No folders, no tags, no enums.

## API first

[openapi.yaml](openapi.yaml) is the source of truth. The backend implements it, every
client consumes it, and nothing talks to storage directly. The API has five file
verbs: drop, get, list, update, delete, an ask endpoint for natural-language
retrieval, a calls endpoint for the recent LLM trace, plus a health check for liveness.

## Layout

| Directory | What it is |
|---|---|
| [backend/](backend/) | Go Lambda implementing the API |
| [infra/](infra/) | CDK app in Go: S3 bucket, DynamoDB tables, S3 Vectors index, Lambda, HTTP API |
| [frontend/](frontend/) | Next.js client, lands at milestone M3 |

## Architecture

- File bytes go straight to S3 through presigned PUT and GET URLs. The Lambda never
  carries file content.
- DynamoDB holds one record per file, the record of truth for listing and CRUD.
- Search is semantic: each file is embedded on drop and its vector stored in Amazon
  S3 Vectors, keyed by file id. An ask embeds the query, pulls the nearest files from
  S3 Vectors, and lets the model re-rank that shortlist, so cost stays constant as the
  vault grows. The model also returns a short answer drawn from the matched files'
  metadata (for example a passport number), with the source file it came from.
- S3 objects transition to Intelligent-Tiering immediately, so storage cost drifts
  down on its own.
- From milestone M2, an S3 event Lambda extracts free-form metadata from each dropped
  file with an LLM (office documents are decoded to text first), and also captures the
  file's own embedded metadata: image EXIF and office document properties.

## Commands

```sh
make build   # build backend and infra
make test    # run backend tests
make lint    # golangci-lint on backend
make mock    # regenerate mocks
make synth   # cdk synth
make deploy  # cdk deploy
```

Local development server:

```sh
cd backend && VAULT_TABLE=... VAULT_BUCKET=... make run
```
