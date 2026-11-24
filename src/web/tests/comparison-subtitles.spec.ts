import { expect, test } from './e2e'
import type { ContentItem, SavedSubtitle, SubtitleTrack } from '../src/shared/api/types'

test('selected subtitle changes source readiness and stale selection cannot overwrite it', async ({ request }) => {
  const id = 'youtube:TrackTest01'
  expect((await request.post('/api/library', { data: {
    id, platform: 'youtube', platformId: 'TrackTest01', kind: 'video', title: '字幕选择 API 回归',
    author: 'Fixture', authorId: '', url: 'https://www.youtube.com/watch?v=TrackTest01',
    thumbnailUrl: '', description: '', durationSeconds: 180, publishedAt: 1720000000,
  } })).ok()).toBe(true)
  const full: SubtitleTrack = { language: 'zh', label: '完整原文', automatic: false, cues: [{ start: 65, end: 70, text: '全文原文。' }] }
  const preview: SubtitleTrack = { language: 'whisper-preview', label: '前 60 秒', automatic: true, cues: [{ start: 0, end: 5, text: '开头预览。' }] }
  const url = '/api/saved-subtitles/' + encodeURIComponent(id) + '/'
  for (const track of [full, preview]) expect((await request.put(url + track.language, { data: { track, revision: 0 } })).ok()).toBe(true)
  const state = async () => {
    const response = await request.post('/api/ai/compare/sources', { data: { contentIds: [id] } })
    expect(response.ok()).toBe(true)
    return (await response.json()).sources[0]
  }
  expect((await state()).state).toBe('track_required')
  const selected = await request.put(url + full.language, { data: { track: full, revision: 1, selected: true } })
  expect(selected.ok()).toBe(true)
  expect((await selected.json()).selected).toBe(true)
  expect((await state()).state).toBe('ready')
  const stale = await request.put(url + full.language, { data: { track: { ...full, cues: preview.cues }, revision: 1, selected: true } })
  expect(stale.status()).toBe(409)
  const { tracks }: { tracks: SavedSubtitle[] } = await (await request.get('/api/saved-subtitles?contentId=' + encodeURIComponent(id))).json()
  expect(tracks.find(value => value.selected)?.track).toEqual(full)
  expect(tracks.find(value => value.track.language === preview.language)?.revision).toBe(1)
})

