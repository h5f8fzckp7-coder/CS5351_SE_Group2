package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"

	"tangentry/internal/citation"
	"tangentry/internal/retrieval"
)

// comparisonSnapshotSource fixes the selected track and article version for
// retrieval, while range verification always reads the current saved source.
type comparisonSnapshotSource struct {
	ComparisonSource
	snapshot retrieval.Snapshot
}

func (s comparisonSnapshotSource) Snapshot(_ context.Context, id string) (retrieval.Snapshot, error) {
	if id != s.snapshot.ContentID {
		return retrieval.Snapshot{}, retrieval.ErrSourceNotFound
	}
	return s.snapshot, nil
}

func comparisonEvidenceFor(ctx context.Context, source ComparisonSource, engine *retrieval.Engine, question string,
	result *ComparisonSourceResult, maxChars, maxBytes, count int) (comparisonEvidence, []retrieval.Passage, error) {
	evidence := comparisonEvidence{ContentID: result.ContentID, Title: result.Title}
	snapshot, err := source.Snapshot(ctx, result.ContentID)
	if err != nil {
		result.Reason = comparisonFailureReason(err)
		return evidence, nil, ctx.Err()
	}
	if snapshot.ContentID != result.ContentID {
		result.Reason = "source_unverified"
		return evidence, nil, nil
	}
	evidence.snapshot = snapshot
	evidence.Kind = string(snapshot.Kind)
	if snapshot.Kind == retrieval.SourceSubtitle {
		evidence.Scope = "Saved subtitle excerpt only. complete means all saved text supplied, NEVER full-video coverage. Unprovided intervals and other parts are unknown."
		if item, itemErr := source.Get(ctx, result.ContentID); itemErr == nil {
			evidence.VideoDurationSeconds = float64(item.DurationSeconds)
		}
		first := true
		for _, track := range snapshot.Tracks {
			for _, cue := range track.Cues {
				start, end := float64(cue.StartMs)/1000, float64(cue.EndMs)/1000
				if first || start < evidence.CoverageStartSeconds {
					evidence.CoverageStartSeconds = start
				}
				if end > evidence.CoverageEndSeconds {
					evidence.CoverageEndSeconds = end
				}
				first = false
			}
		}
	}
	result.SourceVersion = snapshot.Version
	all, dropped := retrieval.SnapshotPassages(snapshot)
	if len(all) == 0 {
		result.Reason = "no_text"
		if dropped > 0 {
			result.Reason = "oversize_dropped"
		}
		return evidence, nil, nil
	}
	evidence.Passages = comparisonPassageTexts(all)
	evidence.Complete = dropped == 0
	encoded, _ := json.Marshal(evidence)
	passages := all
	if utf8.RuneCount(encoded) > maxChars || len(encoded) > maxBytes {
		evidence.Complete = false
		scoped := comparisonSnapshotSource{ComparisonSource: source, snapshot: snapshot}
		run, runErr := engine.ForSource(scoped, count).Run(ctx, retrieval.RunInput{
			ContentID: result.ContentID, Question: question,
			Request: retrieval.Request{SchemaVersion: 1, Enabled: true, MaxChars: maxChars},
		})
		result.Retrieval = &run.Response
		if runErr != nil {
			result.Reason = comparisonFailureReason(runErr)
			evidence.Passages = nil
			return evidence, nil, ctx.Err()
		}
		passages = run.Passages
		evidence.Passages = comparisonPassageTexts(passages)
		// The comparison JSON carries different metadata than B's chat context.
		// Recheck its exact wire size and drop only whole passages.
		for len(passages) > 0 {
			encoded, _ = json.Marshal(evidence)
			if utf8.RuneCount(encoded) <= maxChars && len(encoded) <= maxBytes {
				break
			}
			passages = passages[:len(passages)-1]
			evidence.Passages = comparisonPassageTexts(passages)
		}
	}
	verified, invalid, err := retrieval.VerifyPassages(ctx, source, passages)
	if err != nil {
		return evidence, nil, err
	}
	if len(invalid) > 0 {
		evidence.Complete = false
	}
	if len(verified) == 0 {
		result.Reason = "limited_search"
		if len(invalid) > 0 {
			result.Reason = invalid[0].Reason
		}
	}
	evidence.Passages = comparisonPassageTexts(verified)
	return evidence, verified, nil
}

func comparisonPassageTexts(passages []retrieval.Passage) []comparisonPassage {
	out := make([]comparisonPassage, 0, len(passages))
	for _, passage := range passages {
		out = append(out, comparisonPassage{ID: passage.ID, Text: passage.Text})
	}
	return out
}

