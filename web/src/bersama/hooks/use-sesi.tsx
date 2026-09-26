/**
 * Sesi pengguna toko: profil, izin efektif, dan toko yang sedang aktif.
 *
 * Izin datang dari GET /me sebagai GABUNGAN seluruh peran yang dipegang
 * pengguna (backend mendukung peran ganda). UI tidak pernah menghitungnya
 * sendiri.
 */
import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { authApi } from '@/fitur/auth/api'
import type { ProfilSaya, Toko } from '@/fitur/auth/tipe'
import { GalatAPI } from '@/lib/api-client'
import { adaSesi, ambilSesi, hapusSesi, langganSesi, simpanSesi } from '@/lib/penyimpanan-sesi'
import { db, kosongkanDB } from '@/lib/offline/db'
import { ingat, ingatan, lupakanSemua } from '@/lib/offline/ingatan'
import type { KodeIzin } from '@/lib/izin'

interface NilaiSesi {
  profil: ProfilSaya | undefined
  memuat: boolean
  sudahMasuk: boolean
  /** true bila salah satu kode izin dimiliki (sama seperti Require di backend). */
  boleh: (...kode: KodeIzin[]) => boolean
  /** Toko yang sedang dipakai. Kasir hampir selalu hanya punya satu. */
  tokoAktif: string | undefined
  /** Rincian toko aktif — pajak & biaya layanan untuk total kasir. */
  rincianToko: Toko | undefined
  gantiToko: (id: string) => void
  masuk: (username: string, password: string) => Promise<void>
  /**
   * Keluar dari akun. Menolak (GalatAdaAntrean) bila masih ada transaksi yang
   * belum terkirim — keluar akan membuang data lokal, dan penjualan yang belum
   * sampai ke server akan hilang bersamanya.
   */
  keluar: (paksa?: boolean) => Promise<void>
  /** Dipakai setelah pendaftaran: sesi sudah diberikan server, tinggal disimpan. */
  pakaiSesiBaru: (auth: {
    access_token: string
    refresh_token: string
    expires_in: number
  }) => void
}

/**
 * Dilempar saat keluar sementara masih ada transaksi yang belum terkirim.
 * Layar yang memanggil keluar wajib menangkapnya dan bertanya lebih dulu.
 */
export class GalatAdaAntrean extends Error {
  readonly jumlah: number
  constructor(jumlah: number) {
    super(`${jumlah} transaksi belum terkirim`)
    this.name = 'GalatAdaAntrean'
    this.jumlah = jumlah
  }
}

const KonteksSesi = createContext<NilaiSesi | null>(null)
const KUNCI_TOKO = 'pos.toko-aktif'
const KUNCI_INGAT_PROFIL = 'profil'

