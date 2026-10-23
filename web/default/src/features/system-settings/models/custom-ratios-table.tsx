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
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { cn } from '@/lib/utils'

import {
  CUSTOM_RATIO_MATCH_ANY,
  CUSTOM_RATIO_MATCH_PER_ITEM,
  flattenCustomRatios,
  getCustomRatioMatchMode,
  unflattenCustomRatios,
  type CustomRatioMatchMode,
  type FlatRatioRow,
} from './model-pricing-core'

type CustomRatiosTableProps = {
  value: Record<string, Record<string, number>>
  notes?: Record<string, Record<string, string>>
  onChange: (
    ratios: Record<string, Record<string, number>>,
    notes: Record<string, Record<string, string>>
  ) => void
}

// groupId 标识同一参数名下的一组行，与参数名解耦，
// 这样新建的多个空参数组不会被合并，重命名参数时组也保持稳定。
type EditorRow = FlatRatioRow & { groupId: string }

const GRID_COLS = 'grid grid-cols-[minmax(0,1.3fr)_110px_minmax(0,2fr)_36px]'

export function CustomRatiosTable({
  value,
  notes,
  onChange,
}: CustomRatiosTableProps) {
  const { t } = useTranslation()
  const matchModeOptions: Array<{
    value: CustomRatioMatchMode
    label: string
  }> = [
    { value: 'exact', label: t('Exact value') },
    { value: 'any', label: t('Any value') },
    { value: 'per_item', label: t('Per item') },
  ]
  // 本组件自持行状态：行的 id 由本地稳定分配，永不从内容派生，
  // 避免编辑时因 id 变化触发 <Input> 卸载重挂而丢失焦点。
  // 切换模型时通过调用处的 key={editorReloadToken} 重新挂载来重新播种。
  const idRef = useRef(0)
  const [rows, setRows] = useState<EditorRow[]>(() =>
    flattenCustomRatios(value, notes).map((row) => ({
      ...row,
      id: `row-${idRef.current++}`,
      groupId: `group-${row.paramName}`,
    }))
  )

  const handleRowsChange = useCallback(
    (newRows: EditorRow[]) => {
      setRows(newRows)
      const next = unflattenCustomRatios(newRows)
      onChange(next.ratios, next.notes)
    },
    [onChange]
  )

  // 按 groupId 首次出现的顺序分组
  const groups: Array<{
    groupId: string
    paramName: string
    rows: EditorRow[]
  }> = []
  for (const row of rows) {
    let group = groups.find((g) => g.groupId === row.groupId)
    if (!group) {
      group = { groupId: row.groupId, paramName: row.paramName, rows: [] }
      groups.push(group)
    }
    group.rows.push(row)
  }

  const newRow = (groupId: string, paramName: string): EditorRow => ({
    id: `row-${idRef.current++}`,
    groupId,
    paramName,
    paramValue: '',
    ratio: 1.0,
    note: '',
  })

  const handleAddGroup = () => {
    handleRowsChange([...rows, newRow(`new-${idRef.current}`, '')])
  }

  const handleAddValue = (groupId: string, paramName: string) => {
    // 新行插在该组末尾，保持同组行连续
    const lastIndex = rows.findLastIndex((row) => row.groupId === groupId)
    const next = [...rows]
    next.splice(lastIndex + 1, 0, newRow(groupId, paramName))
    handleRowsChange(next)
  }

  const handleRenameGroup = (groupId: string, paramName: string) => {
    handleRowsChange(
      rows.map((row) => (row.groupId === groupId ? { ...row, paramName } : row))
    )
  }

  const handleDeleteGroup = (groupId: string) => {
    handleRowsChange(rows.filter((row) => row.groupId !== groupId))
  }

  const handleDeleteRow = (id: string) => {
    handleRowsChange(rows.filter((row) => row.id !== id))
  }

  const handleFieldChange = (
    id: string,
    field: 'paramValue' | 'ratio' | 'note',
    fieldValue: string | number
  ) => {
    handleRowsChange(
      rows.map((row) => (row.id === id ? { ...row, [field]: fieldValue } : row))
    )
  }

  // 切换匹配方式：任意值/按个数使用保留参数值，切回精确值时清空让用户重新填写
  const handleMatchModeChange = (id: string, mode: CustomRatioMatchMode) => {
    let paramValue = ''
    if (mode === 'any') paramValue = CUSTOM_RATIO_MATCH_ANY
    if (mode === 'per_item') paramValue = CUSTOM_RATIO_MATCH_PER_ITEM
    handleFieldChange(id, 'paramValue', paramValue)
  }

  // 参数名允许点号以访问嵌套字段，如 input.images
  const paramNameError = (name: string): string | null => {
    if (!name.trim()) return t('Parameter name is required')
    if (!/^[a-zA-Z0-9_.-]+$/.test(name)) {
      return t(
        'Parameter name can only contain letters, numbers, underscores, hyphens and dots'
      )
    }
    if (groups.filter((g) => g.paramName === name).length > 1) {
      return t('This parameter name is already used by another group')
    }
    return null
  }

  const rowErrors = (row: EditorRow) => {
    const errors: string[] = []
    if (!row.paramValue.trim()) {
      errors.push(t('Parameter value is required'))
    } else if (row.paramValue.length > 100) {
      errors.push(t('Parameter value is too long'))
    }
    if (!(row.ratio > 0)) errors.push(t('Multiplier must be greater than 0'))
    const duplicate = rows.some(
      (other) =>
        other.id !== row.id &&
        other.groupId === row.groupId &&
        other.paramValue === row.paramValue
    )
    return { errors, duplicate }
  }

  return (
    <div className='space-y-4'>
      <div>
        <h3 className='text-sm font-semibold'>
          {t('Custom Parameter Multipliers')}
        </h3>
        <p className='text-muted-foreground mt-1 text-xs'>
          {t(
            'Configure request parameter multipliers. Multiple multipliers will be multiplied together.'
          )}
        </p>
        <p className='text-muted-foreground mt-1 text-xs'>
          {t(
            'Exact value: applies when the parameter equals the value. Any value: applies whenever the parameter is present and non-empty. Per item: multiplier × item count (array length; a non-empty single value counts as 1; capped at 128). Use dots for nested fields, e.g. input.images.'
          )}
        </p>
      </div>

      <div className='flex justify-end'>
        <Button
          type='button'
          variant='outline'
          size='sm'
          onClick={handleAddGroup}
        >
          <Plus className='mr-2 size-4' />
          {t('Add Parameter')}
        </Button>
      </div>

      {groups.length === 0 ? (
        <div className='text-muted-foreground rounded-lg border px-4 py-8 text-center text-sm'>
          {t('No custom parameter multipliers configured')}
        </div>
      ) : (
        <div className='space-y-3'>
          {groups.map((group) => {
            const nameError = paramNameError(group.paramName)
            return (
              <div
                key={group.groupId}
                className='overflow-hidden rounded-lg border'
              >
                {/* 组头：参数名 | 倍率 | 备注 */}
                <div
                  className={cn(
                    GRID_COLS,
                    'bg-muted/50 items-center gap-3 border-b px-3 py-2 text-xs font-semibold'
                  )}
                >
                  <Input
                    value={group.paramName}
                    onChange={(e) =>
                      handleRenameGroup(group.groupId, e.target.value)
                    }
                    placeholder={t('Parameter Name')}
                    aria-label={t('Parameter Name')}
                    className={cn(
                      'h-8 font-mono font-semibold',
                      nameError && 'border-red-500'
                    )}
                  />
                  <div>{t('Multiplier')}</div>
                  <div>{t('Note')}</div>
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    className='size-8 p-0'
                    onClick={() => handleDeleteGroup(group.groupId)}
                    title={t('Delete parameter')}
                    aria-label={t('Delete parameter')}
                  >
                    <Trash2 className='size-4' />
                  </Button>
                </div>
                {nameError && (
                  <div className='px-3 pt-1 text-xs text-red-500'>
                    {nameError}
                  </div>
                )}

                {/* 参数值行：参数值 | 倍率 | 备注 */}
                <div className='divide-y'>
                  {group.rows.map((row) => {
                    const matchMode = getCustomRatioMatchMode(row.paramValue)
                    const { errors, duplicate } = rowErrors(row)
                    return (
                      <div key={row.id} className='px-3 py-2'>
                        <div className={cn(GRID_COLS, 'items-center gap-3')}>
                          <div className='flex min-w-0 items-center gap-2'>
                            <Select
                              items={matchModeOptions}
                              value={matchMode}
                              onValueChange={(v) =>
                                v !== null && handleMatchModeChange(row.id, v)
                              }
                            >
                              <SelectTrigger
                                className='h-8 w-24 shrink-0'
                                aria-label={t('Match')}
                              >
                                <SelectValue />
                              </SelectTrigger>
                              <SelectContent alignItemWithTrigger={false}>
                                <SelectGroup>
                                  {matchModeOptions.map((option) => (
                                    <SelectItem
                                      key={option.value}
                                      value={option.value}
                                    >
                                      {option.label}
                                    </SelectItem>
                                  ))}
                                </SelectGroup>
                              </SelectContent>
                            </Select>
                            {matchMode === 'exact' ? (
                              <Input
                                value={row.paramValue}
                                onChange={(e) =>
                                  handleFieldChange(
                                    row.id,
                                    'paramValue',
                                    e.target.value
                                  )
                                }
                                placeholder={t('Parameter Value')}
                                aria-label={t('Parameter Value')}
                                className={cn(
                                  'h-8 min-w-0 font-mono',
                                  errors.length > 0 &&
                                    !row.paramValue.trim() &&
                                    'border-red-500'
                                )}
                              />
                            ) : (
                              <span className='font-mono text-sm'>
                                {row.paramValue}
                              </span>
                            )}
                          </div>
                          <Input
                            type='number'
                            step='0.01'
                            min='0.01'
                            value={row.ratio}
                            onChange={(e) =>
                              handleFieldChange(
                                row.id,
                                'ratio',
                                Number.parseFloat(e.target.value) || 0
                              )
                            }
                            placeholder='1.0'
                            aria-label={t('Multiplier')}
                            className={cn(
                              'h-8',
                              !(row.ratio > 0) && 'border-red-500'
                            )}
                          />
                          <Input
                            value={row.note}
                            onChange={(e) =>
                              handleFieldChange(row.id, 'note', e.target.value)
                            }
                            maxLength={200}
                            placeholder={t('Note (shown in model marketplace)')}
                            aria-label={t('Note')}
                            className='h-8'
                          />
                          <Button
                            type='button'
                            variant='ghost'
                            size='sm'
                            className='size-8 p-0'
                            onClick={() => handleDeleteRow(row.id)}
                            title={t('Delete')}
                            aria-label={t('Delete')}
                          >
                            {duplicate ? (
                              <AlertTriangle className='size-4 text-amber-500' />
                            ) : (
                              <Trash2 className='size-4' />
                            )}
                          </Button>
                        </div>
                        {(errors.length > 0 || duplicate) && (
                          <div className='pt-1 text-xs'>
                            {errors.map((message) => (
                              <div key={message} className='text-red-500'>
                                {message}
                              </div>
                            ))}
                            {duplicate && (
                              <div className='text-amber-600'>
                                {t(
                                  'This parameter value already exists and will override the previous configuration'
                                )}
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    )
                  })}
                </div>

                <div className='border-t px-3 py-1.5'>
                  <Button
                    type='button'
                    variant='ghost'
                    size='sm'
                    className='h-7 text-xs'
                    onClick={() =>
                      handleAddValue(group.groupId, group.paramName)
                    }
                  >
                    <Plus className='mr-1 size-3.5' />
                    {t('Add value')}
                  </Button>
                </div>
              </div>
            )
          })}
        </div>
      )}
    </div>
  )
}
