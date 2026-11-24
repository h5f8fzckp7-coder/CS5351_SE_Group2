import { useEffect, useRef, useState } from 'react'
import { Button, Select } from '../../components/ui'
import { request } from '../../shared/api/client'
import type { SavedSubtitle } from '../../shared/api/types'

export function comparisonSubtitleChoices(tracks: SavedSubtitle[]) {
  return tracks.filter(value => !value.track.translated && !value.track.mismatch && value.track.cues.length > 0)
}

export function comparisonSubtitleLabel(value: SavedSubtitle) {
  const track = value.track
  const origin = track.language.startsWith('whisper') ? '本机转录' : track.source === 'ai' ? '平台 AI 字幕' : '原文字幕'
  return `${track.label} · ${origin} · ${track.cues.length} 段`
}

export function ComparisonSubtitlePicker({ contentId, title, required, disabled, onSelected, onBusyChange }: {
  contentId: string
  title: string
  required: boolean
  disabled: boolean
  onSelected?: () => void
  onBusyChange?: (busy: boolean) => void
}) {
  const [expanded, setExpanded] = useState(required)
  const [tracks, setTracks] = useState<SavedSubtitle[]>([])
  const [language, setLanguage] = useState('')
  const [loading, setLoading] = useState(false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState('')
  const [refresh, setRefresh] = useState(0)
  const operation = useRef<AbortController | null>(null)
  const busyCallback = useRef(onBusyChange)
  busyCallback.current = onBusyChange

  useEffect(() => {
    if (!expanded) return
    const controller = new AbortController()
    setLoading(true)
    setError('')
    request<{ tracks: SavedSubtitle[] }>('/saved-subtitles?contentId=' + encodeURIComponent(contentId), { signal: controller.signal })
      .then(value => {
        if (controller.signal.aborted) return
        const originals = comparisonSubtitleChoices(value.tracks)
        setTracks(originals)
        setLanguage(originals.find(track => track.selected)?.track.language ?? '')
      }).catch(cause => {
        if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : '无法读取已保存字幕。')
      }).finally(() => { if (!controller.signal.aborted) setLoading(false) })
    return () => controller.abort()
  }, [contentId, expanded, refresh])

  useEffect(() => () => { operation.current?.abort(); busyCallback.current?.(false) }, [contentId])

  async function save() {
    const selected = tracks.find(track => track.track.language === language)
    if (!selected || disabled || loading || saving) return
    const controller = new AbortController()
    operation.current = controller
    setSaving(true)
    setError('')
    busyCallback.current?.(true)
    try {
      await request<SavedSubtitle>('/saved-subtitles/' + encodeURIComponent(contentId) + '/' + encodeURIComponent(language), {
        method: 'PUT', signal: controller.signal,
        body: JSON.stringify({ track: selected.track, revision: selected.revision, selected: true }),
      })
      if (!controller.signal.aborted) {
        setExpanded(false)
        onSelected?.()
      }
    } catch (cause) {
      if (!controller.signal.aborted) setError(cause instanceof Error ? cause.message : '字幕选择未保存，请重试。')
    } finally {
      busyCallback.current?.(false)
      if (!controller.signal.aborted) setSaving(false)
    }
  }

  return <div className="comparison-subtitle-picker">
    {!expanded ? <Button variant="quiet" disabled={disabled} onClick={() => setExpanded(true)}>选择比较字幕</Button> : <>
      <label>用于比较的字幕
        <Select aria-label={title + '的比较字幕'} value={language} disabled={disabled || loading || saving} onChange={event => setLanguage(event.target.value)}>
          <option value="">请选择已保存的原文字幕</option>
          {tracks.map(track => <option key={track.track.language} value={track.track.language}>{comparisonSubtitleLabel(track)}</option>)}
        </Select>
      </label>
      <span className="field-hint">平台字幕与本机转录只需选一条；前 60 秒转录只覆盖视频开头。</span>
      {loading && <span role="status">正在读取已保存字幕…</span>}
      {!loading && !error && tracks.length === 0 && <span role="status">暂无可用原文字幕，请先读取字幕或转录。</span>}
      <div className="comparison-source-refresh">
        <Button variant="secondary" disabled={disabled || loading || saving || !language} onClick={() => void save()}>{saving ? '正在保存…' : '保存用于比较'}</Button>
        <Button variant="quiet" disabled={disabled || loading || saving} onClick={() => setRefresh(value => value + 1)}>重新读取字幕</Button>
      </div>
      {error && <p role="alert">{error}</p>}
    </>}
  </div>
}
