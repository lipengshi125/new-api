import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { groupTaskMultipliers, parseTaskExprPricing } from './task-expr'

const VIDEO_EXPR =
  'tier("base", 180000 * (max(0, min(3600, float(param("duration") ?? 0))) + video_seconds("videos"))) * (param("resolution") == "720p" ? 1.75 : 1) * (param("resolution") == "1080p" ? 3.5 : 1)'

describe('parseTaskExprPricing', () => {
  test('per-second video expression with reference video and resolution', () => {
    assert.deepEqual(parseTaskExprPricing(VIDEO_EXPR), {
      unit: 'second',
      unitPrice: 0.18,
      secondsSources: [
        { kind: 'param', path: 'duration' },
        { kind: 'video', path: 'videos' },
      ],
      multipliers: [
        { name: 'resolution', value: '720p', ratio: 1.75 },
        { name: 'resolution', value: '1080p', ratio: 3.5 },
      ],
    })
  })

  test('expression saved by the editor with wrapped base is equivalent', () => {
    const saved = VIDEO_EXPR.replace(/^(tier\([\s\S]+?\)\)\)) \*/, '($1) *')
    assert.notEqual(saved, VIDEO_EXPR)
    assert.deepEqual(
      parseTaskExprPricing(saved),
      parseTaskExprPricing(VIDEO_EXPR)
    )
  })

  test('per-call image expression', () => {
    assert.deepEqual(
      parseTaskExprPricing(
        '(tier("base", 150000)) * (param("resolution") == "1k" ? 1 : 1) * (param("resolution") == "4k" ? 1.6 : 1)'
      ),
      {
        unit: 'request',
        unitPrice: 0.15,
        secondsSources: [],
        multipliers: [
          { name: 'resolution', value: '1k', ratio: 1 },
          { name: 'resolution', value: '4k', ratio: 1.6 },
        ],
      }
    )
  })

  test('fixed() per-call price is equivalent to the scaled coefficient form', () => {
    const fixed = parseTaskExprPricing(
      'tier("request", fixed(0.16)) * (param("resolution") == "4k" ? 1.6 : 1)'
    )
    const legacy = parseTaskExprPricing(
      'tier("request", 160000) * (param("resolution") == "4k" ? 1.6 : 1)'
    )
    assert.deepEqual(fixed, {
      unit: 'request',
      unitPrice: 0.16,
      secondsSources: [],
      multipliers: [{ name: 'resolution', value: '4k', ratio: 1.6 }],
    })
    assert.deepEqual(fixed, legacy)
    assert.equal(
      parseTaskExprPricing('(tier("base", fixed(0.15)))')?.unitPrice,
      0.15
    )
  })

  test('token expressions are left to the tier breakdown', () => {
    assert.equal(parseTaskExprPricing('tier("base", p * 2.5 + c * 15)'), null)
    assert.equal(
      parseTaskExprPricing(
        'tier("base", 1000000 + 2000000 * count(param("images") ?? [], true))'
      ),
      null
    )
  })

  test('unsupported multiplier shapes are not recognized', () => {
    assert.equal(
      parseTaskExprPricing(
        'tier("base", 150000) * (count(param("videos") ?? [], true) > 0 ? 1.5 : 1)'
      ),
      null
    )
    assert.equal(parseTaskExprPricing(''), null)
  })
})

describe('groupTaskMultipliers', () => {
  test('groups by parameter name in first-seen order', () => {
    assert.deepEqual(
      groupTaskMultipliers([
        { name: 'resolution', value: '720p', ratio: 1.75 },
        { name: 'quality', value: 'hd', ratio: 2 },
        { name: 'resolution', value: '1080p', ratio: 3.5 },
      ]),
      [
        {
          name: 'resolution',
          rows: [
            { value: '720p', ratio: 1.75 },
            { value: '1080p', ratio: 3.5 },
          ],
        },
        { name: 'quality', rows: [{ value: 'hd', ratio: 2 }] },
      ]
    )
  })
})