test('comparison selects a saved original subtitle inline, persists it and handles stale saves', async ({ page, request }) => {
  const video: ContentItem = {
    id: 'bilibili:BV15aytzcENP', platform: 'bilibili', platformId: 'BV15aytzcENP', kind: 'video',
    title: '字幕选择回归视频', author: 'Fixture', authorId: '', url: 'https://www.bilibili.com/video/BV15aytzcENP/',
    thumbnailUrl: '', description: '', durationSeconds: 180, publishedAt: 1720000000,
  }
  const article: ContentItem = {
    ...video, id: 'rss:' + 'e'.repeat(64), platform: 'rss', platformId: 'e'.repeat(64), kind: 'article',
    title: '字幕选择回归文章', url: 'https://example.com/subtitle-policy',
    articleBlocks: [{ kind: 'paragraph', text: '文章规定付款后 14 天内可退款。' }],
  }
  const original: SubtitleTrack = { language: 'ai-zh', label: '中文', automatic: true, source: 'ai', cues: [{ start: 65, end: 70, text: '平台原文字幕规定付款后 30 天内可退款。' }] }
  const preview: SubtitleTrack = { language: 'whisper-preview', label: '前 60 秒 · 本机语音转写', automatic: true, cues: [{ start: 0, end: 5, text: '仅在预览出现的文字。' }] }
  const tracks: SubtitleTrack[] = [original, preview,
    { ...original, language: 'translated-en', label: '不应选择的译文', source: 'youtube', translated: true, sourceLanguage: 'ai-zh' },
    { ...original, language: 'mismatch', label: '不应选择的错位字幕', mismatch: true },
  ]
  for (const item of [video, article]) expect((await request.post('/api/library', { data: item })).ok()).toBe(true)
  const subtitleURL = '/api/saved-subtitles/' + encodeURIComponent(video.id) + '/'
  const listURL = '/api/saved-subtitles?contentId=' + encodeURIComponent(video.id)
  for (const track of tracks) expect((await request.put(subtitleURL + track.language, { data: { track, revision: 0 } })).ok()).toBe(true)
  const settings = await (await request.get('/api/ai/settings')).json()
  expect((await request.put('/api/ai/settings', { data: {
    baseUrl: 'https://ai-fixture.example/v1', model: 'fixture-comparison', chatModel: 'fixture-comparison',
    enabledModels: ['fixture-comparison'], revision: settings.revision,
  } })).ok()).toBe(true)
  let comparisons = 0
  page.on('request', value => { if (value.method() === 'POST' && value.url().endsWith('/api/ai/compare')) comparisons++ })
  await page.goto('/library')
  await page.getByRole('button', { name: '比较内容', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '多内容比较' })
  const row = dialog.locator('.comparison-source-option').filter({ has: page.getByLabel(video.title, { exact: true }) })
  await dialog.getByLabel(video.title, { exact: true }).check()
  await dialog.getByLabel(article.title, { exact: true }).check()
  await dialog.getByLabel('比较问题', { exact: true }).fill('退款期限是否一致？')
  const picker = row.getByLabel(video.title + '的比较字幕', { exact: true })
  await expect(picker.locator('option')).toHaveCount(3)
  await expect(row).not.toContainText('不应选择')
  await expect(row.getByRole('link', { name: '选择原文字幕', exact: true })).toHaveCount(0)
  await expect(row.getByRole('button', { name: '保存用于比较', exact: true })).toBeDisabled()
  await picker.selectOption('ai-zh')
  await row.getByRole('button', { name: '保存用于比较', exact: true }).click()
  await expect(row).toContainText('已保存原文')
  const saved = async (): Promise<SavedSubtitle[]> => (await (await request.get(listURL)).json()).tracks
  expect((await saved()).find(value => value.selected)?.track).toEqual(original)
  expect((await saved()).find(value => value.track.language === preview.language)?.track).toEqual(preview)
  expect(comparisons).toBe(0)
  await expect(dialog).toContainText('2 份来源均有已保存原文')
  await dialog.getByRole('button', { name: '开始比较', exact: true }).click()
  const findings = dialog.getByRole('region', { name: '比较结果' })
  await expect(findings).toContainText(original.cues[0].text)
  await expect(findings).not.toContainText(preview.cues[0].text)
  expect(comparisons).toBe(1)

  // Switching sources clears the previous answer; no subtitles are deleted.
  await row.getByRole('button', { name: '选择比较字幕', exact: true }).click()
  await picker.selectOption('whisper-preview')
  await row.getByRole('button', { name: '保存用于比较', exact: true }).click()
  await expect(row).toContainText('已保存原文')
  await expect(findings).toHaveCount(0)
  expect((await saved()).find(value => value.selected)?.track).toEqual(preview)

  // A concurrent edit must not be overwritten using the picker's old revision.
  await row.getByRole('button', { name: '选择比较字幕', exact: true }).click()
  await expect(picker).toHaveValue('whisper-preview')
  const current = (await saved()).find(value => value.track.language === original.language)!
  expect((await request.put(subtitleURL + original.language, { data: { track: original, revision: current.revision, selected: true } })).ok()).toBe(true)
  await picker.selectOption('ai-zh')
  await row.getByRole('button', { name: '保存用于比较', exact: true }).click()
  await expect(row.getByRole('alert')).toBeVisible()
  expect((await saved()).find(value => value.selected)?.revision).toBe(current.revision + 1)
  await row.getByRole('button', { name: '重新读取字幕', exact: true }).click()
  await expect(row.getByRole('alert')).toHaveCount(0)
  await row.getByRole('button', { name: '保存用于比较', exact: true }).click()
  await expect(row).toContainText('已保存原文')
  await page.reload()
  await page.getByRole('button', { name: '比较内容', exact: true }).click()
  await row.getByRole('button', { name: '选择比较字幕', exact: true }).click()
  await expect(picker).toHaveValue('ai-zh')
  await page.setViewportSize({ width: 390, height: 844 })
  expect(await dialog.evaluate(element => element.scrollWidth > element.clientWidth)).toBe(false)
})
