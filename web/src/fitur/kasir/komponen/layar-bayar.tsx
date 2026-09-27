import { useMemo, useState } from 'react'
import { Banknote, Lock, NotebookPen, QrCode } from 'lucide-react'
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
  kunciQris,
}: {
  terbuka: boolean
  onTutup: () => void
  /** Pratinjau total dari keranjang. Angka final tetap dari server. */
  total: number
  mengirim: boolean
  galat?: string | null
  onSelesai: (metode: MetodeBayar, dibayar: number) => void
  /**
   * Diisi bila QRIS tidak termasuk paket langganan: teks paket yang
   * membukanya, mis. "Paket Basic". Petaknya tetap ada tapi tidak bisa
   * dipilih — kalau dihilangkan, kasir yang biasa melihatnya mengira
   * aplikasinya rusak; kalau bisa ditekan, server menolaknya di tengah antrean.
   */
  kunciQris?: string
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
            {CARA_BAYAR.map((c) => {
              const terkunci = c.nilai === 'qris' && !!kunciQris
              return (
                <button
                  key={c.nilai}
                  type="button"
                  disabled={terkunci}
                  onClick={() => setMetode(c.nilai)}
                  aria-pressed={metode === c.nilai}
                  aria-label={terkunci ? `${c.label}, terkunci — tersedia di ${kunciQris}` : undefined}
                  className={cn(
                    'flex h-20 flex-col items-center justify-center gap-1 rounded-kartu border',
                    terkunci
                      ? 'cursor-not-allowed border-dashed border-garis bg-permukaan-2 text-teks-redup'
                      : metode === c.nilai
                        ? 'border-utama bg-sorot text-utama'
                        : 'border-garis bg-permukaan text-teks-sekunder hover:bg-permukaan-2',
                  )}
                >
                  {terkunci ? (
                    <Lock className="h-5 w-5" aria-hidden />
                  ) : (
                    <c.ikon className="h-6 w-6" aria-hidden />
                  )}
                  <span className="text-label font-semibold">{c.label}</span>
                  {terkunci && <span className="text-keterangan leading-none">{kunciQris}</span>}
                </button>
              )
            })}
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
          <p className="rounded-kontrol border border-jingga-600 bg-permukaan-2 px-3 py-2 text-label text-jingga-700">
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
              kurang > 0 ? 'border-jingga-600 bg-permukaan-2' : 'border-utama bg-sorot',
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
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
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