func (rows *comparisonDraftSources) UnmarshalJSON(raw []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if len(bytes.TrimSpace(raw)) > 0 && bytes.TrimSpace(raw)[0] == '[' {
		var legacy []comparisonDraftSource
		if err := decoder.Decode(&legacy); err != nil {
			return err
		}
		*rows = legacy
		return nil
	}
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		return ErrFormat
	}
	seen := make(map[string]bool)
	var result comparisonDraftSources
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return err
		}
		id, ok := token.(string)
		if !ok || seen[id] {
			return ErrFormat
		}
		seen[id] = true
		var row comparisonDraftSource
		if err := decoder.Decode(&row); err != nil {
			return err
		}
		if row.ContentID != id {
			return ErrFormat
		}
		result = append(result, row)
	}
	if _, err := decoder.Token(); err != nil {
		return err
	}
	*rows = result
	return nil
}

func decodeComparison(raw string, ids []string) (comparisonDraft, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "~~~") {
		return comparisonDraft{}, ErrFormat
	}
	// Accept the common Markdown JSON fence without repairing model content.
	if strings.HasPrefix(raw, "```") && strings.HasSuffix(raw, "```") {
		lines := strings.Split(raw, "\n")
		if len(lines) < 3 || lines[0] != "```json" && lines[0] != "```" {
			return comparisonDraft{}, ErrFormat
		}
		raw = strings.Join(lines[1:len(lines)-1], "\n")
	}
	if len(raw) > 128<<10 || !utf8.ValidString(raw) {
		return comparisonDraft{}, ErrFormat
	}
	var draft comparisonDraft
	decoder := json.NewDecoder(bytes.NewBufferString(raw))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&draft) != nil || decoder.Decode(new(any)) != io.EOF || len(draft.Sources) != len(ids) {
		return comparisonDraft{}, ErrFormat
	}
	draft.Answer = strings.TrimSpace(draft.Answer)
	if draft.Answer == "" || utf8.RuneCountInString(draft.Answer) > 600 || strings.ContainsAny(draft.Answer, "\r\n\x00") || !comparisonSingleSentence(draft.Answer) {
		return comparisonDraft{}, ErrFormat
	}
	switch draft.Relation {
	case "", "same", "different", "conflict", "not_mentioned", "insufficient_evidence":
	default:
		return comparisonDraft{}, ErrFormat
	}
	byID := make(map[string]comparisonDraftSource, len(ids))
	allowed := make(map[string]bool, len(ids))
	for _, id := range ids {
		allowed[id] = true
	}
	if draft.AnswerSources != nil {
		used := make(map[string]bool, len(draft.AnswerSources))
		if len(draft.AnswerSources) == 0 || len(draft.AnswerSources) > len(ids) {
			return comparisonDraft{}, ErrFormat
		}
		for _, id := range draft.AnswerSources {
			if !allowed[id] || used[id] {
				return comparisonDraft{}, ErrFormat
			}
			used[id] = true
		}
	}
	for _, row := range draft.Sources {
		_, duplicate := byID[row.ContentID]
		if !allowed[row.ContentID] || duplicate || len(row.Fact) > 4000 || len(row.Citations) > 8 || len(row.Field) > 160 {
			return comparisonDraft{}, ErrFormat
		}
		if row.PassageIDs != nil {
			if row.Citations != nil || len(row.PassageIDs) > 8 || row.State != "supported" && len(row.PassageIDs) != 0 {
				return comparisonDraft{}, ErrFormat
			}
			seenPassages := make(map[string]bool, len(row.PassageIDs))
			for _, id := range row.PassageIDs {
				if id == "" || seenPassages[id] {
					return comparisonDraft{}, ErrFormat
				}
				seenPassages[id] = true
			}
		}
		switch row.State {
		case "supported", "not_mentioned", "not_relevant", "insufficient_evidence":
		default:
			return comparisonDraft{}, ErrFormat
		}
		byID[row.ContentID] = row
	}
	// Model row order is not authority. Restore the user's selection order
	// only after checking exact membership and rejecting duplicate identities.
	for i, id := range ids {
		draft.Sources[i] = byID[id]
	}
	return draft, nil
}

