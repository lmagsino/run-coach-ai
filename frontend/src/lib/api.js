// The HTTP client. Everything goes to /api on the same origin: the Go binary
// serves the UI in production and Vite proxies /api in development (ADR 0007),
// so the session cookie travels with every request and no CORS is involved.

const BASE = '/api'

// ApiError carries the status so the router can treat 401 as "sign in", and
// the server's kind where it sends one (a Garmin sign-in failure's reason).
export class ApiError extends Error {
  constructor(message, status, kind = '') {
    super(message)
    this.status = status
    this.kind = kind
  }
}

async function request(method, path, body) {
  let res
  try {
    res = await fetch(`${BASE}${path}`, {
      method,
      headers: body === undefined ? {} : { 'Content-Type': 'application/json' },
      body: body === undefined ? undefined : JSON.stringify(body),
      credentials: 'same-origin',
    })
  } catch {
    // fetch rejects with "Failed to fetch" for every transport-level problem;
    // say the useful thing instead.
    throw new ApiError('Can’t reach the coach. Is the backend running?', 0)
  }
  if (!res.ok) {
    let detail = `request failed (${res.status})`
    let kind = ''
    try {
      const data = await res.json()
      if (data?.error) detail = data.error
      if (data?.kind) kind = data.kind
    } catch {
      // Non-JSON error body; keep the status-based message.
    }
    throw new ApiError(detail, res.status, kind)
  }
  if (res.status === 204) return null
  return res.json()
}

export const api = {
  health: () => request('GET', '/healthz'),
  me: () => request('GET', '/me'),
  login: (email, password) => request('POST', '/login', { email, password }),
  signup: (fields) => request('POST', '/signup', fields),
  logout: () => request('POST', '/logout', {}),
  listChats: () => request('GET', '/chats').then((d) => d.chats ?? []),
  createChat: () => request('POST', '/chats', {}),
  renameChat: (id, title) => request('PATCH', `/chats/${id}`, { title }),
  deleteChat: (id) => request('DELETE', `/chats/${id}`),
  messages: (id) => request('GET', `/chats/${id}/messages`).then((d) => d.messages ?? []),
  updateAccount: (fields) => request('PATCH', '/account', fields),
  changePassword: (fields) => request('POST', '/account/password', fields),
  deleteAccount: (fields) => request('DELETE', '/account', fields),
  garminTokens: (bundle) => request('POST', '/garmin/tokens', { bundle }),
  garminTokensDelete: () => request('DELETE', '/garmin/tokens'),
  garminLogin: (email, password) => request('POST', '/garmin/login', { email, password }),
  garminLoginCode: (code) => request('POST', '/garmin/login/code', { code }),
}

// Splits an SSE byte stream into {event, data} frames. Frames are separated by a
// blank line and can straddle chunk boundaries, so the tail of an incomplete
// frame has to be carried over rather than parsed early. Comment lines (the
// server's keepalive) carry no data and are skipped.
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
 * Asks a question in a saved chat and reports progress as it arrives. The
 * server holds the chat's history, so only the question travels.
 *
 * @param {string} chatId
 * @param {string} question
 * @param {{onStep: Function, onChart?: Function, onAnswer: Function}} handlers
 */
export async function streamChat(chatId, question, { onStep, onChart, onAnswer }) {
  let res
  try {
    res = await fetch(`${BASE}/chats/${chatId}/stream`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ question }),
      credentials: 'same-origin',
    })
  } catch {
    throw new ApiError('Can’t reach the coach. Is the backend running?', 0)
  }

  // Failures before the stream opens are JSON with a status: the cap (429), a
  // reply already running (409), a missing chat (404), a lost session (401).
  if (!res.ok) {
    let detail = `request failed (${res.status})`
    try {
      const body = await res.json()
      if (body?.error) detail = body.error
    } catch {
      // Non-JSON error body; keep the status-based message.
    }
    throw new ApiError(detail, res.status)
  }

  for await (const { name, data } of readEvents(res)) {
    if (name === 'step') {
      onStep({ key: `${data.source}/${data.tool}`, source: data.source, tool: data.tool, state: data.state })
    } else if (name === 'chart') {
      // Already validated by the backend; the card is held until the answer.
      onChart?.(data)
    } else if (name === 'answer') {
      onAnswer(data.answer, data.sources ?? [])
    } else if (name === 'error') {
      // Errors arrive in-band once the stream is open, since the status code is
      // already committed.
      throw new ApiError(data.error || 'the coach could not finish that answer', 0)
    }
  }
}
