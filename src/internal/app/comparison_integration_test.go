package app

import (
	"context"
	"encoding/json"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"

	"tangentry/internal/ai"
	"tangentry/internal/citation"
	"tangentry/internal/content"
	"tangentry/internal/subtitles"
)

// This fixture supplies deterministic model output. These are integration
// checks, not measurements of a real model's classification accuracy.
type comparisonIntegrationProvider struct{ calls atomic.Int32 }

func (p *comparisonIntegrationProvider) Translate(_ context.Context, _ ai.Config, in []ai.Segment) ([]ai.Segment, error) {
	return in, nil
}
func (p *comparisonIntegrationProvider) Chat(ctx context.Context, _ ai.Config, text string, _ []ai.ChatMessage) (string, error) {
	if err := ai.ReserveCompletion(ctx); err != nil {
		return "", err
	}
	p.calls.Add(1)
	var sources []struct {
		ContentID string `json:"contentId"`
		Passages  []struct {
			ID   string `json:"id"`
			Text string `json:"text"`
		} `json:"passages"`
	}
	_, raw, _ := strings.Cut(text, "\nSOURCES\n")
	if err := json.Unmarshal([]byte(raw), &sources); err != nil {
		return "", err
	}
	rows := map[string]any{}
	dependencies := []string{}
	for _, source := range sources {
		dependencies = append(dependencies, source.ContentID)
		row := map[string]any{"contentId": source.ContentID, "state": "insufficient_evidence", "passageIds": []string{}}
		if len(source.Passages) > 0 {
			passage := source.Passages[0]
			row["state"] = "supported"
			row["fact"] = passage.Text
			row["passageIds"] = []string{passage.ID}
		}
		rows[source.ContentID] = row
	}
	ai.RecordCompletionUsage(ctx, ai.Usage{Prompt: 5, Output: 5, Total: 10})
	result, _ := json.Marshal(map[string]any{"answer": "两个来源分别给出了不同的退款期限，需核对其适用条件。", "answerSources": dependencies, "relation": "conflict", "sources": rows})
	return string(result), nil
}

