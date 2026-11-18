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

// ----------------------------------------------------------------------------
// Task (per-call / per-second) expression pricing parser
//
// Recognizes the billing expressions used by async task models (video/image),
// which contain no token variables, and turns them into a user-facing pricing
// structure. Parsing is purely structural (regex over the expression text) and
// never evaluates it. Anything outside the supported shapes returns null so the
// UI can fall back to the admin-written billing note.
//
// Supported base shapes (optionally wrapped in parentheses, optional "v1:"):
//   tier("name", fixed(X))                           → per call, price X
//   tier("name", K)                                  → per call, price K/1e6
//   tier("name", K * (SECONDS_TERM [+ SECONDS_TERM]))  → per second
// where SECONDS_TERM is one of:
//   max(0, min(N, float(param("duration") ?? 0)))   (any order / without clamp)
//   float(param("seconds") ?? 0)
//   video_seconds("videos")
// Supported multiplier factors (joined with " * "):
//   (param("x") == "v" ? r : 1)
// ----------------------------------------------------------------------------

export type TaskSecondsSource =
  | { kind: 'param'; path: string }
  | { kind: 'video'; path: string; maxSeconds?: number }
  | { kind: 'item_charge'; path: string; chargePerItem: number }

export type TaskParamMultiplier = {
  name: string
  value: string
  ratio: number
}

export type TaskExprPricing = {
  unit: 'request' | 'second'
  /** Price per call / per second, in the expression's currency unit (USD). */
  unitPrice: number
  /**
   * 与倍率相乘的附加基础价（USD，可带秒数项）。
   * 出现在 tier(...) 内部 "K * (倍率) + 固定项" 的加法形式里，
   * 例如 200000 * (param(...) ? 1.6 : 1) 的 200000。
   */
  extraBasePrice: number
  /** 固定加价（USD），tier 内除声明的按个数项之外的常数项。 */
  extraFixedCharge: number
  secondsSources: TaskSecondsSource[]
  /** Per-item charges from item_count/video_count: path -> price in USD. */
  itemCharges: Record<string, number>
  multipliers: TaskParamMultiplier[]
}

const TOKEN_VAR_RE = /\b(p|c|len|cr|cc|cc1h|img|img_o|ai|ao)\b/
const NUM = '(\\d+(?:\\.\\d+)?(?:[eE][+-]?\\d+)?)'
const STR = '"([^"\\\\]*)"'

/** Split on top-level " * " (not inside parentheses or strings). */
function splitTopLevelFactors(expr: string): string[] {
  const parts: string[] = []
  let depth = 0
  let inString = false
  let start = 0
  for (let i = 0; i < expr.length; i += 1) {
    const ch = expr[i]
    if (ch === '"' && expr[i - 1] !== '\\') inString = !inString
    if (inString) continue
    if (ch === '(') depth += 1
    if (ch === ')') depth -= 1
    if (ch === '*' && depth === 0) {
      parts.push(expr.slice(start, i).trim())
      start = i + 1
    }
  }
  parts.push(expr.slice(start).trim())
  return parts.filter(Boolean)
}

function isWrappedInParens(expr: string): boolean {
  if (!expr.startsWith('(') || !expr.endsWith(')')) return false
  let depth = 0
  for (let i = 0; i < expr.length; i += 1) {
    if (expr[i] === '(') depth += 1
    if (expr[i] === ')') depth -= 1
    if (depth === 0 && i < expr.length - 1) return false
  }
  return true
}

function unwrap(expr: string): string {
  let current = expr.trim()
  while (isWrappedInParens(current)) current = current.slice(1, -1).trim()
  return current
}

/** Split on top-level " + ". */
function splitTopLevelTerms(expr: string): string[] {
  const parts: string[] = []
  let depth = 0
  let start = 0
  for (let i = 0; i < expr.length; i += 1) {
    if (expr[i] === '(') depth += 1
    if (expr[i] === ')') depth -= 1
    if (expr[i] === '+' && depth === 0) {
      parts.push(expr.slice(start, i).trim())
      start = i + 1
    }
  }
  parts.push(expr.slice(start).trim())
  return parts.filter(Boolean)
}

// 剥掉 max(0, X) / min(N, X) / max(X, 0) / min(X, N) 这类钳制包裹，返回内层 X
function stripClamp(term: string): string {
  let current = term
  for (;;) {
    const m = current.match(
      /^(?:max|min)\((?:\d+(?:\.\d+)?,([\s\S]+)|([\s\S]+),\d+(?:\.\d+)?)\)$/
    )
    if (!m) return current
    current = m[1] ?? m[2]
  }
}

