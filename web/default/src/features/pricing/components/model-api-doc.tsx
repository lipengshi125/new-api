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
import { Check, Copy } from 'lucide-react'
import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'

import {
  CodeBlock,
  CodeBlockCopyButton,
} from '@/components/ai-elements/code-block'
import { Button } from '@/components/ui/button'
import { Markdown } from '@/components/ui/markdown'
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard'

import { splitMarkdownCodeFences } from '../lib/markdown-segments'

/**
 * Read-only view of an admin-authored Markdown API document. Prose goes
 * through the shared sanitized Markdown renderer; fenced code becomes a code
 * block with its own copy button. The header button copies the raw Markdown.
 */
export function ModelApiDoc(props: { content: string }) {
  const { t } = useTranslation()
  const { copiedText, copyToClipboard } = useCopyToClipboard()
  const segments = useMemo(
    () => splitMarkdownCodeFences(props.content),
    [props.content]
  )
  const isCopied = copiedText === props.content

  return (
    <div className='space-y-3'>
      <div className='flex justify-end'>
        <Button
          size='sm'
          variant='outline'
          className='h-7 gap-1 px-2 text-xs'
          onClick={() => copyToClipboard(props.content)}
        >
          {isCopied ? (
            <Check className='size-3.5' />
          ) : (
            <Copy className='size-3.5' />
          )}
          {isCopied ? t('Copied') : t('Copy Markdown')}
        </Button>
      </div>

      {segments.map((segment) => {
        if (segment.kind === 'code') {
          return (
            <CodeBlock
              key={segment.start}
              code={segment.code}
              language={segment.lang}
              className='my-0'
            >
              <CodeBlockCopyButton />
            </CodeBlock>
          )
        }
        return <Markdown key={segment.start}>{segment.text}</Markdown>
      })}
    </div>
  )
}
