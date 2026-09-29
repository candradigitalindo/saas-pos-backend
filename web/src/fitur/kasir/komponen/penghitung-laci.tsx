import { useState } from 'react'
import { Minus, Plus } from 'lucide-react'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { formatAngka, formatRupiah } from '@/bersama/util/uang'
import { cn } from '@/bersama/util/cn'

/** Uang kertas yang beredar; koin cukup dijumlahkan jadi satu angka. */
const PECAHAN = [100_000, 50_000, 20_000, 10_000, 5_000, 2_000, 1_000] as const

/** Hasil hitung per pecahan — disimpan pemanggil supaya membuka ulang tidak mengulang dari nol. */
export interface HitunganPecahan {
  lembar: Record<number, number>
  koin: number
}

export const HITUNGAN_KOSONG: HitunganPecahan = { lembar: {}, koin: 0 }

export const jumlahPecahan = ({ lembar, koin }: HitunganPecahan) =>
  PECAHAN.reduce((s, p) => s + p * (lembar[p] ?? 0), 0) + koin

/**
 * Menghitung uang laci per pecahan, di dalam dialog.
 *
 * Kenapa ada. Saat serah terima, dua orang menghitung tumpukan uang bersama;
 * menjumlahkan "tujuh lembar lima puluh ribu, dua belas lembar dua puluh
 * ribu…" di kepala adalah sumber salah hitung yang paling sering — dan selisih
 * karena salah jumlah terbaca sebagai uang hilang. Di sini layar yang
 * menjumlahkan; orang cukup menghitung lembar.
 *
 * Kenapa dialog, bukan bagian halaman. Tujuh baris pecahan membuat layar serah
 * terima harus digulir, padahal sebagian besar kasir cukup mengetik totalnya.
 */
export function DialogPecahan({
  awal,
  onPakai,
  onTutup,
}: {
  awal: HitunganPecahan
  onPakai: (total: number, hitungan: HitunganPecahan) => void
  onTutup: () => void
}) {
  const [h, setH] = useState<HitunganPecahan>(awal)
  const total = jumlahPecahan(h)
  const ubah = (p: number, n: number) =>
    setH((l) => ({ ...l, lembar: { ...l.lembar, [p]: Math.max(0, Math.min(9999, n)) } }))

  return (
    <Dialog open onOpenChange={(o) => !o && onTutup()}>
      <IsiDialog judul="Hitung per pecahan" keterangan="Hitung lembar tiap pecahan; jumlahnya dihitung otomatis.">
        <ul className="grid grid-cols-2 gap-2">
          {PECAHAN.map((p) => {
            const n = h.lembar[p] ?? 0
            return (
              <li
                key={p}
                className={cn(
                  'flex flex-col gap-1.5 rounded-kontrol border p-2',
                  n > 0 ? 'border-utama/40 bg-sorot/40' : 'border-garis',
                )}
              >
                <div className="flex items-baseline justify-between gap-1 px-1">
                  <span className="font-semibold tabular-nums text-teks-utama">{formatAngka(p)}</span>
                  {n > 0 && (
                    <span className="truncate text-keterangan tabular-nums text-teks-redup">
                      {formatAngka(p * n)}
                    </span>
                  )}
                </div>
                <div className="flex items-center gap-1">
                  <button
                    type="button"
                    onClick={() => ubah(p, n - 1)}
                    disabled={n === 0}
                    aria-label={`Kurangi lembar ${formatAngka(p)}`}
                    className="flex h-11 w-11 shrink-0 items-center justify-center rounded-kontrol border border-garis bg-permukaan text-teks-utama hover:bg-permukaan-2 disabled:opacity-40"
                  >
                    <Minus className="h-4 w-4" aria-hidden />
                  </button>
                  <input
                    type="text"
                    inputMode="numeric"
                    autoComplete="off"
                    aria-label={`Jumlah lembar ${formatAngka(p)}`}
                    value={n === 0 ? '' : String(n)}
                    placeholder="0"
                    onChange={(e) => ubah(p, Number(e.target.value.replace(/\D/g, '') || 0))}
                    className="h-11 w-full min-w-0 rounded-kontrol border border-garis bg-permukaan text-center font-semibold tabular-nums text-teks-utama focus-visible:outline-2 focus-visible:outline-utama"
                  />
                  <button
                    type="button"
                    onClick={() => ubah(p, n + 1)}
                    aria-label={`Tambah lembar ${formatAngka(p)}`}
                    className="flex h-11 w-11 shrink-0 items-center justify-center rounded-kontrol border border-garis bg-permukaan text-teks-utama hover:bg-permukaan-2"
                  >
                    <Plus className="h-4 w-4" aria-hidden />
                  </button>
                </div>
              </li>
            )
          })}
          <li className="flex flex-col justify-end">
            <KolomUang label="Koin (jumlahnya)" nilai={h.koin} onNilai={(k) => setH((l) => ({ ...l, koin: k }))} />
          </li>
        </ul>

        <div className="flex items-baseline justify-between gap-3 rounded-kontrol bg-permukaan-2 px-3 py-2.5">
          <span className="text-label text-teks-sekunder">Jumlah uang di laci</span>
          <span className="text-judul-kartu font-bold tabular-nums text-teks-utama" aria-live="polite">
            {formatRupiah(total)}
          </span>
        </div>

        <AksiDialog>
          <Tombol onClick={() => onPakai(total, h)}>Pakai {formatRupiah(total)}</Tombol>
          <Tombol jenis="kedua" onClick={onTutup}>
            Batal
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}
