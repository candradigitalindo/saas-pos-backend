import { Link, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ArrowDownLeft, ArrowUpRight, History, SlidersHorizontal } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { api, GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatQty, formatQtySatuan } from '@/bersama/util/desimal'
import { formatTanggalJam } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import type { Produk } from '@/bersama/tipe/katalog'
import { stokApi } from '../api'
import { keadaanSaldo } from '../keadaan-stok'
import { Laku, LencanaKeadaan, Meter, teksSisa } from '../komponen/bagian-saldo'

/**
 * Sebab pergerakan stok, dalam bahasa orang (ui/01-PRINSIP-DESAIN.md §2).
 *
 * Daftarnya WAJIB sama persis dengan CHECK constraint `stock_ledger.kind` di
 * database/migrations/000008_stock_ledger.up.sql. Tipe `JenisGerakan` di bawah
 * membuat kunci yang hilang jadi galat kompilasi, bukan istilah sistem yang
 * diam-diam bocor ke layar.
 *
 * Dua yang sempat lolos dan ditemukan dari layar sungguhan: `initial` tidak
 * ada sama sekali sehingga tampil apa adanya, dan pembatalan transaksi ditulis
 * dengan kunci `sale_void` padahal backend memakai `void` — sehingga kata
 * "void", yang justru dilarang eksplisit di ui/01 §2, akan muncul di layar.
 */
type JenisGerakan =
  | 'sale'
  | 'void'
  | 'refund'
  | 'purchase'
  | 'adjustment'
  | 'transfer_in'
  | 'transfer_out'
  | 'opname'
  | 'recipe'
  | 'initial'

const SEBAB: Record<JenisGerakan, string> = {
  sale: 'Terjual',
  void: 'Pembatalan transaksi',
  refund: 'Retur pembeli',
  purchase: 'Barang masuk',
  adjustment: 'Koreksi manual',
  opname: 'Hasil hitung fisik',
  transfer_in: 'Kiriman masuk dari toko lain',
  transfer_out: 'Dikirim ke toko lain',
  recipe: 'Terpakai sebagai bahan',
  initial: 'Stok awal',
}

/** Kartu stok: riwayat keluar-masuk satu barang. */
export function HalamanKartuStok() {
  const { productId } = useParams()
  const { tokoAktif, boleh } = useSesi()

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

  // Saldo sekarang di atas riwayat: halaman ini dibuka dari daftar Stok
  // untuk menjawab "kenapa sisanya segini", dan di HP inilah jalan ke Koreksi.
  const saldo = useQuery({
    queryKey: ['stok', tokoAktif, 'barang', productId],
    queryFn: () => stokApi.saldo(tokoAktif!, false, 1, 1, { produk: [productId!] }),
    enabled: !!productId && !!tokoAktif,
  })
  const s = saldo.data?.data[0]
  const k = s && keadaanSaldo(s)

  const daftar = q.data?.data ?? []

  return (
    <div className="flex w-full max-w-2xl flex-col gap-4">
      <header>
        <h1 className="text-judul font-bold text-teks-utama">
          {produk.data?.name ?? 'Riwayat Stok'}
        </h1>
        <p className="text-label text-teks-sekunder">
          Setiap barang masuk dan keluar, dari yang terbaru.
        </p>
      </header>

      {s && k && (
        <Kartu className="flex flex-wrap items-center gap-x-4 gap-y-3 p-4">
          <div className="min-w-0 flex-1">
            <div className="flex flex-wrap items-center gap-2">
              <span
                className={cn(
                  'text-judul-kartu font-extrabold tabular-nums',
                  k === 'out' || k === 'negative' ? 'text-bahaya-teks' : 'text-teks-utama',
                )}
              >
                {k === 'negative' ? formatQtySatuan(s.qty, s.unit_name).replace('-', '−') : teksSisa(s, k)}
              </span>
              <LencanaKeadaan keadaan={k} />
            </div>
            <p className="mt-0.5 text-keterangan text-teks-redup">
              {Number.parseFloat(s.min_stock) > 0 && <>batas {formatQtySatuan(s.min_stock, s.unit_name)} · </>}
              <Laku s={s} k={k} sebaris />
            </p>
            <Meter s={s} k={k} className="mt-2 w-40" />
          </div>
          {boleh(IZIN.stockAdjust) && (
            <Tombol jenis={k === 'negative' ? 'utama' : 'kedua'} ukuran="padat" asChild>
              <Link to={`/stok/koreksi?product_id=${productId}`}>
                <SlidersHorizontal className="h-4 w-4" aria-hidden />
                {k === 'negative' ? 'Cocokkan stok' : 'Koreksi stok'}
              </Link>
            </Tombol>
          )}
        </Kartu>
      )}

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
                <Kartu className="flex items-center gap-3 p-3">
                  {/* Arah pergerakan ditandai IKON, bukan hanya warna angkanya.
                      Satu dari dua belas laki-laki sulit membedakan merah-hijau,
                      dan halaman ini justru dibaca saat stok terasa janggal —
                      saat itu arah tiap baris harus terbaca sekilas. */}
                  <span
                    className={cn(
                      'flex h-10 w-10 shrink-0 items-center justify-center rounded-kontrol',
                      naik ? 'bg-utama/10 text-hijau-700' : 'bg-bahaya-teks/10 text-bahaya-teks',
                    )}
                  >
                    {naik ? (
                      <ArrowDownLeft className="h-5 w-5" aria-hidden />
                    ) : (
                      <ArrowUpRight className="h-5 w-5" aria-hidden />
                    )}
                  </span>

                  <div className="min-w-0 flex-1">
                    <p className="truncate font-medium text-teks-utama">
                      {SEBAB[g.kind as JenisGerakan] ?? 'Pergerakan lain'}
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