func validateComparisonFinding(ctx context.Context, source ComparisonSource, question string, row comparisonDraftSource,
	evidence comparisonEvidence, passages []retrieval.Passage, result *ComparisonSourceResult) {
	latest, err := source.Snapshot(ctx, result.ContentID)
	if err != nil {
		result.Reason = comparisonFailureReason(err)
		return
	}
	if latest.Version != evidence.snapshot.Version || retrieval.SnapshotHash(latest) != retrieval.SnapshotHash(evidence.snapshot) {
		result.Reason = "stale_source"
		return
	}
	if row.State == "not_relevant" {
		// Relevance is a model judgment, not proof of absence. Only allow an
		// explicit no-claim exclusion after all supplied evidence was read and
		// revalidated; it cannot bypass a pre-existing read/verification failure.
		if !evidence.Complete || len(passages) == 0 || row.Fact != "" || row.Field != "" || len(row.Citations) != 0 {
			result.Reason = "relevance_unconfirmed"
			return
		}
		verified, invalid, verifyErr := retrieval.VerifyPassages(ctx, source, passages)
		if verifyErr != nil || len(invalid) > 0 || len(verified) != len(passages) {
			result.Reason = "relevance_unconfirmed"
			return
		}
		result.State, result.Reason = "not_relevant", "question_unrelated"
		return
	}
	if row.State == "not_mentioned" {
		result.Coverage = comparisonAbsence(question, row.Field, evidence)
		if result.Coverage != nil {
			result.State = "not_mentioned"
		} else {
			result.Reason = "limited_search"
		}
		return
	}
	if row.State != "supported" || strings.TrimSpace(row.Fact) == "" || len(row.Citations) == 0 {
		result.Reason = "limited_search"
		return
	}
	var raw []citation.RawCitation
	for _, ref := range row.Citations {
		raw = append(raw, citation.RawCitation{PassageID: ref.PassageID, Quote: ref.Quote})
	}
	// Re-read after generation to catch edits while the provider was answering.
	verified, dropped, _ := retrieval.VerifyPassages(ctx, source, passages)
	result.Citations = citation.Validate(result.ContentID, raw, verified)
	for _, ref := range result.Citations {
		if ref.Status != citation.StatusVerified {
			result.Reason = "source_unverified"
			if len(dropped) > 0 {
				result.Reason = dropped[0].Reason
			}
			return
		}
	}
	result.State, result.Fact = "supported", strings.TrimSpace(row.Fact)
}

func comparisonAbsence(question, field string, evidence comparisonEvidence) *ComparisonCoverage {
	field = retrieval.NormalizeMatch(field)
	if !evidence.Complete || evidence.snapshot.Kind != retrieval.SourceArticle || field == "" ||
		!strings.Contains(retrieval.NormalizeMatch(question), field) {
		return nil
	}
	complete, fields := false, 0
	for _, block := range evidence.snapshot.Blocks {
		for _, line := range strings.Split(block.Text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			lower := retrieval.NormalizeMatch(line)
			if comparisonCompleteHeader(lower) {
				complete = true
				continue
			}
			name, _, found := strings.Cut(strings.ReplaceAll(line, "：", ":"), ":")
			name = retrieval.NormalizeMatch(name)
			if !found || name == "" || comparisonFieldKey(name) == comparisonFieldKey(field) ||
				strings.Contains(name, field) || strings.Contains(field, name) {
				return nil
			}
			fields++
		}
	}
	if !complete || fields == 0 {
		return nil
	}
	return &ComparisonCoverage{
		Field: field, SourceVersion: evidence.snapshot.Version,
		SnapshotHash: retrieval.SnapshotHash(evidence.snapshot), Blocks: len(evidence.snapshot.Blocks),
	}
}

// Only affirmative, explicit field-list declarations establish a complete
// scope. In particular "incomplete fields" or "不完整字段" is never proof.
func comparisonCompleteHeader(line string) bool {
	line = strings.TrimRight(strings.TrimSpace(line), ":：.。")
	switch line {
	case "complete fields", "all fields", "the following fields are the complete service card",
		"完整字段", "完整字段如下", "全部字段", "以下为此卡完整字段", "以下是完整规格字段",
		"本登记卡所列字段完整，仅包含下列字段", "本活动登记卡的完整字段如下", "本卡完整列出所有规格字段", "下列字段是本记录全部字段":
		return true
	}
	return false
}

func comparisonFieldKey(field string) string {
	key := strings.NewReplacer(" ", "", "-", "", "_", "").Replace(retrieval.NormalizeMatch(field))
	switch key {
	case "supportemail", "email", "emailaddress", "supportemailaddress", "客服邮箱", "支持邮箱", "电子邮箱", "电子邮件", "邮箱":
		return "support-email"
	case "deposit", "保证金", "押金":
		return "deposit"
	case "waterproofrating", "防水等级", "防水级别":
		return "waterproof-rating"
	case "discountcode", "promocode", "优惠码", "优惠代码", "折扣码":
		return "discount-code"
	case "onlineparticipationlink", "线上参与链接", "线上参会链接", "在线参会链接":
		return "online-link"
	}
	return key
}

// Short, invocation-scoped handles keep small models from confusing a long
// content ID with its passage ID. The model never supplies the native mapping.
func comparisonPromptEvidence(evidence []comparisonEvidence) ([]comparisonEvidence, map[string]string) {
	wire := make([]comparisonEvidence, len(evidence))
	handles := make(map[string]string)
	for i, source := range evidence {
		wire[i] = source
		wire[i].ContentID = "S" + strconv.Itoa(i+1)
		wire[i].Passages = make([]comparisonPassage, len(source.Passages))
		for j, passage := range source.Passages {
			handle := wire[i].ContentID + "-P" + strconv.Itoa(j+1)
			handles[handle] = passage.ID
			wire[i].Passages[j] = comparisonPassage{ID: handle, Text: passage.Text}
		}
	}
	return wire, handles
}
