import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { History } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { api, GalatAPI } from '@/lib/api-client'
import { formatQty } from '@/bersama/util/desimal'
import { formatTanggalJam } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import type { Produk } from '@/bersama/tipe/katalog'
import { stokApi } from '../api'

/** Sebab pergerakan stok, dalam bahasa orang. */
const SEBAB: Record<string, string> = {
  sale: 'Terjual',
  sale_void: 'Pembatalan transaksi',
  refund: 'Retur pembeli',
  purchase: 'Barang masuk',
  adjustment: 'Koreksi manual',
  opname: 'Hasil hitung fisik',
  transfer_in: 'Kiriman masuk dari toko lain',
  transfer_out: 'Dikirim ke toko lain',
  recipe: 'Terpakai sebagai bahan',
}

/** Kartu stok: riwayat keluar-masuk satu barang. */
export function HalamanKartuStok() {
  const { productId } = useParams()
  const { tokoAktif } = useSesi()

  const produk = useQuery({
    queryKey: ['produk', productId],
    queryFn: () => api.get<Produk>(`/products/${productId}`),
    enabled: !!productId,
  })

  const q = useQuery({
    queryKey: ['kartu-stok', productId, tokoAktif],
    queryFn: () => stokApi.kartuStok(productId!, tokoAktif),
    enabled: !!productId,
  })

  const daftar = q.data?.data ?? []

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-4">
      <header>
        <h1 className="text-judul font-bold text-teks-utama">
          {produk.data?.name ?? 'Riwayat Stok'}
        </h1>
        <p className="text-label text-teks-sekunder">
          Setiap barang masuk dan keluar, dari yang terbaru.
        </p>
      </header>

      {q.isLoading ? (
        <KerangkaBaris jumlah={5} />
      ) : q.isError ? (
        <KeadaanGagal
          pesan={q.error instanceof GalatAPI ? q.error.pesan : 'Riwayat belum bisa dimuat.'}
          onCobaLagi={() => q.refetch()}
        />
      ) : daftar.length === 0 ? (
        <KeadaanKosong
          ikon={History}
          judul="Belum ada pergerakan"
          penjelasan="Barang ini belum pernah masuk atau keluar. Riwayatnya muncul setelah ada transaksi atau koreksi."
        />
      ) : (
        <ul className="flex flex-col gap-2">
          {daftar.map((g) => {
            const naik = !g.qty_delta.startsWith('-')
            return (
              <li key={g.id}>
                <Kartu className="flex items-center justify-between gap-3 p-3">
                  <div className="min-w-0">
                    <p className="font-medium text-teks-utama">
                      {SEBAB[g.kind] ?? g.kind}
                    </p>
                    <p className="text-keterangan text-teks-redup">
                      {formatTanggalJam(g.occurred_at)}
                      {g.reason && ` · ${g.reason}`}
                    </p>
                  </div>
                  <div className="shrink-0 text-right">
                    <p
                      className={cn(
                        'font-bold tabular-nums',
                        naik ? 'text-hijau-700' : 'text-bahaya-teks',
                      )}
                    >
                      {naik ? '+' : '−'}
                      {formatQty(g.qty_delta.replace('-', ''))}
                    </p>
                    <p className="text-keterangan tabular-nums text-teks-redup">
                      sisa {formatQty(g.balance_after)}
                    </p>
                  </div>
                </Kartu>
              </li>
            )
          })}
        </ul>
      )}
    </div>
  )
}
