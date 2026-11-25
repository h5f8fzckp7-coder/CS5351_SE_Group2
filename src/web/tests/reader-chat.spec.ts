import { expect, test } from './e2e'
import type { ContentItem, SavedSubtitle, SubtitleTrack } from '../src/shared/api/types'

test('manual YouTube subtitles replace local captions, survive reopening and supply streamed chat and summaries', async ({ page, request }) => {
  const item: ContentItem = {
    id: 'youtube:N6FkmdL0Lg0', platform: 'youtube', platformId: 'N6FkmdL0Lg0', kind: 'video',
    title: 'How Factory is building autonomous software with Claude', author: 'Claude', authorId: '',
    url: 'https://www.youtube.com/watch?v=N6FkmdL0Lg0', thumbnailUrl: '', description: 'Description without selected captions',
    durationSeconds: 120, publishedAt: 1720000000,
  }
  const english: SubtitleTrack = { language: 'en', label: 'English', automatic: false, source: 'youtube', cues: [{ start: 0, end: 5, text: 'MANUALLY CHOSEN English captions for this conversation.' }] }
  const chinese: SubtitleTrack = { language: 'zh-Hans', label: '中文', automatic: true, source: 'youtube', translated: true, sourceLanguage: 'en', cues: [{ start: 0, end: 5, text: '自动选择的旧中文内容。' }] }
  const previous = { ...english, cues: [{ start: 0, end: 3, text: 'Old saved English captions.' }] }
  expect((await request.post('/api/library', { data: item })).ok()).toBe(true)
  expect((await request.put(`/api/saved-subtitles/${encodeURIComponent(item.id)}/en`, { data: { track: previous, revision: 0 } })).ok()).toBe(true)
  expect((await request.put(`/api/saved-subtitles/${encodeURIComponent(item.id)}/zh-Hans`, { data: { track: chinese, revision: 0 } })).ok()).toBe(true)
  const settings = await (await request.get('/api/ai/settings')).json()
  expect((await request.put('/api/ai/settings', { data: {
    baseUrl: 'https://ai-fixture.example/v1', model: 'fixture-reader-chat', enabledModels: ['fixture-reader-chat'],
    chatModel: 'fixture-reader-chat', key: '', revision: settings.revision,
  } })).ok()).toBe(true)
  await page.route('**/api/subtitles/youtube?**', route => route.fulfill({ json: { videoId: item.platformId, part: 1, status: 'available', languages: [chinese, english], tracks: [chinese, english] } }))
  await page.setViewportSize({ width: 1844, height: 1098 })
  await page.goto('/reader?contentId=' + encodeURIComponent(item.id))
  const cues = page.getByRole('list', { name: '字幕段落' })
  await expect(cues).toContainText(chinese.cues[0].text)
  await page.getByRole('button', { name: '字幕来源与保存', exact: true }).click()
  await page.getByLabel('读取字幕来源').selectOption('platform')
  await page.getByRole('button', { name: '关闭字幕来源', exact: true }).click()
  await page.getByLabel('字幕语言', { exact: true }).selectOption('en')
  await expect(cues).toContainText(english.cues[0].text)
  await expect.poll(async () => {
    const { tracks }: { tracks: SavedSubtitle[] } = await (await request.get('/api/saved-subtitles?contentId=' + encodeURIComponent(item.id))).json()
    return tracks.find(value => value.selected)?.track
  }).toEqual(english)
  await page.reload()
  await expect(page.getByLabel('字幕语言', { exact: true })).toHaveValue('en')
  await expect(cues).toContainText(english.cues[0].text)
  await expect(cues).not.toContainText(chinese.cues[0].text)
  await page.getByRole('group', { name: '阅读区域' }).getByRole('button', { name: 'AI 辅助', exact: true }).click()
  await page.getByRole('tab', { name: 'AI 对话', exact: true }).click()
  const chat = page.locator('.ai-chat-panel')
  await expect(chat.getByLabel('对话模型')).toHaveValue('fixture-reader-chat')
  const outgoing = page.waitForRequest(value => value.method() === 'POST' && value.url().endsWith('/api/ai/chat'))
  await chat.getByRole('button', { name: '这个讲了什么', exact: true }).click()
  const sent = await outgoing
  expect(new URL(sent.url()).search).toBe('')
  expect(sent.headers().accept).toBe('text/event-stream')
  expect(sent.postDataJSON().context).toContain(english.cues[0].text)
  expect(sent.postDataJSON().context).not.toContain(chinese.cues[0].text)
  await expect(chat.getByRole('article', { name: 'AI 回答' })).toContainText(english.cues[0].text)
  await expect(chat.getByRole('alert')).toHaveCount(0)
  await expect(chat.getByRole('button', { name: '复制回答' })).toBeVisible()
  await page.reload()
  await page.getByRole('group', { name: '阅读区域' }).getByRole('button', { name: 'AI 辅助', exact: true }).click()
  await page.getByRole('tab', { name: 'AI 对话', exact: true }).click()
  await expect(chat.getByRole('article', { name: 'AI 回答' })).toContainText(english.cues[0].text)
  await page.setViewportSize({ width: 390, height: 844 })
  await expect(chat.getByLabel('输入问题')).toBeVisible()
  expect(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth)).toBe(false)
})

