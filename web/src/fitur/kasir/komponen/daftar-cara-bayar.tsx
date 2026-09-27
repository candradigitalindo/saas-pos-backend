import { formatRupiah } from '@/bersama/util/uang'
import type { RingkasanPenjualan } from '@/bersama/tipe/pos'
import { ikonMetode, namaMetode } from '../label-transaksi'

/**
 * Uang masuk per cara bayar, masing-masing dengan batang sebanding yang
 * terbesar. Dipakai di Riwayat (per hari) dan Ganti Shift (per shift) — angka
 * dari server, tunai sudah dikurangi kembalian.
 */
export function DaftarCaraBayar({ data }: { data: RingkasanPenjualan['by_method'] }) {
  if (data.length === 0) return <p className="text-label text-teks-redup">Belum ada pembayaran.</p>
  const terbesar = Math.max(1, ...data.map((m) => m.amount))
  return (
    <ul className="flex flex-col gap-3">
      {data.map((m) => {
        const Ikon = ikonMetode(m.method)
        return (
          <li key={m.method} className="flex flex-col gap-1.5">
            <div className="flex items-center justify-between gap-3 text-label">
              <span className="flex min-w-0 items-center gap-2 text-teks-sekunder">
                <Ikon className="h-4 w-4 shrink-0" aria-hidden />
                <span className="truncate">
                  {namaMetode(m.method)}
                  <span className="text-teks-redup"> · {m.count}×</span>
                </span>
              </span>
              <span className="shrink-0 font-semibold tabular-nums text-teks-utama">
                {formatRupiah(m.amount)}
              </span>
            </div>
            <div className="h-1.5 overflow-hidden rounded-full bg-permukaan-2" aria-hidden>
              <div
                className="h-full rounded-full bg-utama"
                style={{ width: `${Math.max(4, (m.amount / terbesar) * 100)}%` }}
              />
            </div>
          </li>
        )
      })}
    </ul>
  )
}
