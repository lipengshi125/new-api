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
import { Plus, Trash2 } from 'lucide-react'
import { useEffect, useMemo } from 'react'
import { useFieldArray, useForm } from 'react-hook-form'
import { useTranslation } from 'react-i18next'
import * as z from 'zod'

import { Button } from '@/components/ui/button'
import {
  Form,
  FormControl,
  FormDescription,
  FormField,
  FormLabel,
  FormMessage,
} from '@/components/ui/form'
import { Input } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { sanitizeNavUrl } from '@/lib/nav-modules'

import {
  SettingsControlChildren,
  SettingsForm,
  SettingsSwitchContent,
  SettingsControlGroup,
  SettingsSwitchItem,
} from '../components/settings-form-layout'
import { SettingsPageFormActions } from '../components/settings-page-context'
import { SettingsSection } from '../components/settings-section'
import { useUpdateOption } from '../hooks/use-update-option'
import {
  HEADER_NAV_DEFAULT,
  type HeaderNavModulesConfig,
  serializeHeaderNavModules,
} from './config'

const headerNavSchema = z.object({
  home: z.boolean(),
  console: z.boolean(),
  pricingEnabled: z.boolean(),
  pricingRequireAuth: z.boolean(),
  rankingsEnabled: z.boolean(),
  rankingsRequireAuth: z.boolean(),
  docs: z.boolean(),
  about: z.boolean(),
  custom: z.array(
    z.object({
      name: z.string().trim().min(1, 'Please enter a name'),
      url: z
        .string()
        .refine(
          (value) => sanitizeNavUrl(value) !== '',
          'Enter a full https:// address or an in-app path such as /about'
        ),
      enabled: z.boolean(),
      requireAuth: z.boolean(),
    })
  ),
})

type HeaderNavFormValues = z.infer<typeof headerNavSchema>

type HeaderNavSwitchField = Exclude<keyof HeaderNavFormValues, 'custom'>

type HeaderNavigationSectionProps = {
  config: HeaderNavModulesConfig
  initialSerialized: string
}

const toFormValues = (config: HeaderNavModulesConfig): HeaderNavFormValues => ({
  home:
    config.home === undefined ? HEADER_NAV_DEFAULT.home : Boolean(config.home),
  console:
    config.console === undefined
      ? HEADER_NAV_DEFAULT.console
      : Boolean(config.console),
  pricingEnabled:
    config.pricing?.enabled === undefined
      ? HEADER_NAV_DEFAULT.pricing.enabled
      : Boolean(config.pricing.enabled),
  pricingRequireAuth:
    config.pricing?.requireAuth === undefined
      ? HEADER_NAV_DEFAULT.pricing.requireAuth
      : Boolean(config.pricing.requireAuth),
  rankingsEnabled:
    config.rankings?.enabled === undefined
      ? HEADER_NAV_DEFAULT.rankings.enabled
      : Boolean(config.rankings.enabled),
  rankingsRequireAuth:
    config.rankings?.requireAuth === undefined
      ? HEADER_NAV_DEFAULT.rankings.requireAuth
      : Boolean(config.rankings.requireAuth),
  docs:
    config.docs === undefined ? HEADER_NAV_DEFAULT.docs : Boolean(config.docs),
  about:
    config.about === undefined
      ? HEADER_NAV_DEFAULT.about
      : Boolean(config.about),
  custom: (config.custom ?? []).map((item) => ({
    name: item.name,
    url: item.url,
    enabled: item.enabled,
    requireAuth: item.requireAuth,
  })),
})

