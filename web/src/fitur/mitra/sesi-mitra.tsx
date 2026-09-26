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
import { adaSesi, hapusSesi, langganSesi, simpanSesi } from '@/lib/penyimpanan-sesi'
import { mitraApi, type Mitra, type PenggunaMitra } from './api'
import { keDate } from '@/bersama/util/tanggal'

/**
 * Sesi portal mitra.
 *
 * Sengaja TERPISAH dari sesi toko, bukan sekadar cabang di dalamnya: tokennya
 * berbeda kunci penyimpanan, dan masuk sebagai mitra tidak boleh menyentuh sesi
 * toko yang mungkin sedang terbuka di peramban yang sama.
 *
 * Realm mitra juga TIDAK punya /auth/refresh — sesi habis berarti masuk ulang.
 * Karena itu masa berlakunya disimpan apa adanya dari `expires_at`.
 */
interface NilaiSesiMitra {
  mitra: Mitra | undefined
  pengguna: PenggunaMitra | undefined
  memuat: boolean
  sudahMasuk: boolean
  masuk: (email: string, sandi: string) => Promise<void>
  keluar: () => void
}

const Konteks = createContext<NilaiSesiMitra | null>(null)

export function PenyediaSesiMitra({ children }: { children: ReactNode }) {
  const qc = useQueryClient()
  const [punyaSesi, setPunyaSesi] = useState(() => adaSesi('mitra'))

  useEffect(() => langganSesi(() => setPunyaSesi(adaSesi('mitra'))), [])

  const { data, isLoading } = useQuery({
    queryKey: ['mitra-me'],
    queryFn: mitraApi.saya,
    enabled: punyaSesi,
    staleTime: 5 * 60_000,
    retry: false,
  })

  const masuk = useCallback(
    async (email: string, sandi: string) => {
      const hasil = await mitraApi.masuk(email, sandi)
      simpanSesi('mitra', {
        access_token: hasil.access_token,
        // Realm mitra tidak menyediakan refresh token.
        refresh_token: '',
        kedaluwarsa: keDate(hasil.expires_at).getTime(),
      })
      setPunyaSesi(true)
      await qc.invalidateQueries({ queryKey: ['mitra-me'] })
    },
    [qc],
  )

  const keluar = useCallback(() => {
    hapusSesi('mitra')
    setPunyaSesi(false)
    qc.removeQueries({ queryKey: ['mitra-me'] })
  }, [qc])

  const nilai = useMemo<NilaiSesiMitra>(
    () => ({
      mitra: data?.partner,
      pengguna: data?.user,
      memuat: punyaSesi && isLoading,
      sudahMasuk: punyaSesi && !!data,
      masuk,
      keluar,
    }),
    [data, punyaSesi, isLoading, masuk, keluar],
  )

  return <Konteks.Provider value={nilai}>{children}</Konteks.Provider>
}

export function useSesiMitra(): NilaiSesiMitra {
  const v = useContext(Konteks)
  if (!v) throw new Error('useSesiMitra dipakai di luar PenyediaSesiMitra')
  return v
}
