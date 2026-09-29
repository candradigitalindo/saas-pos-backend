import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Check, Search } from 'lucide-react'
import { Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { FotoBarang } from '@/bersama/komponen/foto-barang'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useDaftarProduk, useKategori } from '@/bersama/hooks/use-katalog'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQtySatuan } from '@/bersama/util/desimal'
import { kelasPetak, petaWarnaKategori } from '@/bersama/util/warna-kategori'
import { cn } from '@/bersama/util/cn'
import type { Produk } from '@/bersama/tipe/katalog'
import { stokApi } from '../api'

/**
 * Pemilih barang untuk barang masuk, koreksi stok, dan kirim antar toko.
 *
 * Tiap baris memperlihatkan SISA STOK di toko ini — yang dibutuhkan orang saat
 * mencatat barang datang, mengoreksi, atau mengirim barang; harga jual saja
 * (versi sebelumnya) tidak menjawab "barang ini tinggal berapa?".
 */
export function PemilihBarang({
  terbuka,
  onTutup,
  onPilih,
  judul = 'Pilih barang',
  harga = 'jual',
  sudahDipilih = [],
}: {
  terbuka: boolean
  onTutup: () => void
  onPilih: (p: Produk) => void
  judul?: string
  /** Harga yang disebut di bawah sisa stok: jual, atau modal (barang masuk). */
  harga?: 'jual' | 'modal'
  /** Barang yang sudah ada di daftar — ditandai centang. */
  sudahDipilih?: string[]
}) {
  const { tokoAktif, boleh } = useSesi()
  const [cari, setCari] = useState('')
  const q = useDaftarProduk(cari)
  const daftar = q.data?.data ?? []
  const kat = useKategori()
  const warna = petaWarnaKategori(kat.data?.data ?? [])

  // Saldo HANYA untuk barang yang sedang tampil (?product_ids=).
  const ids = daftar.filter((p) => p.track_stock).map((p) => p.id)
  const bolehStok = boleh(IZIN.stockView) && !!tokoAktif
  const stok = useQuery({
    queryKey: ['stok', tokoAktif, 'pemilih', ids.join(',')],
    queryFn: () => stokApi.saldo(tokoAktif!, false, 1, 100, { produk: ids }),
    enabled: terbuka && bolehStok && ids.length > 0,
    staleTime: 30_000,
  })
  const sisa = new Map((stok.data?.data ?? []).map((s) => [s.product_id, s.qty]))

  return (
    <Dialog open={terbuka} onOpenChange={(o) => !o && onTutup()}>
      <IsiDialog judul={judul} className="sm:max-w-lg">
        <div className="relative">
          <Search
            className="pointer-events-none absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-teks-redup"
            aria-hidden
          />
          <input
            type="search"
            value={cari}
            onChange={(e) => setCari(e.target.value)}
            placeholder="Cari barang…"
            aria-label="Cari barang"
            autoFocus
            className="h-12 w-full rounded-kontrol border border-garis bg-permukaan pl-10 pr-3 text-isi text-teks-utama placeholder:text-teks-redup"
          />
        </div>

        <div className="max-h-96 overflow-y-auto">
          {q.isLoading ? (
            <KerangkaBaris jumlah={4} />
          ) : daftar.length === 0 ? (
            <p className="py-6 text-center text-isi text-teks-redup">
              {cari ? `Tidak ada barang bernama "${cari}".` : 'Belum ada barang.'}
            </p>
          ) : (
            <ul className="divide-y divide-garis">
              {daftar.map((p) => {
                const qty = sisa.get(p.id)
                const habis = qty !== undefined && Number.parseFloat(qty) <= 0
                const dipilih = sudahDipilih.includes(p.id)
                return (
                  <li key={p.id}>
                    <button
                      type="button"
                      onClick={() => {
                        onPilih(p)
                        setCari('')
                      }}
                      className="flex min-h-14 w-full items-center gap-3 px-1 py-2 text-left hover:bg-permukaan-2"
                    >
                      <FotoBarang
                        nama={p.name}
                        url={p.image_url}
                        kecil
                        kelasWarna={kelasPetak(warna, p.category_id)}
                        className="w-10"
                      />
                      <span className="min-w-0 flex-1">
                        <span className="flex items-center gap-1.5 font-medium text-teks-utama">
                          <span className="truncate">{p.name}</span>
                          {dipilih && (
                            <Check className="h-4 w-4 shrink-0 text-utama" aria-label="sudah di daftar" />
                          )}
                        </span>
                        <span className="block truncate text-keterangan text-teks-redup">
                          {[p.category_name, p.sku].filter(Boolean).join(' · ') || p.unit_name}
                        </span>
                      </span>
                      <span className="shrink-0 text-right tabular-nums">
                        {p.track_stock && qty !== undefined && (
                          <span
                            className={cn(
                              'block text-label font-semibold',
                              habis ? 'text-bahaya-teks' : 'text-teks-utama',
                            )}
                          >
                            {habis ? 'Habis' : `sisa ${formatQtySatuan(qty, p.unit_name)}`}
                          </span>
                        )}
                        <span className="block text-keterangan text-teks-redup">
                          {harga === 'modal'
                            ? `modal ${formatRupiah(p.cost_price)}`
                            : formatRupiah(p.sell_price)}
                        </span>
                      </span>
                    </button>
                  </li>
                )
              })}
            </ul>
          )}
        </div>
      </IsiDialog>
    </Dialog>
  )
}
