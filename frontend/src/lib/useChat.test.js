import { describe, expect, it, vi } from 'vitest'
import { useChat } from './useChat.js'

// The history builder is the whole of the conversation's memory — the server keeps
// no session state. A bug here degrades follow-ups silently: nothing throws, the
// model just loses the thread. Hence the coverage.

// A transport that records what it was handed and answers immediately.
function recordingTransport(answer = 'An answer.') {
  const calls = []
  const transport = async (question, { onStep, onAnswer, history }) => {
    calls.push({ question, history })
    onStep({ key: 'strava/list_activities', source: 'strava', tool: 'list_activities', state: 'active' })
    onStep({ key: 'strava/list_activities', source: 'strava', tool: 'list_activities', state: 'done' })
    onAnswer(answer)
  }
  return { transport, calls }
}

describe('history', () => {
  it('sends nothing on the first question', async () => {
    const { transport, calls } = recordingTransport()
    const { send } = useChat(transport)
    await send('first question')
    expect(calls[0].history).toEqual([])
  })

  it('sends completed exchanges, oldest first, on a follow-up', async () => {
    const { transport, calls } = recordingTransport('The answer.')
    const { send } = useChat(transport)
    await send('q1')
    await send('q2')
    await send('q3')

    expect(calls[2].history).toEqual([
      { question: 'q1', answer: 'The answer.' },
      { question: 'q2', answer: 'The answer.' },
    ])
  })

  it('excludes the in-flight turn — the model would see the question twice', async () => {
    const { transport, calls } = recordingTransport()
    const { send } = useChat(transport)
    await send('q1')
    await send('q2')
    expect(calls[1].history.map((h) => h.question)).not.toContain('q2')
    expect(calls[1].question).toBe('q2')
  })

  it('replays the raw answer text, marker included', async () => {
    // The parsed blocks drop the figure marker, so history must come from the raw
    // text or the replayed answer would differ from what was actually given.
    const raw = 'Lede.\n\n[figure: 6:44 | avg pace]\n\nClosing.'
    const { transport, calls } = recordingTransport(raw)
    const { send } = useChat(transport)
    await send('q1')
    await send('q2')
    expect(calls[1].history[0].answer).toBe(raw)
  })

  it('leaves a failed turn with no answer text, which is what excludes it', async () => {
    const { turns, send } = useChat(async () => {
      throw new Error('backend down')
    })
    await send('doomed question')

    expect(turns.value[1].status).toBe('error')
    expect(turns.value[1].answerText).toBe('')
  })

  it('omits failed exchanges from a later question’s history', async () => {
    let shouldFail = true
    const calls = []
    const transport = async (question, { onAnswer, history }) => {
      calls.push({ question, history })
      if (shouldFail) throw new Error('backend down')
      onAnswer('recovered answer')
    }
    const { send } = useChat(transport)
    await send('failed question')
    shouldFail = false
    await send('working question')
    await send('third question')

    const questions = calls[2].history.map((h) => h.question)
    expect(questions).toEqual(['working question'])
    expect(questions).not.toContain('failed question')
  })
})

describe('turns', () => {
  it('appends a user turn then an agent turn', async () => {
    const { transport } = recordingTransport()
    const { turns, send } = useChat(transport)
    await send('how far did I run?')

    expect(turns.value).toHaveLength(2)
    expect(turns.value[0]).toMatchObject({ role: 'user', text: 'how far did I run?' })
    expect(turns.value[1]).toMatchObject({ role: 'agent', status: 'complete' })
  })

  it('parses the answer into blocks', async () => {
    const { transport } = recordingTransport('Lede.\n\n[figure: 6:44 | avg pace]\n\nClosing.')
    const { turns, send } = useChat(transport)
    await send('q')
    expect(turns.value[1].blocks.map((b) => b.type)).toEqual(['lede', 'figure', 'p'])
  })

  it('flips a step in place instead of appending a duplicate', async () => {
    const { transport } = recordingTransport()
    const { turns, send } = useChat(transport)
    await send('q')
    // The transport sends active then done for the same key.
    expect(turns.value[1].steps).toHaveLength(1)
    expect(turns.value[1].steps[0].state).toBe('done')
  })

  it('labels steps from the source and tool', async () => {
    const { transport } = recordingTransport()
    const { turns, send } = useChat(transport)
    await send('q')
    expect(turns.value[1].steps[0].label).toBe('Reading your Strava training log')
  })
})

describe('failure handling', () => {
  it('surfaces the transport error on the turn', async () => {
    const { turns, send } = useChat(async () => {
      throw new Error('Can’t reach the coach')
    })
    await send('q')
    expect(turns.value[1].status).toBe('error')
    expect(turns.value[1].error).toBe('Can’t reach the coach')
  })

  it('does not leave steps pulsing when a transport ends without answering', async () => {
    // Otherwise the UI reads as "still working" when nothing is.
    const { turns, send } = useChat(async (_q, { onStep }) => {
      onStep({ key: 'garmin/get_sleep_data', source: 'garmin', tool: 'get_sleep_data', state: 'active' })
    })
    await send('q')
    expect(turns.value[1].status).toBe('error')
    expect(turns.value[1].error).toMatch(/without an answer/i)
  })

  it('always says something, even for an error with no message', async () => {
    const { turns, send } = useChat(async () => {
      throw new Error('')
    })
    await send('q')
    expect(turns.value[1].error).toBeTruthy()
  })
})

describe('busy', () => {
  it('is true while in flight and false afterwards', async () => {
    let release
    const gate = new Promise((r) => (release = r))
    const { busy, send } = useChat(async (_q, { onAnswer }) => {
      await gate
      onAnswer('done')
    })

    const pending = send('q')
    expect(busy.value).toBe(true)
    release()
    await pending
    expect(busy.value).toBe(false)
  })

  it('ignores a second question while one is in flight', async () => {
    // Two concurrent answers in one thread would interleave with no way to tell
    // which steps belong to which.
    let release
    const gate = new Promise((r) => (release = r))
    const seen = []
    const { turns, send } = useChat(async (question, { onAnswer }) => {
      seen.push(question)
      await gate
      onAnswer('done')
    })

    const first = send('first')
    await send('second while busy')
    expect(seen).toEqual(['first'])
    expect(turns.value).toHaveLength(2)

    release()
    await first
  })

  it('is false again after a failure, so the composer recovers', async () => {
    const { busy, send } = useChat(async () => {
      throw new Error('nope')
    })
    await send('q')
    expect(busy.value).toBe(false)
  })
})
