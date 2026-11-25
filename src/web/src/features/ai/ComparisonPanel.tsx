import { useEffect, useRef, useState } from 'react'
import { ArrowLeftRight } from 'lucide-react'
import { Button, Dialog, Input, Select } from '../../components/ui'
import { APIError, request } from '../../shared/api/client'
import type { ContentItem } from '../../shared/api/types'
import type { AISettings } from './types'
import { CallLimitField } from './CallLimitField'
import { budgetSummaryText, callLimitField, parseCallLimit } from './callBudget'
import {
  comparisonEligible, comparisonReason, invalidateComparisonSource,
  type ComparisonCitation, type ComparisonEvidence, type ComparisonResult,
} from './comparison'
import './comparison.css'
import { ComparisonSourceOption } from './ComparisonSourceOption'
import { comparisonReadyCount, loadComparisonSources, type ComparisonSourceStatus } from './comparisonSources'

export function ComparisonFindings({ result, onEvidence }: {
  result: ComparisonResult
  onEvidence: (ref: ComparisonCitation) => void
}) {
  return <section className="comparison-result" aria-label="比较结果">
    <div className="comparison-result-heading">
      <h3>比较回答</h3>
      {result.model && <span>{result.model}</span>}
    </div>
    <p className="comparison-answer">{result.answer || '暂未获得模型回答，请查看下方提示。'}</p>
    <p className="field-hint">比较结论由模型判断；引文已核验仅表示来源、版本及原文匹配通过。</p>
    {result.budget && <p role="status">{budgetSummaryText(result.budget)}</p>}
    <div className="comparison-findings">
      {result.sources.map(source => <article className="comparison-finding" key={source.contentId} aria-label={source.title}>
        <h4>{source.title}</h4>
        {result.answer && result.answerSourceIds && !result.answerSourceIds.includes(source.contentId) && source.state !== 'not_relevant' && <p className="field-hint">未用于本次回答</p>}
        <p>{source.state === 'supported' ? source.fact : source.state === 'not_mentioned' ? '未提及' : source.state === 'not_relevant' ? '与本次问题无关' : '证据不足'}</p>
        {source.reason && <p className="field-hint">{comparisonReason(source.reason)}</p>}
        {source.coverage && <p className="field-hint">完整检查 {source.coverage.blocks} 个原文区块，未列出「{source.coverage.field}」。</p>}
        {(source.citations ?? []).map((ref, index) => <div key={index}>
          <blockquote>{ref.quote}</blockquote>
          {ref.status === 'verified'
            ? <Button variant="quiet" onClick={() => onEvidence(ref)}>查看原文 · 已核验</Button>
            : <span className="field-hint">引用未通过核验</span>}
        </div>)}
        {source.retrieval && <p className="field-hint">
          检索方式 {source.retrieval.mode} · 嵌入 {source.retrieval.embedding.requests} 次
        </p>}
      </article>)}
    </div>
  </section>
}

