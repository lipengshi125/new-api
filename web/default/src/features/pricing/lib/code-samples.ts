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
import type { BundledLanguage } from 'shiki/bundle/web'

import type { CodeSampleLang, CodeSampleMap, CodeSampleSource } from '../types'

export const CODE_SAMPLE_LANGS: CodeSampleLang[] = [
  'curl',
  'python',
  'typescript',
  'javascript',
]

export const CODE_SAMPLE_LANG_LABELS: Record<CodeSampleLang, string> = {
  curl: 'cURL',
  python: 'Python',
  typescript: 'TypeScript',
  javascript: 'JavaScript',
}

export const CODE_SAMPLE_HIGHLIGHT: Record<CodeSampleLang, BundledLanguage> = {
  curl: 'bash',
  python: 'python',
  typescript: 'typescript',
  javascript: 'javascript',
}

/** Translation keys describing where a rendered sample came from. */
export const CODE_SAMPLE_SOURCE_LABELS: Record<CodeSampleSource, string> = {
  model: 'Custom for this model',
  global: 'Global template',
  builtin: 'Built-in default',
}

/**
 * Pick the sample to render, preferring the model's own override, then the
 * site-wide template, then the built-in sample. Samples are stored as literal
 * text, so nothing is substituted into them here.
 */
export function resolveCodeSample(params: {
  lang: CodeSampleLang
  endpointType: string
  modelSamples?: CodeSampleMap
  globalTemplates?: CodeSampleMap
  builtin: string
}): { code: string; source: CodeSampleSource } {
  const own = params.modelSamples?.[params.endpointType]?.[params.lang]
  if (own && own.trim()) {
    return { code: own, source: 'model' }
  }
  const template = params.globalTemplates?.[params.endpointType]?.[params.lang]
  if (template && template.trim()) {
    return { code: template, source: 'global' }
  }
  return { code: params.builtin, source: 'builtin' }
}

/**
 * Return a copy of `samples` with one entry replaced. A blank value removes the
 * entry (and its endpoint-type bucket once empty), which is how an admin clears
 * an override and falls back to the global template.
 */
export function withCodeSampleEntry(
  samples: CodeSampleMap | undefined,
  endpointType: string,
  lang: CodeSampleLang,
  code: string
): CodeSampleMap {
  const next: CodeSampleMap = {}
  for (const [type, byLang] of Object.entries(samples ?? {})) {
    next[type] = { ...byLang }
  }

  if (!code.trim()) {
    if (next[endpointType]) {
      delete next[endpointType][lang]
      if (Object.keys(next[endpointType]).length === 0) {
        delete next[endpointType]
      }
    }
    return next
  }

  next[endpointType] = { ...next[endpointType], [lang]: code }
  return next
}
