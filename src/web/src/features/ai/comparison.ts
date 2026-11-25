import type { ContentItem } from '../../shared/api/types'
import type { AIBudgetReport } from './types'
import type { RetrievalResponse } from './retrieval'

export type ComparisonRelation = 'same' | 'different' | 'conflict' | 'not_mentioned' | 'insufficient_evidence'
export interface ComparisonCitation {
  id: string
  passageId: string
  contentId: string
  source: string
  sourceVersion: string
  locator: string
  quote: string
  startRune: number
  endRune: number
  status: 'verified' | 'invalid' | 'unverified' | 'stale' | 'ambiguous'
  reason?: string
}
export interface ComparisonSourceResult {
  contentId: string
  title: string
  state: 'supported' | 'not_mentioned' | 'not_relevant' | 'insufficient_evidence'
  reason?: string
  fact?: string
  sourceVersion?: string
  citations: ComparisonCitation[] | null
  coverage?: { field: string; sourceVersion: string; snapshotHash: string; blocks: number }
  retrieval?: RetrievalResponse
}
export interface ComparisonResult {
  status: string
  relation?: ComparisonRelation
  answer?: string
  answerSourceIds?: string[]
  sources: ComparisonSourceResult[]
  model?: string
  budget?: AIBudgetReport
}
export interface ComparisonEvidence {
  title: string
  contentId: string
  text: string
  startRune: number
  endRune: number
  locator: string
  startSeconds?: number
}
export function comparisonEligible(item: ContentItem) {
  return item.kind === 'article' || item.kind === 'video'
}

// A failed reopen updates only that source/version. The generated answer is
// always retained, and an old request cannot downgrade a newer run.
export function invalidateComparisonSource(result: ComparisonResult, ref: ComparisonCitation): ComparisonResult {
  const matches = (source: ComparisonSourceResult) => source.contentId === ref.contentId && source.sourceVersion === ref.sourceVersion
  if (!result.sources.some(matches)) return result
  return {
    ...result, relation: 'insufficient_evidence',
    sources: result.sources.map(source => matches(source) ? {
      ...source, state: 'insufficient_evidence', reason: 'stale_source', fact: undefined, coverage: undefined,
      citations: source.citations?.map(citation => ({ ...citation, status: 'stale', reason: 'stale_source' })) ?? [],
    } : source),
  }
}
export function comparisonReason(reason?: string) {
  const labels: Record<string, string> = {
    read_failed: '原文读取失败，请重试。',
    no_text: '没有可用的正文或原文字幕，请先打开内容保存正文或字幕。',
    track_required: '有多条原文字幕，请在比较来源中选择一条并保存用于比较。',
    limited_search: '现有片段不足以确认，请补充原文或换一个更明确的问题。',
    question_unrelated: '该来源未用于本次回答。',
    relevance_unconfirmed: '尚不能确认该来源与问题无关，请查看原文或重新比较。',
    no_match: '没有检索到可核验的相关片段。',
    stale_source: '原文已变更或不可读取，请重新比较。',
    source_unverified: '引用未通过来源核验。',
    cross_source_reference: '模型引用了其他来源的片段，无法用于这份内容的结论，请重新比较。',
    unsupported_source: '当前仅支持已保存文章和视频原文字幕。',
    context_budget_exceeded: '该来源超出上下文预算。',
    invalid_budget: '该来源的上下文预算不足，请减少问题长度。',
    no_verified_passages: '没有通过来源核验的片段。',
    source_unavailable: '该收藏已删除或暂不可读取，请刷新收藏列表。',
    check_failed: '来源检查失败，请点击重新检查来源。',
    oversize_dropped: '原文片段超过检索长度限制，目前没有可用的片段。',
  }
  return labels[reason ?? ''] ?? '当前来源的证据不足。'
}
