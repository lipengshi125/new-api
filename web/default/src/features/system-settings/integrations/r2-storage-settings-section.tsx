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
import { zodResolver } from '@hookform/resolvers/zod'
import { useState } from 'react'
import { useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import * as z from 'zod'

import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormItem,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { api } from '@/lib/api'

import {
  SettingsForm,
  SettingsSwitchContent,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useResetForm } from '../hooks/use-reset-form'
import { useUpdateOption } from '../hooks/use-update-option'
import { removeTrailingSlash } from './utils'

const createR2Schema = (t: (key: string) => string) =>
  z.object({
    R2StorageEnabled: z.boolean(),
    R2Endpoint: z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^https?:\/\//.test(trimmed)
    }, t('Provide a valid URL starting with https://')),
    R2Bucket: z.string(),
    R2KeyID: z.string(),
    R2Secret: z.string(),
    R2PublicURL: z.string().refine((value) => {
      const trimmed = value.trim()
      if (!trimmed) return true
      return /^https?:\/\//.test(trimmed)
    }, t('Provide a valid URL starting with https://')),
    R2StoragePath: z.string(),
  })

type R2FormValues = z.infer<ReturnType<typeof createR2Schema>>

type R2StorageSettingsSectionProps = {
  defaultValues: R2FormValues
}

export function R2StorageSettingsSection({
  defaultValues,
}: R2StorageSettingsSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  const r2Schema = createR2Schema(t)
  const [isTesting, setIsTesting] = useState(false)

  const form = useForm<R2FormValues>({
    resolver: zodResolver(r2Schema),
    defaultValues,
  })

  useResetForm(form, defaultValues)

  const testConnection = async () => {
    const values = form.getValues()
    setIsTesting(true)

    try {
      const response = await api.post<{ success: boolean; message: string }>(
        '/api/r2/test',
        {
          endpoint: values.R2Endpoint,
          bucket: values.R2Bucket,
          key_id: values.R2KeyID,
          secret: values.R2Secret,
          public_url: values.R2PublicURL,
        }
      )

      if (response.data.success) {
        toast.success(t('R2 connection test successful'))
      } else {
        toast.error(response.data.message || t('Connection test failed'))
      }
    } catch (error) {
      toast.error(
        error instanceof Error
          ? error.message
          : t('Failed to test R2 connection')
      )
    } finally {
      setIsTesting(false)
    }
  }

  const onSubmit = async (values: R2FormValues) => {
    const sanitizedEndpoint = removeTrailingSlash(values.R2Endpoint)
    const sanitizedPublicURL = removeTrailingSlash(values.R2PublicURL)
    const sanitizedBucket = values.R2Bucket.trim()
    const sanitizedKeyID = values.R2KeyID.trim()
    const sanitizedSecret = values.R2Secret.trim()
    const sanitizedStoragePath = values.R2StoragePath.trim()

    const initialEndpoint = removeTrailingSlash(defaultValues.R2Endpoint)
    const initialPublicURL = removeTrailingSlash(defaultValues.R2PublicURL)
    const initialBucket = defaultValues.R2Bucket.trim()
    const initialKeyID = defaultValues.R2KeyID.trim()
    const initialSecret = defaultValues.R2Secret.trim()
    const initialStoragePath = defaultValues.R2StoragePath.trim()

    const updates: Array<{ key: string; value: string | boolean }> = []

    if (values.R2StorageEnabled !== defaultValues.R2StorageEnabled) {
      updates.push({
        key: 'R2StorageEnabled',
        value: values.R2StorageEnabled,
      })
    }

    if (sanitizedEndpoint !== initialEndpoint) {
      updates.push({ key: 'R2Endpoint', value: sanitizedEndpoint })
    }

    if (sanitizedBucket !== initialBucket) {
      updates.push({ key: 'R2Bucket', value: sanitizedBucket })
    }

    if (sanitizedKeyID !== initialKeyID) {
      updates.push({ key: 'R2KeyID', value: sanitizedKeyID })
    }

    if (sanitizedSecret !== initialSecret) {
      updates.push({ key: 'R2Secret', value: sanitizedSecret })
    }

    if (sanitizedPublicURL !== initialPublicURL) {
      updates.push({ key: 'R2PublicURL', value: sanitizedPublicURL })
    }

    if (sanitizedStoragePath !== initialStoragePath) {
      updates.push({ key: 'R2StoragePath', value: sanitizedStoragePath })
    }

    for (const update of updates) {
      await updateOption.mutateAsync(update)
    }
  }

  return (
    <SettingsSection title={t('Cloudflare R2 Storage')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)} autoComplete='off'>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            isSaving={updateOption.isPending}
            saveLabel='Save R2 settings'
          />

          <FormField
            control={form.control}
            name='R2StorageEnabled'
            render={({ field }) => (
              <SettingsSwitchItem>
                <SettingsSwitchContent>
                  <FormLabel>{t('Enable R2 Storage')}</FormLabel>
                  <FormDescription>
                    {t(
                      'Enable Cloudflare R2 object storage for file uploads and media hosting'
                    )}
                  </FormDescription>
                </SettingsSwitchContent>
                <FormControl>
                  <Switch
                    checked={field.value}
                    onCheckedChange={field.onChange}
                  />
                </FormControl>
              </SettingsSwitchItem>
            )}
          />

          <FormField
            control={form.control}
            name='R2Endpoint'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('R2 Endpoint')}</FormLabel>
                <FormControl>
                  <Input
                    type='url'
                    inputMode='url'
                    placeholder='https://<account-id>.r2.cloudflarestorage.com'
                    autoComplete='off'
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Your Cloudflare R2 account endpoint URL. Find your account ID in the R2 overview page.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='R2Bucket'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Bucket Name')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='api'
                    autoComplete='off'
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t('The name of your R2 bucket for storing uploaded files')}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='R2KeyID'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Access Key ID')}</FormLabel>
                <FormControl>
                  <Input
                    type='password'
                    placeholder={t('Enter new key to update')}
                    autoComplete='new-password'
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'R2 API token Access Key ID. Create one in R2 → Manage API Tokens.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='R2Secret'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Secret Access Key')}</FormLabel>
                <FormControl>
                  <Input
                    type='password'
                    placeholder={t('Enter new secret to update')}
                    autoComplete='new-password'
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'R2 API token Secret Access Key. Only shown once when creating the token.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='R2PublicURL'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Public Domain')}</FormLabel>
                <FormControl>
                  <Input
                    type='url'
                    inputMode='url'
                    placeholder='https://cdn.example.com'
                    autoComplete='off'
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Custom domain bound to your R2 bucket. This will be used in returned file URLs.'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <FormField
            control={form.control}
            name='R2StoragePath'
            render={({ field }) => (
              <FormItem>
                <FormLabel>{t('Storage Path Prefix')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder='uploads/'
                    autoComplete='off'
                    {...field}
                    onChange={(event) => field.onChange(event.target.value)}
                  />
                </FormControl>
                <FormDescription>
                  {t(
                    'Optional path prefix for organizing files in the bucket (e.g., "1day/" or "uploads/")'
                  )}
                </FormDescription>
                <FormMessage />
              </FormItem>
            )}
          />

          <div className='flex gap-2'>
            <Button
              type='button'
              variant='outline'
              onClick={testConnection}
              disabled={isTesting || updateOption.isPending}
            >
              {isTesting ? t('Testing...') : t('Test Connection')}
            </Button>
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
