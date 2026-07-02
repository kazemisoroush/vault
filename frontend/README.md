# Vault Frontend

Placeholder. The Next.js app lands with milestone M3 (Ask Anything).

The frontend is a pure API client. It talks only to the endpoints defined in
[openapi.yaml](../openapi.yaml) and never to S3 or DynamoDB directly. File bytes
move through the presigned URLs the API hands out.
