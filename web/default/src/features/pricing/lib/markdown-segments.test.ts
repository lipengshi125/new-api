import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { splitMarkdownCodeFences } from './markdown-segments'

describe('splitMarkdownCodeFences', () => {
  test('separates prose from fenced code in document order', () => {
    const segments = splitMarkdownCodeFences(
      [
        '### 提交请求',
        '',
        '```curl',
        "curl -X POST 'https://example.com/v1/videos'",
        '```',
        '',
        '| a | b |',
        '| - | - |',
        '',
        '```json',
        '{ "id": "task_1" }',
        '```',
      ].join('\n')
    )

    assert.deepEqual(segments, [
      { kind: 'markdown', start: 0, text: '### 提交请求\n' },
      {
        kind: 'code',
        start: 2,
        code: "curl -X POST 'https://example.com/v1/videos'",
        lang: 'bash',
      },
      { kind: 'markdown', start: 5, text: '\n| a | b |\n| - | - |\n' },
      { kind: 'code', start: 9, code: '{ "id": "task_1" }', lang: 'json' },
    ])
  })

  test('keeps blank lines and indentation inside code verbatim', () => {
    const segments = splitMarkdownCodeFences(
      '```python\ndef f():\n\n    return 1\n```'
    )
    assert.deepEqual(segments, [
      {
        kind: 'code',
        start: 0,
        code: 'def f():\n\n    return 1',
        lang: 'python',
      },
    ])
  })

  test('a shorter or different fence does not close the block', () => {
    const segments = splitMarkdownCodeFences(
      '````md\n```js\ninner\n```\n~~~\n````'
    )
    assert.deepEqual(segments, [
      { kind: 'code', start: 0, code: '```js\ninner\n```\n~~~', lang: 'md' },
    ])
  })

  test('an unclosed fence runs to the end of the document', () => {
    const segments = splitMarkdownCodeFences('intro\n```bash\necho hi')
    assert.deepEqual(segments, [
      { kind: 'markdown', start: 0, text: 'intro' },
      { kind: 'code', start: 1, code: 'echo hi', lang: 'bash' },
    ])
  })

  test('untagged fences fall back to plaintext and CRLF is normalized', () => {
    const segments = splitMarkdownCodeFences('```\r\nraw\r\n```\r\n')
    assert.deepEqual(segments, [
      { kind: 'code', start: 0, code: 'raw', lang: 'plaintext' },
    ])
  })

  test('inline backticks are not treated as fences', () => {
    const segments = splitMarkdownCodeFences('use `model` and ``x``')
    assert.deepEqual(segments, [
      { kind: 'markdown', start: 0, text: 'use `model` and ``x``' },
    ])
  })
})
