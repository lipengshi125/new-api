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
import { Badge } from '@/components/ui/badge'
import { Button } from '@/components/ui/button'

import { updateModelCodeSamples } from '../api'
import {
  CODE_SAMPLE_HIGHLIGHT,
  CODE_SAMPLE_SOURCE_LABELS,
  withCodeSampleEntry,
} from '../lib/code-samples'
import type {
  CodeSampleLang,
  CodeSampleMap,
  CodeSampleSource,
} from '../types'

/** Badge showing whether the visible sample is custom, global, or built-in. */
export function CodeSampleSourceBadge(props: { source: CodeSampleSource }) {
  const { t } = useTranslation()
  const isCustom = props.source === 'model'

  return (
    <Badge
      variant={isCustom ? 'default' : 'secondary'}
      className='h-6 rounded-full px-2 text-[10px] font-medium'
    >
      {t(CODE_SAMPLE_SOURCE_LABELS[props.source])}
    </Badge>
  )
}

type CodeSampleEditorProps = {
  modelName: string
  endpointType: string
  lang: CodeSampleLang
  /** Sample currently rendered, used as the editor's starting content. */
  initialCode: string
  /** Whether this model already has its own override for this entry. */
  hasOverride: boolean
  /** The model's full override map, so unrelated entries are preserved. */
  modelSamples?: CodeSampleMap
  onClose: () => void
}

export function CodeSampleEditor(props: CodeSampleEditorProps) {
  const { t } = useTranslation()
  const queryClient = useQueryClient()
  const [draft, setDraft] = useState(props.initialCode)

  const mutation = useMutation({
    mutationFn: (codeSamples: CodeSampleMap) =>
      updateModelCodeSamples({
        modelName: props.modelName,
        codeSamples,
      }),
    onSuccess: (data) => {
      if (!data.success) {
        toast.error(data.message || t('Failed to save code sample'))
        return
      }
      queryClient.invalidateQueries({ queryKey: ['pricing'] })
      toast.success(t('Code sample saved'))
      props.onClose()
    },
  })

  const save = (code: string) => {
    mutation.mutate(
      withCodeSampleEntry(
        props.modelSamples,
        props.endpointType,
        props.lang,
        code
      )
    )
  }

  return (
    <div className='space-y-2'>
      <CodeBlockEditor
        ariaLabel={t('Edit code sample')}
        language={CODE_SAMPLE_HIGHLIGHT[props.lang]}
        onChange={setDraft}
        rows={14}
        title={`${props.endpointType} · ${props.lang}`}
        value={draft}
      />

      <div className='flex flex-wrap items-center gap-2'>
        <Button
          size='sm'
          onClick={() => save(draft)}
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

        {props.hasOverride && (
          <Button
            size='sm'
            variant='ghost'
            className='text-muted-foreground'
            onClick={() => save('')}
            disabled={mutation.isPending}
          >
            <RotateCcw className='size-3.5' />
            {t('Restore default')}
          </Button>
        )}
      </div>

      <p className='text-muted-foreground text-xs'>
        {t(
          'Saved samples are shown verbatim to everyone who can view this page. Do not include real API keys.'
        )}
      </p>
    </div>
  )
}
