/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/

// `start` is the segment's first source line. It is stable for a given
// document, so renderers can use it as a React key.
export type MarkdownSegment =
  | { kind: 'markdown'; start: number; text: string }
  | { kind: 'code'; start: number; code: string; lang: string }

// Docs are usually pasted from API platforms that tag shell snippets as
// "curl"; highlight them as bash rather than falling back to plain text.
const FENCE_LANG_ALIASES: Record<string, string> = {
  curl: 'bash',
  sh: 'bash',
  shell: 'bash',
  zsh: 'bash',
}

const FENCE_OPEN = /^ {0,3}(`{3,}|~{3,})\s*([^\s`]*)/

/**
 * Splits Markdown into prose and fenced code segments so code blocks can be
 * rendered as interactive components (copy button, highlighting) while the
 * surrounding prose keeps the regular Markdown renderer.
 *
 * Follows CommonMark fence rules: a fence closes only on the same character
 * with at least the opening length, and an unclosed fence runs to the end of
 * the document.
 */
export function splitMarkdownCodeFences(source: string): MarkdownSegment[] {
  const segments: MarkdownSegment[] = []
  const lines = source.replaceAll(/\r\n?/g, '\n').split('\n')
  let prose: string[] = []
  let proseStart = 0
  let code: string[] = []
  let fence: { marker: string; lang: string; start: number } | null = null

  const flushProse = () => {
    const text = prose.join('\n')
    if (text.trim() !== '') {
      segments.push({ kind: 'markdown', start: proseStart, text })
    }
    prose = []
  }

  lines.forEach((line, index) => {
    if (fence === null) {
      const open = FENCE_OPEN.exec(line)
      if (!open) {
        if (prose.length === 0) {
          proseStart = index
        }
        prose.push(line)
        return
      }
      flushProse()
      const lang = open[2].toLowerCase()
      fence = {
        marker: open[1],
        lang: FENCE_LANG_ALIASES[lang] ?? (lang || 'plaintext'),
        start: index,
      }
      return
    }

    const trimmed = line.trim()
    const isClose =
      trimmed.length >= fence.marker.length &&
      trimmed === fence.marker[0].repeat(trimmed.length)
    if (isClose) {
      segments.push({
        kind: 'code',
        start: fence.start,
        code: code.join('\n'),
        lang: fence.lang,
      })
      code = []
      fence = null
      return
    }
    code.push(line)
  })

  if (fence !== null) {
    const open: { lang: string; start: number } = fence
    segments.push({
      kind: 'code',
      start: open.start,
      code: code.join('\n'),
      lang: open.lang,
    })
  }
  flushProse()
  return segments
}
