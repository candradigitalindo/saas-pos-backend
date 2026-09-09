import { beforeEach, describe, expect, it, vi } from 'vitest'
import { api, GalatAPI, GALAT_SESI_HABIS } from './api-client'
import { hapusSesi, simpanSesi } from './penyimpanan-sesi'

function balasan(status: number, badan: unknown): Response {
  return new Response(JSON.stringify(badan), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}

/** Menangkap GalatAPI dari sebuah panggilan yang memang diharapkan gagal. */
async function tangkap(janji: Promise<unknown>): Promise<GalatAPI> {
  try {
    await janji
  } catch (e) {
    if (e instanceof GalatAPI) return e
    throw e
  }
  throw new Error('Panggilan seharusnya gagal, tapi berhasil')
}

type PanggilanFetch = [string, { headers: Record<string, string> }]

function panggilan(palsu: { mock: { calls: unknown[] } }, ke: number): PanggilanFetch {
  return palsu.mock.calls[ke] as PanggilanFetch
}

describe('api-client', () => {
  beforeEach(() => {
    localStorage.clear()
    hapusSesi('tenant')
    vi.restoreAllMocks()
  })

  it('membuka amplop sekali — pemanggil hanya melihat data', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        balasan(200, { success: true, message: 'ok', data: { id: 'X1', name: 'Kopi' } }),
      ),
    )
    await expect(api.get('/products/X1')).resolves.toEqual({ id: 'X1', name: 'Kopi' })
  })

  it('menempelkan Idempotency-Key hanya bila diminta', async () => {
    // Badan Response hanya bisa dibaca sekali — buat yang baru tiap panggilan.
    const palsu = vi.fn(async () => balasan(200, { success: true, message: '', data: {} }))
    vi.stubGlobal('fetch', palsu)

    await api.post('/sales', { total: 1 }, { idempotencyKey: 'KUNCI-1' })
    expect(panggilan(palsu, 0)[1].headers['Idempotency-Key']).toBe('KUNCI-1')

    await api.get('/products')
    expect(panggilan(palsu, 1)[1].headers['Idempotency-Key']).toBeUndefined()
  })

  it('422 membawa pesan per kolom untuk ditempel di bawah kolomnya', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        balasan(422, {
          success: false,
          message: 'Validasi gagal',
          errors: { sell_price: 'sell_price wajib diisi' },
        }),
      ),
    )

    const e = await tangkap(api.post('/products', {}))
    expect(e.status).toBe(422)
    expect(e.kolom.sell_price).toBe('sell_price wajib diisi')
  })

  it('403 memakai kalimat manusia, bukan kode', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        balasan(403, { success: false, message: 'Anda tidak memiliki izin untuk tindakan ini' }),
      ),
    )
    const e = await tangkap(api.get('/reports/profit'))
    expect(e.pesan).toBe('Fitur ini tidak tersedia untuk akun Anda.')
    expect(e.pesan).not.toMatch(/403/)
  })

  it('429 memberi tahu untuk menunggu, bukan menyalahkan', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(balasan(429, { success: false, message: 'too many' })),
    )
    const e = await tangkap(api.post('/auth/login', {}))
    expect(e.pesan).toBe('Terlalu banyak percobaan. Coba lagi sebentar lagi.')
  })

  it('jaringan mati menjadi kesalahan yang bisa diantre, bukan pesan menakutkan', async () => {
    vi.stubGlobal('fetch', vi.fn().mockRejectedValue(new TypeError('Failed to fetch')))
    const e = await tangkap(api.post('/sales', {}))
    expect(e.bisaDiantre).toBe(true)
    expect(e.pesan).toContain('dikirim otomatis nanti')
  })

  it('5xx juga bisa diantre dan menegaskan data tidak hilang', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(balasan(500, { success: false, message: 'boom' })),
    )
    const e = await tangkap(api.post('/sales', {}))
    expect(e.bisaDiantre).toBe(true)
    expect(e.pesan).toContain('tidak hilang')
  })

  it('409 mempertahankan pesan spesifik dari server', async () => {
    vi.stubGlobal(
      'fetch',
      vi.fn().mockResolvedValue(
        balasan(409, { success: false, message: 'Transaksi sudah dibatalkan' }),
      ),
    )
    const e = await tangkap(api.get('/sales/1'))
    expect(e.pesan).toBe('Transaksi sudah dibatalkan')
  })

  it('401 memperbarui token lalu MENGULANG permintaan aslinya', async () => {
    simpanSesi('tenant', {
      access_token: 'lama',
      refresh_token: 'segar',
      kedaluwarsa: Date.now() + 1000,
    })

    const palsu = vi
      .fn()
      .mockResolvedValueOnce(balasan(401, { success: false, message: 'kedaluwarsa' }))
      .mockResolvedValueOnce(
        balasan(200, {
          success: true,
          message: '',
          data: { access_token: 'baru', refresh_token: 'segar2', expires_in: 900 },
        }),
      )
      .mockResolvedValueOnce(balasan(200, { success: true, message: '', data: { ok: true } }))
    vi.stubGlobal('fetch', palsu)

    await expect(api.get('/me')).resolves.toEqual({ ok: true })
    expect(palsu).toHaveBeenCalledTimes(3)
    expect(panggilan(palsu, 1)[0]).toContain('/auth/refresh')
    // Permintaan ulang memakai token yang baru, bukan yang mati.
    expect(panggilan(palsu, 2)[1].headers.Authorization).toBe('Bearer baru')
  })

  it('lima permintaan yang kena 401 bersamaan hanya memicu SATU pembaruan token', async () => {
    simpanSesi('tenant', {
      access_token: 'lama',
      refresh_token: 'segar',
      kedaluwarsa: Date.now() + 1000,
    })

    let refresh = 0
    const palsu = vi.fn(async (url: string) => {
      if (url.includes('/auth/refresh')) {
        refresh++
        return balasan(200, {
          success: true,
          message: '',
          data: { access_token: 'baru', refresh_token: 'segar2', expires_in: 900 },
        })
      }
      return balasan(401, { success: false, message: 'kedaluwarsa' })
    })
    vi.stubGlobal('fetch', palsu)

    // Semua akan gagal (401 lagi setelah refresh), yang diuji adalah jumlah refresh.
    await Promise.allSettled([
      api.get('/a'),
      api.get('/b'),
      api.get('/c'),
      api.get('/d'),
      api.get('/e'),
    ])
    expect(refresh).toBe(1)
  })

  it('sesi dibuang bila pembaruan token gagal', async () => {
    simpanSesi('tenant', {
      access_token: 'lama',
      refresh_token: 'basi',
      kedaluwarsa: Date.now() - 1,
    })
    vi.stubGlobal(
      'fetch',
      vi.fn(async (url: string) =>
        url.includes('/auth/refresh')
          ? balasan(401, { success: false, message: 'refresh mati' })
          : balasan(401, { success: false, message: 'kedaluwarsa' }),
      ),
    )

    const e = await tangkap(api.get('/me'))
    expect(e.pesan).toBe(GALAT_SESI_HABIS)
    expect(localStorage.getItem('pos.sesi.toko')).toBeNull()
  })

  it('query kosong tidak ikut dikirim ke URL', async () => {
    const palsu = vi.fn().mockResolvedValue(balasan(200, { success: true, message: '', data: [] }))
    vi.stubGlobal('fetch', palsu)
    await api.get('/products', { query: { search: '', page: 2, outlet_id: undefined } })
    expect(panggilan(palsu, 0)[0]).toBe('/api/v1/products?page=2')
  })
})
