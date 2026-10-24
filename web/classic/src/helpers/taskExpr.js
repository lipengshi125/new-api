/*
Copyright (C) 2025 QuantumNous

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

// 任务模型（按次/按秒）表达式定价解析，与 web/default/src/features/pricing/lib/task-expr.ts
// 保持同一套识别规则。只做结构识别，不执行表达式；识别不了返回 null。
//
// 支持的基础部分（可带括号、可带 v1: 前缀）：
//   tier("name", fixed(X))                          → 按次，单价 X
//   tier("name", K)                                 → 按次，单价 K/1e6
//   tier("name", K * (秒数项 [+ 秒数项]))            → 按秒
// 秒数项：float(param("duration") ?? 0)（可被 max/min 钳制包裹）、video_seconds("videos")
// 倍率因子（以 * 连接）：(param("x") == "v" ? r : 1)

const TOKEN_VAR_RE = /\b(p|c|len|cr|cc|cc1h|img|img_o|ai|ao)\b/;
const NUM = '(\\d+(?:\\.\\d+)?(?:[eE][+-]?\\d+)?)';
const STR = '"([^"\\\\]*)"';

function splitTopLevel(expr, separator) {
  const parts = [];
  let depth = 0;
  let inString = false;
  let start = 0;
  for (let i = 0; i < expr.length; i += 1) {
    const ch = expr[i];
    if (ch === '"' && expr[i - 1] !== '\\') inString = !inString;
    if (inString) continue;
    if (ch === '(') depth += 1;
    if (ch === ')') depth -= 1;
    if (ch === separator && depth === 0) {
      parts.push(expr.slice(start, i).trim());
      start = i + 1;
    }
  }
  parts.push(expr.slice(start).trim());
  return parts.filter(Boolean);
}

function isWrappedInParens(expr) {
  if (!expr.startsWith('(') || !expr.endsWith(')')) return false;
  let depth = 0;
  for (let i = 0; i < expr.length; i += 1) {
    if (expr[i] === '(') depth += 1;
    if (expr[i] === ')') depth -= 1;
    if (depth === 0 && i < expr.length - 1) return false;
  }
  return true;
}

function unwrap(expr) {
  let current = expr.trim();
  while (isWrappedInParens(current)) current = current.slice(1, -1).trim();
  return current;
}

function stripClamp(term) {
  let current = term;
  for (;;) {
    const m = current.match(
      /^(?:max|min)\((?:\d+(?:\.\d+)?,([\s\S]+)|([\s\S]+),\d+(?:\.\d+)?)\)$/,
    );
    if (!m) return current;
    current = m[1] ?? m[2];
  }
}

function parseSecondsTerm(term) {
  const compact = stripClamp(unwrap(term).replace(/\s+/g, ''));
  // video_seconds("videos") 或 video_seconds("videos", 15)（第二个参数为总时长上限）
  const video = compact.match(
    /^video_seconds\("([^"]+)"(?:,(\d+(?:\.\d+)?))?\)$/,
  );
  if (video) {
    return {
      kind: 'video',
      path: video[1],
      ...(video[2] ? { maxSeconds: Number(video[2]) } : {}),
    };
  }
  const param = compact.match(
    /^(?:float\()?param\("([^"]+)"\)(?:\?\?\d+(?:\.\d+)?)?\)?$/,
  );
  if (param) return { kind: 'param', path: param[1] };
  return null;
}

function parseBase(base) {
  const tier = unwrap(base).match(
    new RegExp(`^tier\\(\\s*${STR}\\s*,\\s*([\\s\\S]+)\\)$`),
  );
  if (!tier) return null;
  const body = unwrap(tier[2]);

  // fixed(0.16)：直接写每次价格（与上游 new-api 一致）
  const fixed = body.match(new RegExp(`^fixed\\(\\s*${NUM}\\s*\\)$`));
  if (fixed) {
    return { unit: 'request', unitPrice: Number(fixed[1]), secondsSources: [] };
  }

  // 160000：旧写法，系数按 $/1M 换算
  const flat = body.match(new RegExp(`^${NUM}$`));
  if (flat) {
    return {
      unit: 'request',
      unitPrice: Number(flat[1]) / 1000000,
      secondsSources: [],
    };
  }

  const factors = splitTopLevel(body, '*');
  if (factors.length !== 2) return null;
  const [coef, secondsExpr] = factors;
  if (!new RegExp(`^${NUM}$`).test(coef)) return null;

  const sources = [];
  for (const term of splitTopLevel(unwrap(secondsExpr), '+')) {
    const source = parseSecondsTerm(term);
    if (!source) return null;
    sources.push(source);
  }
  if (sources.length === 0) return null;
  return {
    unit: 'second',
    unitPrice: Number(coef) / 1000000,
    secondsSources: sources,
  };
}

// 比较值支持字符串（"1080p"）、布尔（true/false）和数字（5）
const COMPARE_VALUE = `(?:${STR}|(true|false|-?\\d+(?:\\.\\d+)?))`;

function parseMultiplierFactor(factor) {
  const m = unwrap(factor).match(
    new RegExp(
      `^param\\(\\s*${STR}\\s*\\)\\s*==\\s*${COMPARE_VALUE}\\s*\\?\\s*${NUM}\\s*:\\s*1$`,
    ),
  );
  if (!m) return null;
  return { name: m[1], value: m[2] ?? m[3], ratio: Number(m[4]) };
}

export function parseTaskExprPricing(exprStr) {
  if (!exprStr) return null;
  const body = exprStr.trim().replace(/^v\d+:/, '');
  if (TOKEN_VAR_RE.test(body.replace(/"[^"]*"/g, '""'))) return null;

  const [base, ...rest] = splitTopLevel(body, '*');
  if (!base) return null;
  const parsedBase = parseBase(base);
  if (!parsedBase) return null;

  const multipliers = [];
  for (const factor of rest) {
    const multiplier = parseMultiplierFactor(factor);
    if (!multiplier) return null;
    multipliers.push(multiplier);
  }
  return { ...parsedBase, multipliers };
}

export function groupTaskMultipliers(multipliers) {
  const groups = [];
  for (const m of multipliers || []) {
    let group = groups.find((g) => g.name === m.name);
    if (!group) {
      group = { name: m.name, rows: [] };
      groups.push(group);
    }
    group.rows.push({ value: m.value, ratio: m.ratio });
  }
  return groups;
}
