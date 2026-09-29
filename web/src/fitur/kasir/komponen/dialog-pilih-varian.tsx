import type { Produk, VarianProduk } from '@/bersama/tipe/katalog'
import { Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQty } from '@/bersama/util/desimal'
import { cn } from '@/bersama/util/cn'

/**
 * Pilih varian ("Besar", "Es", "Level 3") sebelum barang masuk keranjang.
 *
 * Wajib pilih satu — sama seperti di aplikasi antar — karena harga barang
 * bervarian bergantung pada pilihannya. Satu ketukan langsung menambahkan
 * dan menutup dialog; jumlah diatur di keranjang seperti barang lain.
 */
export function DialogPilihVarian({
  produk,
  qtyPerVarian,
  onPilih,
  onTutup,
}: {
  produk: Produk
  /** id varian → qty yang sudah di keranjang, untuk lencana. */
  qtyPerVarian: (varianId: string) => string
  onPilih: (v: VarianProduk) => void
  onTutup: () => void
}) {
  const varian = produk.varian ?? []
  return (
    <Dialog open onOpenChange={(o) => !o && onTutup()}>
      <IsiDialog judul={produk.name} keterangan="Pilih salah satu.">
        <ul className="flex flex-col gap-2">
          {varian.map((v) => {
            const qty = qtyPerVarian(v.id)
            const ada = Number.parseFloat(qty) > 0
            return (
              <li key={v.id}>
                <button
                  type="button"
                  onClick={() => onPilih(v)}
                  className={cn(
                    'flex min-h-14 w-full items-center gap-3 rounded-kontrol border px-4 py-2 text-left',
                    'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-utama',
                    ada ? 'border-utama bg-sorot' : 'border-garis bg-permukaan hover:border-utama/40',
                  )}
                >
                  {/* Dua baris, bukan dipotong: nama varian yang harus dibaca kasir. */}
                  <span className="line-clamp-2 min-w-0 flex-1 break-words text-isi font-semibold text-teks-utama">
                    {v.name}
                  </span>
                  {ada && (
                    <span className="flex h-7 min-w-7 shrink-0 items-center justify-center rounded-full bg-utama px-1.5 text-keterangan font-bold text-utama-teks">
                      {formatQty(qty)}
                    </span>
                  )}
                  <span className="shrink-0 text-isi font-bold tabular-nums text-teks-utama">
                    {formatRupiah(produk.sell_price + v.price_delta)}
                  </span>
                </button>
              </li>
            )
          })}
        </ul>
      </IsiDialog>
    </Dialog>
  )
}
