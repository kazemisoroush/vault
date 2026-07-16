// Package evals runs the agent over golden cases, offline against a scripted model and, when
// VAULT_EVAL_BEDROCK is set, against the real model on Bedrock. Both use the same seeded in-memory
// vault, so the golden cases and their assertions are shared.
package evals

import (
	"context"
	"strconv"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/index"
	"github.com/kazemisoroush/vault/backend/internal/kb"
)

// fakeRetriever returns the seeded case files as passages, standing in for hybrid retrieval over the
// Knowledge Base. The scripted model (offline) or the real model (Bedrock) picks the right file.
type fakeRetriever struct {
	passages []kb.Passage
}

func (f *fakeRetriever) add(file domain.File) {
	f.passages = append(f.passages, kb.Passage{FileID: file.ID, FileName: file.Name, Text: file.SearchText()})
}

func (f *fakeRetriever) Retrieve(_ context.Context, _ string, _ int) ([]kb.Passage, error) {
	return f.passages, nil
}

// fakeIndex is an in-memory file index scoped by owner, enough to serve get, list, and paging.
type fakeIndex struct {
	files map[string]domain.File
	order []string
}

func newFakeIndex() *fakeIndex { return &fakeIndex{files: map[string]domain.File{}} }

func (f *fakeIndex) Put(_ context.Context, file domain.File) error {
	if _, ok := f.files[file.ID]; !ok {
		f.order = append(f.order, file.ID)
	}
	f.files[file.ID] = file
	return nil
}

func (f *fakeIndex) Get(_ context.Context, id string) (domain.File, error) {
	file, ok := f.files[id]
	if !ok {
		return domain.File{}, index.ErrNotFound
	}
	return file, nil
}

func (f *fakeIndex) List(_ context.Context, ownerID string, limit int32, cursor string) ([]domain.File, string, error) {
	start := 0
	if cursor != "" {
		start, _ = strconv.Atoi(cursor)
	}
	page := make([]domain.File, 0, limit)
	i := start
	for ; i < len(f.order) && int32(len(page)) < limit; i++ {
		file := f.files[f.order[i]]
		if file.OwnerID == ownerID {
			page = append(page, file)
		}
	}
	next := ""
	if i < len(f.order) {
		next = strconv.Itoa(i)
	}
	return page, next, nil
}

func (f *fakeIndex) Delete(_ context.Context, id string) error {
	delete(f.files, id)
	return nil
}

// cosine is the cosine similarity of two equal-length vectors, zero when either has no magnitude.
