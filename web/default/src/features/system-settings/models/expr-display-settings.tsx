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
import { useTranslation } from 'react-i18next'

import { Input } from '@/components/ui/input'
import { Textarea } from '@/components/ui/textarea'
import { parseTiersFromExpr } from '@/features/pricing/lib/billing-expr'
import {
  groupTaskMultipliers,
  parseTaskExprPricing,
} from '@/features/pricing/lib/task-expr'

type ExprDisplaySettingsProps = {
  /** Full expression (base + request rules) as it will be saved */
  expr: string
  notes: Record<string, Record<string, string>>
  onNotesChange: (notes: Record<string, Record<string, string>>) => void
  billingNote: string
  onBillingNoteChange: (note: string) => void
}

/**
 * 表达式计费的模型广场展示设置：
 * - 表达式里识别出的参数倍率（param == 值 ? 倍率 : 1）可逐条填写备注
 * - 计费说明：识别不了的表达式在模型广场只展示这段说明（不展示表达式）
 * 仅影响展示，不参与计费。
 */
export function ExprDisplaySettings(props: ExprDisplaySettingsProps) {
  const { t } = useTranslation()
  const taskPricing = parseTaskExprPricing(props.expr)
  const recognized =
    taskPricing !== null || parseTiersFromExpr(props.expr).length > 0
  const groups = taskPricing
    ? groupTaskMultipliers(taskPricing.multipliers)
    : []

  const setNote = (name: string, value: string, note: string) => {
    const next = { ...props.notes, [name]: { ...props.notes[name] } }
    if (note) next[name][value] = note
    else delete next[name][value]
    if (Object.keys(next[name]).length === 0) delete next[name]
    props.onNotesChange(next)
  }

  return (
    <div className='mt-6 space-y-4'>
      <div>
        <h3 className='text-sm font-semibold'>
          {t('Model marketplace display')}
        </h3>
        <p className='text-muted-foreground mt-1 text-xs'>
          {recognized
            ? t(
                'The model marketplace shows this pricing in a readable form. Notes below are display-only and do not affect billing.'
              )
            : t(
                'This expression cannot be shown in a readable form. The model marketplace hides the expression and shows only the billing note below.'
              )}
        </p>
      </div>

      {groups.length > 0 && (
        <div className='space-y-3'>
          {groups.map((group) => (
            <div key={group.name} className='overflow-hidden rounded-lg border'>
              <div className='bg-muted/50 grid grid-cols-[minmax(0,1.2fr)_90px_minmax(0,2fr)] gap-3 border-b px-3 py-2 text-xs font-semibold'>
                <span className='font-mono break-all'>{group.name}</span>
                <span>{t('Multiplier')}</span>
                <span>{t('Note')}</span>
              </div>
              <div className='divide-y'>
                {group.rows.map((row) => (
                  <div
                    key={row.value}
                    className='grid grid-cols-[minmax(0,1.2fr)_90px_minmax(0,2fr)] items-center gap-3 px-3 py-2'
                  >
                    <span className='font-mono text-sm break-all'>
                      {row.value}
                    </span>
                    <span className='font-mono text-sm'>×{row.ratio}</span>
                    <Input
                      value={props.notes[group.name]?.[row.value] ?? ''}
                      onChange={(e) =>
                        setNote(group.name, row.value, e.target.value)
                      }
                      maxLength={200}
                      placeholder={t('Note (shown in model marketplace)')}
                      aria-label={t('Note')}
                      className='h-8'
                    />
                  </div>
                ))}
              </div>
            </div>
          ))}
        </div>
      )}

      <div className='space-y-1.5'>
        <label htmlFor='model-billing-note' className='text-sm font-medium'>
          {t('Billing Note')}
        </label>
        <Textarea
          id='model-billing-note'
          value={props.billingNote}
          onChange={(e) => props.onBillingNoteChange(e.target.value)}
          maxLength={1000}
          rows={3}
          placeholder={t(
            'Explain how this model is billed, e.g. ¥0.18 per second of output plus reference video length.'
          )}
        />
      </div>
    </div>
  )
}
