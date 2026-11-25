package ai

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"tangentry/internal/citation"
	"tangentry/internal/content"
	"tangentry/internal/retrieval"
)

type compareSourceFixture struct {
	snapshots map[string]retrieval.Snapshot
	failures  map[string]error
}

func (f *compareSourceFixture) Get(_ context.Context, id string) (content.Item, error) {
	snapshot, ok := f.snapshots[id]
	if !ok {
		return content.Item{}, content.ErrNotFound
	}
	kind := "article"
	if snapshot.Kind == retrieval.SourceSubtitle {
		kind = "video"
	}
	return content.Item{ID: id, Title: "Source " + id[len(id)-1:], Kind: kind}, nil
}

func (f *compareSourceFixture) Snapshot(_ context.Context, id string) (retrieval.Snapshot, error) {
	if err := f.failures[id]; err != nil {
		return retrieval.Snapshot{}, err
	}
	snapshot, ok := f.snapshots[id]
	if !ok {
		return retrieval.Snapshot{}, retrieval.ErrSourceNotFound
	}
	return snapshot, nil
}

func (f *compareSourceFixture) ReadRange(ctx context.Context, id, locator string, start, end int) (string, string, error) {
	snapshot, err := f.Snapshot(ctx, id)
	if err != nil {
		return "", "", err
	}
	return retrieval.SnapshotRange(snapshot, locator, start, end)
}

type compareProviderFixture struct {
	calls  atomic.Int32
	answer func(context.Context, []comparisonEvidence) (string, error)
}

func (p *compareProviderFixture) Translate(_ context.Context, _ Config, segments []Segment) ([]Segment, error) {
	return segments, nil
}

func (p *compareProviderFixture) Chat(ctx context.Context, _ Config, source string, _ []ChatMessage) (string, error) {
	if err := ReserveCompletion(ctx); err != nil {
		return "", err
	}
	p.calls.Add(1)
	var evidence []comparisonEvidence
	_, data, _ := strings.Cut(source, "\nSOURCES\n")
	if err := json.Unmarshal([]byte(data), &evidence); err != nil {
		return "", err
	}
	RecordCompletionUsage(ctx, Usage{Prompt: 2, Output: 3, Total: 5})
	if p.answer != nil {
		return p.answer(ctx, evidence)
	}
	return compareDraftFixture(evidence, "conflict"), nil
}

func compareDraftFixture(evidence []comparisonEvidence, relation string) string {
	draft := comparisonDraft{Relation: relation, Answer: "两份资料给出的退款期限需要结合各自原文进行比较。"}
	for _, source := range evidence {
		row := comparisonDraftSource{ContentID: source.ContentID, State: "supported"}
		if len(source.Passages) == 0 {
			row.State = "insufficient_evidence"
		} else {
			row.Fact = source.Passages[0].Text
			row.Citations = append(row.Citations, struct {
				PassageID string `json:"passageId"`
				Quote     string `json:"quote"`
			}{source.Passages[0].ID, source.Passages[0].Text})
		}
		draft.Sources = append(draft.Sources, row)
	}
	raw, _ := json.Marshal(draft)
	return string(raw)
}

func newComparisonFixture(t *testing.T) (*Service, *compareSourceFixture, *compareProviderFixture, CompareRequest) {
	t.Helper()
	ids := []string{"rss:" + strings.Repeat("a", 64), "rss:" + strings.Repeat("b", 64)}
	source := &compareSourceFixture{snapshots: make(map[string]retrieval.Snapshot), failures: make(map[string]error)}
	for i, id := range ids {
		source.snapshots[id] = retrieval.Snapshot{ContentID: id, Kind: retrieval.SourceArticle, Version: "content-revision:1",
			Blocks: []retrieval.Block{{Index: 0, Kind: "paragraph", Text: fmt.Sprintf("退款期限为 %d 天。", 14+i*16)}}}
	}
	provider := &compareProviderFixture{}
	service, err := New(filepath.Join(t.TempDir(), "ai"), NewMemoryResults(), provider)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(service.Close)
	status, err := service.Configure(ConfigureInput{BaseURL: "http://127.0.0.1:11434/v1", Model: "fixture", ChatModel: func() *string { v := "fixture"; return &v }(), Revision: 1})
	if err != nil {
		t.Fatal(err)
	}
	service.SetComparisonSource(source)
	service.SetRetrievalEngine(&retrieval.Engine{Source: source})
	return service, source, provider, CompareRequest{ContentIDs: ids, Question: "退款期限是否一致？", Revision: status.Revision, MaxProviderCalls: limit(1)}
}

