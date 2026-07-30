import { describe, expect, it } from 'vitest'
import { parseAnswer } from './answer.js'

// The answer is model-written plain text, so this parser is the only thing
// standing between a reply and how it renders. Everything DESIGN.md §5 promises
// about answer structure is enforced here, not in the components.

const blocksOf = (text) => parseAnswer(text).map((b) => b.type)

describe('paragraphs', () => {
  it('splits on blank lines', () => {
    const blocks = parseAnswer('First para.\n\nSecond para.\n\nThird para.')
    expect(blocksOf('First para.\n\nSecond para.\n\nThird para.')).toEqual(['lede', 'p', 'p'])
    expect(blocks[1].text).toBe('Second para.')
  })

  it('collapses hard-wrapped lines so the measure controls line breaks', () => {
    const blocks = parseAnswer('Short lede.\n\nA sentence the model\nwrapped early\nfor no reason.')
    expect(blocks[1].text).toBe('A sentence the model wrapped early for no reason.')
  })

  it('ignores stray whitespace and empty input', () => {
    expect(parseAnswer('   \n\n  \n')).toEqual([])
    expect(parseAnswer('')).toEqual([])
    expect(parseAnswer(null)).toEqual([])
    expect(parseAnswer(undefined)).toEqual([])
  })
})

describe('lede', () => {
  it('promotes a short first paragraph when body follows', () => {
    expect(blocksOf('Yes — you are ahead of pace.\n\nThe detail.')).toEqual(['lede', 'p'])
  })

  it('leaves a long first paragraph as body copy', () => {
    // Bolding this would produce a wall of heavy text and destroy the contrast
    // the treatment exists for.
    const long = 'x'.repeat(121)
    expect(blocksOf(`${long}\n\nThe detail.`)).toEqual(['p', 'p'])
  })

  it('does not promote a lone paragraph — there would be nothing to contrast with', () => {
    expect(blocksOf('Just one short line.')).toEqual(['p'])
  })

  it('does not treat a figure as the body that justifies a lede', () => {
    // A lede followed only by a figure has no prose to contrast against.
    expect(blocksOf('Short line.\n\n[figure: 6:44 | a caption]')).toEqual(['p', 'figure'])
  })
})

describe('figure pull-quote', () => {
  it('parses value and caption', () => {
    const [figure] = parseAnswer('[figure: 6:44 | avg long-run pace per mile]')
    expect(figure).toEqual({
      type: 'figure',
      value: '6:44',
      caption: 'avg long-run pace per mile',
    })
  })

  it('keeps its position between paragraphs', () => {
    expect(blocksOf('Lede.\n\nBody.\n\n[figure: 25s | slower]\n\nClosing.')).toEqual([
      'lede',
      'p',
      'figure',
      'p',
    ])
  })

  it('keeps only the first — DESIGN.md §5 allows at most one', () => {
    const blocks = parseAnswer(
      'Lede.\n\nBody.\n\n[figure: 1 | first]\n\nMore.\n\n[figure: 2 | second]',
    )
    const figures = blocks.filter((b) => b.type === 'figure')
    expect(figures).toHaveLength(1)
    expect(figures[0].value).toBe('1')
  })

  it('omits the figure entirely when there is no marker', () => {
    expect(blocksOf('Lede.\n\nBody.\n\nClosing.')).not.toContain('figure')
  })

  it('is case-insensitive and tolerates loose spacing', () => {
    const [figure] = parseAnswer('[FIGURE:   6:44   |   a caption   ]')
    expect(figure.type).toBe('figure')
    expect(figure.value).toBe('6:44')
    expect(figure.caption).toBe('a caption')
  })

  it('ignores a marker that is not alone on its line', () => {
    // Anchored to the line so a bracket mid-sentence cannot hijack a paragraph.
    const blocks = parseAnswer('I ran [figure: 6:44 | nope] on Tuesday.')
    expect(blocks).toHaveLength(1)
    expect(blocks[0].type).toBe('p')
  })

  it('treats a malformed marker as prose rather than dropping it', () => {
    // Losing text is worse than rendering it plainly.
    for (const bad of ['[figure: no pipe here]', '[figure: | empty value]', '[figure:]']) {
      const blocks = parseAnswer(bad)
      expect(blocks).toHaveLength(1)
      expect(blocks[0].type).toBe('p')
      expect(blocks[0].text).toBe(bad)
    }
  })
})
