package domain

import "time"

// Check status values describe where a check is in its pipeline.
const (
	CheckPending = "pending"
	CheckRunning = "running"
	CheckDone    = "done"
	CheckFailed  = "failed"
)

// Claim verdict values. Verified means the claim's supporting span was confirmed by code,
// character for character, against the cited file's stored text; review means the model judged
// the span supportive but only a human can confirm a paraphrase; unsupported means nothing in
// the owner's files backs the claim.
const (
	VerdictVerified    = "verified"
	VerdictReview      = "review"
	VerdictUnsupported = "unsupported"
)

// Reference tier values describe how the model says a span supports its claim. Verbatim is a
// direct restatement eligible for code verification; paraphrase supports the claim in different
// words; none means no supporting span was found.
const (
	TierVerbatim   = "verbatim"
	TierParaphrase = "paraphrase"
	TierNone       = "none"
)

// Reference is one supporting span in one of the owner's files. Start and End are byte offsets
// into the file's stored canonical text, located deterministically by the gate, never taken from
// the model. Verified is set only when the gate re-read the text at those offsets and matched the
// span character for character.
type Reference struct {
	FileID   string `json:"fileId" dynamodbav:"fileId"`
	FileName string `json:"fileName" dynamodbav:"fileName"`
	SpanText string `json:"spanText" dynamodbav:"spanText"`
	Start    int    `json:"start" dynamodbav:"start"`
	End      int    `json:"end" dynamodbav:"end"`
	Tier     string `json:"tier" dynamodbav:"tier"`
	Verified bool   `json:"verified" dynamodbav:"verified"`
}

// Claim is one atomic checkable assertion from the checked text. Start and End are byte offsets
// into the check's own text, so the client can highlight the claim where it appears.
type Claim struct {
	Text      string     `json:"text" dynamodbav:"text"`
	Start     int        `json:"start" dynamodbav:"start"`
	End       int        `json:"end" dynamodbav:"end"`
	Verdict   string     `json:"verdict" dynamodbav:"verdict"`
	Reference *Reference `json:"reference,omitempty" dynamodbav:"reference,omitempty"`
}

// Check is one verification run: the pasted text, split into claims, each carrying its verdict.
type Check struct {
	ID        string    `json:"id" dynamodbav:"id"`
	OwnerID   string    `json:"-" dynamodbav:"ownerId"`
	Text      string    `json:"text" dynamodbav:"text"`
	Status    string    `json:"status" dynamodbav:"status"`
	Claims    []Claim   `json:"claims,omitempty" dynamodbav:"claims,omitempty"`
	CreatedAt time.Time `json:"createdAt" dynamodbav:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt" dynamodbav:"updatedAt"`
}