func TestComparisonVerifiedConflictAndSourceNavigation(t *testing.T) {
	service, source, provider, request := newComparisonFixture(t)
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Relation != "conflict" || result.Budget.Started != 1 || result.Budget.Status != "completed" {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	if provider.calls.Load() != 1 {
		t.Fatal("expected one generation request")
	}
	for _, row := range result.Sources {
		if row.State != "supported" || len(row.Citations) != 1 || row.Citations[0].Status != citation.StatusVerified {
			t.Fatalf("unverified finding: %+v", row)
		}
		evidence, err := service.ComparisonEvidence(context.Background(), row.Citations[0])
		if err != nil || evidence.Text != row.Fact {
			t.Fatalf("evidence=%+v error=%v", evidence, err)
		}
		invalid := row.Citations[0]
		invalid.StartRune = -1
		if _, err := service.ComparisonEvidence(context.Background(), invalid); !errors.Is(err, ErrInvalid) {
			t.Fatalf("negative source range accepted: %v", err)
		}
		invalid = row.Citations[0]
		invalid.EndRune += 100
		if _, err := service.ComparisonEvidence(context.Background(), invalid); err == nil {
			t.Fatal("out-of-range source jump accepted")
		}
	}
	// Deletion between displaying the result and opening evidence fails closed.
	delete(source.snapshots, request.ContentIDs[0])
	if _, err := service.ComparisonEvidence(context.Background(), result.Sources[0].Citations[0]); err == nil {
		t.Fatal("deleted source still navigable")
	}
}

func TestComparisonValidationBeforeProviderCalls(t *testing.T) {
	for _, mutation := range []func(*CompareRequest){
		func(r *CompareRequest) { r.ContentIDs = nil },
		func(r *CompareRequest) { r.ContentIDs = r.ContentIDs[:1] },
		func(r *CompareRequest) { r.ContentIDs = append(r.ContentIDs, r.ContentIDs...) },
		func(r *CompareRequest) { r.ContentIDs[1] = r.ContentIDs[0] },
		func(r *CompareRequest) { r.ContentIDs[1] = "rss:invalid" },
		func(r *CompareRequest) { r.ContentIDs[1] = "rss:" + strings.Repeat("c", 64) },
		func(r *CompareRequest) { r.Question = " " },
		func(r *CompareRequest) { r.MaxProviderCalls = limit(0) },
		func(r *CompareRequest) { r.MaxProviderCalls = limit(101) },
		func(r *CompareRequest) { r.MaxChars = limit(0) },
		func(r *CompareRequest) { r.MaxChars = limit(1) },
		func(r *CompareRequest) { r.Revision-- },
	} {
		service, _, provider, request := newComparisonFixture(t)
		mutation(&request)
		if _, err := service.Compare(context.Background(), request); err == nil {
			t.Fatalf("invalid input accepted: %+v", request)
		}
		if provider.calls.Load() != 0 {
			t.Fatal("invalid input contacted provider")
		}
	}
}

func TestComparisonPartialAndAllSourceFailures(t *testing.T) {
	service, source, provider, request := newComparisonFixture(t)
	source.failures[request.ContentIDs[1]] = errors.New("disk error")
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Relation != "insufficient_evidence" || result.Sources[0].State != "supported" ||
		result.Sources[1].Reason != "read_failed" || result.Sources[1].Fact != "" {
		t.Fatalf("result=%+v error=%v", result, err)
	}
	source.failures[request.ContentIDs[0]] = errors.New("disk error")
	result, err = service.Compare(context.Background(), request)
	if err != nil || result.Relation != "insufficient_evidence" || provider.calls.Load() != 1 || result.Budget.Started != 0 {
		t.Fatalf("all failed result=%+v calls=%d err=%v", result, provider.calls.Load(), err)
	}
}

func TestComparisonRejectsWrongSourceQuoteAndMalformedOutput(t *testing.T) {
	for _, mode := range []string{"wrong_source", "altered", "duplicate", "malformed"} {
		t.Run(mode, func(t *testing.T) {
			service, _, provider, request := newComparisonFixture(t)
			provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
				if mode == "malformed" {
					return "This is not JSON", nil
				}
				var draft comparisonDraft
				_ = json.Unmarshal([]byte(compareDraftFixture(evidence, "conflict")), &draft)
				switch mode {
				case "wrong_source":
					draft.Sources[0].Citations[0].PassageID = evidence[1].Passages[0].ID
				case "altered":
					draft.Sources[0].Citations[0].Quote = "退款期限为 999 天。"
				case "duplicate":
					draft.Sources[1].ContentID = draft.Sources[0].ContentID
				}
				raw, _ := json.Marshal(draft)
				return string(raw), nil
			}
			result, err := service.Compare(context.Background(), request)
			if mode == "duplicate" || mode == "malformed" {
				if !errors.Is(err, ErrFormat) {
					t.Fatalf("expected format error: %+v %v", result, err)
				}
			} else if err != nil || result.Relation != "insufficient_evidence" || result.Sources[0].Fact != "" {
				t.Fatalf("invalid citation accepted: %+v err=%v", result, err)
			}
		})
	}
}

