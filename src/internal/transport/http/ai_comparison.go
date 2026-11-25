package httptransport

import (
	"context"
	"errors"
	"net/http"

	"tangentry/internal/ai"
	"tangentry/internal/citation"
)

// Optional port preserves compatibility for existing AIService callers and
// focused test doubles that only implement ordinary single-content chat.
type comparisonService interface {
	Compare(context.Context, ai.CompareRequest) (ai.ComparisonResult, error)
	ComparisonEvidence(context.Context, citation.Citation) (ai.ComparisonEvidenceResult, error)
}

func (h *aiHandler) aiComparisonSources(w http.ResponseWriter, r *http.Request) {
	service, ok := h.ai.AI.(interface {
		InspectComparisonSources(context.Context, []string) (ai.ComparisonSourcesResult, error)
	})
	if !ok {
		write(w, 503, apiError{"ai_unavailable", "比较来源检查暂不可用。"})
		return
	}
	var input struct {
		ContentIDs []string `json:"contentIds"`
	}
	if !decodeAIRequest(w, r, &input, 32<<10) {
		return
	}
	result, err := service.InspectComparisonSources(r.Context(), input.ContentIDs)
	if err != nil {
		status, body := classifyChatError(err)
		if errors.Is(err, ai.ErrInvalid) {
			body = apiError{"invalid_comparison_sources", "请提交 1–100 个不同的有效收藏编号。"}
		}
		write(w, status, body)
		return
	}
	write(w, http.StatusOK, result)
}

func (h *aiHandler) aiCompare(w http.ResponseWriter, r *http.Request) {
	service, ok := h.ai.AI.(comparisonService)
	if !ok {
		write(w, 503, apiError{"ai_unavailable", "比较功能暂不可用。"})
		return
	}
	var input ai.CompareRequest
	if !decodeAIRequest(w, r, &input, 32<<10) {
		return
	}
	if err := setAILongWriteDeadline(w); err != nil && !errors.Is(err, http.ErrNotSupported) {
		chatFail(w, ai.ErrProvider)
		return
	}
	result, err := service.Compare(r.Context(), input)
	if err != nil {
		status, body := classifyChatError(err)
		if errors.Is(err, ai.ErrConflict) {
			status = http.StatusConflict
		}
		if errors.Is(err, ai.ErrFormat) {
			body.Message = "模型返回的比较结果格式无效，请重新比较。"
		}
		if errors.Is(err, ai.ErrInvalid) {
			body.Message = "请选择 2–3 份不同的已保存文章或视频，并填写有效问题和调用上限。所选内容必须可读取。"
		}
		if se, ok := retrievalStatusError(err); ok {
			body.Message = "比较上下文预算不足，请增加预算或减少问题长度。"
			body.Code = se.Status
			status = http.StatusBadRequest
		}
		write(w, status, struct {
			apiError
			Result ai.ComparisonResult `json:"result"`
		}{body, result})
		return
	}
	write(w, http.StatusOK, result)
}

func (h *aiHandler) aiComparisonEvidence(w http.ResponseWriter, r *http.Request) {
	service, ok := h.ai.AI.(comparisonService)
	if !ok {
		write(w, 503, apiError{"ai_unavailable", "比较功能暂不可用。"})
		return
	}
	var ref citation.Citation
	if !decodeAIRequest(w, r, &ref, 32<<10) {
		return
	}
	result, err := service.ComparisonEvidence(r.Context(), ref)
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, ai.ErrConflict) {
			status = http.StatusConflict
		}
		write(w, status, apiError{"comparison_evidence_unavailable", "原文已变更、已删除或引用无效，请重新比较。"})
		return
	}
	write(w, http.StatusOK, result)
}
