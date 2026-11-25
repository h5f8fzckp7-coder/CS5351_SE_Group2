import { request } from '../../shared/api/client'

export interface ComparisonSourceStatus {
  contentId: string
  kind?: string
  state: string
  passages: number
}

export function comparisonReadyCount(ids: string[], statuses: Record<string, ComparisonSourceStatus>) {
  return ids.filter(id => statuses[id]?.state === 'ready').length
}

// Read-only preparation checks. Neither this endpoint nor retries invoke AI.
export async function loadComparisonSources(ids: string[], signal: AbortSignal) {
  const sources: Record<string, ComparisonSourceStatus> = {}
  for (let offset = 0; offset < ids.length; offset += 100) {
    const result = await request<{ sources: ComparisonSourceStatus[] }>('/ai/compare/sources', {
      method: 'POST', signal, body: JSON.stringify({ contentIds: ids.slice(offset, offset + 100) }),
    })
    for (const source of result.sources) sources[source.contentId] = source
  }
  return sources
}