func TestComparisonHTTPWithSavedArticleAndSelectedOriginalCaptions(t *testing.T) {
	provider := &comparisonIntegrationProvider{}
	application := newIntegrationApp(t, provider)
	revision := configureIntegrationAI(t, application)
	body := integrationJSON(t, application, "PUT", "/api/ai/settings", map[string]any{
		"baseUrl": "https://ai-fixture.example/v1", "model": "fixture", "chatModel": "fixture", "enabledModels": []string{"fixture"}, "revision": revision,
	}, 200)
	var settings ai.Status
	if err := json.Unmarshal(body, &settings); err != nil {
		t.Fatal(err)
	}
	hash := strings.Repeat("a", 64)
	article := content.Item{ID: "rss:" + hash, Platform: "rss", PlatformID: hash, Kind: "article", Title: "Article policy", URL: "https://example.com/policy", Description: "Summary is not original evidence", ArticleBlocks: []content.ArticleBlock{{Kind: "paragraph", Text: "付款后 14 天内可申请退款😀。"}}}
	video := content.Item{ID: "youtube:Compare0001", Platform: "youtube", PlatformID: "Compare0001", Kind: "video", Title: "Video policy", URL: "https://www.youtube.com/watch?v=Compare0001"}
	third := content.Item{ID: "youtube:Compare0002", Platform: "youtube", PlatformID: "Compare0002", Kind: "video", Title: "Missing original captions", URL: "https://www.youtube.com/watch?v=Compare0002"}
	for _, item := range []content.Item{article, video, third} {
		integrationJSON(t, application, "POST", "/api/library", item, 200)
	}
	en := subtitles.Track{Language: "en", Label: "English", Source: "youtube", Cues: []subtitles.Cue{{Start: 12.5, End: 20, Text: "Refunds are accepted within 30 days."}}}
	zh := subtitles.Track{Language: "zh-Hans", Label: "中文", Source: "youtube", Cues: []subtitles.Cue{{Start: 0, End: 5, Text: "Unused original language."}}}
	translated := subtitles.Track{Language: "fr", Label: "French", Source: "youtube", Translated: true, SourceLanguage: "en", Cues: []subtitles.Cue{{Start: 0, End: 5, Text: "Never use translated evidence."}}}
	saveTrack := func(id string, track subtitles.Track, revision int) {
		integrationJSON(t, application, "PUT", "/api/saved-subtitles/"+url.PathEscape(id)+"/"+url.PathEscape(track.Language), map[string]any{"track": track, "revision": revision, "selected": true}, 200)
	}
	saveTrack(video.ID, zh, 0)
	saveTrack(video.ID, en, 0)
	saveTrack(video.ID, translated, 0)
	// Explicitly select the original English track after saving the others.
	saveTrack(video.ID, en, 1)
	saveTrack(third.ID, translated, 0)
	request := map[string]any{"contentIds": []string{article.ID, video.ID}, "question": "退款期限是否一致？", "revision": settings.Revision, "maxProviderCalls": 1}
	var result ai.ComparisonResult
	decode := func(raw []byte) {
		t.Helper()
		if err := json.Unmarshal(raw, &result); err != nil {
			t.Fatal(err)
		}
	}
	decode(integrationJSON(t, application, "POST", "/api/ai/compare", request, 200))
	if result.Relation != "conflict" || len(result.Sources) != 2 || result.Budget.Started != 1 {
		t.Fatalf("unexpected comparison: %+v", result)
	}
	if len(result.AnswerSourceIDs) != 2 || result.AnswerSourceIDs[0] != article.ID || result.AnswerSourceIDs[1] != video.ID {
		t.Fatalf("answer source handles were not restored to saved IDs: %+v", result)
	}
	for i, row := range result.Sources {
		if row.State != "supported" || len(row.Citations) != 1 || row.Citations[0].Status != citation.StatusVerified {
			t.Fatalf("source %d: %+v", i, row)
		}
	}
	if result.Sources[1].Fact != en.Cues[0].Text || result.Sources[0].Fact != article.ArticleBlocks[0].Text {
		t.Fatalf("wrong original sources: %+v", result.Sources)
	}
	ref := result.Sources[1].Citations[0]
	var evidence ai.ComparisonEvidenceResult
	if err := json.Unmarshal(integrationJSON(t, application, "POST", "/api/ai/compare/evidence", ref, 200), &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.Text != en.Cues[0].Text || evidence.StartSeconds == nil || *evidence.StartSeconds != 12.5 {
		t.Fatalf("wrong subtitle jump: %+v", evidence)
	}
	en.Cues[0].Text = "Edited original caption."
	saveTrack(video.ID, en, 2)
	integrationJSON(t, application, "POST", "/api/ai/compare/evidence", ref, 409)
	// Third source has only translated captions: retain valid facts but never
	// claim a full comparison succeeded using a translation as original evidence.
	request["contentIds"] = []string{article.ID, video.ID, third.ID}
	decode(integrationJSON(t, application, "POST", "/api/ai/compare", request, 200))
	if result.Relation != "insufficient_evidence" || result.Sources[2].Reason != "no_text" || result.Sources[0].State != "supported" {
		t.Fatalf("lost partial evidence: %+v", result)
	}
	if result.Answer != "两个来源分别给出了不同的退款期限，需核对其适用条件。" {
		t.Fatalf("missing original evidence hid the model answer: %+v", result)
	}
	calls := provider.calls.Load()
	request["contentIds"] = []string{article.ID, article.ID}
	integrationJSON(t, application, "POST", "/api/ai/compare", request, 400)
	request["contentIds"] = []string{article.ID, "youtube:Missing0001"}
	integrationJSON(t, application, "POST", "/api/ai/compare", request, 400)
	request["contentIds"] = []string{article.ID, video.ID}
	request["unexpected"] = true
	integrationJSON(t, application, "POST", "/api/ai/compare", request, 400)
	if provider.calls.Load() != calls {
		t.Fatal("invalid input contacted the provider")
	}
	// Comparison must not populate the single-content chat history.
	history := integrationJSON(t, application, "GET", "/api/ai/chat?contentId="+url.QueryEscape(article.ID), nil, 200)
	var chat ai.ChatResult
	if err := json.Unmarshal(history, &chat); err != nil || len(chat.Messages) != 0 {
		t.Fatalf("chat history altered: %s %v", history, err)
	}
}

// D-T01/D-T03: real HTTP + SQLite readiness works before AI configuration,
// retains unsupported images, and treats missing/deleted IDs generically.
func TestComparisonSourceInspectionHTTPWithoutAIConfiguration(t *testing.T) {
	provider := &comparisonIntegrationProvider{}
	application := newIntegrationApp(t, provider)
	hash := strings.Repeat("e", 64)
	article := content.Item{ID: "rss:" + hash, Platform: "rss", PlatformID: hash, Kind: "article", Title: "Readable policy", URL: "https://example.com/policy", ArticleBlocks: []content.ArticleBlock{{Kind: "paragraph", Text: "退款期限为 14 天。"}}}
	video := content.Item{ID: "youtube:ReadyTest01", Platform: "youtube", PlatformID: "ReadyTest01", Kind: "video", Title: "No saved captions", URL: "https://www.youtube.com/watch?v=ReadyTest01"}
	image := content.Item{ID: "xiaohongshu:6af5dcc3000000002500b6f5", Platform: "xiaohongshu", PlatformID: "6af5dcc3000000002500b6f5", Kind: "image", Title: "Image note", URL: "https://www.xiaohongshu.com/explore/6af5dcc3000000002500b6f5"}
	for _, item := range []content.Item{article, video, image} {
		integrationJSON(t, application, "POST", "/api/library", item, 200)
	}
	missing := "youtube:Missing0001"
	ids := []string{article.ID, video.ID, image.ID, missing}
	raw := integrationJSON(t, application, "POST", "/api/ai/compare/sources", map[string]any{"contentIds": ids}, 200)
	var result ai.ComparisonSourcesResult
	if err := json.Unmarshal(raw, &result); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"ready", "no_text", "unsupported_source", "source_unavailable"} {
		if len(result.Sources) != 4 || result.Sources[i].ContentID != ids[i] || result.Sources[i].State != want {
			t.Fatalf("inspection=%s", raw)
		}
	}
	integrationJSON(t, application, "POST", "/api/ai/compare/sources", map[string]any{"contentIds": []string{article.ID, article.ID}}, 400)
	integrationJSON(t, application, "POST", "/api/ai/compare/sources", map[string]any{"contentIds": ids, "unexpected": true}, 400)
	if provider.calls.Load() != 0 {
		t.Fatal("readiness invoked AI")
	}
}
