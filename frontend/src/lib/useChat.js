import { computed, ref } from 'vue'
import { parseAnswer } from './answer.js'
import { stepLabel } from './stepLabels.js'

// Owns the thread: appends turns, mutates the in-flight agent turn as status
// steps arrive, and swaps in the answer on completion. The transport is injected
// so the thread logic is independent of it — task 3 passes the real SSE client
// where task 2 passes a local stand-in.

let nextId = 0
const newId = () => `turn-${nextId++}`

/**
 * @param {(question: string, handlers: {onStep: Function, onAnswer: Function, history: Array}) => Promise<void>} transport
 */
export function useChat(transport) {
  const turns = ref([])
  const busy = computed(() => turns.value.some((t) => t.role === 'agent' && t.status === 'working'))

  // The completed exchanges so far, oldest first. Built before the new turn is
  // appended, and only from turns that actually produced an answer — the backend
  // holds no session state, so this is the whole of the conversation's memory.
  function history() {
    const out = []
    for (let i = 0; i < turns.value.length - 1; i++) {
      const q = turns.value[i]
      const a = turns.value[i + 1]
      if (q.role === 'user' && a.role === 'agent' && a.status === 'complete' && a.answerText) {
        out.push({ question: q.text, answer: a.answerText })
      }
    }
    return out
  }

  async function send(question) {
    if (busy.value) return

    const prior = history()
    turns.value.push({ id: newId(), role: 'user', text: question })

    turns.value.push({
      id: newId(),
      role: 'agent',
      status: 'working',
      steps: [],
      blocks: [],
      // The raw answer text is kept alongside the parsed blocks because it is
      // what gets replayed as history; reassembling it from blocks would lose
      // the figure marker.
      answerText: '',
      error: '',
    })
    // Read the turn back out of the ref rather than keeping the object that was
    // pushed. `ref` deep-reactivity hands out a proxy on access, and mutating the
    // raw object instead updates the data without notifying anything — the status
    // steps and answer would never render.
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

    const onAnswer = (answer) => {
      agentTurn.answerText = answer
      agentTurn.blocks = parseAnswer(answer)
      agentTurn.status = 'complete'
    }

    try {
      await transport(question, { onStep, onAnswer, history: prior })
      // A transport that finished without ever delivering an answer would
      // otherwise leave the status steps pulsing forever, which reads as "still
      // working" when nothing is.
      if (agentTurn.status === 'working') {
        agentTurn.status = 'error'
        agentTurn.error = 'That request ended without an answer. Try asking again.'
      }
    } catch (err) {
      agentTurn.status = 'error'
      agentTurn.error = err?.message || 'Something went wrong reaching the coach.'
    }
  }

  return { turns, busy, send }
}