func TestComparisonDetectsChangeDuringGeneration(t *testing.T) {
	service, source, provider, request := newComparisonFixture(t)
	provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
		snapshot := source.snapshots[request.ContentIDs[1]]
		snapshot.Version = "content-revision:2"
		source.snapshots[request.ContentIDs[1]] = snapshot
		return compareDraftFixture(evidence, "same"), nil
	}
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Relation != "insufficient_evidence" || result.Sources[1].Reason != "stale_source" {
		t.Fatalf("stale source passed: %+v %v", result, err)
	}
}

func TestComparisonCompleteAbsenceAndLimitedCoverage(t *testing.T) {
	for _, complete := range []bool{true, false} {
		service, source, provider, request := newComparisonFixture(t)
		header := "The following fields are the complete service card."
		if !complete {
			header = "This is only a service card excerpt."
		}
		snapshot := source.snapshots[request.ContentIDs[1]]
		snapshot.Blocks = []retrieval.Block{{Index: 0, Kind: "paragraph", Text: header},
			{Index: 1, Kind: "paragraph", Text: "Hours: 09:00–17:00."}}
		source.snapshots[request.ContentIDs[1]] = snapshot
		request.Question = "Which service cards specify a support email?"
		provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
			var draft comparisonDraft
			_ = json.Unmarshal([]byte(compareDraftFixture(evidence, "not_mentioned")), &draft)
			draft.Sources[1] = comparisonDraftSource{ContentID: evidence[1].ContentID, State: "not_mentioned", Field: "support email"}
			raw, _ := json.Marshal(draft)
			return string(raw), nil
		}
		result, err := service.Compare(context.Background(), request)
		if err != nil {
			t.Fatal(err)
		}
		if complete && (result.Relation != "not_mentioned" || result.Sources[1].Coverage == nil) {
			t.Fatalf("missing complete coverage: %+v", result)
		}
		if !complete && result.Relation != "insufficient_evidence" {
			t.Fatalf("partial coverage marked absent: %+v", result)
		}
	}
}

func TestComparisonBudgetUnknownCancellationAndInheritedLimit(t *testing.T) {
	for _, failure := range []error{ErrUsageUnknown, context.Canceled, ErrBudgetExhausted} {
		service, _, provider, request := newComparisonFixture(t)
		provider.answer = func(ctx context.Context, _ []comparisonEvidence) (string, error) {
			if errors.Is(failure, ErrUsageUnknown) {
				MarkUsageUnknown(ctx)
			}
			return "", failure
		}
		result, err := service.Compare(context.Background(), request)
		if !errors.Is(err, failure) || result.Status == "completed" || result.Budget.Started != 1 || provider.calls.Load() != 1 {
			t.Fatalf("failure overwritten: %+v err=%v", result, err)
		}
	}
	service, _, provider, request := newComparisonFixture(t)
	ctx, _ := NewBudgetContext(context.Background(), 1)
	if err := ReserveCompletion(ctx); err != nil {
		t.Fatal(err)
	}
	RecordCompletionUsage(ctx, Usage{1, 1, 2})
	request.MaxProviderCalls = nil
	result, err := service.Compare(ctx, request)
	if !errors.Is(err, ErrBudgetExhausted) || result.Budget.Started != 1 || provider.calls.Load() != 0 {
		t.Fatalf("inherited budget bypass: %+v %v", result, err)
	}
}

