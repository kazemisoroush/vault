# Vault

Personal data vault that sits on top of Google Drive. Provides metadata indexing, tagging, categorization, and search for all your personal files.

## Architecture

- **Backend**: Go, deployed on AWS Lambda + API Gateway
- **Frontend**: Next.js (TypeScript + Tailwind), deployed on Vercel
- **Database**: DynamoDB for file metadata
- **Storage**: Google Drive API

## Project Structure

```
api/          Go backend
web/          Next.js frontend
infra/        AWS SAM templates
```

## Getting Started

### Backend

```bash
cd api
go mod tidy
go run cmd/lambda/main.go
```

### Frontend

```bash
cd web
npm install
npm run dev
```
