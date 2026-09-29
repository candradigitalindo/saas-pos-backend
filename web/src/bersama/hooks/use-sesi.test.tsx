import 'fake-indexeddb/auto'
import type { ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { renderHook, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { simpanSesi } from '@/lib/penyimpanan-sesi'
import { ingat, ingatan } from '@/lib/offline/ingatan'
import { PenyediaSesi, useSesi } from './use-sesi'

const PROFIL = {
  user: { id: 'U1', name: 'Budi', username: 'budi', email: 'b@x', role_name: 'Kasir', is_active: true, created_at: '', updated_at: '' },
  tenant: { id: 'T1', business_name: 'Warung', status: 'active', created_at: '' },
  permissions: ['sale.create'],
  outlet_ids: ['O1'],
  outlets: [],
}

function pembungkus() {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return ({ children }: { children: ReactNode }) => (
    <QueryClientProvider client={qc}>
      <PenyediaSesi>{children}</PenyediaSesi>
    </QueryClientProvider>
  )
}

describe('sesi toko saat offline', () => {
  beforeEach(() => {
    localStorage.clear()
    vi.restoreAllMocks()
  })

  it('aplikasi yang dibuka ulang tanpa sinyal tetap masuk dengan profil terakhir', async () => {
    simpanSesi('tenant', { access_token: 'a', refresh_token: 'r', kedaluwarsa: Date.now() + 60_000 })
    ingat('profil', PROFIL)
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))

    const { result } = renderHook(() => useSesi(), { wrapper: pembungkus() })

    // Dulu: /me gagal → profil kosong → sudahMasuk false → dilempar ke /masuk.
    expect(result.current.sudahMasuk).toBe(true)
    expect(result.current.tokoAktif).toBe('O1')
    await waitFor(() => expect(vi.mocked(fetch)).toHaveBeenCalled())
    expect(result.current.sudahMasuk).toBe(true)
  })

  it('profil segar dari server menggantikan dan disimpan untuk lain kali', async () => {
    simpanSesi('tenant', { access_token: 'a', refresh_token: 'r', kedaluwarsa: Date.now() + 60_000 })
    vi.stubGlobal(
      'fetch',
      vi.fn(async () =>
        new Response(JSON.stringify({ success: true, message: '', data: PROFIL }), { status: 200 }),
      ),
    )

    const { result } = renderHook(() => useSesi(), { wrapper: pembungkus() })
    await waitFor(() => expect(result.current.sudahMasuk).toBe(true))
    expect(ingatan<typeof PROFIL>('profil')?.user.id).toBe('U1')
  })

  it('tanpa sesi, profil yang tersimpan tidak dipakai', () => {
    ingat('profil', PROFIL)
    const { result } = renderHook(() => useSesi(), { wrapper: pembungkus() })
    expect(result.current.sudahMasuk).toBe(false)
  })
})
