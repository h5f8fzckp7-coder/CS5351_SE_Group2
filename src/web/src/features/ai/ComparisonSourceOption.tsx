import { Checkbox } from '../../components/ui'
import type { ContentItem } from '../../shared/api/types'
import { comparisonEligible, comparisonReason } from './comparison'
import type { ComparisonSourceStatus } from './comparisonSources'
import { ComparisonSubtitlePicker } from './ComparisonSubtitlePicker'

export function ComparisonSourceOption({ item, status, checked, disabled, onChange, onPrepared, onPreparing }: {
  item: ContentItem
  status?: ComparisonSourceStatus
  checked: boolean
  disabled: boolean
  onChange: () => void
  onPrepared?: () => void
  onPreparing?: (busy: boolean) => void
}) {
  const eligible = comparisonEligible(item)
  const state = status?.state
  const needsText = eligible && (state === 'no_text' || state === 'track_required')
  const type = item.kind === 'video' ? '原文字幕' : item.kind === 'article' ? '文章正文'
    : item.kind === 'image' ? '图文/图片' : item.kind === 'pdf' ? 'PDF' : '音频'
  const detail = !eligible ? `${type}暂不支持比较。`
    : !status ? '正在检查已保存的文字…'
    : state === 'ready' ? `已保存原文 · ${status.passages} 个可读取片段`
    : state === 'no_text' && item.kind === 'video'
      ? item.platform === 'xiaohongshu' || item.platform === 'douyin'
        ? '需先转录或导入原文字幕；下载视频不会自动生成字幕。'
        : '尚未保存原文字幕，请先读取平台字幕或转录。'
      : state === 'no_text' ? '尚未保存文章正文，请先在阅读页读取正文。'
      : comparisonReason(state)
  return <div className="comparison-source-option">
    <label className="comparison-option">
      <Checkbox aria-label={item.title} checked={checked} disabled={disabled || !eligible} onChange={onChange}/>
      <span>{item.title}</span><small>{type}</small>
    </label>
    <div className="comparison-source-detail">
      <span className="field-hint">{detail}</span>
      {needsText && state !== 'track_required' && <a href={'/reader?contentId=' + encodeURIComponent(item.id)} target="_blank" rel="noopener noreferrer">
        {item.kind === 'video' ? '准备原文字幕' : '准备正文'}
      </a>}
      {needsText && item.kind === 'video' && state === 'no_text' && <a href="/tools#ai/asr" target="_blank" rel="noopener noreferrer">字幕转录设置</a>}
    </div>
    {item.kind === 'video' && (state === 'track_required' || state === 'ready') && <ComparisonSubtitlePicker
      contentId={item.id} title={item.title} required={state === 'track_required'} disabled={disabled}
      onSelected={onPrepared} onBusyChange={onPreparing}/>}
  </div>
}
