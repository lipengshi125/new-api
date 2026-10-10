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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { Loader2, RotateCcw, Save, X } from 'lucide-react'
import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { CodeBlockEditor } from '@/components/ai-elements/code-block'
import { Button } from '@/components/ui/button'

import { updateModelApiDoc } from '../api'
import { ModelApiDoc } from './model-api-doc'

/** Admin editor: paste Markdown on the left, see the user view on the right. */
export function ModelApiDocEditor(props: {
  modelName: string
  initialContent: string
  hasDoc: boolean
  onClose: () => void
}) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState(props.initialContent)

  const mutation = useMutation({
    mutationFn: (content: string) =>
      updateModelApiDoc({ modelName: props.modelName, content }),
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to save API documentation'))
        return
      }
      queryClient.invalidateQueries({ queryKey: ['pricing'] })
      queryClient.invalidateQueries({
        queryKey: ['model-api-doc', props.modelName],
      })
      toast.success(t('API documentation saved'))
      props.onClose()
    },
  })

  return (
    <div className='space-y-3'>
      <div className='grid gap-3 lg:grid-cols-2'>
        <CodeBlockEditor
          ariaLabel={t('Edit API documentation')}
          className='my-0'
          language='markdown'
          onChange={setDraft}
          rows={24}
          title='Markdown'
          value={draft}
        />
        <div className='border-border/60 max-h-[640px] overflow-auto rounded-lg border p-4'>
          <ModelApiDoc content={draft} />
        </div>
      </div>

      <div className='flex flex-wrap items-center gap-2'>
        <Button
          size='sm'
          onClick={() => mutation.mutate(draft)}
          disabled={mutation.isPending}
        >
          {mutation.isPending ? (
            <Loader2 className='size-3.5 animate-spin' />
          ) : (
            <Save className='size-3.5' />
          )}
          {t('Save')}
        </Button>
        <Button
          size='sm'
          variant='outline'
          onClick={props.onClose}
          disabled={mutation.isPending}
        >
          <X className='size-3.5' />
          {t('Cancel')}
        </Button>
        {props.hasDoc && (
          <Button
            size='sm'
            variant='ghost'
            className='text-muted-foreground'
            onClick={() => mutation.mutate('')}
            disabled={mutation.isPending}
          >
            <RotateCcw className='size-3.5' />
            {t('Restore default')}
          </Button>
        )}
      </div>
    </div>
  )
}
