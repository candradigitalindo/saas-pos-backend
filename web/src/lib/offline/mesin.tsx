import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useOnline } from '@/bersama/hooks/use-online'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useToast } from '@/bersama/komponen/toast'
import {
  jumlahMenunggu,
  jumlahPerluDiperiksa,
  kirimAntrean,
  pulihkanYangMenggantung,
} from './antrean'
import { tarikMasterData } from './tarik'

interface NilaiSinkron {
  /** Transaksi yang masih menunggu dikirim. */
  menunggu: number
  /** Transaksi yang gagal berkali-kali dan perlu dilihat manusia. */
  perluDiperiksa: number
  /** true saat sedang menarik master data pertama kali. */
  menyiapkan: boolean
  /** Memaksa hitung ulang setelah menambah/menghapus antrean. */
  segarkan: () => void
  /** Menjalankan pengiriman sekarang, mis. dari tombol "Coba kirim lagi". */
  kirimSekarang: () => Promise<void>
}

const KonteksSinkron = createContext<NilaiSinkron | null>(null)

/** Selang pengiriman berkala saat ada antrean menumpuk. */
const SELANG_KIRIM_MS = 30_000

/**
 * Mesin sinkronisasi.
 *
 * Berjalan diam-diam: menarik master data saat masuk, lalu mengirim antrean
 * begitu internet kembali dan setiap 30 detik selama masih ada yang menunggu.
 *
 * Pengguna TIDAK pernah diminta menekan tombol sinkronisasi. Yang dilihatnya
 * hanya kalimat kecil di pojok layar.
 */
export function PenyediaSinkron({ children }: { children: ReactNode }) {
  const { sudahMasuk, tokoAktif } = useSesi()
  const online = useOnline()
  const toast = useToast()
  const qc = useQueryClient()

  const [menunggu, setMenunggu] = useState(0)
  const [perluDiperiksa, setPerluDiperiksa] = useState(0)
  const [menyiapkan, setMenyiapkan] = useState(false)

  // Mencegah dua pengiriman berjalan bersamaan (mis. timer bertemu event online).
  const sedangKirim = useRef(false)

  const segarkan = useCallback(() => {
    void jumlahMenunggu().then(setMenunggu)
    void jumlahPerluDiperiksa().then(setPerluDiperiksa)
  }, [])

  const kirimSekarang = useCallback(async () => {
    if (sedangKirim.current || !navigator.onLine) return
    sedangKirim.current = true
    try {
      const hasil = await kirimAntrean()
      if (hasil) {
        // Duplikat sengaja TIDAK disebut: bagi pengguna itu bukan peristiwa.
        if (hasil.terkirim > 0) {
          toast.berhasil(`${hasil.terkirim} transaksi berhasil dikirim.`)
          qc.invalidateQueries({ queryKey: ['riwayat-transaksi'] })
          qc.invalidateQueries({ queryKey: ['shift-aktif'] })
        }
        if (hasil.gagal > 0) {
          toast.tampilkan(
            `${hasil.gagal} transaksi perlu diperiksa.`,
            'perhatian',
          )
        }
      }
    } finally {
      sedangKirim.current = false
      segarkan()
    }
  }, [toast, qc, segarkan])

  // Tarik master data sekali saat masuk, supaya kasir siap sebelum sinyal hilang.
  useEffect(() => {
    if (!sudahMasuk || !tokoAktif || !online) return
    let batal = false
    setMenyiapkan(true)
    tarikMasterData(tokoAktif)
      .catch((e) => {
        // Gagal menarik bukan alasan menghentikan kasir: data lama di Dexie
        // masih bisa dipakai berjualan. Tapi menelan galatnya diam-diam
        // membuat "katalog kosong" mustahil didiagnosis — jadi tetap dicatat.
        console.error('[sinkron] gagal menarik master data:', e)
      })
      .finally(() => {
        if (!batal) setMenyiapkan(false)
      })
    return () => {
      batal = true
    }
  }, [sudahMasuk, tokoAktif, online])

  // Kirim begitu internet kembali. Sebelum pengiriman pertama di halaman ini,
  // operasi yang tertahan "mengirim" dari sesi sebelumnya dikembalikan ke
  // antrean (lihat pulihkanYangMenggantung).
  const sudahDipulihkan = useRef(false)
  useEffect(() => {
    if (!sudahMasuk || !online) return
    void (async () => {
      if (!sudahDipulihkan.current) {
        sudahDipulihkan.current = true
        await pulihkanYangMenggantung()
      }
      await kirimSekarang()
    })()
  }, [sudahMasuk, online, kirimSekarang])

  // Lalu setiap 30 detik selama masih ada yang menunggu.
  useEffect(() => {
    if (!sudahMasuk || menunggu === 0) return
    const t = window.setInterval(() => void kirimSekarang(), SELANG_KIRIM_MS)
    return () => window.clearInterval(t)
  }, [sudahMasuk, menunggu, kirimSekarang])

  useEffect(() => {
    if (sudahMasuk) segarkan()
  }, [sudahMasuk, segarkan])

  return (
    <KonteksSinkron.Provider
      value={{ menunggu, perluDiperiksa, menyiapkan, segarkan, kirimSekarang }}
    >
      {children}
    </KonteksSinkron.Provider>
  )
}

export function useSinkron(): NilaiSinkron {
  const v = useContext(KonteksSinkron)
  if (!v) throw new Error('useSinkron dipakai di luar PenyediaSinkron')
  return v
}
