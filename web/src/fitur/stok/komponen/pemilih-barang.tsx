import { useState } from 'react'
import { Search } from 'lucide-react'
import { Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useDaftarProduk } from '@/bersama/hooks/use-katalog'
import { formatRupiah } from '@/bersama/util/uang'
import type { Produk } from '@/bersama/tipe/katalog'

/** Pemilih barang untuk barang masuk, koreksi stok, dan hitung fisik. */
export function PemilihBarang({
  terbuka,
  onTutup,
  onPilih,
  judul = 'Pilih barang',
}: {
  terbuka: boolean
  onTutup: () => void
  onPilih: (p: Produk) => void
  judul?: string
}) {
  const [cari, setCari] = useState('')
  const q = useDaftarProduk(cari)
  const daftar = q.data?.data ?? []

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

        <div className="max-h-80 overflow-y-auto">
          {q.isLoading ? (
            <KerangkaBaris jumlah={4} />
          ) : daftar.length === 0 ? (
            <p className="py-6 text-center text-isi text-teks-redup">
              {cari ? `Tidak ada barang bernama "${cari}".` : 'Belum ada barang.'}
            </p>
          ) : (
            <ul className="divide-y divide-garis">
              {daftar.map((p) => (
                <li key={p.id}>
                  <button
                    type="button"
                    onClick={() => {
                      onPilih(p)
                      setCari('')
                    }}
                    className="flex min-h-14 w-full items-center justify-between gap-3 px-1 py-2 text-left hover:bg-permukaan-2"
                  >
                    <span className="min-w-0">
                      <span className="block truncate font-medium text-teks-utama">
                        {p.name}
                      </span>
                      <span className="block text-keterangan text-teks-redup">
                        {p.unit_name}
                        {p.sku && ` · ${p.sku}`}
                      </span>
                    </span>
                    <span className="shrink-0 tabular-nums text-teks-sekunder">
                      {formatRupiah(p.sell_price)}
                    </span>
                  </button>
                </li>
              ))}
            </ul>
          )}
        </div>
      </IsiDialog>
    </Dialog>
  )
}
