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

  const handleAddRow = useCallback(() => {
    const newRow: FlatRatioRow = {
      id: `new-${Date.now()}`,
      paramName: '',
      paramValue: '',
      ratio: 1.0,
    }
    handleRowsChange([...rows, newRow])
  }, [rows, handleRowsChange])

  const handleDeleteRow = useCallback(
    (id: string) => {
      handleRowsChange(rows.filter((row) => row.id !== id))
    },
    [rows, handleRowsChange]
  )

  const handleFieldChange = useCallback(
    (id: string, field: keyof FlatRatioRow, value: string | number) => {
      const newRows = rows.map((row) =>
        row.id === id ? { ...row, [field]: value } : row
      )
      handleRowsChange(newRows)
    },
    [rows, handleRowsChange]
  )

  // 验证参数名格式
  const validateParamName = (name: string): string | null => {
    if (!name.trim()) return t('Parameter name is required')
    if (!/^[a-zA-Z0-9_-]+$/.test(name)) {
      return t(
        'Parameter name can only contain letters, numbers, underscores and hyphens'
      )
    }
    return null
  }

  // 验证参数值
  const validateParamValue = (value: string): string | null => {
    if (!value.trim()) return t('Parameter value is required')
    if (value.length > 100) return t('Parameter value is too long')
    return null
  }

  // 验证倍率
  const validateRatio = (ratio: number): string | null => {
    if (ratio <= 0) return t('Multiplier must be greater than 0')
    return null
  }

  // 检查重复的参数名+参数值组合
  const checkDuplicate = (
    paramName: string,
    paramValue: string,
    currentId: string
  ): boolean => {
    return rows.some(
      (row) =>
        row.id !== currentId &&
        row.paramName === paramName &&
        row.paramValue === paramValue
    )
  }

  // 获取每行的验证错误
  const getRowErrors = useCallback(
    (row: FlatRatioRow) => {
      return {
        paramName: validateParamName(row.paramName),
        paramValue: validateParamValue(row.paramValue),
        ratio: validateRatio(row.ratio),
        duplicate: checkDuplicate(row.paramName, row.paramValue, row.id),
      }
    },
    [rows]
  )

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

      {/* 工具栏 */}
      <div className='flex justify-end'>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={handleAddRow}
        >
          <Plus className='mr-2 size-4' />
          {t('Add Parameter')}
        </Button>
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
              <div key={row.id}>
                <div
                  className={cn(
                    'grid grid-cols-[1fr_1fr_120px_60px] gap-4 px-4 py-2',
                    getRowBackground(row.paramName, index)
                  )}
                >
                  {/* 参数名列 */}
                  <div className='flex items-center'>
                    {isFirstRowOfParam(index) ? (
                      <Input
                        value={row.paramName}
                        onChange={(e) =>
                          handleFieldChange(row.id, 'paramName', e.target.value)
                        }
                        placeholder={t('Parameter Name')}
                        className={cn(
                          'h-8',
                          getRowErrors(row).paramName && 'border-red-500'
                        )}
                      />
                    ) : (
                      <div className='flex items-center gap-2'>
                        <span className='text-muted-foreground'>↳</span>
                        <Input
                          value={row.paramName}
                          onChange={(e) =>
                            handleFieldChange(row.id, 'paramName', e.target.value)
                          }
                          placeholder={t('Parameter Name')}
                          className={cn(
                            'h-8',
                            getRowErrors(row).paramName && 'border-red-500'
                          )}
                        />
                      </div>
                    )}
                  </div>

                  {/* 参数值列 */}
                  <div className='flex items-center'>
                    <Input
                      value={row.paramValue}
                      onChange={(e) =>
                        handleFieldChange(row.id, 'paramValue', e.target.value)
                      }
                      placeholder={t('Parameter Value')}
                      className={cn(
                        'h-8',
                        getRowErrors(row).paramValue && 'border-red-500'
                      )}
                    />
                  </div>

                  {/* 倍率列 */}
                  <div className='flex items-center'>
                    <Input
                      type='number'
                      step='0.01'
                      min='0.01'
                      value={row.ratio}
                      onChange={(e) =>
                        handleFieldChange(row.id, 'ratio', parseFloat(e.target.value) || 0)
                      }
                      placeholder='1.0'
                      className={cn(
                        'h-8',
                        getRowErrors(row).ratio && 'border-red-500'
                      )}
                    />
                  </div>

                  {/* 操作列 */}
                  <div className='flex items-center justify-end'>
                    {/* 重复警告图标 */}
                    {getRowErrors(row).duplicate && (
                      <AlertTriangle className='text-amber-500 mr-2 size-4' />
                    )}
                    <Button
                      type='button'
                      variant='ghost'
                      size='sm'
                      className='size-8 p-0'
                      onClick={() => handleDeleteRow(row.id)}
                    >
                      <Trash2 className='size-4' />
                    </Button>
                  </div>
                </div>

                {/* 错误提示 */}
                {(getRowErrors(row).paramName ||
                  getRowErrors(row).paramValue ||
                  getRowErrors(row).ratio ||
                  getRowErrors(row).duplicate) && (
                  <div className='px-4 pb-2 text-xs'>
                    {getRowErrors(row).paramName && (
                      <div className='text-red-500'>{getRowErrors(row).paramName}</div>
                    )}
                    {getRowErrors(row).paramValue && (
                      <div className='text-red-500'>{getRowErrors(row).paramValue}</div>
                    )}
                    {getRowErrors(row).ratio && (
                      <div className='text-red-500'>{getRowErrors(row).ratio}</div>
                    )}
                    {getRowErrors(row).duplicate && (
                      <div className='text-amber-600'>
                        {t(
                          'This parameter value already exists and will override the previous configuration'
                        )}
                      </div>
                    )}
                  </div>
                )}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