export function ComparisonPanel({ items }: { items: ContentItem[] }) {
  const [open, setOpen] = useState(false)
  const [selected, setSelected] = useState<string[]>([])
  const [filter, setFilter] = useState('')
  const [question, setQuestion] = useState('')
  const [limit, setLimit] = useState('')
  const [model, setModel] = useState('')
  const [settings, setSettings] = useState<AISettings | null>(null)
  const [loading, setLoading] = useState(false)
  const [pending, setPending] = useState(false)
  const [error, setError] = useState('')
  const [result, setResult] = useState<ComparisonResult | null>(null)
  const [resultQuestion, setResultQuestion] = useState('')
  const [evidence, setEvidence] = useState<ComparisonEvidence | null>(null)
  const [evidenceError, setEvidenceError] = useState('')
  const [loadingEvidence, setLoadingEvidence] = useState(false)
  const [sources, setSources] = useState<Record<string, ComparisonSourceStatus>>({})
  const [checkingSources, setCheckingSources] = useState(false)
  const [preparingTracks, setPreparingTracks] = useState<Record<string, boolean>>({})
  const [sourcesError, setSourcesError] = useState('')
  const [refresh, setRefresh] = useState(0)
  const operation = useRef<AbortController | null>(null)
  const evidenceOperation = useRef<AbortController | null>(null)
  const evidenceView = useRef<HTMLElement | null>(null)
  const parsedLimit = parseCallLimit(limit)
  const eligible = items.filter(comparisonEligible)
  const sourceKey = JSON.stringify(items.map(item => item.id).sort())
  const readyCount = comparisonReadyCount(selected, sources)
  const preparingSources = Object.values(preparingTracks).some(Boolean)
  const models = (settings?.enabledModels ?? []).filter(value => !/embed|rerank|whisper/i.test(value))

  useEffect(() => {
    if (!open) return
    const controller = new AbortController()
    setLoading(true)
    setSettings(null)
    setError('')
    request<AISettings>('/ai/settings', { signal: controller.signal }).then(value => {
      setSettings(value)
      setModel(current => value.enabledModels?.includes(current) ? current : value.chatModel || value.model)
    }).catch(cause => {
      if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : '无法读取 AI 设置。')
    }).finally(() => { if (!controller.signal.aborted) setLoading(false) })
    return () => controller.abort()
  }, [open, refresh])

  useEffect(() => {
    if (!open) return
    const controller = new AbortController()
    const ids = JSON.parse(sourceKey) as string[]
    setCheckingSources(true)
    setSources({})
    setSourcesError('')
    loadComparisonSources(ids, controller.signal).then(value => {
      if (!controller.signal.aborted) setSources(value)
    }).catch(cause => {
      if (!controller.signal.aborted) {
        setSourcesError(cause instanceof Error ? cause.message : '无法检查来源，请重试。')
        setSources(Object.fromEntries(ids.map(contentId => [contentId, { contentId, state: 'check_failed', passages: 0 }])))
      }
    }).finally(() => { if (!controller.signal.aborted) setCheckingSources(false) })
    return () => controller.abort()
  }, [open, sourceKey, refresh])

  useEffect(() => {
    const available = new Set(items.filter(comparisonEligible).map(item => item.id))
    setSelected(current => current.filter(id => available.has(id)))
  }, [items])

  useEffect(() => () => { operation.current?.abort(); evidenceOperation.current?.abort() }, [])

  useEffect(() => {
    if (evidence) {
      evidenceView.current?.focus()
      evidenceView.current?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
    }
  }, [evidence])

  function close() {
    operation.current?.abort()
    evidenceOperation.current?.abort()
    setPending(false)
    setLoadingEvidence(false)
    setCheckingSources(false)
    setOpen(false)
  }

  function select(id: string) {
    evidenceOperation.current?.abort()
    setLoadingEvidence(false)
    setEvidenceError('')
    setError('')
    setSelected(current => current.includes(id) ? current.filter(value => value !== id) : current.length < 3 ? [...current, id] : current)
    setResult(null)
    setEvidence(null)
  }

  async function compare() {
    if (!settings?.ready || pending || checkingSources || preparingSources || readyCount === 0 || !parsedLimit.ok || selected.length < 2 || selected.length > 3 || !question.trim() || !models.includes(model)) return
    operation.current?.abort()
    evidenceOperation.current?.abort()
    const controller = new AbortController()
    operation.current = controller
    setPending(true)
    setError('')
    setResult(null)
    setEvidence(null)
    setEvidenceError('')
    setResultQuestion(question.trim())
    try {
      const response = await request<ComparisonResult>('/ai/compare', {
        method: 'POST', signal: controller.signal,
        body: JSON.stringify({ contentIds: selected, question: question.trim(), model, revision: settings.revision, ...callLimitField(parsedLimit.value) }),
      })
      if (!controller.signal.aborted) setResult(response)
    } catch (cause) {
      if (controller.signal.aborted) return
      setError(cause instanceof Error ? cause.message : '比较失败，请重试。')
      if (cause instanceof APIError) {
        const partial = (cause.payload as { result?: ComparisonResult } | undefined)?.result
        if (partial?.sources?.length) setResult(partial)
      }
    } finally {
      if (!controller.signal.aborted) setPending(false)
    }
  }

  async function showEvidence(ref: ComparisonCitation) {
    evidenceOperation.current?.abort()
    const controller = new AbortController()
    evidenceOperation.current = controller
    setEvidence(null)
    setEvidenceError('')
    setLoadingEvidence(true)
    try {
      const value = await request<ComparisonEvidence>('/ai/compare/evidence', {
        method: 'POST', signal: controller.signal, body: JSON.stringify(ref),
      })
      if (!controller.signal.aborted) setEvidence(value)
    } catch (cause) {
      if (!controller.signal.aborted) {
        setEvidenceError(cause instanceof Error ? cause.message : '无法读取原文。')
        if (cause instanceof APIError && (cause.status === 400 || cause.status === 409)) {
          setResult(current => current ? invalidateComparisonSource(current, ref) : current)
        }
      }
    } finally {
      if (!controller.signal.aborted) setLoadingEvidence(false)
    }
  }

  const text = Array.from(evidence?.text ?? '')
  return <>
    <Button variant="secondary" onClick={() => setOpen(true)}><ArrowLeftRight size={16} aria-hidden="true"/>比较内容</Button>
    <Dialog open={open} title="多内容比较" onClose={close} className="comparison-dialog">
      <div className="comparison-workspace">
        <div className="comparison-source-heading"><h3>选择 2–3 份已保存文章或视频</h3><span role="status">已选 {selected.length}/3</span></div>
        <p className="field-hint">比较读取已保存的文章正文或原文字幕。没有字幕的视频需先读取字幕或转录；本版暂不支持图文图片、PDF 和音频。准备文字后，返回并点击「重新检查来源」。</p>
        <div className="comparison-source-refresh">
          <Button variant="quiet" disabled={pending || checkingSources || preparingSources} onClick={() => setRefresh(value => value + 1)}>重新检查来源</Button>
          {checkingSources && <span role="status">正在检查已保存的文字…</span>}
        </div>
        {sourcesError && <p role="alert">{sourcesError}</p>}
        <Input aria-label="搜索比较来源" type="search" placeholder="搜索标题" value={filter} onChange={event => setFilter(event.target.value)} disabled={pending}/>
        <div className="comparison-selection" role="group" aria-label="比较来源">
          {items.filter(item => item.title.toLocaleLowerCase().includes(filter.toLocaleLowerCase())).map(item =>
            <ComparisonSourceOption key={item.id} item={item} status={sources[item.id]} checked={selected.includes(item.id)}
              disabled={pending || preparingSources || selected.length === 3 && !selected.includes(item.id)} onChange={() => select(item.id)}
              onPreparing={busy => setPreparingTracks(current => current[item.id] === busy ? current : { ...current, [item.id]: busy })}
              onPrepared={() => {
                evidenceOperation.current?.abort()
                setLoadingEvidence(false)
                setResult(null)
                setEvidence(null)
                setEvidenceError('')
                setRefresh(value => value + 1)
              }}/>
          )}
          {!eligible.length && <p>先在阅读页收藏至少两份文章或视频。</p>}
        </div>
        {selected.length > 0 && <p className="field-hint">已选 {selected.map(id => items.find(item => item.id === id)?.title).join('、')}</p>}
        {!checkingSources && selected.length >= 2 && <p role="status" className="field-hint">
          {readyCount === 0 ? '所选来源均没有已确认可用的原文，请先准备正文或字幕，再重新检查。'
            : readyCount < selected.length ? `当前仅 ${readyCount} 份来源有可用原文；可继续查看部分证据，提交时会重新检查。` : `${readyCount} 份来源均有已保存原文。`}
        </p>}
        <label className="comparison-question">比较问题
          <textarea aria-label="比较问题" maxLength={12000} rows={3} placeholder="例如：这些材料规定的退款期限一致吗？"
            value={question} onChange={event => setQuestion(event.target.value)} disabled={pending}/>
        </label>
        {loading ? <p role="status">正在读取模型设置…</p> : !settings?.ready || !models.length
          ? <p><a href="/tools#ai/text">请先配置 AI 服务并启用对话模型。</a></p>
          : <label className="comparison-model">对话模型<Select aria-label="比较模型" value={model} disabled={pending} onChange={event => setModel(event.target.value)}>
            {models.map(value => <option key={value} value={value}>{value}</option>)}
          </Select></label>}
        <fieldset className="comparison-limit-field" disabled={pending}><CallLimitField id="comparison-call-limit" value={limit} onChange={setLimit} error={parsedLimit.ok ? '' : parsedLimit.message}/></fieldset>
        <div className="comparison-actions">
          <Button variant="primary" disabled={pending || loading || checkingSources || preparingSources || readyCount === 0 || !settings?.ready || !models.includes(model) || selected.length < 2 || !question.trim() || !model || !parsedLimit.ok}
            onClick={() => void compare()}>{pending ? '正在比较…' : '开始比较'}</Button>
          {pending && <Button variant="secondary" onClick={() => { operation.current?.abort(); setPending(false); setError('已取消比较。') }}>取消比较</Button>}
        </div>
        {error && <p role="alert" className="comparison-error">{error}</p>}
        {result && <><p className="comparison-result-question">{resultQuestion}</p><ComparisonFindings result={result} onEvidence={ref => void showEvidence(ref)}/></>}
        {loadingEvidence && <p role="status">正在核对当前原文…</p>}
        {evidenceError && <p role="alert">{evidenceError}</p>}
        {evidence && <section ref={evidenceView} className="comparison-evidence" aria-label="引用原文" tabIndex={-1}>
          <h3>{evidence.title}</h3>
          <p className="field-hint">{evidence.locator}{evidence.startSeconds !== undefined ? ' · ' + evidence.startSeconds.toFixed(1) + ' 秒' : ''}</p>
          <p>{text.slice(0, evidence.startRune).join('')}<mark>{text.slice(evidence.startRune, evidence.endRune).join('')}</mark>{text.slice(evidence.endRune).join('')}</p>
        </section>}
      </div>
    </Dialog>
  </>
}
