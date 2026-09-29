import { formatRupiah } from '@/bersama/util/uang'
import { cn } from '@/bersama/util/cn'
import type { BarisLaporan } from '../api'

/**
 * "Jam berapa warung saya paling ramai?"
 *
 * Pertanyaan itu dijawab di atas grafik sebagai KALIMAT, bukan diserahkan ke
 * mata pembaca untuk menaksir batang mana yang tertinggi. Grafiknya menyusul
 * sebagai konteks — bentuk hari itu, kapan ramai mulai naik dan kapan reda.
 *
 * Dua puluh empat jam SELALU digambar penuh, termasuk jam-jam yang nol. Jam
 * sepi adalah informasi: warung yang tutup pukul 14.00 harus terlihat tutup,
 * bukan terlihat seperti data yang hilang.
 *
 * Ramai diukur dengan JUMLAH TRANSAKSI, bukan rupiah. "Ramai" bagi pemilik
 * warung berarti banyak orang datang — satu pembelian borongan Rp 2 juta pukul
 * 03.00 tidak membuat pukul 03.00 jadi jam sibuk yang perlu ditambah pegawai.
 *
 * Jamnya sudah dihitung server di zona waktu outlet, bukan UTC — lihat
 * repositories.SalesByHour. Di sini angka `key` "00".."23" dipakai apa adanya.
 */
export function GrafikJam({ rows }: { rows: BarisLaporan[] }) {
  const perJam = Array.from({ length: 24 }, (_, jam) => {
    const r = rows.find((x) => Number(x.key) === jam)
    return { jam, transaksi: r?.sales_count ?? 0, uang: r?.net_amount ?? 0 }
  })

  const maks = Math.max(...perJam.map((p) => p.transaksi), 1)
  const teramai = perJam.reduce((a, b) => (b.transaksi > a.transaksi ? b : a))
  const adaIsi = perJam.some((p) => p.transaksi > 0)

  return (
    <div className="flex flex-col gap-4">
      {adaIsi && (
        <div className="rounded-kartu bg-permukaan-2 p-4">
          <p className="text-label text-teks-sekunder">Paling ramai</p>
          <p className="mt-1 text-judul font-extrabold tabular-nums text-teks-utama">
            {rentangJam(teramai.jam)}
          </p>
          <p className="mt-1 text-keterangan text-teks-sekunder">
            {teramai.transaksi.toLocaleString('id-ID')} transaksi ·{' '}
            {formatRupiah(teramai.uang)}
          </p>
        </div>
      )}

      <div
        role="img"
        aria-label={
          adaIsi
            ? `Jumlah transaksi per jam. Paling ramai ${rentangJam(teramai.jam)} dengan ${teramai.transaksi} transaksi.`
            : 'Belum ada transaksi pada periode ini.'
        }
        className="flex h-40 items-end gap-[3px] sm:gap-1"
      >
        {perJam.map((p) => (
          <div
            key={p.jam}
            // title dibaca sebagai tooltip tetikus; pembaca layar memakai
            // aria-label ringkasan di atas, bukan 24 batang satu per satu.
            title={`${rentangJam(p.jam)} — ${p.transaksi} transaksi, ${formatRupiah(p.uang)}`}
            className="flex h-full flex-1 flex-col justify-end"
          >
            <div
              className={cn(
                'w-full rounded-t-kontrol transition-colors',
                // Yang teramai diberi warna penuh; sisanya meredup. Ini menandai
                // jawaban di dalam grafik, bukan sekadar menghias.
                p.transaksi === 0
                  ? 'bg-garis'
                  : p.jam === teramai.jam
                    ? 'bg-utama'
                    : 'bg-utama/35',
              )}
              // Jam nol tetap menyisakan garis tipis: terbaca sebagai "tidak ada
              // transaksi", bukan sebagai celah kosong di grafik.
              style={{ height: `${Math.max(2, (p.transaksi / maks) * 100)}%` }}
            />
          </div>
        ))}
      </div>

      {/* Sumbu jam: hanya tiap enam jam. Dua puluh empat label di layar HP
          bertumpuk jadi bubur dan tak satu pun terbaca. */}
      <div className="flex justify-between text-keterangan tabular-nums text-teks-redup">
        {[0, 6, 12, 18, 23].map((j) => (
          <span key={j}>{String(j).padStart(2, '0')}.00</span>
        ))}
      </div>
    </div>
  )
}

/** "07" → "07.00–07.59" — supaya tidak terbaca sebagai satu titik waktu. */
function rentangJam(jam: number): string {
  const dua = (n: number) => String(n).padStart(2, '0')
  return `${dua(jam)}.00–${dua(jam)}.59`
}
