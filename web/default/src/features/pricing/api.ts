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
import { api } from '@/lib/api'

import type { CodeSampleMap, PricingData } from './types'

// ----------------------------------------------------------------------------
// Pricing APIs
// ----------------------------------------------------------------------------

// Get model pricing data
export async function getPricing(): Promise<PricingData> {
  const res = await api.get('/api/pricing')
  return res.data
}

/**
 * Save a model's call-sample overrides. Keyed by model name rather than row id
 * because non-exact name rules let one metadata row back many model names.
 * An empty map clears the override.
 */
export type ModelApiDoc = {
  model_name: string
  content: string
  updated_time?: number
}

/** Fetch a model's Markdown API document; content is empty when none is set. */
export async function getModelApiDoc(modelName: string): Promise<ModelApiDoc> {
  const res = await api.get('/api/pricing/api_doc', {
    params: { model_name: modelName },
  })
  return res.data.data
}

/** Save a model's Markdown API document. Empty content restores the default. */
export async function updateModelApiDoc(params: {
  modelName: string
  content: string
}): Promise<{ success: boolean; message?: string }> {
  const res = await api.put('/api/models/api_doc', {
    model_name: params.modelName,
    content: params.content,
  })
  return res.data
}

export async function updateModelCodeSamples(params: {
  modelName: string
  codeSamples: CodeSampleMap
}): Promise<{ success: boolean; message?: string }> {
  const res = await api.put('/api/models/code_samples', {
    model_name: params.modelName,
    code_samples: params.codeSamples,
  })
  return res.data
}