test('reader title translates on click and toggles without a separate translation control', async ({ page, request }) => {
  const settings = await (await request.get('/api/ai/settings')).json()
  expect((await request.put('/api/ai/settings', { data: { baseUrl: 'https://ai-fixture.example/v1', model: 'fixture-title', key: '', revision: settings.revision } })).ok()).toBe(true)
  const id = 'youtube:TitleTest01'
  expect((await request.post('/api/library', { data: {
    id, platform: 'youtube', platformId: 'TitleTest01', kind: 'video',
    title: 'An original English title', author: 'Author', authorId: '', url: 'https://www.youtube.com/watch?v=TitleTest01', thumbnailUrl: '', description: '', durationSeconds: 10, publishedAt: 1720000000,
  } })).ok()).toBe(true)
  await page.goto('/reader?contentId=' + encodeURIComponent(id))
  const title = page.locator('.reading-nav-title h1')
  await expect(title.getByRole('button')).toBeEnabled()
  await title.getByRole('button').click()
  await expect(title).toContainText('这是第1句的中文译文。')
  await title.getByRole('button').click()
  await expect(title).toContainText('An original English title')
  await title.getByRole('button').click()
  await expect(title).toContainText('这是第1句的中文译文。')
  await expect(page.locator('.reading-nav-title .text-translation-control')).toHaveCount(0)
})

test('saved content comparison verifies original passages and rejects an edited source on reopen', async ({ page, request }) => {
  const settings = await (await request.get('/api/ai/settings')).json()
  expect((await request.put('/api/ai/settings', { data: {
    baseUrl: 'https://ai-fixture.example/v1', model: 'fixture-comparison', chatModel: 'fixture-comparison',
    enabledModels: ['fixture-comparison'], revision: settings.revision,
  } })).ok()).toBe(true)
  const items: ContentItem[] = ['a', 'b', 'c', 'd'].map((letter, index) => ({
    id: 'rss:' + letter.repeat(64), platform: 'rss', platformId: letter.repeat(64), kind: 'article',
    title: `比较测试资料 ${index + 1}`, author: 'Fixture', authorId: '', url: `https://example.com/policy-${letter}`,
    thumbnailUrl: '', description: '', durationSeconds: 0, publishedAt: 1720000000,
    articleBlocks: [{ kind: 'paragraph', text: `付款后 ${14 + index * 16} 天内可申请退款😀。` }],
  }))
  for (const item of items) expect((await request.post('/api/library', { data: item })).ok()).toBe(true)
  await page.goto('/library')
  await page.getByRole('button', { name: '比较内容', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '多内容比较' })
  const start = dialog.getByRole('button', { name: '开始比较', exact: true })
  await expect(start).toBeDisabled()
  await dialog.getByLabel(items[0].title, { exact: true }).check()
  await dialog.getByLabel(items[1].title, { exact: true }).check()
  await dialog.getByLabel(items[2].title, { exact: true }).check()
  await expect(dialog.getByLabel(items[3].title, { exact: true })).toBeDisabled()
  await dialog.getByLabel(items[2].title, { exact: true }).uncheck()
  await dialog.getByLabel('比较问题', { exact: true }).fill('退款期限是否一致？')
  await dialog.getByLabel('调用上限', { exact: true }).fill('1')
  await start.click()
  const findings = dialog.getByRole('region', { name: '比较结果' })
  await expect(findings.getByRole('heading', { name: '比较回答', exact: true })).toBeVisible()
  await expect(findings).toContainText('两份资料分别规定了不同的退款期限')
  await expect(findings.getByRole('article')).toHaveCount(2)
  await expect(findings).toContainText('已发起 1/1 次')
  await findings.getByRole('button', { name: '查看原文 · 已核验' }).first().click()
  const original = dialog.getByRole('region', { name: '引用原文' })
  await expect(original.locator('mark')).toHaveText(items[0].articleBlocks![0].text)
  // Reopening re-reads the source, rather than trusting the earlier result.
  const changed = { ...items[0], articleBlocks: [{ kind: 'paragraph', text: '原文已修改。' }] }
  expect((await request.post('/api/library', { data: changed })).ok()).toBe(true)
  await findings.getByRole('button', { name: '查看原文 · 已核验' }).first().click()
  await expect(dialog.getByRole('alert')).toContainText('原文已变更')
  await expect(original).toHaveCount(0)
  await expect(findings.locator('.comparison-answer')).toContainText('两份资料分别规定了不同的退款期限')
  await page.setViewportSize({ width: 390, height: 844 })
  expect(await dialog.evaluate(element => element.scrollWidth > element.clientWidth)).toBe(false)
})