func TestComparisonLongSourceUsesBoundedRetrieval(t *testing.T) {
	service, source, provider, request := newComparisonFixture(t)
	snapshot := source.snapshots[request.ContentIDs[0]]
	snapshot.Blocks = nil
	for i := range 300 {
		snapshot.Blocks = append(snapshot.Blocks, retrieval.Block{Index: i, Kind: "paragraph", Text: "普通背景材料，只介绍设施布置与文档结构。"})
	}
	snapshot.Blocks[150].Text = "退款期限为 14 天。"
	source.snapshots[request.ContentIDs[0]] = snapshot
	request.MaxChars = limit(8000)
	provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
		encoded, _ := json.Marshal(evidence)
		if len(encoded) > retrieval.DefaultMaxBytes || !strings.Contains(string(encoded), "退款期限为 14 天") {
			t.Errorf("missing middle evidence or exceeded budget")
		}
		if evidence[0].Complete {
			t.Error("partial retrieval reported complete")
		}
		return compareDraftFixture(evidence, "same"), nil
	}
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Sources[0].Retrieval == nil {
		t.Fatalf("long retrieval failed: %+v %v", result, err)
	}
}

func TestComparisonThreeSourcesRemainInInputOrder(t *testing.T) {
	service, source, provider, request := newComparisonFixture(t)
	third := "rss:" + strings.Repeat("c", 64)
	source.snapshots[third] = retrieval.Snapshot{ContentID: third, Kind: retrieval.SourceArticle, Version: "1", Blocks: []retrieval.Block{{Index: 0, Kind: "paragraph", Text: "付款后 30 天。"}}}
	request.ContentIDs = append(request.ContentIDs, third)
	for _, relation := range []string{"same", "conflict", "insufficient_evidence"} {
		provider.answer = func(_ context.Context, e []comparisonEvidence) (string, error) {
			return compareDraftFixture(e, relation), nil
		}
		result, err := service.Compare(context.Background(), request)
		if err != nil || result.Relation != relation || len(result.Sources) != 3 {
			t.Fatalf("%s: %+v %v", relation, result, err)
		}
		for i, row := range result.Sources {
			if row.ContentID != request.ContentIDs[i] || row.State != "supported" {
				t.Fatalf("source ordering: %+v", result)
			}
		}
	}
}

func TestComparisonAbsenceRequiresAffirmativeScopeAndMissingField(t *testing.T) {
	for _, tc := range []struct {
		header, name string
		want         bool
	}{
		{"Complete fields:", "Hours", true},
		{"Incomplete fields:", "Hours", false},
		{"不完整字段如下。", "Hours", false},
		{"完整字段如下。", "客服邮箱", false},
		{"Complete fields:", "E-mail", false},
		{"The following is possibly complete.", "Hours", false},
		{"Exhaustive property register:", "Hours", false},
	} {
		evidence := comparisonEvidence{Complete: true, snapshot: retrieval.Snapshot{Kind: retrieval.SourceArticle, Version: "1", Blocks: []retrieval.Block{{Index: 0, Text: tc.header}, {Index: 1, Text: tc.name + ": value"}}}}
		got := comparisonAbsence("Which cards specify a support email?", "support email", evidence)
		if (got != nil) != tc.want {
			t.Errorf("header=%q name=%q got=%+v", tc.header, tc.name, got)
		}
	}
}

// D-T02/D-T04: short model handles are scoped to one invocation. Public
// results and citation provenance always use the authorized saved IDs.
func TestComparisonScopedSourceHandlesAndReorderedFindings(t *testing.T) {
	service, _, provider, request := newComparisonFixture(t)
	provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
		if evidence[0].ContentID != "S1" || evidence[1].ContentID != "S2" || evidence[0].Passages[0].ID != "S1-P1" || evidence[1].Passages[0].ID != "S2-P1" {
			t.Fatalf("unexpected wire handles: %+v", evidence)
		}
		var draft comparisonDraft
		_ = json.Unmarshal([]byte(compareDraftFixture(evidence, "conflict")), &draft)
		draft.Sources[0], draft.Sources[1] = draft.Sources[1], draft.Sources[0]
		raw, _ := json.Marshal(draft)
		return string(raw), nil
	}
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Relation != "conflict" {
		t.Fatalf("comparison=%+v error=%v", result, err)
	}
	for i, row := range result.Sources {
		if row.ContentID != request.ContentIDs[i] || len(row.Citations) != 1 || row.Citations[0].ContentID != request.ContentIDs[i] || row.Citations[0].Status != citation.StatusVerified {
			t.Fatalf("lost provenance or ordering: %+v", result)
		}
	}
}

