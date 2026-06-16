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
/* eslint-disable react-refresh/only-export-components */
import { useState, useMemo } from 'react'
import type { ColumnDef } from '@tanstack/react-table'
import { Music } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { getUserAvatarFallback, getUserAvatarStyle } from '@/lib/avatar'
import { formatTimestampToDate } from '@/lib/format'
import { cn } from '@/lib/utils'
import { Avatar, AvatarFallback } from '@/components/ui/avatar'
import { StatusBadge } from '@/components/status-badge'
import { TASK_PLATFORM_MAPPINGS, TASK_STATUS } from '../../constants'
import { taskPlatformMapper, taskStatusMapper } from '../../lib/mappers'
import type { TaskLog } from '../../types'
import {
  AudioPreviewDialog,
  type AudioClip,
} from '../dialogs/audio-preview-dialog'
import { FailReasonDialog } from '../dialogs/fail-reason-dialog'
import { useUsageLogsContext } from '../usage-logs-provider'
import {
  createDurationColumn,
  createChannelColumn,
  createProgressColumn,
  createTimestampColumn,
} from './column-helpers'

function parseTaskData(data: unknown): unknown[] {
  if (Array.isArray(data)) return data
  if (typeof data === 'string') {
    try {
      const parsed = JSON.parse(data)
      return Array.isArray(parsed) ? parsed : []
    } catch {
      return []
    }
  }
  return []
}

/** Recursively extract all { url, isVideo } pairs from a JSON blob. */
function extractMediaUrls(
  obj: unknown
): Array<{ url: string; isVideo: boolean }> {
  const results: Array<{ url: string; isVideo: boolean }> = []
  function walk(node: unknown) {
    if (!node || typeof node !== 'object') return
    if (Array.isArray(node)) {
      node.forEach(walk)
      return
    }
    const rec = node as Record<string, unknown>
    for (const [k, v] of Object.entries(rec)) {
      if (k === 'url' && typeof v === 'string' && v.startsWith('http')) {
        const isVideo = /\.(mp4|mov|webm|m3u8)/i.test(v)
        results.push({ url: v, isVideo })
      } else {
        walk(v)
      }
    }
  }
  walk(obj)
  return results
}

function AudioPreviewCell({ log }: { log: TaskLog }) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const clips = useMemo(() => {
    const data = parseTaskData(log.data)
    return data.filter(
      (c) =>
        c && typeof c === 'object' && (c as Record<string, unknown>).audio_url
    )
  }, [log.data])

  if (clips.length === 0) return null

  return (
    <>
      <button
        type='button'
        className='group flex items-center gap-1 text-left text-xs'
        onClick={() => setOpen(true)}
      >
        <Music className='text-muted-foreground size-3' />
        <span className='text-foreground leading-snug group-hover:underline'>
          {t('Click to preview audio')}
        </span>
      </button>
      <AudioPreviewDialog
        open={open}
        onOpenChange={setOpen}
        clips={clips as AudioClip[]}
      />
    </>
  )
}

function MediaPreviewCell({ log }: { log: TaskLog }) {
  const media = useMemo(() => {
    if (log.status !== TASK_STATUS.SUCCESS || !log.data) return []
    try {
      const parsed =
        typeof log.data === 'string' ? JSON.parse(log.data) : log.data
      return extractMediaUrls(parsed)
    } catch {
      return []
    }
  }, [log.data, log.status])

  if (media.length === 0) return null

  return (
    <div className='flex flex-wrap gap-1'>
      {media.map((item, i) =>
        item.isVideo ? (
          <a
            key={i}
            href={item.url}
            target='_blank'
            rel='noopener noreferrer'
            className='block'
          >
            {/* eslint-disable-next-line jsx-a11y/media-has-caption */}
            <video
              src={item.url}
              className='h-14 w-14 cursor-pointer rounded object-cover'
              muted
            />
          </a>
        ) : (
          <a
            key={i}
            href={item.url}
            target='_blank'
            rel='noopener noreferrer'
            className='block'
          >
            <img
              src={item.url}
              className='h-14 w-14 cursor-pointer rounded object-cover'
              alt=''
            />
          </a>
        )
      )}
    </div>
  )
}

