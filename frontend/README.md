# Vault Frontend

Next.js (App Router, TypeScript) single-page app, exported as a static site and
served from S3 behind CloudFront. It is a pure API client: it talks only to the
endpoints in [openapi.yaml](../openapi.yaml), never to S3 or DynamoDB directly.
File bytes move through the presigned URLs the API hands out.

## Layout

- `app/`: routes `/login` (Cognito sign in) and `/` (auth-guarded shell). No
  product screens yet; the ask box lands in the next issue.
- `lib/config.ts`: the config layer. It reads `/config.json` at runtime, so the
  static build carries no environment-specific values.
- `lib/api/`: the typed API client. `schema.ts` is generated from
  `openapi.yaml`, and `client.ts` wraps it and attaches the bearer token.
- `lib/auth/`: the Cognito boundary and the React auth context.

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
