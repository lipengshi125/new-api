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
import { Copy, Check } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import type { TaskLog } from '../../types'

interface TaskDetailsDialogProps {
  log: TaskLog | null
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function TaskDetailsDialog({
  log,
  open,
  onOpenChange,
}: TaskDetailsDialogProps) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard({ notify: false })

  // 与经典前端一致：展示整条任务记录（含上游返回的 data），data 若为字符串则展开为对象
  const rawJson = useMemo(() => {
    if (!log) return ''
    let data: unknown = log.data
    if (typeof data === 'string' && data !== '') {
      try {
        data = JSON.parse(data)
      } catch {
        // 非 JSON 字符串原样展示
      }
    }
    return JSON.stringify({ ...log, data }, null, 2)
  }, [log])

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={t('Task Details')}
      description={t('View the complete details for this task')}
      contentClassName='sm:max-w-3xl'
      contentHeight='min(70dvh, 640px)'
    >
      <div className='bg-muted/50 relative rounded-md border p-3'>
        <Button
          variant='ghost'
          size='sm'
          className='absolute top-2 right-2 h-8 w-8 p-0'
          onClick={() => copyToClipboard(rawJson)}
          title={t('Copy to clipboard')}
          aria-label={t('Copy to clipboard')}
        >
          {copiedText === rawJson ? (
            <Check className='size-4 text-green-600' />
          ) : (
            <Copy className='size-4' />
          )}
        </Button>
        <pre className='pr-10 font-mono text-xs leading-relaxed break-all whitespace-pre-wrap'>
          {rawJson}
        </pre>
      </div>
    </Dialog>
  )
}