function parseSecondsTerm(term: string): TaskSecondsSource | null {
  const compact = stripClamp(unwrap(term).replaceAll(/\s+/g, ''))
  // video_seconds("videos") 或 video_seconds("videos", 15)（第二个参数为总时长上限）
  const video = compact.match(
    /^video_seconds\("([^"]+)"(?:,(\d+(?:\.\d+)?))?\)$/
  )
  if (video) {
    return {
      kind: 'video',
      path: video[1],
      ...(video[2] ? { maxSeconds: Number(video[2]) } : {}),
    }
  }
  const param = compact.match(
    /^(?:float\()?param\("([^"]+)"\)(?:\?\?\d+(?:\.\d+)?)?\)?$/
  )
  if (param) return { kind: 'param', path: param[1] }
  // item_count("videos") * 500000 或 video_count(...) * 500000
  const itemCharge = compact.match(
    /^(?:item_count|video_count)\("([^"]+)"\)\*(\d+(?:\.\d+)?)$/
  )
  if (itemCharge) {
    return {
      kind: 'item_charge',
      path: itemCharge[1],
      chargePerItem: Number(itemCharge[2]) / 1_000_000,
    }
  }
  return null
}

type ParsedBase = Pick<
  TaskExprPricing,
  'unit' | 'unitPrice' | 'secondsSources' | 'itemCharges'
> & {
  /** tier(...) 内部解析出的参数倍率 */
  inlineMultipliers?: TaskParamMultiplier[]
  extraBasePrice?: number
  extraFixedCharge?: number
}

function parseBase(base: string): ParsedBase | null {
  const tier = unwrap(base).match(
    new RegExp(`^tier\\(\\s*${STR}\\s*,\\s*([\\s\\S]+)\\)$`)
  )
  if (!tier) return null
  const body = unwrap(tier[2])

  // fixed(0.16)：直接写每次价格（与上游 new-api 一致）
  const fixed = body.match(new RegExp(`^fixed\\(\\s*${NUM}\\s*\\)$`))
  if (fixed) {
    return {
      unit: 'request',
      unitPrice: Number(fixed[1]),
      secondsSources: [],
      itemCharges: {},
    }
  }

  // 160000：旧写法，系数按 $/1M 换算
  const flat = body.match(new RegExp(`^${NUM}$`))
  if (flat) {
    return {
      unit: 'request',
      unitPrice: Number(flat[1]) / 1_000_000,
      secondsSources: [],
      itemCharges: {},
    }
  }

  // tier 内部的加法形式：K * (param(...) ? r : 1) + item_count("p") * N
  const additiveTerms = splitTopLevelTerms(body)
  if (additiveTerms.length >= 2) {
    const inlineMultipliers: TaskParamMultiplier[] = []
    const itemCharges: Record<string, number> = {}
    let extraBasePrice = 0
    let extraFixedCharge = 0
    let recognized = true
    for (const term of additiveTerms) {
      // 1) K * (cond ? r : 1)
      const inline = parseInlineMultiplierTerm(term)
      if (inline) {
        extraBasePrice += inline.coef / 1_000_000
        extraFixedCharge += inline.constant / 1_000_000
        inlineMultipliers.push({
          name: inline.name,
          value: inline.value,
          ratio: inline.ratio,
        })
        continue
      }
      // 2) item_count("p") * N / video_count("p") * N
      const source = parseSecondsTerm(term)
      if (source?.kind === 'item_charge') {
        itemCharges[source.path] = source.chargePerItem
        continue
      }
      // 3) 纯常数
      const constant = unwrap(term).match(new RegExp(`^${NUM}$`))
      if (constant) {
        extraFixedCharge += Number(constant[1]) / 1_000_000
        continue
      }
      recognized = false
      break
    }
    if (
      recognized &&
      (inlineMultipliers.length > 0 || Object.keys(itemCharges).length > 0)
    ) {
      return {
        unit: 'request',
        unitPrice: 0,
        extraBasePrice,
        extraFixedCharge,
        secondsSources: [],
        itemCharges,
        inlineMultipliers,
      }
    }
  }

  const factors = splitTopLevelFactors(body)
  if (factors.length < 2) return null
  const [coef, ...rest] = factors
  if (!new RegExp(`^${NUM}$`).test(coef)) return null

  const sources: TaskSecondsSource[] = []
  const itemCharges: Record<string, number> = {}
  for (const term of splitTopLevelTerms(unwrap(rest.join('*')))) {
    const source = parseSecondsTerm(term)
    if (!source) return null
    if (source.kind === 'item_charge') {
      itemCharges[source.path] = source.chargePerItem
    } else {
      sources.push(source)
    }
  }
  // 如果只有 item_count 没有秒数项，按次计费；否则按秒
  if (sources.length === 0 && Object.keys(itemCharges).length === 0) {
    return null
  }
  return {
    unit: sources.length > 0 ? 'second' : 'request',
    unitPrice: Number(coef) / 1_000_000,
    secondsSources: sources,
    itemCharges,
  }
}

