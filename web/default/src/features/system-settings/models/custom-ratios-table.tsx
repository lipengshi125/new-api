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
import { AlertTriangle, Plus, Trash2 } from 'lucide-react'
import { useCallback, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Alert, AlertDescription } from '@/components/ui/alert'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

import {
  flattenCustomRatios,
  unflattenCustomRatios,
  type FlatRatioRow,
} from './model-pricing-core'

type CustomRatiosTableProps = {
  value: Record<string, Record<string, number>>
  onChange: (value: Record<string, Record<string, number>>) => void
}

export function CustomRatiosTable({ value, onChange }: CustomRatiosTableProps) {
  const { t } = useTranslation()
  const [rows, setRows] = useState<FlatRatioRow[]>([])

  // 从 props.value 初始化行数据
  useMemo(() => {
    setRows(flattenCustomRatios(value))
  }, [value])

  const handleRowsChange = useCallback(
    (newRows: FlatRatioRow[]) => {
      setRows(newRows)
      onChange(unflattenCustomRatios(newRows))
    },
    [onChange]
  )

  // 判断当前行是否是该参数名的第一行
  const isFirstRowOfParam = (index: number): boolean => {
    if (index === 0) return true
    return rows[index].paramName !== rows[index - 1].paramName
  }

  // 获取相同参数名的行背景色
  const getRowBackground = (paramName: string, index: number): string => {
    // 找到该参数名首次出现的位置
    const firstIndex = rows.findIndex((r) => r.paramName === paramName)
    // 偶数组用浅色背景，奇数组用默认背景
    const groupIndex = rows
      .slice(0, firstIndex + 1)
      .filter((_, i) => isFirstRowOfParam(i)).length
    return groupIndex % 2 === 0 ? 'bg-muted/30' : ''
  }

  return (
    <div className='space-y-4'>
      <div>
        <h3 className='text-sm font-semibold'>
          {t('Custom Parameter Multipliers')}
        </h3>
        <p className='text-muted-foreground text-xs mt-1'>
          {t(
            'Configure request parameter multipliers. Multiple multipliers will be multiplied together.'
          )}
        </p>
      </div>

      <div className='rounded-lg border'>
        {/* 表格头 */}
        <div className='grid grid-cols-[1fr_1fr_120px_60px] gap-4 border-b bg-muted/50 px-4 py-2 text-xs font-semibold'>
          <div>{t('Parameter Name')}</div>
          <div>{t('Parameter Value')}</div>
          <div>{t('Multiplier')}</div>
          <div>{t('Actions')}</div>
        </div>

        {/* 表格体 */}
        {rows.length === 0 ? (
          <div className='text-muted-foreground px-4 py-8 text-center text-sm'>
            {t('No custom parameter multipliers configured')}
          </div>
        ) : (
          <div className='divide-y'>
            {rows.map((row, index) => (
              <div
                key={row.id}
                className={cn(
                  'grid grid-cols-[1fr_1fr_120px_60px] gap-4 px-4 py-2',
                  getRowBackground(row.paramName, index)
                )}
              >
                {/* 参数名列 */}
                <div className='flex items-center'>
                  {isFirstRowOfParam(index) ? (
                    <span className='font-medium'>{row.paramName}</span>
                  ) : (
                    <span className='text-muted-foreground'>↳</span>
                  )}
                </div>

                {/* 参数值列 */}
                <div className='flex items-center'>
                  <span>{row.paramValue}</span>
                </div>

                {/* 倍率列 */}
                <div className='flex items-center'>
                  <span>×{row.ratio}</span>
                </div>

                {/* 操作列 */}
                <div className='flex items-center justify-end'>
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    className='size-8 p-0'
                    onClick={() => {
                      // 删除逻辑将在下一个任务添加
                    }}
                  >
                    <Trash2 className='size-4' />
                  </Button>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
