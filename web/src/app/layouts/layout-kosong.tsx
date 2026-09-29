import { Outlet } from 'react-router-dom'
import { PackageCheck, TrendingUp, WifiOff, type LucideIcon } from 'lucide-react'
import { MerekKasir } from '@/bersama/komponen/logo'

/**
 * Layout halaman publik: masuk & daftar. Tanpa navigasi apa pun.
 *
 * Layar ini yang PERTAMA dilihat setiap orang — dan versi awalnya hanyalah
 * formulir abu-abu di tengah layar kosong: tanpa nama produk, tanpa warna
 * merek, dengan tombol utama yang abu-abu karena kolomnya belum diisi. Tidak
 * ada yang memberi tahu aplikasi apa ini dan kenapa layak dipakai.
 *
 * Sekarang:
 *   - Layar besar: panel merek berwarna penuh di kiri, formulir di kanan.
 *   - HP: pita merek di atas, kartu formulir menumpang di bawahnya.
 *
 * Warna panel adalah `.permukaan-sorotan` dengan `text-utama-teks` — pasangan
 * yang SAMA dengan kartu sorotan di beranda, rasio kontrasnya sudah diukur di
 * mode terang maupun gelap (ui/02). Ikon & lencana di atasnya BERGARIS, bukan
 * berisi, supaya latar di balik teks tetap warna yang sudah diverifikasi.
 */
export function LayoutKosong() {
  return (
    <div className="min-h-dvh bg-latar lg:grid lg:grid-cols-2 xl:grid-cols-[minmax(0,3fr)_minmax(0,2fr)]">
      <PanelMerek />

      {/* Pita merek — HP & tablet potret. */}
      <header className="permukaan-sorotan px-6 pb-14 pt-8 text-utama-teks lg:hidden">
        <MerekKasir varian="terbalik" />
        {/* Disembunyikan di layar pendek (HP kecil): di sana 36px ini yang
            mendorong tombol "Buat Usaha Saya" keluar layar. */}
        <p className="mt-3 max-w-sm text-isi opacity-90 [@media(max-height:760px)]:hidden">
          Jualan jalan terus, walau sinyal putus.
        </p>
      </header>

      <main className="flex flex-col items-center px-4 pb-10 lg:justify-center lg:px-10 lg:py-12">
        <div className="gerak-lapis -mt-10 w-full max-w-md rounded-dialog border border-garis bg-permukaan p-6 shadow-melayang sm:p-8 lg:mt-0">
          <Outlet />
        </div>

        {/* Di HP panel merek tidak muat, tapi alasan memakai aplikasinya tetap
            perlu terbaca — dan tanpa ini separuh bawah layar kosong. */}
        <ul className="mt-8 flex w-full max-w-md flex-col gap-4 px-2 lg:hidden">
          {KEUNGGULAN.map(({ ikon: Ikon, judul, isi }) => (
            <li key={judul} className="flex gap-3">
              <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-kontrol bg-sorot text-hijau-800">
                <Ikon className="h-5 w-5" aria-hidden />
              </span>
              <div>
                <p className="text-label font-semibold text-teks-utama">{judul}</p>
                <p className="text-keterangan text-teks-sekunder">{isi}</p>
              </div>
            </li>
          ))}
        </ul>
      </main>
    </div>
  )
}

/** Keunggulan yang BENAR-BENAR ada di aplikasi — bukan janji pemasaran. */
const KEUNGGULAN: { ikon: LucideIcon; singkat: string; judul: string; isi: string }[] = [
  {
    ikon: WifiOff,
    singkat: 'Jalan tanpa internet',
    judul: 'Tetap jualan tanpa internet',
    isi: 'Transaksi tersimpan di HP dan terkirim sendiri begitu sinyal kembali.',
  },
  {
    ikon: TrendingUp,
    singkat: 'Untung terlihat tiap hari',
    judul: 'Untung hari ini langsung terlihat',
    isi: 'Uang masuk, untung kotor, dan jam teramai di beranda.',
  },
  {
    ikon: PackageCheck,
    singkat: 'Stok berkurang sendiri',
    judul: 'Stok berkurang sendiri',
    isi: 'Setiap barang terjual langsung memotong stok, bahan resep pun ikut.',
  },
]

