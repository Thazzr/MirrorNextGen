import { api, unwrap, unwrapEmpty } from '../../../shared/api/client.ts'
import type { components } from '../../../shared/api/schema.gen.ts'

export type Icon = components['schemas']['Icon']
export type IconRef = components['schemas']['IconRef']

export const iconsQueryKey = ['icons'] as const

export const iconFileUrl = (id: string) => `/api/v1/icons/${encodeURIComponent(id)}/file`

export async function fetchIcons(): Promise<Icon[]> {
  return unwrap(await api.GET('/api/v1/icons'))
}

/** Uploads through the generated client; the serializer builds the multipart body. */
export async function uploadIcon(input: { file: File; name?: string }): Promise<Icon> {
  return unwrap(
    await api.POST('/api/v1/icons', {
      // The schema types the binary part as string; the serializer below sends the real File.
      body: { file: '' },
      bodySerializer: () => {
        const form = new FormData()
        form.append('file', input.file)
        if (input.name) form.append('name', input.name)
        return form
      },
    }),
  )
}

export async function deleteIcon(id: string): Promise<void> {
  unwrapEmpty(await api.DELETE('/api/v1/icons/{id}', { params: { path: { id } } }))
}
