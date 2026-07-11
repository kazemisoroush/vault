// Package evals runs the agent over golden cases, offline against a scripted model and, when
// VAULT_EVAL_BEDROCK is set, against the real model on Bedrock. Both use the same seeded in-memory
// vault, so the golden cases and their assertions are shared.
package evals

import (
	"context"
	"hash/fnv"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/kazemisoroush/vault/backend/internal/domain"
	"github.com/kazemisoroush/vault/backend/internal/index"
)

// embedDimension is the width of the toy embedding used to seed and search the fake vector store.
const embedDimension = 16

// fakeEmbedder turns text into a deterministic bag-of-words vector, so nearest-neighbour search is
// reproducible without calling a real embedding model.
type fakeEmbedder struct{}

func (fakeEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	vector := make([]float32, embedDimension)
	for token := range strings.FieldsSeq(strings.ToLower(text)) {
		h := fnv.New32a()
		_, _ = h.Write([]byte(token))
		vector[h.Sum32()%embedDimension]++
	}
	return vector, nil
}

// fakeVectors is an in-memory vector store: one owner-tagged vector per file, queried by cosine.
type fakeVectors struct {
	vectors map[string]ownedVector
}

type ownedVector struct {
	owner  string
	vector []float32
}

func newFakeVectors() *fakeVectors { return &fakeVectors{vectors: map[string]ownedVector{}} }

func (f *fakeVectors) Put(_ context.Context, id, ownerID string, vector []float32) error {
	f.vectors[id] = ownedVector{owner: ownerID, vector: vector}
	return nil
}

func (f *fakeVectors) Query(_ context.Context, ownerID string, vector []float32, topK int32) ([]string, error) {
	type scored struct {
		id    string
		score float64
	}
	ranked := make([]scored, 0, len(f.vectors))
	for id, entry := range f.vectors {
		if entry.owner != ownerID {
			continue
		}
		ranked = append(ranked, scored{id: id, score: cosine(vector, entry.vector)})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })

	ids := make([]string, 0, len(ranked))
	for i, r := range ranked {
		if int32(i) >= topK {
			break
		}
		ids = append(ids, r.id)
	}
	return ids, nil
}

func (f *fakeVectors) Delete(_ context.Context, id string) error {
	delete(f.vectors, id)
	return nil
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
func cosine(a, b []float32) float64 {
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
