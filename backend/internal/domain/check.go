package domain

import "time"

// Check status values describe where a check is in its pipeline.
const (
	CheckPending = "pending"
	CheckRunning = "running"
	CheckDone    = "done"
	CheckFailed  = "failed"
)

// Claim verdict values, in precedence order. Disputed outranks everything: presenting a green
// while knowingly holding contradicting evidence would be lying by omission. Verified means a
// reference restates the claim and code confirmed it character for character; review means
// reworded support was found and a human decides; unsupported means the search came back empty,
// which is silence, not falsehood.
const (
	VerdictVerified    = "verified"
	VerdictDisputed    = "disputed"
	VerdictReview      = "review"
	VerdictUnsupported = "unsupported"
)

// Reference relation values describe how a passage bears on its claim, as judged by the model.
// The passage's existence is always confirmed by code before a reference is persisted; the
// relation itself is the model's judgment, except that verbatim additionally survives a
// code-level claim-span comparison or is demoted to paraphrase.
const (
	RelationVerbatim    = "verbatim"
	RelationParaphrase  = "paraphrase"
	RelationContradicts = "contradicts"
)

// Reference is one passage in one of the owner's files that bears on a claim. Start and End are
// byte offsets into the file's stored canonical text, located deterministically by the gate,
// never taken from the model. Every persisted Reference passed the existence gate: the span
// occurs in the stored text character for character.
type Reference struct {
	FileID   string `json:"fileId" dynamodbav:"fileId"`
	FileName string `json:"fileName" dynamodbav:"fileName"`
	SpanText string `json:"spanText" dynamodbav:"spanText"`
	Start    int    `json:"start" dynamodbav:"start"`
	End      int    `json:"end" dynamodbav:"end"`
	Relation string `json:"relation" dynamodbav:"relation"`
}

// Claim is one sentence of the checked text. Start and End are byte offsets into the check's own
// text, so the client can highlight the claim where it appears. References carry every
// gate-verified passage that bears on the claim, supporting and contradicting alike.
type Claim struct {
	Text       string      `json:"text" dynamodbav:"text"`
	Start      int         `json:"start" dynamodbav:"start"`
	End        int         `json:"end" dynamodbav:"end"`
	Verdict    string      `json:"verdict" dynamodbav:"verdict"`
	References []Reference `json:"references,omitempty" dynamodbav:"references,omitempty"`
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