// 比较值支持字符串（"1080p"）、布尔（true/false）和数字（5）
const COMPARE_VALUE = `(?:${STR}|(true|false|-?\\d+(?:\\.\\d+)?))`

function parseMultiplierFactor(factor: string): TaskParamMultiplier | null {
  const m = unwrap(factor).match(
    new RegExp(
      `^param\\(\\s*${STR}\\s*\\)\\s*==\\s*${COMPARE_VALUE}\\s*\\?\\s*${NUM}\\s*:\\s*1$`
    )
  )
  if (!m) return null
  return { name: m[1], value: m[2] ?? m[3], ratio: Number(m[4]) }
}

/** 拆出 tier 内的 "A * (cond ? r : 1) + B" 形式：返回系数、倍率因子与常数项 */
function parseInlineMultiplierTerm(
  term: string
): {
  coef: number
  name: string
  value: string
  ratio: number
  constant: number
} | null {
  const compact = unwrap(term).replaceAll(/\s+/g, '')
  const m = compact.match(
    new RegExp(
      `^${NUM}\\*\\(param\\(${STR}\\)==${COMPARE_VALUE}\\?${NUM}:1\\)(?:\\+${NUM})?$`
    )
  )
  if (!m) return null
  return {
    coef: Number(m[1]),
    name: m[2],
    value: m[3] ?? m[4],
    ratio: Number(m[5]),
    constant: m[6] != null ? Number(m[6]) : 0,
  }
}

export function parseTaskExprPricing(
  exprStr: string | null | undefined
): TaskExprPricing | null {
  if (!exprStr) return null
  const body = exprStr.trim().replace(/^v\d+:/, '')
  // 含 token 变量的是 token 计费表达式，交给原有的分档展示
  if (TOKEN_VAR_RE.test(body.replaceAll(/"[^"]*"/g, '""'))) return null

  const [base, ...rest] = splitTopLevelFactors(body)
  if (!base) return null
  const parsedBase = parseBase(base)
  if (!parsedBase) return null

  const multipliers: TaskParamMultiplier[] = [
    ...(parsedBase.inlineMultipliers ?? []),
  ]
  for (const factor of rest) {
    const multiplier = parseMultiplierFactor(factor)
    if (!multiplier) return null
    multipliers.push(multiplier)
  }
  return {
    unit: parsedBase.unit,
    unitPrice: parsedBase.unitPrice,
    extraBasePrice: parsedBase.extraBasePrice ?? 0,
    extraFixedCharge: parsedBase.extraFixedCharge ?? 0,
    secondsSources: parsedBase.secondsSources,
    itemCharges: parsedBase.itemCharges || {},
    multipliers,
  }
}

/** Group multipliers by parameter name, keeping first-seen order. */
export function groupTaskMultipliers(
  multipliers: TaskParamMultiplier[]
): Array<{ name: string; rows: Array<{ value: string; ratio: number }> }> {
  const groups: Array<{
    name: string
    rows: Array<{ value: string; ratio: number }>
  }> = []
  for (const m of multipliers) {
    let group = groups.find((g) => g.name === m.name)
    if (!group) {
      group = { name: m.name, rows: [] }
      groups.push(group)
    }
    group.rows.push({ value: m.value, ratio: m.ratio })
  }
  return groups
}

/** True when the model uses expression billing (tiered_expr). */
export function isExpressionPricedModel(model: {
  billing_mode?: string
  billing_expr?: string
}): boolean {
  return model.billing_mode === 'tiered_expr' && Boolean(model.billing_expr)
}

/**
 * True when an expression model's pricing cannot be shown structurally —
 * neither as token tiers nor as task pricing. The UI then shows only the
 * admin's billing note and hides the raw expression.
 */
export function isUnrecognizedExpression(
  exprStr: string,
  tokenTierCount: number
): boolean {
  return tokenTierCount === 0 && parseTaskExprPricing(exprStr) === null
}
