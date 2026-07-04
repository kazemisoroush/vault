# Vault Frontend

Next.js (App Router, TypeScript) single-page app, exported as a static site and
served from S3 behind CloudFront. It is a pure API client: it talks only to the
endpoints in [openapi.yaml](../openapi.yaml), never to S3 or DynamoDB directly.
File bytes move through the presigned URLs the API hands out.

## Layout

- `app/`: routes `/login` (Cognito sign in) and `/` (the auth-guarded home).
  The home both asks and drops: an ask box for natural-language retrieval on
  top, and a drop surface below where a dragged file uploads and lists with its
  status.
- `components/`: the presentational pieces, an `AskBox` and `Results` for the
  read side, and a `DropZone` and `FileList` for the write side.
- `lib/config.ts`: the config layer. It reads `/config.json` at runtime, so the
  static build carries no environment-specific values.
- `lib/api/`: the typed API client. `schema.ts` is generated from
  `openapi.yaml`, and `client.ts` wraps it and attaches the bearer token.
- `lib/auth/`: the Cognito boundary and the React auth context.
- `lib/files/`: the file operations, `dropFile` (register then upload), the
  presigned `upload` PUT, and `listFiles`.
- `lib/ask/`: the natural-language retrieval call, `ask`, over the typed client.

## Config

The app fetches `/config.json` at startup:

```json
{
  "apiUrl": "https://<api>.execute-api.<region>.amazonaws.com",
  "cognitoUserPoolId": "<pool id>",
  "cognitoClientId": "<client id>"
}
```

The CDK deploy writes this file into the bucket from the stack outputs, so the
UI and backend never drift.

## Commands

```bash
npm install
npm run generate:api   # regenerate lib/api/schema.ts from ../openapi.yaml
npm run typecheck
npm run lint
npm run test
npm run build          # static export into out/
```