// D-T04: a snapshot from another content ID must never enter the prompt,
// even if a misconfigured source adapter returns it after access validation.
func TestComparisonForeignSnapshotNeverReachesModel(t *testing.T) {
	service, source, provider, request := newComparisonFixture(t)
	snapshot := source.snapshots[request.ContentIDs[0]]
	snapshot.ContentID = request.ContentIDs[1]
	snapshot.Blocks[0].Text = "FOREIGN SNAPSHOT MUST NOT ENTER PROMPT"
	source.snapshots[request.ContentIDs[0]] = snapshot
	provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
		encoded, _ := json.Marshal(evidence)
		if strings.Contains(string(encoded), "FOREIGN SNAPSHOT") || len(evidence[0].Passages) != 0 {
			t.Fatal("unauthorized snapshot entered model prompt")
		}
		return compareDraftFixture(evidence, "same"), nil
	}
	result, err := service.Compare(context.Background(), request)
	if err != nil || result.Relation != "insufficient_evidence" || result.Sources[0].Reason != "source_unverified" || result.Sources[1].State != "supported" {
		t.Fatalf("wrong failure isolation: %+v %v", result, err)
	}
}

func TestComparisonUnknownSourceHandleFailsClosed(t *testing.T) {
	service, _, provider, request := newComparisonFixture(t)
	provider.answer = func(_ context.Context, evidence []comparisonEvidence) (string, error) {
		var draft comparisonDraft
		_ = json.Unmarshal([]byte(compareDraftFixture(evidence, "same")), &draft)
		draft.Sources[0].ContentID = request.ContentIDs[0]
		raw, _ := json.Marshal(draft)
		return string(raw), nil
	}
	if _, err := service.Compare(context.Background(), request); !errors.Is(err, ErrFormat) {
		t.Fatalf("unissued source handle accepted: %v", err)
	}
}

type comparisonStructuredFixture struct {
	*compareProviderFixture
	inspect func(ComparisonPrompt)
}

func (p *comparisonStructuredFixture) CompareEvidence(ctx context.Context, c Config, input ComparisonPrompt) (string, error) {
	if p.inspect != nil {
		p.inspect(input)
	}
	return p.Chat(ctx, c, input.Source, []ChatMessage{{Role: "user", Content: input.Question}})
}

// D-T04/D-T05: the real adapter receives only server-issued handles, and
// schema constraints count toward both character and UTF-8 byte budgets.
func TestComparisonStructuredContractAndContextBounds(t *testing.T) {
	service, _, provider, request := newComparisonFixture(t)
	request.MaxChars = limit(8000)
	inspected := false
	service.provider = &comparisonStructuredFixture{compareProviderFixture: provider, inspect: func(input ComparisonPrompt) {
		inspected = true
		encoded, _ := json.Marshal(input.ResponseFormat)
		data := string(encoded)
		for _, id := range request.ContentIDs {
			if strings.Contains(input.Source, id) || strings.Contains(data, id) {
				t.Fatal("native source IDs leaked into the model contract")
			}
		}
		if !strings.Contains(data, `"enum":["S1"]`) || !strings.Contains(data, `"enum":["S2"]`) || !strings.Contains(data, `"enum":["S1-P1"]`) || !strings.Contains(data, `"enum":["S2-P1"]`) {
			t.Fatalf("unscoped output contract: %s", data)
		}
		if strings.Index(data, `"sources":`) > strings.Index(data, `"answer":`) || strings.Index(data, `"fact":`) > strings.Index(data, `"state":`) || strings.Index(data, `"state":`) > strings.Index(data, `"passageIds":`) {
			t.Fatal("structured output must extract facts before answering")
		}
		if len([]rune(comparisonSystemInstructions))+2+len([]rune(input.Source))+len([]rune(input.Question))+len([]rune(data)) > *request.MaxChars || len(comparisonSystemInstructions)+2+len(input.Source)+len(input.Question)+len(encoded) > retrieval.DefaultMaxBytes {
			t.Fatal("generation context exceeds declared bounds")
		}
	}}
	result, err := service.Compare(context.Background(), request)
	if err != nil || !inspected || result.Sources[0].State != "supported" {
		t.Fatalf("structured result=%+v inspected=%v error=%v", result, inspected, err)
	}
}
