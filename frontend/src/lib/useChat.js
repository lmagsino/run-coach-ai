import { computed, ref } from 'vue'
import { parseAnswer } from './answer.js'
import { stepLabel } from './stepLabels.js'

// Owns one chat's thread: loads the saved turns, appends new ones, mutates the
// in-flight agent turn as status steps arrive, and swaps in the answer on
// completion. The transport is injected so the thread logic is independent of
// it; the app passes the real SSE client, tests pass a stand-in. The server
// holds the history, so nothing is replayed from here.

let nextId = 0
const newId = () => `turn-${nextId++}`

// A saved assistant message becomes a completed agent turn.
function fromMessages(messages) {
  return messages.map((m) =>
    m.role === 'user'
      ? { id: newId(), role: 'user', text: m.content }
      : {
          id: newId(),
          role: 'agent',
          status: 'complete',
          steps: [],
          blocks: parseAnswer(m.content),
          answerText: m.content,
          chart: m.chart ?? null,
          error: '',
        },
  )
}

/**
 * @param {(chatId: string, question: string, handlers: {onStep: Function, onChart: Function, onAnswer: Function}) => Promise<void>} transport
 * @param {{loadMessages?: (chatId: string) => Promise<Array>}} [options]
 */
export function useChat(transport, { loadMessages } = {}) {
  const turns = ref([])
  const chatId = ref(null)
  const busy = computed(() => turns.value.some((t) => t.role === 'agent' && t.status === 'working'))

  // Switches the thread to a saved chat (or to none, for a fresh page). Opening
  // the chat that is already open is a no-op, so the page's route watcher and
  // its own open() after creating a chat cannot race and replace an in-flight
  // turn with the saved messages.
  async function open(id) {
    if (id && chatId.value === id) return
    chatId.value = id
    turns.value = []
    if (!id || !loadMessages) return
    const messages = await loadMessages(id)
    // The chat may have changed while loading; keep the newest request's result.
    if (chatId.value === id) turns.value = fromMessages(messages)
  }

  async function send(question) {
    if (busy.value || !chatId.value) return
    const id = chatId.value

    turns.value.push({ id: newId(), role: 'user', text: question })
    turns.value.push({
      id: newId(),
      role: 'agent',
      status: 'working',
      steps: [],
      blocks: [],
      answerText: '',
      // The chart arrives before the answer and is held until the answer does,
      // so the card and the prose appear together (ADR 0009).
      chart: null,
      error: '',
    })
    // Read the turn back out of the ref rather than keeping the object that was
    // pushed. `ref` deep-reactivity hands out a proxy on access, and mutating the
    // raw object instead updates the data without notifying anything.
    const agentTurn = turns.value[turns.value.length - 1]

    // Steps are keyed so a later event can flip an existing step's state in
    // place instead of appending a duplicate line.
    const onStep = ({ key, source, tool, state }) => {
      const existing = agentTurn.steps.find((s) => s.id === key)
      if (existing) {
        existing.state = state
        return
      }
      agentTurn.steps.push({ id: key, label: stepLabel({ source, tool }), state })
    }
    const onChart = (spec) => {
      agentTurn.chart = spec
    }
    const onAnswer = (answer) => {
      agentTurn.answerText = answer
      agentTurn.blocks = parseAnswer(answer)
      agentTurn.status = 'complete'
    }

    try {
      await transport(id, question, { onStep, onChart, onAnswer })
      // A transport that finished without ever delivering an answer would
      // otherwise leave the status steps pulsing forever.
      if (agentTurn.status === 'working') {
        agentTurn.status = 'error'
        agentTurn.error = 'That request ended without an answer. Try asking again.'
      }
    } catch (err) {
      agentTurn.status = 'error'
      agentTurn.error = err?.message || 'Something went wrong reaching the coach.'
    }
  }

  return { turns, busy, chatId, open, send }
}
