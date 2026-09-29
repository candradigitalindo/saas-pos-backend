import { useQuery } from '@tanstack/react-query'
import { KartuBagian, TautanBagian } from '@/bersama/komponen/kartu-bagian'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQtySatuan } from '@/bersama/util/desimal'
import { cn } from '@/bersama/util/cn'
import { laporanApi } from '../api'
import { geserTanggal } from '../deret'

/** Berapa barang yang ditampilkan. Lima cukup untuk sekali lirik. */
const JUMLAH = 5

/**
 * "Barang apa yang paling laku?" — lima teratas menurut uang masuk.
 *
 * Diurutkan menurut RUPIAH, bukan jumlah: satuannya berbeda-beda (kg, pcs,
 * porsi), dan "30 pcs permen" tidak sebanding dengan "3 kg daging". Jumlahnya
 * tetap ditulis di bawah nama, lengkap dengan satuannya.
 *
 * Angkanya bersih dari void dan retur (dijaga di server, lihat
 * SalesByProduct), jadi barang yang sering dikembalikan tidak ikut naik.
 * Batang tipis di bawah setiap baris adalah porsinya terhadap barang teratas —
 * cukup untuk melihat "yang pertama jauh meninggalkan yang lain" tanpa grafik.
 */
export function DaftarTerlaris({
  tokoAktif,
  hariIni,
  hari,
}: {
  tokoAktif?: string
  hariIni: string
  hari: number
}) {
  const dari = geserTanggal(hariIni, -(hari - 1))
  const q = useQuery({
    queryKey: ['terlaris', tokoAktif, dari, hariIni],
    queryFn: () => laporanApi.penjualan(dari, hariIni, 'product', tokoAktif),
  })

  // Barang yang habis diretur (uang bersih ≤ 0) bukan "terlaris".
  const baris = (q.data?.rows ?? []).filter((r) => r.net_amount > 0).slice(0, JUMLAH)
  const tertinggi = Math.max(...baris.map((r) => r.net_amount), 1)

  return (
    <KartuBagian
      judul="Barang terlaris"
      keterangan={`${hari} hari terakhir · menurut uang masuk`}
      aksi={<TautanBagian ke="/laporan">Laporan</TautanBagian>}
    >
      {q.isLoading ? (
        <KerangkaBaris jumlah={JUMLAH} />
      ) : q.isError ? (
        <p className="py-6 text-center text-label text-teks-sekunder">
          Daftar belum bisa dimuat.
        </p>
      ) : baris.length === 0 ? (
        <p className="py-6 text-center text-label text-teks-sekunder">
          Belum ada barang terjual dalam {hari} hari terakhir.
        </p>
      ) : (
        <ol className="flex flex-col gap-3.5">
          {baris.map((r, i) => (
            <li key={r.key} className="flex items-start gap-3">
              <span
                className={cn(
                  'flex h-7 w-7 shrink-0 items-center justify-center rounded-full text-keterangan font-bold tabular-nums',
                  i === 0 ? 'bg-sorot text-hijau-800' : 'bg-permukaan-2 text-teks-sekunder',
                )}
                aria-label={`Peringkat ${i + 1}`}
              >
                {i + 1}
              </span>
              <div className="min-w-0 flex-1">
                <div className="flex items-baseline justify-between gap-3">
                  <p className="min-w-0 truncate text-label font-medium text-teks-utama">
                    {r.label ?? r.key}
                  </p>
                  <p className="shrink-0 text-label font-semibold tabular-nums text-teks-utama">
                    {formatRupiah(r.net_amount)}
                  </p>
                </div>
                <p className="text-keterangan text-teks-redup">
                  {formatQtySatuan(r.qty ?? '0', r.unit)} terjual
                </p>
                <div className="mt-1.5 h-1.5 overflow-hidden rounded-full bg-permukaan-2" aria-hidden>
                  <div
                    className="h-full rounded-full bg-utama"
                    style={{ width: `${Math.max(4, (r.net_amount / tertinggi) * 100)}%` }}
                  />
                </div>
              </div>
            </li>
          ))}
        </ol>
      )}
    </KartuBagian>
  )
}