/**
 * Panel merek (layar besar). Polanya dari halaman masuk SaaS kasir yang paling
 * meyakinkan — Pawoon, Square: PERLIHATKAN produknya, jangan menjelaskannya.
 * Versi sebelumnya berisi tiga paragraf keunggulan dan kartu tiruan; sekarang
 * keunggulan tinggal tiga label, dan sisanya etalase layar aplikasi sungguhan.
 */
function PanelMerek() {
  return (
    <aside className="permukaan-sorotan relative hidden overflow-hidden text-utama-teks lg:sticky lg:top-0 lg:flex lg:h-dvh lg:flex-col lg:px-12 lg:pt-12 xl:px-16 xl:pt-14">
      {/* Cincin hiasan. Garis saja, tanpa isian: tidak mengubah warna latar di
          balik teks mana pun. */}
      <div aria-hidden className="pointer-events-none absolute -right-24 -top-24 h-80 w-80 rounded-full border border-current opacity-15" />
      <div aria-hidden className="pointer-events-none absolute -right-8 -top-8 h-48 w-48 rounded-full border border-current opacity-15" />

      <MerekKasir varian="terbalik" className="relative" />

      <div className="relative mt-10 max-w-2xl xl:mt-12">
        {/* Judul besar TANPA tanda baca. Koma & titik Plus Jakarta Sans
            lebar (diukur: koma 15,5px pada 40px/800, ¾ lebar huruf "s"),
            sehingga di ukuran ini tampak terlepas dari katanya: "terus ,". */}
        {/* text-balance: tanpa ini "putus" tertinggal sendirian di baris kedua. */}
        <h2 className="text-balance text-angka font-extrabold">
          Jualan jalan terus walau sinyal putus
        </h2>
        <p className="mt-3 text-balance text-judul-kartu opacity-90">
          Kasir, stok, dan laporan untung warung Anda — di HP, tablet, atau komputer.
        </p>
        <ul className="mt-6 flex flex-wrap gap-2">
          {KEUNGGULAN.map(({ ikon: Ikon, singkat }) => (
            <li
              key={singkat}
              className="inline-flex items-center gap-2 rounded-full border border-current/30 px-3 py-1.5 text-label font-medium"
            >
              <Ikon className="h-4 w-4" aria-hidden />
              {singkat}
            </li>
          ))}
        </ul>
      </div>

      <EtalaseProduk />
    </aside>
  )
}

/**
 * Layar aplikasi SUNGGUHAN dalam bingkai tablet (kasir) dan HP (laporan).
 * Gambarnya dibuat ulang dari aplikasi yang berjalan dengan
 * `npm run foto:pratinjau` — jangan diganti gambar buatan tangan yang akan
 * basi diam-diam.
 *
 * Tablet sengaja LEBIH LEBAR dari panel dan menjulur ke bawah: terpotong di
 * tepi kanan & bawah, ia terbaca sebagai aplikasi yang sedang dipakai, bukan
 * gambar tempelan yang dibingkai rapi. Hiasan murni — disembunyikan dari
 * pembaca layar.
 */
function EtalaseProduk() {
  return (
    <div aria-hidden className="relative mt-10 min-h-0 flex-1 xl:mt-12">
      {/* max(46rem, 118%): di layar sedang tetap 46rem (terpotong di tepi);
          di layar lebar ikut membesar supaya tetap menjulur keluar panel —
          tablet yang muat utuh terbaca sebagai gambar tempelan. */}
      <div className="absolute left-0 top-0 w-[max(46rem,118%)] rounded-[1.75rem] bg-bingkai p-3 shadow-dialog">
        <img
          src="/gambar/pratinjau-kasir.jpg"
          alt=""
          width={1280}
          height={800}
          decoding="async"
          className="block h-auto w-full rounded-[1rem]"
        />
      </div>
      <div className="absolute right-6 top-10 w-40 rounded-[1.75rem] bg-bingkai p-2 shadow-dialog xl:right-12 xl:w-44 2xl:w-52">
        <img
          src="/gambar/pratinjau-laporan.jpg"
          alt=""
          width={390}
          height={844}
          decoding="async"
          className="block h-auto w-full rounded-[1.25rem]"
        />
      </div>
    </div>
  )
}
