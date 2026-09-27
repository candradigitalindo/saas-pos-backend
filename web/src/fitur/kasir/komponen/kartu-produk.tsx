import type { Produk } from '@/bersama/tipe/katalog'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQty, formatSisaStok } from '@/bersama/util/desimal'
import { FotoBarang } from '@/bersama/komponen/foto-barang'
import { cn } from '@/bersama/util/cn'

/**
 * Kartu barang di grid kasir.
 *
 * Barang HABIS tetap terlihat, hanya tidak bisa ditekan. Kalau disembunyikan,
 * kasir mengira barangnya hilang dari sistem lalu menelepon pemilik
 * (ui/05-ALUR-UTAMA.md §2).
 *
 * Susunannya: nama di atas, HARGA sebagai isi utama di bawah. Harga yang
 * dibaca berulang-ulang sepanjang hari, bukan namanya — nama dipakai untuk
 * menemukan, harga untuk memastikan. Karena itu harga diberi ukuran lebih besar
 * daripada nama, bukan sebaliknya.
 */
export function KartuProduk({
  produk,
  stok,
  diKeranjang,
  kelasWarna,
  onPilih,
}: {
  produk: Produk
  stok?: string
  diKeranjang: string
  /** Warna petak kategori barang ini (kelasPetak). */
  kelasWarna?: string
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
        'group relative flex min-h-30 flex-col justify-between gap-2 rounded-kartu border p-3 text-left',
        'transition-[background-color,border-color,box-shadow,transform] duration-150 ease-out',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-utama focus-visible:ring-offset-2',
        habis
          ? 'cursor-not-allowed border-garis bg-permukaan-2'
          : adaDiKeranjang
            ? 'border-utama bg-sorot shadow-kartu'
            : 'border-garis bg-permukaan shadow-kartu hover:border-utama/40 hover:shadow-melayang',
        // Umpan balik tekan: kartu menyusut sedikit. Di layar kasir yang
        // ditekan bertubi-tubi, ini yang memberi tahu "ketukan tadi masuk"
        // tanpa perlu menunggu toast.
        !habis && 'active:scale-[0.98] motion-reduce:active:scale-100',
      )}
    >
      {adaDiKeranjang && (
        <span
          className="absolute -right-2 -top-2 flex h-7 min-w-7 items-center justify-center rounded-full bg-utama px-1.5 text-keterangan font-bold text-utama-teks shadow-kartu"
          aria-hidden
        >
          {formatQty(diKeranjang)}
        </span>
      )}

      <FotoBarang
        nama={produk.name}
        url={produk.image_url}
        kelasWarna={kelasWarna}
        className={cn(habis && 'opacity-50')}
      />

      <span
        className={cn(
          'line-clamp-2 text-label font-medium',
          habis ? 'text-teks-redup' : 'text-teks-sekunder',
        )}
      >
        {produk.name}
      </span>

      <span className="flex flex-col gap-1">
        <span
          className={cn(
            'text-judul-kartu font-extrabold tabular-nums',
            habis ? 'text-teks-redup' : 'text-teks-utama',
          )}
        >
          {formatRupiah(produk.sell_price)}
        </span>

        {/* Warna saja tidak cukup: keadaan habis ditulis dengan kata, dan
            diberi lencana supaya terbaca sebagai status — bukan sebagai teks
            merah yang mudah tertukar dengan galat. */}
        {habis ? (
          <span className="w-fit rounded-full bg-permukaan px-2 py-0.5 text-keterangan font-semibold text-bahaya-teks">
            Habis
          </span>
        ) : (
          produk.track_stock &&
          stok !== undefined && (
            <span className="text-keterangan text-teks-redup">
              {formatSisaStok(stok, produk.unit_name)}
            </span>
          )
        )}
      </span>
    </button>
  )
}
