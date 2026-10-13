import { describe, expect, it } from 'vitest'
import { useChat } from './useChat.js'

// A transport that records what it was handed and answers immediately.
function recordingTransport(answer = 'An answer.', chart = null) {
  const calls = []
  const transport = async (chatId, question, { onStep, onChart, onAnswer }) => {
    calls.push({ chatId, question })
    if (chart) onChart(chart)
    onStep({ key: 'garmin/get_sleep_data', source: 'garmin', tool: 'get_sleep_data', state: 'active' })
    onStep({ key: 'garmin/get_sleep_data', source: 'garmin', tool: 'get_sleep_data', state: 'done' })
    onAnswer(answer)
  }
  return { transport, calls }
}

async function opened(transport, options) {
  const chat = useChat(transport, options)
  await chat.open('chat-1')
  return chat
}

describe('sending', () => {
  it('appends a user turn and a completed agent turn', async () => {
    const { transport, calls } = recordingTransport('Yes, clearly.\n\nFour short nights, four slow runs.')
    const { turns, send } = await opened(transport)
    await send('does sleep matter?')
    expect(calls).toEqual([{ chatId: 'chat-1', question: 'does sleep matter?' }])
    expect(turns.value).toHaveLength(2)
    expect(turns.value[0]).toMatchObject({ role: 'user', text: 'does sleep matter?' })
    expect(turns.value[1]).toMatchObject({ role: 'agent', status: 'complete' })
    expect(turns.value[1].answerText).toMatch(/^Yes, clearly\./)
    expect(turns.value[1].blocks[0]).toMatchObject({ type: 'lede', text: 'Yes, clearly.' })
  })

  it('refuses to send without an open chat', async () => {
    const { transport, calls } = recordingTransport()
    const { send } = useChat(transport)
    await send('q')
    expect(calls).toHaveLength(0)
  })

  it('labels steps from the source and tool and flips state in place', async () => {
    const { transport } = recordingTransport()
    const { turns, send } = await opened(transport)
    await send('q')
    expect(turns.value[1].steps).toEqual([
      { id: 'garmin/get_sleep_data', label: 'Checking your Garmin sleep', state: 'done' },
    ])
  })

  it('is busy while a reply is in flight and ignores a second send', async () => {
    let release
    const transport = (id, q, { onAnswer }) =>
      new Promise((resolve) => {
        release = () => {
          onAnswer('done')
          resolve()
        }
      })
    const { busy, send, turns } = await opened(transport)
    const first = send('one')
    expect(busy.value).toBe(true)
    await send('two')
    expect(turns.value).toHaveLength(2)
    release()
    await first
    expect(busy.value).toBe(false)
  })
})

describe('charts', () => {
  it('holds the chart on the turn and keeps the turn working until the answer', async () => {
    const chart = { kind: 'bar', title: 'Weekly km' }
    let seenWhileWorking
    const transport = async (id, q, { onChart, onAnswer }) => {
      onChart(chart)
      seenWhileWorking = { chart: turns.value[1].chart, status: turns.value[1].status }
      onAnswer('Prose.')
    }
    const { turns, send } = await opened(transport)
    await send('q')
    // ADR 0009: the card never renders before its prose, so the turn stays
    // "working" (steps shown) even though the chart has already arrived.
    expect(seenWhileWorking).toEqual({ chart, status: 'working' })
    expect(turns.value[1].status).toBe('complete')
    expect(turns.value[1].chart).toEqual(chart)
  })

  it('leaves chart null when the transport sends none', async () => {
    const { transport } = recordingTransport()
    const { turns, send } = await opened(transport)
    await send('q')
    expect(turns.value[1].chart).toBeNull()
  })
})

describe('failures', () => {
  it('turns a thrown transport error into an error turn with its message', async () => {
    const transport = async () => {
      throw new Error('daily cap reached; it resets at midnight UTC')
    }
    const { turns, send, busy } = await opened(transport)
    await send('q')
    expect(turns.value[1]).toMatchObject({ status: 'error', error: 'daily cap reached; it resets at midnight UTC' })
    expect(busy.value).toBe(false)
  })

  it('marks a transport that ends without an answer as an error', async () => {
    const { turns, send } = await opened(async () => {})
    await send('q')
    expect(turns.value[1].status).toBe('error')
    expect(turns.value[1].error).toMatch(/without an answer/)
  })
})

describe('opening a saved chat', () => {
  it('rebuilds the thread from the stored messages, charts included', async () => {
    const loadMessages = async () => [
      { role: 'user', content: 'How did I sleep?' },
      { role: 'assistant', content: 'Badly.\n\nFour short nights.', chart: { kind: 'bar' } },
    ]
    const { turns } = await opened(recordingTransport().transport, { loadMessages })
    expect(turns.value).toHaveLength(2)
    expect(turns.value[1]).toMatchObject({ role: 'agent', status: 'complete', chart: { kind: 'bar' } })
    expect(turns.value[1].blocks[0].text).toBe('Badly.')
  })

  it('ignores a second open of the chat already open', async () => {
    let loads = 0
    const loadMessages = async () => {
      loads++
      return []
    }
    const chat = useChat(recordingTransport().transport, { loadMessages })
    await chat.open('a')
    await chat.send('q')
    await chat.open('a')
    expect(loads).toBe(1)
    expect(chat.turns.value).toHaveLength(2)
  })

  it('keeps only the newest open when chats switch while loading', async () => {
    const resolvers = {}
    const loadMessages = (id) => new Promise((resolve) => (resolvers[id] = resolve))
    const chat = useChat(recordingTransport().transport, { loadMessages })
    const a = chat.open('a')
    const b = chat.open('b')
    resolvers.b([{ role: 'user', content: 'from b' }])
    resolvers.a([{ role: 'user', content: 'from a' }])
    await Promise.all([a, b])
    expect(chat.turns.value.map((t) => t.text)).toEqual(['from b'])
  })
})
