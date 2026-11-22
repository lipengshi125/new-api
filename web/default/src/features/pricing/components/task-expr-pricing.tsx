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

import { formatTaskUnitPrice } from '../lib/price'
import {
  groupTaskMultipliers,
  type TaskExprPricing,
  type TaskSecondsSource,
} from '../lib/task-expr'
import type { PricingModel } from '../types'

type PriceDisplayOptions = {
  showRechargePrice: boolean
  priceRate: number
  usdExchangeRate: number
}

function formatRatio(ratio: number): string {
  return String(Number(ratio.toFixed(4)))
}

export function TaskUnitLabel(props: { pricing: TaskExprPricing }) {
  const { t } = useTranslation()
  return (
    <span>{props.pricing.unit === 'second' ? t('sec') : t('request')}</span>
  )
}

function describeSecondsSource(
  source: TaskSecondsSource,
  t: (key: string, options?: Record<string, unknown>) => string
): string {
  if (source.kind === 'video') {
    if (source.maxSeconds) {
      return t('Reference video duration ({{path}}, max {{max}}s)', {
        path: source.path,
        max: source.maxSeconds,
      })
    }
    return t('Reference video duration ({{path}})', { path: source.path })
  }
  return t('Output duration ({{path}})', { path: source.path })
}

/**
 * 任务模型（按次/按秒）的表达式定价展示：计费方式、基础价格、计费秒数、
 * 参数倍率表、计费公式和示例，替代面向 token 的分档价格表。
 */
