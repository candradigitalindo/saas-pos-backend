import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { CheckCircle2, Info, TriangleAlert, XCircle } from 'lucide-react'
import { cn } from '@/bersama/util/cn'

/**
 * Notifikasi singkat di bawah layar.
 *
 * Dua aturan dari dokumen prinsip:
 *   §5 Tidak boleh ada ketukan yang "sunyi" — setiap aksi selesai memberi kabar,
 *      lalu hilang sendiri 4 detik.
 *   §6 Aksi ringan langsung jalan + tombol "Urungkan" 8 detik, BUKAN dialog
 *      konfirmasi. Dialog "Anda yakin?" akan diklik "Ya" tanpa dibaca.
 */
type NadaToast = 'berhasil' | 'gagal' | 'perhatian' | 'info'

interface Toast {
  id: number
  nada: NadaToast
  pesan: string
  urungkan?: () => void
}

interface NilaiToast {
  tampilkan: (pesan: string, nada?: NadaToast) => void
  berhasil: (pesan: string) => void
  gagal: (pesan: string) => void
  /** Aksi ringan: jalankan dulu, beri 8 detik untuk mengurungkan. */
  denganUrungkan: (pesan: string, urungkan: () => void) => void
}

const KonteksToast = createContext<NilaiToast | null>(null)

const IKON: Record<NadaToast, typeof CheckCircle2> = {
  berhasil: CheckCircle2,
  gagal: XCircle,
  perhatian: TriangleAlert,
  info: Info,
}

const WARNA: Record<NadaToast, string> = {
  berhasil: 'border-hijau-600 bg-hijau-50 text-hijau-800',
  gagal: 'border-bahaya bg-red-50 text-bahaya-teks',
  perhatian: 'border-jingga-600 bg-jingga-100 text-jingga-700',
  info: 'border-info bg-blue-50 text-info-teks',
}

export function PenyediaToast({ children }: { children: ReactNode }) {
  const [daftar, setDaftar] = useState<Toast[]>([])
  const nomor = useRef(0)

  const buang = useCallback((id: number) => {
    setDaftar((d) => d.filter((t) => t.id !== id))
  }, [])

  const dorong = useCallback(
    (t: Omit<Toast, 'id'>, umurMs: number) => {
      const id = ++nomor.current
      setDaftar((d) => [...d, { ...t, id }])
      window.setTimeout(() => buang(id), umurMs)
    },
    [buang],
  )

  const nilai = useMemo<NilaiToast>(
    () => ({
      tampilkan: (pesan, nada = 'info') => dorong({ pesan, nada }, 4000),
      berhasil: (pesan) => dorong({ pesan, nada: 'berhasil' }, 4000),
      gagal: (pesan) => dorong({ pesan, nada: 'gagal' }, 6000),
      // 8 detik: cukup untuk sadar salah, tidak sampai menghalangi layar.
      denganUrungkan: (pesan, urungkan) =>
        dorong({ pesan, nada: 'berhasil', urungkan }, 8000),
    }),
    [dorong],
  )

  return (
    <KonteksToast.Provider value={nilai}>
      {children}
      <div
        className="pointer-events-none fixed inset-x-0 bottom-[var(--sela-bilah-bawah)] z-[60] flex flex-col items-center gap-2 p-4 pb-[calc(1rem+env(safe-area-inset-bottom))]"
        role="status"
        aria-live="polite"
      >
        {daftar.map((t) => {
          const Ikon = IKON[t.nada]
          return (
            <div
              key={t.id}
              className={cn(
                'pointer-events-auto flex w-full max-w-md items-center gap-3',
                'rounded-kartu border px-4 py-3 shadow-melayang',
                WARNA[t.nada],
              )}
            >
              <Ikon className="h-5 w-5 shrink-0" aria-hidden />
              <p className="flex-1 text-label font-medium">{t.pesan}</p>
              {t.urungkan && (
                <button
                  type="button"
                  onClick={() => {
                    t.urungkan?.()
                    buang(t.id)
                  }}
                  className="shrink-0 font-semibold underline underline-offset-2"
                >
                  Urungkan
                </button>
              )}
            </div>
          )
        })}
      </div>
    </KonteksToast.Provider>
  )
}

export function useToast(): NilaiToast {
  const v = useContext(KonteksToast)
  if (!v) throw new Error('useToast dipakai di luar PenyediaToast')
  return v
}
