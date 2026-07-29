// Client for the Go backend.
//
// The chat endpoint is SSE over POST rather than GET, so EventSource can't be
// used (it only does GET) — we read the response body as a stream and parse the
// frames. The tradeoff is worth it: the question stays in the request body
// instead of a query string, and it keeps the same shape as POST /chat.

const BASE = import.meta.env.VITE_API_BASE ?? 'http://localhost:8080'

/**
 * What the backend is actually connected to.
 * @returns {Promise<{sources: string[], mock: boolean}>}
 */
export async function fetchSources() {
  const res = await fetch(`${BASE}/sources`)
  if (!res.ok) throw new Error(`could not read backend sources (${res.status})`)
  return res.json()
}

// Splits an SSE byte stream into {event, data} frames. Frames are separated by a
// blank line and can straddle chunk boundaries, so the tail of an incomplete
// frame has to be carried over rather than parsed early.
async function* readEvents(response) {
  const reader = response.body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''

  while (true) {
    const { done, value } = await reader.read()
    if (done) break
    buffer += decoder.decode(value, { stream: true })

    let split
    while ((split = buffer.indexOf('\n\n')) !== -1) {
      const frame = buffer.slice(0, split)
      buffer = buffer.slice(split + 2)

      let name = 'message'
      let data = ''
      for (const line of frame.split('\n')) {
        if (line.startsWith('event: ')) name = line.slice(7)
        else if (line.startsWith('data: ')) data += line.slice(6)
      }
      if (!data) continue
      try {
        yield { name, data: JSON.parse(data) }
      } catch {
        // A frame we can't parse is a bug on one side or the other. Surfacing it
        // as an error beats silently dropping progress the user is watching for.
        yield { name: 'error', data: { error: 'received a malformed event from the server' } }
      }
    }
  }
}

/**
 * Asks a question and reports progress as it arrives.
 *
 * @param {string} question
 * @param {{onStep: Function, onAnswer: Function}} handlers
 */
export async function streamChat(question, { onStep, onAnswer }) {
  const res = await fetch(`${BASE}/chat/stream`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ question }),
  })

  // Failures before the stream opens are JSON with a status; the backend's
  // messages are written for a person ("authorize first at /auth/strava/login"),
  // so prefer them over a generic string.
  if (!res.ok) {
    let detail = `request failed (${res.status})`
    try {
      const body = await res.json()
      if (body?.error) detail = body.error
    } catch {
      // Non-JSON error body; keep the status-based message.
    }
    throw new Error(detail)
  }

  for await (const { name, data } of readEvents(res)) {
    if (name === 'step') {
      onStep({ key: `${data.source}/${data.tool}`, source: data.source, tool: data.tool, state: data.state })
    } else if (name === 'answer') {
      onAnswer(data.answer, data.sources ?? [])
    } else if (name === 'error') {
      // Errors arrive in-band once the stream is open, since the status code is
      // already committed.
      throw new Error(data.error || 'the coach could not finish that answer')
    }
  }
}
