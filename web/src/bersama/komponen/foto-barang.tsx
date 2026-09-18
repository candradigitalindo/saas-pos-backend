import { useState } from 'react'
import { inisialProduk } from '@/bersama/util/inisial'
import { cn } from '@/bersama/util/cn'

/**
 * Foto barang, dengan penanda huruf sebagai gantinya bila belum ada.
 *
 * Kasir mencari barang dengan MATA, bukan dengan membaca. Grid tanpa gambar
 * adalah dinding teks seragam yang memaksa tiap kartu dibaca satu per satu —
 * dan itu terjadi di layar yang dipakai sambil ada antrean.
 *
 * Penanda hurufnya bukan penambal sementara: warung dengan seratus barang
 * tidak akan pernah selesai memotret semuanya, jadi kartu tanpa foto harus
 * tetap enak dipindai mata selamanya. Bentuk dan warnanya dibuat SAMA dengan
 * kartu berfoto supaya grid tidak belang.
 *
 * Gambar yang gagal dimuat jatuh ke penanda huruf, tidak meninggalkan ikon
 * rusak: alamat foto bisa basi setelah pemulihan cadangan, dan kasir tidak
 * seharusnya melihat bekas kerusakan itu.
 */
export function FotoBarang({
  nama,
  url,
  className,
}: {
  nama: string
  url?: string | null
  className?: string
}) {
  const [gagal, setGagal] = useState(false)
  const tampilkanFoto = !!url && !gagal

  return (
    <div
      className={cn(
        // 3:2, bukan persegi. Persegi memakan 160px tinggi di kartu HP dua
        // kolom dan memangkas grid kasir dari empat baris terlihat jadi dua
        // setengah — di layar yang dipakai sambil ada antrean, baris yang
        // hilang itu lebih mahal daripada gambar yang lebih besar.
        'relative aspect-[3/2] w-full shrink-0 overflow-hidden rounded-kontrol',
        'bg-utama/10',
        className,
      )}
    >
      {tampilkanFoto ? (
        <img
          src={url ?? ''}
          alt=""
          // alt kosong & aria-hidden: nama barangnya sudah tertulis tepat di
          // bawah kartu ini, dan pembaca layar tidak perlu mendengarnya dua kali.
          aria-hidden
          loading="lazy"
          decoding="async"
          onError={() => setGagal(true)}
          className="h-full w-full object-cover"
        />
      ) : (
        <span
          aria-hidden
          className={cn(
            'flex h-full w-full items-center justify-center px-1 text-center',
            // TANPA uppercase: huruf besar di awal tiap potongan justru yang
            // membuat "KopSusGul" terbaca sebagai tiga kata. Dijadikan kapital
            // semua, ia berubah jadi "KOPSUSGUL" — tembok huruf yang persis
            // menghapus pembeda yang dibuat aturan tiga-hurufnya.
            // `hijau-800`, bukan `utama`. Keping ini duduk di atas kartu putih
            // MAUPUN kartu `permukaan-2` (barang habis), dan di atas yang
            // kedua `utama` hanya mencapai 4,36:1 — lulus di satu tempat,
            // gagal di tempat lain. Yang lebih pekat aman di keduanya.
            'text-judul-kartu font-extrabold tracking-tight text-hijau-800',
          )}
        >
          {inisialProduk(nama)}
        </span>
      )}
    </div>
  )
}