export function PenyediaSesi({ children }: { children: ReactNode }) {
  const qc = useQueryClient()
  const [punyaSesi, setPunyaSesi] = useState(() => adaSesi('tenant'))
  const [tokoDipilih, setTokoDipilih] = useState<string | null>(() =>
    localStorage.getItem(KUNCI_TOKO),
  )

  useEffect(() => langganSesi(() => setPunyaSesi(adaSesi('tenant'))), [])

  const { data: profil, isLoading } = useQuery({
    queryKey: ['me'],
    queryFn: async () => {
      const p = await authApi.saya()
      ingat(KUNCI_INGAT_PROFIL, p)
      return p
    },
    enabled: punyaSesi,
    // Profil terakhir dipakai sebagai data awal supaya aplikasi yang dibuka
    // ulang tanpa sinyal tetap masuk ke kasir (lihat lib/offline/ingatan.ts).
    // updatedAt 0 = dianggap basi, jadi tetap diambil ulang begitu bisa.
    initialData: () => (adaSesi('tenant') ? ingatan<ProfilSaya>(KUNCI_INGAT_PROFIL) : undefined),
    initialDataUpdatedAt: 0,
    staleTime: 5 * 60 * 1000,
    retry: (gagal, e) => !(e instanceof GalatAPI && e.status === 401) && gagal < 2,
  })

  const izin = useMemo(() => new Set(profil?.permissions ?? []), [profil])

  const boleh = useCallback(
    (...kode: KodeIzin[]) => kode.some((k) => izin.has(k)),
    [izin],
  )

  const tokoAktif = useMemo(() => {
    const daftar = profil?.outlet_ids ?? []
    if (tokoDipilih && daftar.includes(tokoDipilih)) return tokoDipilih
    return daftar[0]
  }, [profil, tokoDipilih])

  const rincianToko = useMemo(
    () => profil?.outlets?.find((t) => t.id === tokoAktif),
    [profil, tokoAktif],
  )

  const gantiToko = useCallback((id: string) => {
    localStorage.setItem(KUNCI_TOKO, id)
    setTokoDipilih(id)
  }, [])

  const pakaiSesiBaru = useCallback(
    (auth: { access_token: string; refresh_token: string; expires_in: number }) => {
      // Ingatan milik pengguna sebelumnya dibuang SEBELUM sesi baru dipakai.
      lupakanSemua()
      simpanSesi('tenant', {
        access_token: auth.access_token,
        refresh_token: auth.refresh_token,
        kedaluwarsa: Date.now() + auth.expires_in * 1000,
      })
      setPunyaSesi(true)
    },
    [],
  )

  const masuk = useCallback(
    async (username: string, password: string) => {
      const hasil = await authApi.masuk(username, password)
      pakaiSesiBaru(hasil)
      await qc.invalidateQueries({ queryKey: ['me'] })
    },
    [pakaiSesiBaru, qc],
  )

  const keluar = useCallback(async (paksa = false) => {
    if (!paksa) {
      const belumTerkirim = await db.antrean
        .where('status')
        .anyOf('menunggu', 'mengirim', 'perlu-diperiksa')
        .count()
      if (belumTerkirim > 0) throw new GalatAdaAntrean(belumTerkirim)
    }

    const sesi = ambilSesi('tenant')
    try {
      if (sesi?.refresh_token) await authApi.keluar(sesi.refresh_token)
    } catch {
      // Keluar tidak boleh gagal dari sisi pengguna. Kalau server tak terjangkau,
      // sesi lokal tetap dibuang.
    }
    hapusSesi('tenant')
    localStorage.removeItem(KUNCI_TOKO)
    lupakanSemua()
    // Data lokal WAJIB dibuang: satu tablet kasir dipakai bergantian, dan
    // katalog — apalagi antrean transaksi — milik usaha yang sedang masuk,
    // bukan milik perangkatnya.
    await kosongkanDB()
    setPunyaSesi(false)
    qc.clear()
  }, [qc])

  const nilai = useMemo<NilaiSesi>(
    () => ({
      profil,
      memuat: punyaSesi && isLoading,
      sudahMasuk: punyaSesi && !!profil,
      boleh,
      tokoAktif,
      rincianToko,
      gantiToko,
      masuk,
      keluar,
      pakaiSesiBaru,
    }),
    [
      profil,
      punyaSesi,
      isLoading,
      boleh,
      tokoAktif,
      rincianToko,
      gantiToko,
      masuk,
      keluar,
      pakaiSesiBaru,
    ],
  )

  return <KonteksSesi.Provider value={nilai}>{children}</KonteksSesi.Provider>
}

export function useSesi(): NilaiSesi {
  const v = useContext(KonteksSesi)
  if (!v) throw new Error('useSesi dipakai di luar PenyediaSesi')
  return v
}

/**
 * Pintasan untuk penjagaan izin di dalam komponen.
 *
 *   const { boleh } = useIzin()
 *   {boleh('sale.void') && <TombolBatalkan />}
 *
 * Ingat aturannya: izin MENYEMBUNYIKAN, bukan menonaktifkan. Tombol abu-abu
 * yang tidak bisa diklik membuat pengguna menelepon dukungan.
 */
export function useIzin() {
  const { boleh } = useSesi()
  return { boleh }
}
