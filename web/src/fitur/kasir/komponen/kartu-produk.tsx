import type { Produk } from '@/bersama/tipe/katalog'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQty } from '@/bersama/util/desimal'
import { cn } from '@/bersama/util/cn'

/**
 * Kartu barang di grid kasir.
 *
 * Barang HABIS tetap terlihat, hanya tidak bisa ditekan. Kalau disembunyikan,
 * kasir mengira barangnya hilang dari sistem lalu menelepon pemilik
 * (ui/05-ALUR-UTAMA.md §2).
 */
export function KartuProduk({
  produk,
  stok,
  diKeranjang,
  onPilih,
}: {
  produk: Produk
  stok?: string
  diKeranjang: string
  onPilih: (p: Produk) => void
}) {
  const habis = produk.track_stock && stok !== undefined && Number.parseFloat(stok) <= 0
  const adaDiKeranjang = Number.parseFloat(diKeranjang) > 0

  return (
    <button
      type="button"
      disabled={habis}
      onClick={() => onPilih(produk)}
      aria-label={`${produk.name}, ${formatRupiah(produk.sell_price)}${habis ? ', habis' : ''}`}
      className={cn(
        'relative flex min-h-30 flex-col justify-between gap-1 rounded-kartu border p-3 text-left',
        'transition-colors duration-150 ease-out',
        habis
          ? 'cursor-not-allowed border-garis bg-permukaan-2 opacity-60'
          : adaDiKeranjang
            ? 'border-utama bg-sorot'
            : 'border-garis bg-permukaan hover:bg-permukaan-2',
      )}
    >
      {adaDiKeranjang && (
        <span
          className="absolute -right-2 -top-2 flex h-7 min-w-7 items-center justify-center rounded-full bg-utama px-1.5 text-keterangan font-bold text-utama-teks"
          aria-hidden
        >
          {formatQty(diKeranjang)}
        </span>
      )}

      <span className="line-clamp-2 text-label font-semibold text-teks-utama">
        {produk.name}
      </span>

      <span className="flex flex-col">
        <span className="text-isi font-bold tabular-nums text-teks-utama">
          {formatRupiah(produk.sell_price)}
        </span>
        {/* Warna saja tidak cukup: keadaan habis ditulis dengan kata. */}
        {habis ? (
          <span className="text-keterangan font-semibold text-bahaya-teks">HABIS</span>
        ) : (
          produk.track_stock &&
          stok !== undefined && (
            <span className="text-keterangan text-teks-redup">
              sisa {formatQty(stok)} {produk.unit_name}
            </span>
          )
        )}
      </span>
    </button>
  )
}
