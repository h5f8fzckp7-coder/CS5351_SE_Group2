import { createElement } from 'react'
import { renderToStaticMarkup } from 'react-dom/server'
import { describe, expect, it } from 'vitest'
import type { ContentItem } from '../../shared/api/types'
import { ComparisonSourceOption } from './ComparisonSourceOption'
import { comparisonReadyCount, type ComparisonSourceStatus } from './comparisonSources'
import { comparisonSubtitleChoices, comparisonSubtitleLabel } from './ComparisonSubtitlePicker'
import type { SavedSubtitle } from '../../shared/api/types'

const video = { id: 'xiaohongshu:6af5dcc3000000002500b6f5', kind: 'video', platform: 'xiaohongshu', title: '测试视频' } as ContentItem

describe('D-T01 / D-T03 comparison source preparation', () => {
  it('shows an unsupported image instead of silently hiding the saved item', () => {
    const html = renderToStaticMarkup(createElement(ComparisonSourceOption, {
      item: { ...video, kind: 'image', title: '测试图文' }, checked: false, disabled: false, onChange: () => {},
    }))
    expect(html).toContain('测试图文')
    expect(html).toContain('图文/图片暂不支持比较')
    expect(html).toContain('disabled')
  })
  it('explains missing Xiaohongshu captions and provides a reader preparation link', () => {
    const html = renderToStaticMarkup(createElement(ComparisonSourceOption, {
      item: video, status: { contentId: video.id, kind: 'video', state: 'no_text', passages: 0 },
      checked: true, disabled: false, onChange: () => {},
    }))
    expect(html).toContain('需先转录或导入原文字幕')
    expect(html).toContain('下载视频不会自动生成字幕')
    expect(html).toContain('/reader?contentId=' + encodeURIComponent(video.id))
    expect(html).toContain('准备原文字幕')
  })
  it('counts only server-checked ready sources while retaining a missing source', () => {
    const states: Record<string, ComparisonSourceStatus> = {
      a: { contentId: 'a', kind: 'article', state: 'ready', passages: 2 },
      b: { contentId: 'b', kind: 'video', state: 'no_text', passages: 0 },
    }
    expect(comparisonReadyCount(['a', 'b'], states)).toBe(1)
    expect(comparisonReadyCount(['b', 'unknown'], states)).toBe(0)
    expect(comparisonReadyCount([], states)).toBe(0)
  })
  it('offers an inline subtitle picker when the original track is ambiguous', () => {
    const html = renderToStaticMarkup(createElement(ComparisonSourceOption, {
      item: video, status: { contentId: video.id, kind: 'video', state: 'track_required', passages: 0 },
      checked: true, disabled: false, onChange: () => {},
    }))
    expect(html).toContain('用于比较的字幕')
    expect(html).toContain('保存用于比较')
    expect(html).toContain('前 60 秒转录只覆盖视频开头')
    expect(html).not.toContain('/reader?contentId=')
  })
  it('excludes translated, mismatched and empty tracks while distinguishing preview coverage', () => {
    const original: SavedSubtitle = { contentId: video.id, revision: 1, savedAt: 1,
      track: { language: 'ai-zh', label: '中文', automatic: true, source: 'ai', cues: [{ start: 0, end: 3, text: '原文' }] } }
    const preview: SavedSubtitle = { ...original, track: { ...original.track, language: 'whisper-preview', label: '前 60 秒 · 本机语音转写' } }
    const tracks = [original, preview,
      { ...original, track: { ...original.track, translated: true } },
      { ...original, track: { ...original.track, mismatch: true } },
      { ...original, track: { ...original.track, cues: [] } },
    ]
    expect(comparisonSubtitleChoices(tracks)).toEqual([original, preview])
    expect(comparisonSubtitleLabel(original)).toContain('平台 AI 字幕 · 1 段')
    expect(comparisonSubtitleLabel(preview)).toContain('前 60 秒')
  })
})
