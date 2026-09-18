import { useCallback, useEffect, useState } from 'react'
import { Flashlight, Keyboard, ScanLine } from 'lucide-react'
import { Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { Tombol } from '@/bersama/ui/tombol'
import { Kolom } from '@/bersama/ui/kolom'
import { usePemindai } from '@/bersama/hooks/use-pemindai'
import { cn } from '@/bersama/util/cn'

/**
 * Pemindai barcode.
 *
 * Dua hal yang membuatnya layak dipakai di warung:
 *
 *   - **Selalu ada jalan keluar.** Kamera bisa ditolak izinnya, rusak, atau
 *     barcodenya sobek. Karena itu kolom ketik manual SELALU tersedia di layar
 *     yang sama, bukan disembunyikan di balik pesan galat (ui/01 §7).
 *   - **Kamera mati begitu dialog ditutup.** Ditangani di usePemindai.
 */
export function PemindaiBarcode({
  terbuka,
  onTutup,
  onKode,
  judul = 'Pindai Barcode',
  keterangan = 'Arahkan kamera ke barcode di kemasan barang.',
}: {
  terbuka: boolean
  onTutup: () => void
  /**
   * Dipanggil dengan kode yang terbaca. Kembalikan `false` bila kodenya tidak
   * dikenal — pencarian barang biasanya asinkron (dibaca dari Dexie), jadi
   * Promise ikut diterima dan ditunggu.
   */
  onKode: (kode: string) => boolean | void | Promise<boolean | void>
  judul?: string
  keterangan?: string
}) {
  const [manual, setManual] = useState('')
  const [tidakDikenal, setTidakDikenal] = useState<string | null>(null)

  const tangani = useCallback(
    async (kode: string) => {
      const dikenal = await onKode(kode)
      if (dikenal === false) {
        setTidakDikenal(kode)
        // Getar dua kali sebagai tanda gagal — kasir sering tidak melihat layar
        // saat memindai; getaran lebih cepat sampai daripada teks.
        navigator.vibrate?.([80, 60, 80])
      } else {
        setTidakDikenal(null)
        navigator.vibrate?.(40)
      }
    },
    [onKode],
  )

  // usePemindai memanggilnya dari gelung bingkai yang tidak menunggu Promise.
  const tanganiSinkron = useCallback((kode: string) => void tangani(kode), [tangani])

  const { video, status, pesan, adaSenter, senterMenyala, ubahSenter } = usePemindai(
    terbuka,
    tanganiSinkron,
  )

  useEffect(() => {
    if (!terbuka) {
      setManual('')
      setTidakDikenal(null)
    }
  }, [terbuka])

  const kameraJalan = status === 'memindai'

  return (
    <Dialog open={terbuka} onOpenChange={(o) => !o && onTutup()}>
      <IsiDialog judul={judul} keterangan={keterangan} className="sm:max-w-md">
        <div className="relative overflow-hidden rounded-kartu bg-teks-utama">
          {/* Video selalu dipasang: elemennya harus ada sebelum aliran kamera
              disambungkan. */}
          <video
            ref={video}
            className={cn('aspect-[4/3] w-full object-cover', !kameraJalan && 'opacity-0')}
            playsInline
            muted
            aria-label="Tampilan kamera"
          />

          {kameraJalan && (
            <>
              {/* Bingkai bidik: memberi tahu ke mana barcode harus diarahkan. */}
              <div
                className="pointer-events-none absolute inset-0 flex items-center justify-center"
                aria-hidden
              >
                <div className="h-28 w-4/5 rounded-kontrol border-2 border-white/90 shadow-[0_0_0_9999px_rgba(0,0,0,0.35)]" />
              </div>
              {adaSenter && (
                <button
                  type="button"
                  onClick={ubahSenter}
                  aria-pressed={senterMenyala}
                  className={cn(
                    'absolute bottom-3 right-3 flex h-12 w-12 items-center justify-center',
                    'rounded-full border border-white/40 backdrop-blur',
                    senterMenyala ? 'bg-jingga-400 text-teks-di-jingga' : 'bg-black/40 text-white',
                  )}
                >
                  <Flashlight className="h-5 w-5" aria-hidden />
                  <span className="sr-only">
                    {senterMenyala ? 'Matikan senter' : 'Nyalakan senter'}
                  </span>
                </button>
              )}
            </>
          )}

          {!kameraJalan && (
            <div className="absolute inset-0 flex flex-col items-center justify-center gap-2 p-6 text-center">
              <ScanLine className="h-10 w-10 text-white/70" aria-hidden />
              <p className="text-label text-white/90">{pesan}</p>
            </div>
          )}
        </div>

        {tidakDikenal && (
          <p className="rounded-kontrol border border-jingga-600 bg-permukaan-2 px-3 py-2 text-label text-jingga-700">
            Barcode <strong className="tabular-nums">{tidakDikenal}</strong> belum
            terdaftar di barang Anda. Tambahkan barangnya dulu, lalu isi
            barcodenya di bagian &ldquo;Detail lainnya&rdquo;.
          </p>
        )}

        {/* Jalan keluar yang selalu ada — bukan hanya saat kamera gagal. */}
        <form
          onSubmit={(e) => {
            e.preventDefault()
            const k = manual.trim()
            if (!k) return
            void tangani(k)
            setManual('')
          }}
          className="flex flex-col gap-2"
        >
          <Kolom
            label="Atau ketik kodenya"
            inputMode="numeric"
            value={manual}
            onChange={(e) => setManual(e.target.value)}
            bantuan="Dipakai kalau barcodenya sobek atau kamera susah membaca."
            autoComplete="off"
          />
          <div className="flex gap-2">
            <Tombol type="submit" lebarPenuh disabled={!manual.trim()}>
              <Keyboard className="h-5 w-5" aria-hidden />
              Cari Kode Ini
            </Tombol>
            <Tombol type="button" jenis="kedua" onClick={onTutup}>
              Tutup
            </Tombol>
          </div>
        </form>
      </IsiDialog>
    </Dialog>
  )
}
