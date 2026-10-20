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
import { Copy, ExternalLink } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'

import { Dialog } from '@/components/dialog'
import { Button } from '@/components/ui/button'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

interface MediaPreviewDialogProps {
  url: string
  isVideo: boolean
  open: boolean
  onOpenChange: (open: boolean) => void
}

export function MediaPreviewDialog({
  url,
  isVideo,
  open,
  onOpenChange,
}: MediaPreviewDialogProps) {
  const { t } = useTranslation()
  const { copyToClipboard } = useCopyToClipboard()
  // 按 URL 记录失败状态，切换到另一个媒体时自动重置
  const [failedUrl, setFailedUrl] = useState<string | null>(null)
  const hasError = failedUrl === url

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title={isVideo ? t('Video Preview') : t('Image Preview')}
      contentClassName='sm:max-w-4xl'
      contentHeight='auto'
      bodyClassName='space-y-3'
    >
      <div className='bg-muted/50 flex min-h-[240px] items-center justify-center rounded-lg border'>
        {hasError && (
          <p className='text-muted-foreground px-4 text-center text-sm'>
            {isVideo ? t('Failed to load video') : t('Failed to load image')}
          </p>
        )}
        {!hasError &&
          (isVideo ? (
            // eslint-disable-next-line jsx-a11y/media-has-caption
            <video
              src={url}
              controls
              autoPlay
              className='max-h-[65vh] w-full rounded-lg'
              onError={() => setFailedUrl(url)}
            />
          ) : (
            <img
              src={url}
              alt={t('Generated image')}
              className='max-h-[65vh] w-full rounded-lg object-contain'
              onError={() => setFailedUrl(url)}
            />
          ))}
      </div>

      <div className='flex flex-wrap items-center gap-2'>
        <Button
          variant='outline'
          size='sm'
          onClick={() => window.open(url, '_blank', 'noopener,noreferrer')}
        >
          <ExternalLink className='size-3.5' />
          {t('Open in new tab')}
        </Button>
        <Button
          variant='outline'
          size='sm'
          onClick={() => copyToClipboard(url)}
        >
          <Copy className='size-3.5' />
          {t('Copy link')}
        </Button>
      </div>

      <p className='bg-muted text-muted-foreground rounded-md p-2 font-mono text-xs break-all'>
        {url}
      </p>
    </Dialog>
  )
}
