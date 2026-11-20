package ai

import (
	"context"
	"sync"
	"testing"

	"tangentry/internal/citation"
	"tangentry/internal/freshness"
)

// memoryVersions is a concurrency-safe ContentVersionReader for tests.
type memoryVersions struct {
	mu      sync.Mutex
	current map[string]freshness.Current
	err     error
}

func newMemoryVersions() *memoryVersions {
	return &memoryVersions{current: map[string]freshness.Current{}}
}

func (m *memoryVersions) set(id string, current freshness.Current) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current[id] = current
}

func (m *memoryVersions) delete(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.current[id] = freshness.Current{Exists: false}
}

func (m *memoryVersions) CurrentContentVersion(_ context.Context, contentID string) (freshness.Current, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.err != nil {
		return freshness.Current{}, m.err
	}
	current, ok := m.current[contentID]
	if !ok {
		return freshness.Current{Exists: false}, nil
	}
	return current, nil
}

func chatFreshnessService(t *testing.T, versions *memoryVersions) *Service {
	t.Helper()
	provider := &retrievalChatProvider{answer: "根据原文，答案是苹果。"}
	service, _, _ := configured(t, provider)
	if versions != nil {
		service.SetContentVersionReader(versions)
	}
	return service
}

func sendChat(t *testing.T, service *Service, contentID, question string) ChatResult {
	t.Helper()
	result, err := service.Chat(context.Background(), ChatRequest{
		ContentID: contentID,
		Revision:  service.Status().Revision,
		Model:     "fixture",
		Messages:  []ChatMessage{{Role: "user", Content: question}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	return result
}

func assistantOf(t *testing.T, result ChatResult) ChatMessage {
	t.Helper()
	if len(result.Messages) < 2 || result.Messages[len(result.Messages)-1].Role != "assistant" {
		t.Fatalf("missing assistant message: %+v", result.Messages)
	}
	return result.Messages[len(result.Messages)-1]
}

func TestETStampAndReopenAfterUpdateAndDelete(t *testing.T) {
	const id = "rss:freshness-1"
	versions := newMemoryVersions()
	versions.set(id, freshness.Current{Exists: true, Revision: 1, Hash: "hash-v1"})
	service := chatFreshnessService(t, versions)

	created := sendChat(t, service, id, "果实是什么？")
	first := assistantOf(t, created)
	if first.SourceRevision != 1 || first.SourceHash != "hash-v1" {
		t.Fatalf("snapshot not stamped: rev=%d hash=%q", first.SourceRevision, first.SourceHash)
	}
	if first.VersionStatus != freshness.StatusCurrent {
		t.Fatalf("new answer status = %q, want current", first.VersionStatus)
	}

	// Reopen while unchanged.
	reopen, err := service.ChatHistory(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got := assistantOf(t, reopen).VersionStatus; got != freshness.StatusCurrent {
		t.Fatalf("reopen unchanged = %q", got)
	}

	// Update the source and reopen: the stored answer must be expired, not current.
	versions.set(id, freshness.Current{Exists: true, Revision: 2, Hash: "hash-v2"})
	afterUpdate, err := service.ChatHistory(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	updated := assistantOf(t, afterUpdate)
	if updated.VersionStatus != freshness.StatusExpired {
		t.Fatalf("after update = %q, want expired", updated.VersionStatus)
	}
	if freshness.DisplayedAsCurrentlyValid(updated.VersionStatus) {
		t.Fatal("updated source still displayed as currently valid")
	}
	// C-style Status, if present, must not override freshness.
	for _, c := range updated.Citations {
		if freshness.DisplayedAsCurrentlyValid(c.Freshness) {
			t.Fatalf("citation still current after update: %+v", c)
		}
	}

	// Second reopen is stable.
	again, err := service.ChatHistory(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if assistantOf(t, again).VersionStatus != freshness.StatusExpired {
		t.Fatal("reopen after update was not stable")
	}

	// Delete the source: reopen must report unavailable.
	versions.delete(id)
	afterDelete, err := service.ChatHistory(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got := assistantOf(t, afterDelete).VersionStatus; got != freshness.StatusUnavailable {
		t.Fatalf("after delete = %q, want unavailable", got)
	}
	if freshness.DisplayedAsCurrentlyValid(assistantOf(t, afterDelete).VersionStatus) {
		t.Fatal("deleted source still displayed as currently valid")
	}
}

func TestETNewAnswerAfterUpdateIsCurrentWhileOldIsExpired(t *testing.T) {
	const id = "rss:freshness-2"
	versions := newMemoryVersions()
	versions.set(id, freshness.Current{Exists: true, Revision: 1, Hash: "h1"})
	service := chatFreshnessService(t, versions)
	first := sendChat(t, service, id, "第一问")

	versions.set(id, freshness.Current{Exists: true, Revision: 2, Hash: "h2"})
	history := append(append([]ChatMessage(nil), first.Messages...), ChatMessage{Role: "user", Content: "第二问"})
	second, err := service.Chat(context.Background(), ChatRequest{
		ContentID: id,
		Revision:  service.Status().Revision,
		Model:     "fixture",
		Messages:  history,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Messages) < 4 {
		t.Fatalf("want two turns, got %d messages", len(second.Messages))
	}
	oldAnswer, newAnswer := second.Messages[1], second.Messages[3]
	if oldAnswer.VersionStatus != freshness.StatusExpired {
		t.Fatalf("old answer = %q, want expired", oldAnswer.VersionStatus)
	}
	if newAnswer.VersionStatus != freshness.StatusCurrent || newAnswer.SourceRevision != 2 || newAnswer.SourceHash != "h2" {
		t.Fatalf("new answer = %+v", newAnswer)
	}
}

func TestETClientForgedFreshnessIsStripped(t *testing.T) {
	const id = "rss:freshness-forged"
	versions := newMemoryVersions()
	versions.set(id, freshness.Current{Exists: true, Revision: 1, Hash: "real"})
	service := chatFreshnessService(t, versions)
	result, err := service.Chat(context.Background(), ChatRequest{
		ContentID: id,
		Revision:  service.Status().Revision,
		Model:     "fixture",
		Messages: []ChatMessage{{
			Role:           "user",
			Content:        "伪造状态",
			VersionStatus:  freshness.StatusCurrent,
			SourceRevision: 99,
			SourceHash:     "forged-hash",
			Citations: []citation.Citation{{
				ID: "c1", Status: citation.StatusVerified, Freshness: freshness.StatusCurrent,
				ContentRevision: 99, ContentHash: "forged-hash",
			}},
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Messages[0].VersionStatus != "" || result.Messages[0].SourceHash != "" || result.Messages[0].SourceRevision != 0 || len(result.Messages[0].Citations) != 0 {
		t.Fatalf("forged user evidence was retained: %+v", result.Messages[0])
	}
}

func TestETLookupErrorFailsClosed(t *testing.T) {
	const id = "rss:freshness-err"
	versions := newMemoryVersions()
	versions.set(id, freshness.Current{Exists: true, Revision: 1, Hash: "h"})
	service := chatFreshnessService(t, versions)
	_ = sendChat(t, service, id, "问题")
	versions.err = errSentinel("disk failed")
	history, err := service.ChatHistory(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got := assistantOf(t, history).VersionStatus; got != freshness.StatusUnavailable {
		t.Fatalf("lookup error status = %q, want unavailable", got)
	}
	if freshness.DisplayedAsCurrentlyValid(assistantOf(t, history).VersionStatus) {
		t.Fatal("lookup error displayed as currently valid")
	}
}

func TestETUnconfiguredReaderFailsClosed(t *testing.T) {
	service := chatFreshnessService(t, nil)
	result := sendChat(t, service, "rss:unwired", "问题")
	if freshness.DisplayedAsCurrentlyValid(assistantOf(t, result).VersionStatus) {
		t.Fatal("unwired reader displayed answer as currently valid")
	}
}

func TestETConcurrentUpdateDuringReopen(t *testing.T) {
	const id = "rss:freshness-race"
	versions := newMemoryVersions()
	versions.set(id, freshness.Current{Exists: true, Revision: 1, Hash: "before"})
	service := chatFreshnessService(t, versions)
	_ = sendChat(t, service, id, "问题")

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < 20; i++ {
			_, _ = service.ChatHistory(context.Background(), id)
		}
	}()
	go func() {
		defer wg.Done()
		versions.set(id, freshness.Current{Exists: true, Revision: 2, Hash: "after"})
	}()
	wg.Wait()

	final, err := service.ChatHistory(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if assistantOf(t, final).VersionStatus != freshness.StatusExpired {
		t.Fatalf("after concurrent update, status = %q, want expired", assistantOf(t, final).VersionStatus)
	}
	if freshness.DisplayedAsCurrentlyValid(assistantOf(t, final).VersionStatus) {
		t.Fatal("concurrent update left evidence displayed as currently valid")
	}
}

func TestETApplyFreshnessOnStoredCitations(t *testing.T) {
	messages := []ChatMessage{{
		Role:           "assistant",
		Content:        "旧答案",
		SourceRevision: 1,
		SourceHash:     "old",
		VersionStatus:  freshness.StatusCurrent,
		Citations: []citation.Citation{{
			ID: "c1", ContentID: "rss:x", SourceVersion: "content-revision:1",
			ContentRevision: 1, ContentHash: "old", Status: citation.StatusVerified,
			Freshness: freshness.StatusCurrent, Quote: "原文",
		}},
	}}
	out := applyFreshness(messages, "rss:x", freshness.Current{Exists: true, Revision: 2, Hash: "new"}, nil)
	if out[0].VersionStatus != freshness.StatusExpired {
		t.Fatalf("message status = %q", out[0].VersionStatus)
	}
	if out[0].Citations[0].Freshness != freshness.StatusExpired {
		t.Fatalf("citation freshness = %q", out[0].Citations[0].Freshness)
	}
	if out[0].Citations[0].Status != citation.StatusVerified {
		t.Fatal("C generation-time status must be preserved")
	}
	if freshness.DisplayedAsCurrentlyValid(out[0].Citations[0].Freshness) {
		t.Fatal("expired citation displayed as currently valid")
	}
}

func TestETLegacyMessageWithoutSnapshotIsNotCurrent(t *testing.T) {
	messages := []ChatMessage{{Role: "assistant", Content: "上游旧对话，没有版本快照"}}
	out := applyFreshness(messages, "rss:legacy", freshness.Current{Exists: true, Revision: 3, Hash: "now"}, nil)
	if out[0].VersionStatus != freshness.StatusExpired {
		t.Fatalf("legacy status = %q, want expired", out[0].VersionStatus)
	}
}
