import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import { ComparisonFindings } from './ComparisonPanel'
import { comparisonEligible, invalidateComparisonSource, type ComparisonCitation, type ComparisonResult } from './comparison'
import type { ContentItem } from '../../shared/api/types'

const reference: ComparisonCitation = {
  id: 'citation-1', passageId: 'fixture#article:0:s0:0-10', contentId: 'fixture', source: 'article',
  sourceVersion: '1', locator: 'article:0:s0', quote: '付款后14天😀。', startRune: 0, endRune: 9, status: 'verified',
}
function render(result: ComparisonResult) {
  return renderToStaticMarkup(createElement(ComparisonFindings, { result, onEvidence: () => {} }))
}

describe('comparison source findings', () => {
  it('shows the answer even when every source is unverified', () => {
    const answer = '现有材料不足以确定售价。'
    const html = render({ status: 'completed', answer, answerSourceIds: ['fixture'], sources: [
      { contentId: 'fixture', title: '无法核验的文章', state: 'insufficient_evidence', reason: 'source_unverified', citations: [{ ...reference, status: 'invalid' }] },
      { contentId: 'missing', title: '缺少正文', state: 'insufficient_evidence', reason: 'no_text', citations: [] },
    ] })
    expect(html).toContain(answer)
    expect(html).toContain('引用未通过来源核验')
    expect(html).toContain('没有可用的正文')
    expect(html).not.toContain('查看原文 · 已核验')
  })
  it('preserves a scoped answer and identifies unused sources with failures', () => {
    const answer = '第二篇推荐了电影、剧集、动画和纪录片。'
    const result: ComparisonResult = { status: 'completed', answer, answerSourceIds: ['film'], sources: [
      { contentId: 'fixture', title: '其他文章', sourceVersion: '1', state: 'supported', fact: '其他事实', citations: [reference] },
      { contentId: 'film', title: '影视推荐', sourceVersion: '1', state: 'supported', fact: '电影和剧集', citations: [{ ...reference, contentId: 'film' }] },
      { contentId: 'failed', title: '暂不可用的文章', state: 'insufficient_evidence', reason: 'read_failed', citations: [] },
    ] }
    const html = render(result)
    expect(html).toContain(answer)
    expect(html).toContain('未用于本次回答')
    expect(html).toContain('原文读取失败')
    const changed = invalidateComparisonSource(result, reference)
    expect(changed.answer).toBe(answer)
    expect(changed.sources[0].state).toBe('insufficient_evidence')
    expect(invalidateComparisonSource(result, { ...reference, contentId: 'film' }).answer).toBe(answer)
  })
  it('keeps a focused answer while explaining unrelated sources', () => {
    const answer = '根据游戏评测那篇文章，主题是游戏体验与作品评价。'
    const html = render({ status: 'completed', answer, sources: [
      { contentId: 'other', title: '其他文章', state: 'not_relevant', reason: 'question_unrelated', citations: [] },
      { contentId: 'fixture', title: '游戏评测', state: 'supported', fact: '介绍游戏体验', citations: [reference] },
    ] })
    expect(html).toContain(answer)
    expect(html).toContain('与本次问题无关')
    expect(html).toContain('该来源未用于本次回答')
    expect(html).not.toContain('证据不足')
    expect(html.match(/查看原文 · 已核验/g)).toHaveLength(1)
  })
  it('escapes the generated answer as text', () => {
    const html = render({ status: 'completed', answer: '<script>alert(1)</script>', sources: [] })
    expect(html).toContain('&lt;script&gt;')
    expect(html).not.toContain('<script>')
  })
  it('shows the model sentence instead of a categorical verdict', () => {
    const answer = '两篇文章主题不一致，第一篇介绍应用开发，第二篇推荐影视作品。'
    const html = render({ status: 'completed', answer, relation: 'different', sources: [
      { contentId: 'fixture', title: '应用开发', state: 'supported', fact: '开发实用应用', citations: [reference] },
      { contentId: 'other', title: '影视推荐', state: 'supported', fact: '介绍影视作品', citations: [{ ...reference, contentId: 'other' }] },
    ] })
    expect(html).toContain(answer)
    expect(html).toContain('<h3>比较回答</h3>')
    expect(html).not.toContain('<h3>不同</h3>')
    expect(html).not.toContain('<h3>相同</h3>')
    expect(html).not.toContain('<h3>冲突</h3>')
    expect(html).toContain('比较结论由模型判断')
  })
  it('explains a cross-source citation without presenting it as verified', () => {
    const html = render({ status: 'completed', relation: 'insufficient_evidence', sources: [
      { contentId: 'fixture', title: '鸿蒙应用', state: 'insufficient_evidence', reason: 'cross_source_reference', citations: [{ ...reference, status: 'invalid' }] },
    ] })
    expect(html).toContain('模型引用了其他来源的片段')
    expect(html).not.toContain('查看原文 · 已核验')
    expect(html).toContain('引用未通过核验')
  })
  it('keeps failed sources alongside valid facts and enables only verified jumps', () => {
    const html = render({ status: 'completed', relation: 'insufficient_evidence', sources: [
      { contentId: 'fixture', title: '退款规则', state: 'supported', fact: '14天', citations: [reference] },
      { contentId: 'missing', title: '失败的资料', state: 'insufficient_evidence', reason: 'no_text', citations: null },
      { contentId: 'stale', title: '过期引用', state: 'insufficient_evidence', citations: [{ ...reference, status: 'stale' }] },
    ] })
    expect(html).toContain('证据不足')
    expect(html).toContain('失败的资料')
    expect(html).toContain('没有可用的正文或原文字幕')
    expect(html).toContain(reference.quote)
    expect(html.match(/查看原文 · 已核验/g)).toHaveLength(1)
    expect(html).toContain('引用未通过核验')
  })
  it('renders absence only with a clearly identified complete field scope', () => {
    const html = render({ status: 'completed', relation: 'not_mentioned', sources: [
      { contentId: 'fixture', title: '<script>untrusted</script>', state: 'not_mentioned', citations: [], coverage: { field: 'support email', blocks: 2, sourceVersion: '1', snapshotHash: 'hash' } },
    ] })
    expect(html).toContain('完整检查 2 个原文区块')
    expect(html).toContain('support email')
    expect(html).toContain('&lt;script&gt;')
    expect(html).not.toContain('<script>')
  })
  it('limits the selection to articles and original video subtitle sources', () => {
    for (const kind of ['article', 'video']) expect(comparisonEligible({ kind } as ContentItem)).toBe(true)
    for (const kind of ['pdf', 'audio', 'image']) expect(comparisonEligible({ kind } as ContentItem)).toBe(false)
  })
  it('invalidates a changed source on reopen while preserving the other source', () => {
    const other = { contentId: 'other', title: '另一来源', state: 'supported' as const, fact: '30天', sourceVersion: '1', citations: [{ ...reference, contentId: 'other' }] }
    const result: ComparisonResult = { status: 'completed', answer: '两份资料给出的退款期限不一致。', relation: 'conflict', sources: [
      { contentId: reference.contentId, title: '变更来源', state: 'supported', fact: '14天', sourceVersion: '1', citations: [reference] }, other,
    ] }
    const changed = invalidateComparisonSource(result, reference)
    expect(changed.relation).toBe('insufficient_evidence')
    expect(changed.answer).toBe(result.answer)
    expect(render(changed)).toContain(result.answer)
    expect(changed.sources[0].state).toBe('insufficient_evidence')
    expect(changed.sources[0].fact).toBeUndefined()
    expect(changed.sources[0].citations?.[0].status).toBe('stale')
    expect(changed.sources[1]).toBe(other)
    expect(invalidateComparisonSource(result, { ...reference, sourceVersion: 'old' })).toBe(result)
  })
})
