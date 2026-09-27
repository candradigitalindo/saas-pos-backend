import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ReceiptText } from 'lucide-react'
import { KartuBagian, TautanBagian } from '@/bersama/komponen/kartu-bagian'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { LencanaTransaksi } from '@/bersama/komponen/lencana-status'
import { formatRupiah } from '@/bersama/util/uang'
import { formatJam, formatTanggalAkrab } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { kasirApi } from '@/fitur/kasir/api'

/** Lima nota terakhir: cukup untuk "barusan ada yang bayar?" tanpa membuka riwayat. */
const JUMLAH = 5

/**
 * Transaksi terakhir di toko yang sedang dipakai, terbaru di atas.
 *
 * Setiap baris membuka Riwayat Penjualan PADA TANGGAL NOTA itu — bukan hari
 * ini — supaya nota kemarin tidak berujung di daftar kosong. Lencana status
 * hanya muncul bila bukan penjualan biasa (dibatalkan, retur): dua puluh
 * lencana "Lunas" berderet hanya menjadi derau.
 */
export function TransaksiTerakhir({
  tokoAktif,
  zona,
}: {
  tokoAktif?: string
  /** Zona waktu toko, supaya jam nota sama dengan jam di dinding toko. */
  zona?: string
}) {
  const q = useQuery({
    queryKey: ['transaksi-terakhir', tokoAktif],
    queryFn: () => kasirApi.daftarTransaksi({ outlet_id: tokoAktif }, 1, JUMLAH),
    enabled: !!tokoAktif,
  })
  const daftar = q.data?.data ?? []

  return (
    <KartuBagian
      judul="Transaksi terakhir"
      aksi={<TautanBagian ke="/kasir/riwayat">Lihat semua</TautanBagian>}
      isiClassName="px-2 pb-2 sm:px-2"
    >
      {q.isLoading ? (
        <div className="px-2">
          <KerangkaBaris jumlah={JUMLAH} />
        </div>
      ) : q.isError ? (
        <p className="px-2 py-6 text-center text-label text-teks-sekunder">
          Daftar transaksi belum bisa dimuat.
        </p>
      ) : daftar.length === 0 ? (
        <p className="px-2 py-6 text-center text-label text-teks-sekunder">
          Belum ada transaksi. Nota pertama akan muncul di sini.
        </p>
      ) : (
        <ul className="flex flex-col">
          {daftar.map((t) => {
            const batal = t.status === 'canceled' || !!t.voided_at
            const biasa = t.status === 'completed' && !batal
            return (
              <li key={t.id}>
                <Link
                  to={`/kasir/riwayat?tanggal=${t.business_date}`}
                  className="flex min-h-14 items-center gap-3 rounded-kontrol px-2 py-2 hover:bg-permukaan-2"
                >
                  <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-kontrol bg-utama/10 text-utama">
                    <ReceiptText className="h-[18px] w-[18px]" aria-hidden />
                  </span>
                  <div className="min-w-0 flex-1">
                    <p className="truncate text-label font-medium tabular-nums text-teks-utama">
                      {t.receipt_no}
                    </p>
                    <p className="flex flex-wrap items-center gap-x-2 text-keterangan text-teks-redup">
                      {formatTanggalAkrab(t.occurred_at, zona)} · {formatJam(t.occurred_at, zona)}
                      {!biasa && <LencanaTransaksi status={batal ? 'canceled' : t.status} />}
                    </p>
                  </div>
                  {/* Nota batal dicoret: uangnya tidak pernah masuk. Retur TIDAK
                      dicoret — itu uang yang sungguh keluar, dan angkanya sudah
                      bertanda minus. */}
                  <p
                    className={cn(
                      'shrink-0 text-label font-semibold tabular-nums',
                      batal ? 'text-teks-redup line-through decoration-1' : 'text-teks-utama',
                    )}
                  >
                    {formatRupiah(t.total)}
                  </p>
                </Link>
              </li>
            )
          })}
        </ul>
      )}
    </KartuBagian>
  )
}
