# Vault — Product Strategy (v0)

> Working doc. Opinionated on purpose. Update it as reality argues back.

## The one-line

**Point Vault at your Google Drive. Every receipt and invoice comes back
extracted, verified against the source file, totaled, and export-ready.
Stop doing expense data entry.**

## The wedge (pick one, be 10x, expand later)

**Receipts & invoices for freelancers, sole traders, and small businesses.**

Not "a vault for everything." Everything-for-everyone has no trigger moment
and no buyer. This one does:

- **Frequency** — expenses happen every week.
- **ROI is a number** — captured tax deductions + hours of data entry saved.
- **Willingness to pay** — businesses already pay Dext/Expensify/Xero for this.

The engine stays general (any file type). The *story* is sharp. Keep
"find my passport / any document" as a **marketing hook and demo**, never as
the wedge — consumers love it and won't pay for it.

## Why this is defensible (use what you already built)

- **Verification, not just extraction.** The `checks` layer (judge / verifier /
  gate) is the headline. Every extracted number cites its source file and is
  verified. Generic GPT wrappers hallucinate totals; you can say
  *"trust the number — here's the receipt it came from."*
- **Lens, not silo (SIT: subtraction + closed world).** Don't make people
  re-upload their life. Read the Drive they already have. Lower activation,
  higher retention.

## Applying "Inside the Box" (SIT)

| Technique | Applied to Vault |
|---|---|
| **Subtraction** | Remove the storage silo. Be a read-only lens over existing Drive. |
| **Task unification** | The `checks` layer does double duty: it both improves extraction quality *and* becomes the trust story you sell on. |
| **Attribute dependency** | Extraction confidence drives UX: high-confidence rows auto-post; low-confidence rows get flagged for one-tap review. |

## The thinnest shippable slice (ship ASAP = startup rule #1)

Cut everything that isn't this loop:

1. **Connect one Google Drive** (read-only, one folder is fine to start).
2. **Auto-detect receipts/invoices** among the files already there.
3. **Extract the five fields that matter**: vendor, date, amount, tax/VAT,
   category. Verify each against the source; flag low confidence.
4. **One screen**: a table of everything found, running total, source thumbnail
   on click.
5. **One export**: CSV first (works with every accountant + Xero/QuickBooks).

That is a sellable product. Everything else — multi-source, chat/ask UI, the
full "any document" vision — is post-ship.

### Explicitly NOT in the first ship
- Chat / natural-language "ask" as the primary UI (it's a feature, not the job).
- Multiple connectors (Gmail, Dropbox, etc.).
- Sharing, teams, roles.
- Mobile app (mobile web is enough).
- Any file type beyond receipts/invoices in the *marketing*.

## The one metric to watch

**Time-to-first-verified-export** — from connecting Drive to a downloaded CSV
the user trusts. You already track `time-to-file`; this is the v2 of that.
If a new user can't reach a trusted export in one sitting, nothing else matters.

## First 20 users

Not a launch. Twenty freelancers/bookkeepers you can talk to. Watch them hit
"connect Drive," see where trust breaks, fix that. Charge them something small
from day one — free users don't tell you the truth about value.

## Open questions to resolve with usage, not debate
- Read-only Drive lens vs. also being a place to forward receipts to?
- Per-seat SaaS vs. per-document vs. flat monthly?
- Do bookkeepers (who do it for many clients) beat end-freelancers as buyer?

---

### On the "4 things to succeed" blog post
You told me rule #1: **ship ASAP.** I won't invent the other three — I don't
know which post you read. Send them and I'll pressure-test this strategy
against all four. A common version worth comparing against: *ship fast →
talk to users → do things that don't scale → charge money early.* This plan
already leans into all of those, but your source is the one that matters.