export function TaskExprPricingDetails(
  props: {
    model: PricingModel
    pricing: TaskExprPricing
  } & PriceDisplayOptions
) {
  const { t } = useTranslation()
  const { pricing } = props
  const notes = props.model.custom_ratio_notes
  const groups = groupTaskMultipliers(pricing.multipliers)
  // 加法形式 tier("base", K * (倍率) + 常数 + item_count(...) * N) 里，
  // 基础价是 K 加上常数项；倍率和按个数加价另行展示。
  const additiveBase =
    (pricing.extraBasePrice ?? 0) + (pricing.extraFixedCharge ?? 0)
  const basePriceUSD = pricing.unitPrice + additiveBase
  const unitPrice = formatTaskUnitPrice(
    basePriceUSD,
    1,
    props.showRechargePrice,
    props.priceRate,
    props.usdExchangeRate
  )
  const unitLabel = pricing.unit === 'second' ? t('sec') : t('request')
  const hasVideo = pricing.secondsSources.some((s) => s.kind === 'video')
  // 只有表达式里真有输出时长项（param("duration") 等）才计入输出秒数；
  // 只按参考视频计费（如 video_seconds("videos")）时示例不能凭空加上输出 5 秒。
  const hasOutput = pricing.secondsSources.some((s) => s.kind === 'param')

  // 公式：单价 × (秒数项之和) × 各参数倍率 × 分组倍率
  const formulaParts = [unitPrice]
  if (pricing.unit === 'second') {
    const terms = pricing.secondsSources.map((s) =>
      s.kind === 'video' ? t('reference video seconds') : t('output seconds')
    )
    formulaParts.push(terms.length > 1 ? `(${terms.join(' + ')})` : terms[0])
  }
  for (const group of groups) {
    formulaParts.push(t('{{name}} multiplier', { name: group.name }))
  }
  formulaParts.push(t('group ratio'))

  // 示例：表达式里有的秒数项各取 5 秒（参考视频 / 输出），使用倍率最高的参数值
  const exampleOutput = hasOutput ? 5 : 0
  const exampleVideo = hasVideo ? 5 : 0
  const exampleSeconds =
    pricing.unit === 'second' ? exampleOutput + exampleVideo : 1
  const exampleChoices = groups.map((group) => {
    const top = [...group.rows].sort((a, b) => b.ratio - a.ratio)[0]
    return { name: group.name, value: top.value, ratio: top.ratio }
  })
  const exampleMultiplier = exampleChoices.reduce((acc, c) => acc * c.ratio, 1)
  const itemChargeEntries = Object.entries(pricing.itemCharges ?? {}).sort(
    ([a], [b]) => a.localeCompare(b)
  )
  const exampleItemCount = 2
  const exampleItemTotal = itemChargeEntries.reduce(
    (acc, [, usd]) => acc + usd * exampleItemCount,
    0
  )
  const exampleBaseUSD = (pricing.unitPrice + additiveBase) *
    exampleSeconds *
    exampleMultiplier
  const examplePrice = formatTaskUnitPrice(
    exampleBaseUSD + exampleItemTotal,
    1,
    props.showRechargePrice,
    props.priceRate,
    props.usdExchangeRate
  )
  let exampleSecondsCondition = ''
  if (hasVideo && hasOutput) {
    exampleSecondsCondition = t(
      '{{video}}s reference video, {{output}}s output',
      { video: exampleVideo, output: exampleOutput }
    )
  } else if (hasVideo) {
    exampleSecondsCondition = t('{{video}}s reference video', {
      video: exampleVideo,
    })
  } else if (hasOutput) {
    exampleSecondsCondition = t('{{output}}s output', { output: exampleOutput })
  }
  let exampleSecondsStep = ''
  if (hasVideo && hasOutput) {
    exampleSecondsStep = `(${exampleVideo}+${exampleOutput})`
  } else if (hasVideo || hasOutput) {
    exampleSecondsStep = String(exampleVideo + exampleOutput)
  }
  const exampleConditions = [
    ...(pricing.unit === 'second' && exampleSecondsCondition
      ? [exampleSecondsCondition]
      : []),
    ...exampleChoices.map((c) => `${c.name}=${c.value}`),
    ...itemChargeEntries.map(([path]) =>
      t('{{count}} {{path}}', { count: exampleItemCount, path })
    ),
  ]
  // 计算过程：单价 × 秒数 × 各参数倍率，例如 $0.05×5×2×1.35
  const exampleSteps = [
    unitPrice,
    ...(pricing.unit === 'second' && exampleSecondsStep
      ? [exampleSecondsStep]
      : []),
    ...exampleChoices.map((c) => formatRatio(c.ratio)),
  ].join('×') +
    (itemChargeEntries.length > 0
      ? ` + ${itemChargeEntries
          .map(([, usd]) => `$${Number((usd * exampleItemCount).toFixed(4))}`)
          .join(' + ')}`
      : '')

  const rowClass =
    'grid grid-cols-[7rem_minmax(0,1fr)] gap-3 py-1.5 text-sm sm:grid-cols-[8rem_minmax(0,1fr)]'

  return (
    <section className='space-y-4'>
      <div className='bg-muted/20 rounded-lg border px-3 py-1.5'>
        <div className={rowClass}>
          <span className='text-muted-foreground'>{t('Billing Mode')}</span>
          <span>
            {pricing.unit === 'second' ? t('Per Second') : t('Per Request')}
          </span>
        </div>
        <div className={rowClass}>
          <span className='text-muted-foreground'>{t('Base Price')}</span>
          <span className='font-mono font-semibold text-amber-600 tabular-nums dark:text-amber-400'>
            {unitPrice}
            <span className='text-muted-foreground ml-1 text-xs font-normal'>
              / {unitLabel}
            </span>
          </span>
        </div>
        {pricing.unit === 'second' && (
          <div className={rowClass}>
            <span className='text-muted-foreground'>{t('Billed Seconds')}</span>
            <span>
              {pricing.secondsSources
                .map((s) => describeSecondsSource(s, t))
                .join(' + ')}
            </span>
          </div>
        )}
      </div>

      {groups.length > 0 && (
        <div>
          <div className='text-foreground mb-2 text-sm font-semibold'>
            {t('Parameter Multipliers')}
          </div>
          <div className='bg-muted/20 overflow-hidden rounded-lg border'>
            <table className='w-full table-fixed text-sm'>
              <colgroup>
                <col className='w-[30%]' />
                <col className='w-[18%]' />
                <col />
              </colgroup>
              {groups.map((group) => (
                <tbody key={group.name} className='border-b last:border-b-0'>
                  <tr className='bg-muted/40'>
                    <th
                      scope='rowgroup'
                      className='text-foreground px-3 py-2 text-left font-mono text-xs font-semibold break-all'
                    >
                      {group.name}
                    </th>
                    <th className='text-muted-foreground px-3 py-2 text-left text-xs font-medium'>
                      {t('Multiplier')}
                    </th>
                    <th className='text-muted-foreground px-3 py-2 text-left text-xs font-medium'>
                      {t('Note')}
                    </th>
                  </tr>
                  {group.rows.map((row) => (
                    <tr key={row.value} className='border-t'>
                      <td className='text-muted-foreground px-3 py-2.5 font-mono break-all'>
                        {row.value}
                      </td>
                      <td className='text-foreground px-3 py-2.5 font-mono font-semibold'>
                        ×{formatRatio(row.ratio)}
                      </td>
                      <td className='text-muted-foreground px-3 py-2.5 wrap-break-word'>
                        {notes?.[group.name]?.[row.value] || '-'}
                      </td>
                    </tr>
                  ))}
                </tbody>
              ))}
            </table>
          </div>
          <p className='text-muted-foreground mt-1.5 text-xs'>
            {t('Values not listed are billed at ×1.')}
          </p>
        </div>
      )}

      {Object.keys(pricing.itemCharges).length > 0 && (
        <div>
          <div className='text-foreground mb-2 text-sm font-semibold'>
            {t('Per-Item Charges')}
          </div>
          <div className='bg-muted/20 space-y-1 rounded-lg border px-3 py-2.5'>
            {Object.entries(pricing.itemCharges)
              .sort(([a], [b]) => a.localeCompare(b))
              .map(([path, usd]) => (
                <div key={path} className='flex justify-between items-center'>
                  <span className='text-muted-foreground font-mono text-sm'>
                    {path}
                  </span>
                  <span className='font-mono font-semibold text-blue-600 dark:text-blue-400'>
                    +${Number(usd.toFixed(4))}
                  </span>
                </div>
              ))}
          </div>
          <p className='text-muted-foreground mt-1.5 text-xs'>
            {t('Added per non-empty item at the path')}
          </p>
        </div>
      )}

      <div className='bg-muted/20 space-y-1.5 rounded-lg border px-3 py-2.5 text-sm'>
        <div>
          <span className='text-muted-foreground'>
            {t('Billing formula')}：
          </span>
          <span className='font-mono'>{formulaParts.join(' × ')}</span>
        </div>
        <div>
          <span className='text-muted-foreground'>{t('Example')}：</span>
          {exampleConditions.join('，')}
          {exampleConditions.length > 0 && ' → '}
          <span className='text-muted-foreground font-mono'>
            {exampleSteps}
          </span>
          {' → '}
          <span className='font-mono font-semibold'>{examplePrice}</span>
          <span className='text-muted-foreground text-xs'>
            {' '}
            ({t('before group ratio')})
          </span>
        </div>
      </div>

      {props.model.billing_note && (
        <BillingNoteBlock note={props.model.billing_note} />
      )}
    </section>
  )
}

/** 管理员填写的计费说明 */
export function BillingNoteBlock(props: { note: string }) {
  const { t } = useTranslation()
  return (
    <div className='bg-muted/20 rounded-lg border px-3 py-2.5'>
      <div className='text-muted-foreground mb-1 text-xs font-medium'>
        {t('Billing Note')}
      </div>
      <p className='text-sm whitespace-pre-wrap'>{props.note}</p>
    </div>
  )
}

/**
 * 无法结构化识别的表达式：不展示原始表达式，只展示管理员的计费说明。
 */
export function UnrecognizedExprPricing(props: { model: PricingModel }) {
  const { t } = useTranslation()
  if (props.model.billing_note) {
    return <BillingNoteBlock note={props.model.billing_note} />
  }
  return (
    <p className='text-muted-foreground text-sm'>
      {t(
        'This model uses dynamic pricing. Please contact the administrator for details.'
      )}
    </p>
  )
}
