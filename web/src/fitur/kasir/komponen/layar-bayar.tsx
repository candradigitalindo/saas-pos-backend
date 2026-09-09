import { useMemo, useState } from 'react'
import { Banknote, NotebookPen, QrCode } from 'lucide-react'
import { Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { Tombol } from '@/bersama/ui/tombol'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { formatRupiah } from '@/bersama/util/uang'
import { cn } from '@/bersama/util/cn'
import type { MetodeBayar } from '@/bersama/tipe/pos'

/**
 * Layar bayar (ui/05-ALUR-UTAMA.md §2).
 *
 * KEMBALIAN adalah angka terbesar di layar — itu yang dibutuhkan kasir dalam
 * tekanan antrean, bukan nomor struk.
 */

const CARA_BAYAR: { nilai: MetodeBayar; label: string; ikon: typeof Banknote }[] = [
  { nilai: 'cash', label: 'Tunai', ikon: Banknote },
  { nilai: 'qris', label: 'QRIS', ikon: QrCode },
  { nilai: 'credit', label: 'Kasbon', ikon: NotebookPen },
]

export function LayarBayar({
  terbuka,
  onTutup,
  total,
  mengirim,
  galat,
  onSelesai,
}: {
  terbuka: boolean
  onTutup: () => void
  /** Pratinjau total dari keranjang. Angka final tetap dari server. */
  total: number
  mengirim: boolean
  galat?: string | null
  onSelesai: (metode: MetodeBayar, dibayar: number) => void
}) {
  const [metode, setMetode] = useState<MetodeBayar>('cash')
  const [diterima, setDiterima] = useState(0)

  // Kasbon dan QRIS selalu pas: tidak ada uang fisik yang dikembalikan.
  const pasOtomatis = metode !== 'cash'
  const dibayar = pasOtomatis ? total : diterima
  const kembalian = Math.max(0, dibayar - total)
  const kurang = Math.max(0, total - dibayar)

  // Pintasan nominal yang benar-benar dipakai di laci, dibulatkan ke atas dari
  // total — bukan daftar tetap yang sering tidak relevan.
  const pintasan = useMemo(() => {
    const kandidat = new Set<number>([total])
    for (const kelipatan of [5000, 10000, 50000, 100000]) {
      const naik = Math.ceil(total / kelipatan) * kelipatan
      if (naik > total) kandidat.add(naik)
    }
    return [...kandidat].sort((a, b) => a - b).slice(0, 4)
  }, [total])

  function tutupDanReset() {
    setDiterima(0)
    setMetode('cash')
    onTutup()
  }

  return (
    <Dialog open={terbuka} onOpenChange={(o) => !o && !mengirim && tutupDanReset()}>
      <IsiDialog judul="Bayar" className="sm:max-w-lg">
        <div className="flex items-baseline justify-between border-b border-garis pb-3">
          <span className="text-isi text-teks-sekunder">Total belanja</span>
          <span className="text-judul font-extrabold tabular-nums text-teks-utama">
            {formatRupiah(total)}
          </span>
        </div>

        <fieldset className="flex flex-col gap-2">
          <legend className="mb-1 text-label font-medium text-teks-sekunder">
            Cara bayar
          </legend>
          <div className="grid grid-cols-3 gap-2">
            {CARA_BAYAR.map((c) => (
              <button
                key={c.nilai}
                type="button"
                onClick={() => setMetode(c.nilai)}
                aria-pressed={metode === c.nilai}
                className={cn(
                  'flex h-20 flex-col items-center justify-center gap-1 rounded-kartu border',
                  metode === c.nilai
                    ? 'border-utama bg-sorot text-utama'
                    : 'border-garis bg-permukaan text-teks-sekunder hover:bg-permukaan-2',
                )}
              >
                <c.ikon className="h-6 w-6" aria-hidden />
                <span className="text-label font-semibold">{c.label}</span>
              </button>
            ))}
          </div>
        </fieldset>

        {metode === 'cash' && (
          <div className="flex flex-col gap-2">
            <KolomUang
              label="Uang diterima"
              nilai={diterima}
              onNilai={setDiterima}
              bantuan="Uang yang diberikan pembeli."
              autoFocus
            />
            <div className="flex flex-wrap gap-2">
              {pintasan.map((n) => (
                <button
                  key={n}
                  type="button"
                  onClick={() => setDiterima(n)}
                  className="h-12 rounded-kontrol border border-garis bg-permukaan px-4 text-label font-semibold tabular-nums text-teks-utama hover:bg-permukaan-2"
                >
                  {n === total ? 'Pas' : formatRupiah(n)}
                </button>
              ))}
            </div>
          </div>
        )}

        {metode === 'credit' && (
          <p className="rounded-kontrol border border-jingga-600 bg-jingga-100 px-3 py-2 text-label text-jingga-700">
            Belanja ini dicatat sebagai utang pelanggan. Pastikan Anda tahu siapa
            pembelinya.
          </p>
        )}

        {/* Kembalian: angka paling besar di layar.
            Baru muncul setelah kasir mengisi sesuatu — menyambut layar dengan
            "Uang belum cukup" padahal belum sempat mengetik itu menuduh tanpa
            sebab, dan nada seperti itu yang membuat orang berhenti percaya. */}
        {metode === 'cash' && diterima > 0 && (
          <div
            className={cn(
              'rounded-kartu border-2 px-4 py-3',
              kurang > 0 ? 'border-jingga-600 bg-jingga-100' : 'border-utama bg-sorot',
            )}
          >
            <p className="text-label font-medium text-teks-sekunder">
              {kurang > 0 ? 'Uang belum cukup' : 'KEMBALIAN'}
            </p>
            <p
              className={cn(
                'text-angka font-extrabold tabular-nums',
                kurang > 0 ? 'text-jingga-700' : 'text-teks-utama',
              )}
            >
              {formatRupiah(kurang > 0 ? kurang : kembalian)}
            </p>
            {kurang > 0 && (
              <p className="text-keterangan text-jingga-700">
                Masih kurang segitu dari total belanja.
              </p>
            )}
          </div>
        )}

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-red-50 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <Tombol
          ukuran="kasir"
          lebarPenuh
          memuat={mengirim}
          labelMemuat="Menyimpan transaksi…"
          disabled={metode === 'cash' && kurang > 0}
          onClick={() => onSelesai(metode, dibayar)}
        >
          SELESAI
        </Tombol>
      </IsiDialog>
    </Dialog>
  )
}
