import { describe, expect, it } from 'vitest'
import { groupByDay, titleFrom } from './useChats.js'

describe('groupByDay', () => {
  it('labels today and yesterday and keeps list order inside a group', () => {
    const now = new Date('2026-10-13T12:00:00Z')
    const groups = groupByDay(
      [
        { id: 'a', updated_at: '2026-10-13T09:00:00Z' },
        { id: 'b', updated_at: '2026-10-13T08:00:00Z' },
        { id: 'c', updated_at: '2026-10-12T20:00:00Z' },
        { id: 'd', updated_at: '2026-10-02T20:00:00Z' },
      ],
      now,
    )
    expect(groups.map((g) => g.label)).toEqual(['Today', 'Yesterday', '2 Oct'])
    expect(groups[0].items.map((c) => c.id)).toEqual(['a', 'b'])
  })

  it('returns nothing for no chats', () => {
    expect(groupByDay([])).toEqual([])
  })
})

describe('titleFrom', () => {
  it('matches the server: spaces collapsed, cut at 60 characters', () => {
    expect(titleFrom('  How   did I\nsleep? ')).toBe('How did I sleep?')
    const long = 'a'.repeat(80)
    expect(titleFrom(long)).toBe('a'.repeat(59) + '…')
    expect([...titleFrom(long)].length).toBe(60)
  })
})
