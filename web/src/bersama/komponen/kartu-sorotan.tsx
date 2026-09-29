import { ArrowDown, ArrowUp, Minus } from 'lucide-react'
import { GrafikMini } from '@/bersama/komponen/grafik-mini'
import { formatRupiah, persenSelisih } from '@/bersama/util/uang'
import { cn } from '@/bersama/util/cn'

/**
 * Kartu sorotan: SATU angka terpenting di sebuah layar, di atas permukaan
 * berwarna penuh.
 *
 * Kenapa berwarna penuh. Beranda versi sebelumnya menaruh semua angka di kartu
 * putih yang seragam, sehingga tidak ada yang menonjol — dan diukur, warna
 * merek hanya meliputi 0–1,3% layar. Akibatnya dua: aplikasinya tidak terasa
 * punya identitas, dan yang paling berwarna di layar justru kabar buruk (merah
 * "turun 67%", jingga "habis" lima baris).
 *
 * Kedalamannya datang dari `permukaan-sorotan` — gradien antara dua token hijau
 * yang rasio kontrasnya sudah diverifikasi, jadi tidak ada pasangan warna baru
 * yang perlu diukur. Bayangan `melayang`, bukan `kartu`: ini satu-satunya
 * elemen yang boleh terasa terangkat dari halaman.
 *
 * Beda dari KartuAngka: kartu itu untuk angka pendamping yang berjajar setara.
 * Yang ini HANYA SATU per layar — kalau dua angka sama-sama disorot, tidak ada
 * yang tersorot.
 */
export function KartuSorotan({
  label,
  nilai,
  pembanding,
  labelPembanding = 'dibanding kemarin',
  riwayat,
  labelRiwayat,
  uang = true,
  aksi,
  rincian,
  nada = 'utama',
  className,
}: {
  label: string
  nilai: number
  pembanding?: number
  labelPembanding?: string
  /** Nilai beberapa hari terakhir untuk grafik. Hari ini paling belakang. */
  riwayat?: number[]
  labelRiwayat?: string
  uang?: boolean
  /** Elemen kecil di pojok kanan atas, mis. tautan "Laporan". */
  aksi?: React.ReactNode
  /** Angka pendukung di kaki kartu, dipisah garis. Maksimal tiga. */
  rincian?: { label: string; nilai: string }[]
  /**
   * `utama` — hijau penuh, untuk angka yang layak dirayakan.
   * `bahaya` — permukaan biasa dengan angka merah, untuk RUGI.
   *
   * Kenapa bukan sekadar mewarnai angkanya merah di atas hijau: di latar hijau
   * penuh, merah tidak akan pernah lolos 4,5:1. Dan lebih penting dari
   * rasionya — kerugian yang ditampilkan di atas kartu perayaan berwarna
   * "berhasil" memberi tahu pemilik warung hal yang salah. Saat beritanya
   * buruk, kartunya mundur jadi permukaan biasa.
   */
  nada?: 'utama' | 'bahaya'
  className?: string
}) {
  const persen = pembanding === undefined ? null : persenSelisih(nilai, pembanding)
  const naik = persen !== null && persen > 0
  const turun = persen !== null && persen < 0
  const Panah = naik ? ArrowUp : turun ? ArrowDown : Minus

  const merugi = nada === 'bahaya'

  return (
    <div
      className={cn(
        'flex flex-col rounded-kartu p-5 shadow-melayang sm:p-6',
        merugi
          ? 'border border-garis bg-permukaan text-teks-utama'
          : 'permukaan-sorotan text-utama-teks',
        className,
      )}
    >
      <div className="flex items-start justify-between gap-3">
        {/* Label kecil berhuruf besar: ia penanda, bukan kalimat. Angkanya yang
            dibaca, dan huruf besar berjarak membuat label tidak bersaing
            dengannya. */}
        <p
          className={cn(
            'text-keterangan font-semibold uppercase tracking-wider',
            merugi ? 'text-teks-sekunder' : 'opacity-90',
          )}
        >
          {label}
        </p>
        {aksi}
      </div>

      {/* Angka di kiri, grafik di kanan — pola stat+grafik yang lazim di dasbor.
          Grafiknya SENGAJA dibatasi lebarnya: dibiarkan selebar kartu, tujuh
          batang di atas seribu piksel menjadi balok selebar 140px yang terbaca
          sebagai hiasan, bukan sebagai grafik. */}
      {/* Di HP grafik turun ke bawah angka: disandingkan, ia menyisakan kurang
          dari 210px untuk angka 40px dan "Rp 324.000" pecah jadi dua baris. */}
      <div className="mt-2 flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between sm:gap-6">
        <div className="min-w-0">
          <p
            className={cn(
              'text-angka font-extrabold tabular-nums',
              merugi && 'text-bahaya-teks',
            )}
          >
            {uang ? formatRupiah(nilai) : nilai.toLocaleString('id-ID')}
          </p>

          {/* Lencana bergaris, bukan berisi: garis tidak mengubah warna latar di
              baliknya, jadi rasio teksnya tetap yang sudah diverifikasi. Arah
              perubahan ditandai PANAH — merah "turun" mustahil dibuat terbaca di
              atas hijau penuh, dan memang tidak perlu: yang jadi berita adalah
              angka di atasnya, bukan alarmnya. */}
          {persen !== null && (
            <span
              className={cn(
                'mt-2 inline-flex items-center gap-1 rounded-full px-2.5 py-1',
                'text-keterangan font-semibold',
                merugi
                  ? 'bg-permukaan-2 text-bahaya-teks'
                  : 'border border-current/30',
              )}
            >
              <Panah className="h-3.5 w-3.5" aria-hidden />
              {Math.abs(persen)}% {labelPembanding}
            </span>
          )}
        </div>

        {riwayat && riwayat.length > 1 && (
          <GrafikMini
            nilai={riwayat}
            label={labelRiwayat ?? 'Beberapa hari terakhir'}
            className="h-12 w-full max-w-[200px] shrink-0 sm:h-14 sm:w-44 sm:max-w-none"
          />
        )}
      </div>

      {rincian && rincian.length > 0 && (
        <div
          className={cn(
            'mt-5 grid gap-4 border-t border-current/20 pt-4',
            rincian.length >= 3 ? 'grid-cols-3' : 'grid-cols-2',
          )}
        >
          {rincian.map((r) => (
            <div key={r.label} className="min-w-0">
              <p className="truncate text-keterangan opacity-90">{r.label}</p>
              <p className="mt-0.5 truncate text-judul-kartu font-bold tabular-nums">
                {r.nilai}
              </p>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
