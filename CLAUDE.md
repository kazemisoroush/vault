# Vault

Personal data vault on pure AWS. Drop any file and get it back almost instantly
by free-form phrase: no folders, no tags, no enums. Go on Lambda behind API
Gateway, DynamoDB as the metadata index, S3 for blobs, Next.js on Vercel.
`openapi.yaml` is the source of truth for the API.

# Review

This file is the rulebook and the review bar. The automated reviewer in
[review.yml](.github/workflows/review.yml) reviews every pull request against
it, and it is the only standard the review judges against. Change the rules by
pull request.

## Review bar

CI already checks formatting, vet, lint, tests, and secrets. A review never
repeats a check CI runs.

A review requests changes only for one of these four, and it quotes the diff for
each blocker:

1. **correctness** — a logic bug, wrong result, race, unhandled error, or a broken edge case.
2. **security** — a leaked secret, missing authn/authz, injection, or unsafe input at a trust boundary.
3. **docs** — a claim in prose or a doc-block the code does not support, or a comment that misleads.
4. **abstraction** — an external integration called straight from business logic, an env var or flag read inline instead of through a config layer, a magic literal that should be a named constant, or a one-implementation abstraction that earns nothing.

Anything else is a note, not a blocker. Say it once in the body, then approve.
Taste that no rule here covers is not a reason to hold a pull request.

## Review comments

Every review body follows this format. It is the only source of the shape.

Open with one of three headings, and nothing else:

```md
**🤖 Claude review: approved**
**🤖 Claude review: approved with notes**
**🤖 Claude review: changes requested**
```

Then one sentence saying what was found. Then the findings, if there are any.

Severity has three levels and nothing else. A blocker is 🔴 or 🟠. A note is 🟡,
and a note never blocks a merge.

| Badge | Level | Meaning |
|---|---|---|
| 🔴 | Critical | Breaks the system or exposes it. Merging this causes harm. |
| 🟠 | Major | Wrong or misleading. Merging this leaves a defect behind. |
| 🟡 | Minor | Worth fixing, but safe to merge without it. |

Each finding takes this shape, with `Why.` and `Fix.` each one sentence:

```md
**🟠 Major · correctness · [`backend/internal/api/controller/fill.go:106`](https://github.com/kazemisoroush/vault/blob/<sha>/backend/internal/api/controller/fill.go#L106)**
The agent-error branch returns found=false, which the schema defines as a genuine miss.

**Why.** A throttled Bedrock call is a transient error, not the vault lacking the value.
**Fix.** Return a distinct error flag on the row instead of found=false.
```

Close every body with this line:

```md
<sub>Posted by Claude from the review workflow.</sub>
```

Rules for the body:

- Link every file with a full URL against the head commit, as above. A relative
  link resolves against the repository root and lands on a page that does not
  exist, because the comment renders on the pull request page.
- Support each finding with a quote from the diff or the code. A finding with no
  supporting quote is not posted.
- Keep each field to one sentence. A finding that needs a paragraph is really
  several findings, so split it.
- Never leave inline comments. An unresolved review thread blocks the pull
  request, so put every finding in the single review body.

# Code rules

- Always divide tests into 3 parts separated with comments. // Arrange // Act // Assert.
- Remove unnecessary and unused tests on your way of developing new things.
- Never remove an existing inline comment unless asked to, and avoid adding new ones.
- Keep doc-blocks to one short sentence. Add more only to record a correctness or
  security subtlety a reader needs; anything else goes in the PR.
- Do not keep history in comments while changing the code.
- Never reference a ticket number in the code. `SOR-248` belongs in the pull
  request or the commit message, not in a comment.
- Variable re-assignment is generally bad practice.
- Every integration lives behind its own abstraction. Any external integration
  (HTTP client, database, queue, third-party SDK, filesystem, cloud service)
  sits behind its own dedicated boundary, never called directly from business logic.
- Always have a config layer for env variables or CLI parameters. Env variables
  and CLI flags are read through a config layer, never inline where they are used.
- Prefer named constants or enums over magic strings and numbers.
- Name things explicitly, never abbreviate a word to save characters.
  `configuration` not `cfg`, `fileSystemEntry` not `fsEntry`.
- Every linter rule applies to all projects, not one.

# Go

- Always wrap returned errors with `fmt.Errorf` and `%w`. A bare `return err` or
  a delegating `return someFunc(...)` that forwards an error is a violation
  everywhere, including package-internal calls.
- Never pass `context.Background()` to per-request or bounded I/O. Thread the
  caller context, which in Lambda is the handler's context. Reserve
  `context.Background()` for process-lifetime contexts, and bound one-shot
  startup I/O with `context.WithTimeout`.
- Prefer testify `require.NoError(...)` and `assert.*` over manual
  `if err != nil { t.Error... }` checks in tests.

# Frontend (Next.js)

- Keep components typed. No `any` where a real type is known.
- Data access goes through the generated API client, never a raw `fetch` in a component.
