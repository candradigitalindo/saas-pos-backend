import { formatRupiah } from '@/bersama/util/uang'
import { cn } from '@/bersama/util/cn'

/**
 * "Dari mana penjualannya?" (ui/05-ALUR-UTAMA.md §6).
 *
 * Batang mendatar dengan LABEL LANGSUNG di setiap baris — bukan grafik dengan
 * legenda terpisah. Untuk lima-enam kategori, ini bentuk yang paling cepat
 * dibaca, dan nilainya tertulis apa adanya sehingga tidak perlu ditaksir dari
 * panjang batang.
 *
 * Label langsung juga BUKAN pilihan gaya: pemisahan palet grafik untuk buta
 * warna biru-kuning ada di bawah ambang aman, jadi warna tidak boleh jadi
 * satu-satunya pembeda (lihat ui/02-SISTEM-DESAIN.md).
 */
export interface BarisBatang {
  kunci: string
  label: string
  nilai: number
  /** Keterangan kecil di kanan, mis. persentase atau jumlah transaksi. */
  keterangan?: string
}

/** Urutan warna TETAP dan tidak pernah diputar — warna mengikuti entitas. */
const WARNA = [
  'bg-grafik-1',
  'bg-grafik-2',
  'bg-grafik-3',
  'bg-grafik-4',
  'bg-grafik-5',
  'bg-grafik-6',
] as const

export function BatangKanal({
  baris,
  judul,
  satuWarna = false,
}: {
  baris: BarisBatang[]
  judul?: string
  /**
   * Semua batang satu warna. Untuk daftar panjang (per barang): enam warna
   * yang berulang di dua puluh baris tidak lagi menandai entitas apa pun,
   * hanya membuat daftarnya belang.
   */
  satuWarna?: boolean
}) {
  const tertinggi = Math.max(...baris.map((b) => b.nilai), 1)

  return (
    <section className="flex flex-col gap-3">
      {judul && (
        <h2 className="text-judul-kartu font-semibold text-teks-utama">{judul}</h2>
      )}

      <ul className="flex flex-col gap-3">
        {baris.map((b, i) => (
          <li key={b.kunci} className="flex flex-col gap-1">
            <div className="flex items-baseline justify-between gap-3">
              <span className="min-w-0 truncate text-label font-medium text-teks-utama">
                {b.label}
              </span>
              <span className="shrink-0 tabular-nums text-label font-semibold text-teks-utama">
                {formatRupiah(b.nilai)}
              </span>
            </div>

            <div className="flex items-center gap-2">
              <div
                className="h-3 flex-1 overflow-hidden rounded-full bg-permukaan-2"
                role="img"
                aria-label={`${b.label}: ${formatRupiah(b.nilai)}`}
              >
                <div
                  className={cn('h-full rounded-full', satuWarna ? 'bg-grafik-1' : WARNA[i % WARNA.length])}
                  style={{ width: `${Math.max(2, (b.nilai / tertinggi) * 100)}%` }}
                />
              </div>
              {b.keterangan && (
                <span className="shrink-0 text-keterangan tabular-nums text-teks-redup">
                  {b.keterangan}
                </span>
              )}
            </div>
          </li>
        ))}
      </ul>
    </section>
  )
}