// D-T01/D-T03: readiness and preparation cover the missing-subtitle journey.
test('comparison explains unsupported notes and refreshes saved captions before partial comparison', async ({ page, request }) => {
  const settings = await (await request.get('/api/ai/settings')).json()
  expect((await request.put('/api/ai/settings', { data: {
    baseUrl: 'https://ai-fixture.example/v1', model: 'fixture-comparison', chatModel: 'fixture-comparison',
    enabledModels: ['fixture-comparison'], revision: settings.revision,
  } })).ok()).toBe(true)
  const videos: ContentItem[] = ['ReadyTest01', 'ReadyTest02'].map((id, index) => ({
    id: 'youtube:' + id, platform: 'youtube', platformId: id, kind: 'video', title: `字幕准备测试 ${index + 1}`,
    author: 'Fixture', authorId: '', url: 'https://www.youtube.com/watch?v=' + id, thumbnailUrl: '',
    description: '', durationSeconds: 60, publishedAt: 1720000000,
  }))
  const note: ContentItem = {
    ...videos[0], id: 'xiaohongshu:6af5dcc3000000002500b6f5', platform: 'xiaohongshu',
    platformId: '6af5dcc3000000002500b6f5', kind: 'image', title: '不支持的图文测试',
    url: 'https://www.xiaohongshu.com/explore/6af5dcc3000000002500b6f5',
  }
  for (const item of [...videos, note]) expect((await request.post('/api/library', { data: item })).ok()).toBe(true)
  await page.goto('/library')
  await page.getByRole('button', { name: '比较内容', exact: true }).click()
  const dialog = page.getByRole('dialog', { name: '多内容比较' })
  await expect(dialog.getByLabel(note.title, { exact: true })).toBeDisabled()
  await expect(dialog).toContainText('图文/图片暂不支持比较')
  await dialog.getByLabel(videos[0].title, { exact: true }).check()
  await dialog.getByLabel(videos[1].title, { exact: true }).check()
  await dialog.getByLabel('比较问题', { exact: true }).fill('退款期限是否一致？')
  const start = dialog.getByRole('button', { name: '开始比较', exact: true })
  await expect(dialog).toContainText('所选来源均没有已确认可用的原文')
  await expect(start).toBeDisabled()
  await expect(dialog.getByRole('link', { name: '准备原文字幕' }).first()).toHaveAttribute('href', '/reader?contentId=' + encodeURIComponent(videos[0].id))
  const track: SubtitleTrack = { language: 'zh', label: '中文', automatic: false, source: 'youtube', cues: [{ start: 12, end: 20, text: '付款后 14 天内可以申请退款。' }] }
  expect((await request.put('/api/saved-subtitles/' + encodeURIComponent(videos[0].id) + '/zh', { data: { track, revision: 0, selected: true } })).ok()).toBe(true)
  await dialog.getByRole('button', { name: '重新检查来源', exact: true }).click()
  await expect(dialog).toContainText('仅 1 份来源有可用原文')
  await expect(start).toBeEnabled()
  await start.click()
  const results = dialog.getByRole('region', { name: '比较结果' })
  await expect(results).toContainText('两份资料分别规定了不同的退款期限，适用条件需要结合原文确认。')
  await expect(results.getByRole('article', { name: videos[0].title, exact: true })).toContainText(track.cues[0].text)
  await expect(results.getByRole('article', { name: videos[1].title, exact: true })).toContainText('没有可用的正文或原文字幕')
  await expect(results.getByRole('button', { name: '查看原文 · 已核验' })).toHaveCount(1)
})