export function HeaderNavigationSection({
  config,
  initialSerialized,
}: HeaderNavigationSectionProps) {
  const { t } = useTranslation()
  const updateOption = useUpdateOption()
  // `config` is a fresh object on every parent render, so key the defaults off
  // the serialized value instead: otherwise an unrelated re-render would reset
  // the form and wipe half-typed custom entries.
  const formDefaults = useMemo(
    () => toFormValues(config),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [initialSerialized]
  )

  const form = useForm<HeaderNavFormValues>({
    resolver: zodResolver(headerNavSchema),
    defaultValues: formDefaults,
  })
  const customItems = useFieldArray({ control: form.control, name: 'custom' })

  useEffect(() => {
    form.reset(formDefaults)
  }, [formDefaults, form])

  const onSubmit = async (values: HeaderNavFormValues) => {
    const payload: HeaderNavModulesConfig = {
      ...config,
      home: values.home,
      console: values.console,
      docs: values.docs,
      about: values.about,
      pricing: {
        ...(config.pricing ?? HEADER_NAV_DEFAULT.pricing),
        enabled: values.pricingEnabled,
        requireAuth: values.pricingRequireAuth,
      },
      rankings: {
        ...(config.rankings ?? HEADER_NAV_DEFAULT.rankings),
        enabled: values.rankingsEnabled,
        requireAuth: values.rankingsRequireAuth,
      },
      // Store the normalized target so the header and the backend see the same
      // value the admin will get back on the next edit.
      custom: values.custom.map((item) => ({
        name: item.name.trim(),
        url: sanitizeNavUrl(item.url),
        enabled: item.enabled,
        requireAuth: item.requireAuth,
      })),
    }

    const serialized = serializeHeaderNavModules(payload)
    if (serialized === initialSerialized) {
      return
    }

    await updateOption.mutateAsync({
      key: 'HeaderNavModules',
      value: serialized,
    })
  }

  const resetToDefault = () => {
    form.reset(toFormValues(HEADER_NAV_DEFAULT))
  }

  const simpleModules: Array<{
    key: HeaderNavSwitchField
    title: string
    description: string
  }> = [
    {
      key: 'home',
      title: t('Home'),
      description: t('Landing page with system overview.'),
    },
    {
      key: 'console',
      title: t('Console'),
      description: t('User dashboard and quota controls.'),
    },
    {
      key: 'docs',
      title: t('Docs'),
      description: t('Documentation or external knowledge base.'),
    },
    {
      key: 'about',
      title: t('About'),
      description: t('Static page describing the platform.'),
    },
  ]

  const accessModules: Array<{
    enabledKey: HeaderNavSwitchField
    requireAuthKey: HeaderNavSwitchField
    requireAuthDependsOn: 'pricingEnabled' | 'rankingsEnabled'
    title: string
    description: string
    requireAuthTitle: string
    requireAuthDescription: string
  }> = [
    {
      enabledKey: 'pricingEnabled',
      requireAuthKey: 'pricingRequireAuth',
      requireAuthDependsOn: 'pricingEnabled',
      title: t('Model Square'),
      description: t('Public model catalog and pricing page.'),
      requireAuthTitle: t('Require login to view models'),
      requireAuthDescription: t(
        'Visitors must authenticate before accessing the pricing directory.'
      ),
    },
    {
      enabledKey: 'rankingsEnabled',
      requireAuthKey: 'rankingsRequireAuth',
      requireAuthDependsOn: 'rankingsEnabled',
      title: t('Rankings'),
      description: t('Public rankings page based on live usage data.'),
      requireAuthTitle: t('Require login to view rankings'),
      requireAuthDescription: t(
        'Visitors must authenticate before accessing the rankings page.'
      ),
    },
  ]

  return (
    <SettingsSection title={t('Header navigation')}>
      <Form {...form}>
        <SettingsForm onSubmit={form.handleSubmit(onSubmit)}>
          <SettingsPageFormActions
            onSave={form.handleSubmit(onSubmit)}
            onReset={resetToDefault}
            isSaving={updateOption.isPending}
            resetLabel='Reset to default'
            saveLabel='Save navigation'
          />
          <div className='grid gap-4 md:grid-cols-2'>
            {simpleModules.map((module) => (
              <FormField
                key={module.key}
                control={form.control}
                name={module.key}
                render={({ field }) => (
                  <SettingsSwitchItem>
                    <SettingsSwitchContent>
                      <FormLabel>{module.title}</FormLabel>
                      <FormDescription>{module.description}</FormDescription>
                    </SettingsSwitchContent>
                    <FormControl>
                      <Switch
                        checked={field.value}
                        onCheckedChange={field.onChange}
                      />
                    </FormControl>
                    <FormMessage />
                  </SettingsSwitchItem>
                )}
              />
            ))}
          </div>

          <div className='grid gap-4 lg:grid-cols-2'>
            {accessModules.map((module) => (
              <SettingsControlGroup key={module.enabledKey}>
                <FormField
                  control={form.control}
                  name={module.enabledKey}
                  render={({ field }) => (
                    <SettingsSwitchItem>
                      <SettingsSwitchContent>
                        <FormLabel>{module.title}</FormLabel>
                        <FormDescription>{module.description}</FormDescription>
                      </SettingsSwitchContent>
                      <FormControl>
                        <Switch
                          checked={field.value}
                          onCheckedChange={field.onChange}
                        />
                      </FormControl>
                      <FormMessage />
                    </SettingsSwitchItem>
                  )}
                />

                <FormField
                  control={form.control}
                  name={module.requireAuthKey}
                  render={({ field }) => (
                    <SettingsControlChildren>
                      <SettingsSwitchItem className='py-2'>
                        <SettingsSwitchContent>
                          <FormLabel>{module.requireAuthTitle}</FormLabel>
                          <FormDescription>
                            {module.requireAuthDescription}
                          </FormDescription>
                        </SettingsSwitchContent>
                        <FormControl>
                          <Switch
                            checked={field.value}
                            onCheckedChange={field.onChange}
                            disabled={!form.watch(module.requireAuthDependsOn)}
                          />
                        </FormControl>
                        <FormMessage />
                      </SettingsSwitchItem>
                    </SettingsControlChildren>
                  )}
                />
              </SettingsControlGroup>
            ))}
          </div>

          <div className='space-y-3'>
            <div className='flex flex-wrap items-start justify-between gap-3'>
              <div className='space-y-0.5'>
                <FormLabel className='text-sm font-medium'>
                  {t('Custom pages')}
                </FormLabel>
                <FormDescription>
                  {t(
                    'Extra header entries. Clicking the name opens the address you enter: a full https:// URL opens in a new tab, a path such as /about stays in the app.'
                  )}
                </FormDescription>
              </div>
              <Button
                type='button'
                variant='outline'
                size='sm'
                onClick={() =>
                  customItems.append({
                    name: '',
                    url: '',
                    enabled: true,
                    requireAuth: false,
                  })
                }
              >
                <Plus className='mr-2 h-4 w-4' />
                {t('Add page')}
              </Button>
            </div>

            {customItems.fields.length === 0 ? (
              <p className='text-muted-foreground rounded-lg border border-dashed p-6 text-center text-sm'>
                {t('No custom pages yet. Click "Add page" to create one.')}
              </p>
            ) : (
              <div className='space-y-3'>
                {customItems.fields.map((field, index) => (
                  <SettingsControlGroup key={field.id} className='space-y-2.5'>
                    <div className='grid gap-3 sm:grid-cols-2'>
                      <FormField
                        control={form.control}
                        name={`custom.${index}.name`}
                        render={({ field: nameField }) => (
                          <div className='space-y-1.5'>
                            <FormLabel>{t('Name')}</FormLabel>
                            <FormControl>
                              <Input
                                placeholder={t('Help Center')}
                                {...nameField}
                              />
                            </FormControl>
                            <FormMessage />
                          </div>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name={`custom.${index}.url`}
                        render={({ field: urlField }) => (
                          <div className='space-y-1.5'>
                            <FormLabel>{t('Link address')}</FormLabel>
                            <FormControl>
                              <Input
                                placeholder='https://help.example.com'
                                {...urlField}
                              />
                            </FormControl>
                            <FormMessage />
                          </div>
                        )}
                      />
                    </div>

                    <div className='flex flex-wrap items-center justify-between gap-x-6 gap-y-2'>
                      <FormField
                        control={form.control}
                        name={`custom.${index}.enabled`}
                        render={({ field: enabledField }) => (
                          <div className='flex items-center gap-2'>
                            <FormControl>
                              <Switch
                                checked={enabledField.value}
                                onCheckedChange={enabledField.onChange}
                              />
                            </FormControl>
                            <FormLabel className='text-xs font-medium'>
                              {t('Show in navigation')}
                            </FormLabel>
                          </div>
                        )}
                      />
                      <FormField
                        control={form.control}
                        name={`custom.${index}.requireAuth`}
                        render={({ field: requireAuthField }) => (
                          <div className='flex items-center gap-2'>
                            <FormControl>
                              <Switch
                                checked={requireAuthField.value}
                                onCheckedChange={requireAuthField.onChange}
                                disabled={
                                  !form.watch(`custom.${index}.enabled`)
                                }
                              />
                            </FormControl>
                            <FormLabel className='text-xs font-medium'>
                              {t('Require login to open')}
                            </FormLabel>
                          </div>
                        )}
                      />
                      <Button
                        type='button'
                        variant='ghost'
                        size='sm'
                        className='ms-auto'
                        onClick={() => customItems.remove(index)}
                        aria-label={t('Delete')}
                      >
                        <Trash2 className='h-4 w-4' />
                      </Button>
                    </div>
                  </SettingsControlGroup>
                ))}
              </div>
            )}
          </div>
        </SettingsForm>
      </Form>
    </SettingsSection>
  )
}
