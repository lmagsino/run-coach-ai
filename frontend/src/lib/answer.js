// Turns the backend's plain-text `answer` into the block structure DESIGN.md §5
// describes: an optional bold lede, body paragraphs, and at most one figure
// pull-quote positioned between paragraphs.
//
// Why parse rather than have the backend return structured JSON: the answer is
// written by the model, and asking it for a JSON envelope makes every answer a
// parse risk — a malformed field would cost us the whole reply. A plain string
// always renders. The one thing text can't express is which number deserves the
// pull-quote treatment, so that gets a single inline marker (see FIGURE_RE); an
// answer without one simply has no figure, which DESIGN.md §5 explicitly allows
// ("if an answer has no single hero number, omit entirely — don't force it").

// [figure: 6:44 | avg long-run pace per mile — eight seconds ahead of a 2:30]
// Anchored to its own line so a stray bracket mid-sentence can't match.
const FIGURE_RE = /^\[figure:\s*([^|\]]+?)\s*\|\s*([^\]]+?)\s*\]$/i

// A lede is "the headline answer in one line" (DESIGN.md §5). Bolding a long
// first paragraph would produce a wall of heavy text and lose the contrast the
// treatment exists for, so past this length the first paragraph stays body copy.
const MAX_LEDE_LENGTH = 120

// Matches a figure marker, or returns null so the caller renders the line as
// prose instead. Both halves must have real content: the regex's value group can
// otherwise be satisfied by the whitespace before the pipe, which would render an
// empty pull-quote with a caption floating beside it.
function matchFigure(chunk) {
  const m = chunk.match(FIGURE_RE)
  if (!m) return null
  const value = m[1].trim()
  const caption = m[2].trim()
  if (!value || !caption) return null
  return { value, caption }
}

/**
 * @param {string} answer Raw answer text from the backend.
 * @returns {Array<{type: 'lede'|'p'}|{type: 'figure', value: string, caption: string}>}
 */
export function parseAnswer(answer) {
  const chunks = String(answer ?? '')
    .split(/\n\s*\n/)
    .map((chunk) => chunk.trim())
    .filter(Boolean)

  const blocks = []
  let sawFigure = false

  for (const chunk of chunks) {
    const figure = matchFigure(chunk)
    if (figure) {
      // DESIGN.md §5: max one per answer. If the model emits more, later ones
      // are dropped rather than rendered — the treatment only reads as emphasis
      // when it is rare.
      if (!sawFigure) {
        blocks.push({ type: 'figure', value: figure.value, caption: figure.caption })
        sawFigure = true
      }
      continue
    }
    // Collapse hard-wrapped lines within a paragraph so the measure controls
    // line breaks, not wherever the model happened to wrap.
    blocks.push({ type: 'p', text: chunk.replace(/\s*\n\s*/g, ' ') })
  }

  const first = blocks[0]
  const hasBodyAfter = blocks.some((block, i) => i > 0 && block.type === 'p')
  if (first?.type === 'p' && hasBodyAfter && first.text.length <= MAX_LEDE_LENGTH) {
    first.type = 'lede'
  }

  return blocks
}
