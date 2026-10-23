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

import type { PricingModel } from '../types'

/**
 * 模型广场列表中的参数倍率简表：按参数名分组，
 * 每个参数值一行「参数值 ×倍率 备注」。
 */
export function CustomRatiosSummary(props: { model: PricingModel }) {
  const { t } = useTranslation()
  const ratios = props.model.custom_ratios
  if (!ratios || Object.keys(ratios).length === 0) return null
  const notes = props.model.custom_ratio_notes

  const groups = Object.entries(ratios)
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([paramName, valueMap]) => ({
      paramName,
      rows: Object.entries(valueMap).sort(([a], [b]) =>
        a.localeCompare(b, undefined, { numeric: true })
      ),
    }))
    .filter((group) => group.rows.length > 0)

  return (
    <div className='border-border/30 mt-1.5 space-y-1 border-t pt-1'>
      <div className='text-muted-foreground/70 text-[9px] font-semibold tracking-wide uppercase'>
        {t('Parameter Multipliers')}
      </div>
      {groups.map((group) => (
        <div key={group.paramName} className='space-y-0.5'>
          <div className='text-foreground/80 font-mono text-[10px] font-semibold break-all'>
            {group.paramName}
          </div>
          {group.rows.map(([paramValue, ratio]) => {
            const note = notes?.[group.paramName]?.[paramValue]
            return (
              <div
                key={paramValue}
                className='grid grid-cols-[auto_auto_minmax(0,1fr)] items-baseline gap-x-1.5 pl-2 text-[10px]'
              >
                <span className='text-muted-foreground font-mono break-all'>
                  {paramValue}
                </span>
                <span className='font-mono font-semibold text-purple-600 dark:text-purple-400'>
                  ×{ratio}
                </span>
                <span className='text-muted-foreground/80 wrap-break-word'>
                  {note}
                </span>
              </div>
            )
          })}
        </div>
      ))}
    </div>
  )
}