export function useTaskLogsColumns(isAdmin: boolean): ColumnDef<TaskLog>[] {
  const { t } = useTranslation()
  const columns: ColumnDef<TaskLog>[] = [
    // 提交时间
    {
      accessorKey: 'submit_time',
      header: t('Submit Time'),
      cell: ({ row }) => {
        const submitTime = row.getValue('submit_time') as number
        return (
          <span className='font-mono text-xs tabular-nums'>
            {formatTimestampToDate(submitTime, 'seconds')}
          </span>
        )
      },
      size: 150,
    },
    // 结束时间
    createTimestampColumn<TaskLog>({
      accessorKey: 'finish_time',
      title: t('Finish Time'),
      unit: 'seconds',
    }),
    // 花费时间
    createDurationColumn<TaskLog>({
      submitTimeKey: 'submit_time',
      finishTimeKey: 'finish_time',
      unit: 'seconds',
      headerLabel: t('Duration'),
      warningThresholdSec: 300,
    }),
    // 平台
    {
      accessorKey: 'platform',
      header: t('Platform'),
      cell: ({ row }) => {
        const platform = row.getValue('platform') as string
        if (!platform)
          return <span className='text-muted-foreground/60 text-xs'>-</span>
        return (
          <StatusBadge
            label={taskPlatformMapper.getLabel(platform, platform)}
            variant={
              (TASK_PLATFORM_MAPPINGS[platform]?.variant as
                | 'green'
                | 'blue'
                | 'violet'
                | 'orange'
                | 'pink'
                | undefined) ?? 'neutral'
            }
            size='sm'
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
    },
  ]

  if (isAdmin) {
    columns.push(createChannelColumn<TaskLog>({ headerLabel: t('Channel') }), {
      id: 'user',
      header: t('User'),
      accessorFn: (row) => row.username || row.user_id,
      cell: function UserCell({ row }) {
        const { sensitiveVisible, setSelectedUserId, setUserInfoDialogOpen } =
          useUsageLogsContext()
        const log = row.original
        const displayName = log.username || String(log.user_id || '?')

        return (
          <button
            type='button'
            className='flex items-center gap-1.5 text-left'
            onClick={(e) => {
              e.stopPropagation()
              setSelectedUserId(log.user_id)
              setUserInfoDialogOpen(true)
            }}
          >
            <Avatar className='ring-border/60 size-6 ring-1 max-sm:hidden'>
              <AvatarFallback
                className={cn(
                  'text-[11px] font-semibold',
                  !sensitiveVisible && 'bg-muted text-muted-foreground'
                )}
                style={
                  sensitiveVisible ? getUserAvatarStyle(displayName) : undefined
                }
              >
                {sensitiveVisible ? getUserAvatarFallback(displayName) : '•'}
              </AvatarFallback>
            </Avatar>
            <span className='text-muted-foreground truncate text-sm hover:underline'>
              {sensitiveVisible ? displayName : '••••'}
            </span>
          </button>
        )
      },
    })
  }

  columns.push(
    // 令牌名称
    {
      accessorKey: 'token_name',
      header: t('Token Name'),
      cell: ({ row }) => {
        const tokenName = row.getValue('token_name') as string
        if (!tokenName)
          return <span className='text-muted-foreground/60 text-xs'>-</span>
        return (
          <StatusBadge
            label={tokenName}
            autoColor={tokenName}
            size='sm'
            copyable={false}
            className='border-border/60 bg-muted/30 max-w-[120px] truncate rounded-md border px-1.5 py-0.5'
          />
        )
      },
    },
    // 任务ID
    {
      accessorKey: 'task_id',
      header: t('Task ID'),
      cell: ({ row }) => {
        const taskId = row.getValue('task_id') as string
        if (!taskId) {
          return <span className='text-muted-foreground/60 text-xs'>-</span>
        }
        return (
          <StatusBadge
            label={taskId}
            autoColor={taskId}
            size='sm'
            className='border-border/60 bg-muted/30 max-w-[170px] truncate rounded-md border px-1.5 py-0.5 font-mono'
          />
        )
      },
      meta: { mobileTitle: true },
    },
    // 任务状态
    {
      accessorKey: 'status',
      header: t('Status'),
      cell: ({ row }) => {
        const status = row.getValue('status') as string
        return (
          <StatusBadge
            label={t(taskStatusMapper.getLabel(status, status || 'Submitting'))}
            variant={taskStatusMapper.getVariant(status)}
            size='sm'
            copyable={false}
            className='-ml-1.5'
          />
        )
      },
    },
    // 进度
    createProgressColumn<TaskLog>({ headerLabel: t('Progress') }),
    // 计费单位
    {
      accessorKey: 'billing_seconds',
      header: t('Billing'),
      cell: ({ row }) => {
        const seconds = row.original.billing_seconds
        if (seconds && seconds > 0) {
          return <span className='font-mono text-xs tabular-nums'>{seconds}s</span>
        }
        return <span className='font-mono text-xs tabular-nums'>1{t('count-unit')}</span>
      },
      size: 80,
    },
    // 详情
    {
      accessorKey: 'fail_reason',
      header: t('Details'),
      cell: function DetailsCell({ row }) {
        const log = row.original
        const failReason = row.getValue('fail_reason') as string
        const status = log.status
        const [dialogOpen, setDialogOpen] = useState(false)

        // Suno: audio preview
        const isSunoSuccess =
          log.platform === 'suno' && status === TASK_STATUS.SUCCESS
        if (isSunoSuccess) {
          const data = parseTaskData(log.data)
          if (
            data.some(
              (c) =>
                c &&
                typeof c === 'object' &&
                (c as Record<string, unknown>).audio_url
            )
          ) {
            return <AudioPreviewCell log={log} />
          }
        }

        // Video/image: extract URLs from data and render thumbnails
        if (status === TASK_STATUS.SUCCESS && log.data) {
          try {
            const parsed =
              typeof log.data === 'string' ? JSON.parse(log.data) : log.data
            const media = extractMediaUrls(parsed)
            if (media.length > 0) {
              return <MediaPreviewCell log={log} />
            }
          } catch {
            // fall through to fail_reason
          }
        }

        if (!failReason) {
          return <span className='text-muted-foreground/60 text-xs'>-</span>
        }

        return (
          <>
            <button
              type='button'
              className='group flex max-w-[200px] items-center gap-1 text-left text-xs'
              onClick={() => setDialogOpen(true)}
              title={t('Click to view full error message')}
            >
              <span className='truncate leading-snug text-red-600 group-hover:underline dark:text-red-400'>
                {failReason}
              </span>
            </button>
            <FailReasonDialog
              failReason={failReason}
              open={dialogOpen}
              onOpenChange={setDialogOpen}
            />
          </>
        )
      },
      size: 200,
      maxSize: 300,
    }
  )

  return columns
}
