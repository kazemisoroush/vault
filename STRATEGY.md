# Vault — Product Strategy (v0)

> Working doc. Opinionated on purpose. Update it as reality argues back.

## The one-line

**A form asks for fifteen things you already have proof of. Vault fills them
from your documents — every field cited to the source file — so paperwork
stops meaning "open six documents and transcribe by hand."**

## The wedge (one engine, one form at a time)

**Evidence-to-form autofill.** One engine (`POST /fill`, already built in
PR #58) that answers a form's fields from the owner's documents, each field
backed by a cited source file. Go to market through **one form at a time**, not
"any form" — that's how a horizontal engine gets a sharp wedge instead of
boiling the ocean.

Every target — tax return, visa/immigration pack, loan/mortgage, daycare
enrolment, job application — is the *same product*: a list of fields, filled
from evidence. They are not separate use cases; they are one engine pointed at
different forms.

### Which form first — and why NOT tax return
Tax return is the owner's personal itch and the obvious pull, but it's the
wrong *first* market:
- **Once a year** — no habit loop, no retention signal, nothing learned between
  filings.
- **Highest liability** — a wrong number means an audit; the least forgiving
  place to earn first trust.
- **Deepest incumbents** (TurboTax/QuickBooks et al.).

**Launch instead on a form that is document-heavy, genuinely painful, higher
willingness-to-pay, and not once-a-year** — immigration/visa document packs or
loan/mortgage applications are the strongest candidates. **Dogfood on the
owner's own tax return** (scratch the itch, do the unscalable thing), but
position tax as a *proof point*, not the product.

## Not competing with tax software — a different category

Don't fight TurboTax/QuickBooks on tax *logic* (deductions, rules, filing) —
that's their moat and a losing fight. Their weakness is the part Vault owns:
**gathering and transcribing the source numbers.** Vault is the
**evidence-to-form autofill layer upstream of them** — it fills the fields from
the documents; they do the math and file. Same shape for visa/loan/daycare: it
never replaces the official form, it fills it from proof. This category is
under-served precisely because incumbents assume the user will type it in.

## Why this is defensible (use what you already built)

- **Every field cites its source.** `POST /fill` already enforces the rule that
  matters: `found=true` only when a document backs the value; an uncited or
  errored field returns blank, not a guess. On a real form, a plausible-wrong
  number is worse than a blank. *This is the moat.* Protect it above all else.
- **Lens, not silo (SIT: subtraction + closed world).** Don't make people
  re-upload their life. Fill from the Drive they already have. Lower activation,
  higher trust.

## Applying "Inside the Box" (SIT)

| Technique | Applied to Vault |
|---|---|
| **Subtraction** | Remove the storage silo. Fill forms as a read-only lens over documents people already have. |
| **Task unification** | One `/fill` engine serves tax, visa, loan, daycare, job apps — every form is the same job (fields ← evidence). |
| **Attribute dependency** | Confidence drives UX: cited high-confidence fields auto-fill; uncited/low-confidence fields surface for one-tap human review. |

## The thinnest shippable slice (ship ASAP = startup rule #1)

`/fill` exists. The gap to a usable product is turning a *blank form* into
*filled fields a human can trust*. Cut everything that isn't this loop:

1. **Take one real form** for the chosen wedge (upload PDF, or a fixed field
   list to start — SOR-249 turns a blank form into that field list).
2. **Point at the owner's documents** (existing Drive, read-only).
3. **Run `/fill`** — each field answered with its cited source file.
4. **One review screen**: filled fields on the left, the cited source document
   on the right; uncited/blank fields flagged for the human to resolve.
5. **One output**: a filled PDF or a copy-paste-ready field list.

That is a sellable product. Everything else — many forms, chat/ask UI, the full
"any document" vision — is post-ship.

### Explicitly NOT in the first ship
- More than one form/vertical in the *marketing* (engine stays general; story
  stays narrow).
- Tax return as the launch vertical (dogfood only — see wedge section).
- Multiple connectors (Gmail, Dropbox, etc.).
- Sharing, teams, roles.
- Native mobile app (mobile web is enough).

## The one metric to watch

**Time-to-first-trusted-filled-form** — from pointing at documents to a filled
form the user signs off on without re-checking every field by hand. You already
track `time-to-file`; this is the v2 of that. If a new user can't reach a
form they trust in one sitting, nothing else matters.

## First 20 users

Not a launch. Twenty people filling the *same* form (the chosen wedge) you can
talk to. Watch them hit the review screen, see which fields they don't trust
and why, fix that. Charge something small from day one — free users don't tell
you the truth about value.

## Open questions to resolve with usage, not debate
- Which wedge form first: visa/immigration pack vs. loan/mortgage vs. other?
- Per-form pricing vs. subscription vs. per-seat (for agents who fill on behalf
  of clients — immigration consultants, mortgage brokers, migration agents)?
- Is the real buyer the individual, or the professional who fills the same form
  for many clients?

---

### On the "4 things to succeed" blog post
You told me rule #1: **ship ASAP.** I won't invent the other three — I don't
know which post you read. Send them and I'll pressure-test this strategy
against all four. A common version worth comparing against: *ship fast →
talk to users → do things that don't scale → charge money early.* This plan
already leans into all of those, but your source is the one that matters.
