import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ulid } from 'ulid'
import { Check, Clock, CreditCard, Lock, Minus, Sparkles } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KerangkaBaris, KerangkaKartuAngka } from '@/bersama/komponen/kerangka'
import { KeadaanGagal } from '@/bersama/komponen/keadaan-kosong'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { formatRupiah } from '@/bersama/util/uang'
import { formatTanggal } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import {
  langgananApi,
  type InfoBayar,
  type Langganan,
  type Paket,
  type Pengembalian,
  type TagihanLangganan,
} from '../api'

/**
 * Langganan aplikasi.
 *
 * Yang perlu diketahui pemilik cuma dua: sampai kapan aktif, dan ada tagihan
 * yang harus dibayar atau tidak. Keduanya di layar pertama; katalog paket
 * menyusul di bawah.
 */
export function HalamanLangganan() {
  const [bayarUntuk, setBayarUntuk] = useState<TagihanLangganan | null>(null)
  const [berhenti, setBerhenti] = useState(false)

  const ringkasan = useQuery({
    queryKey: ['langganan'],
    queryFn: langgananApi.ringkasan,
    retry: false,
  })

  const paket = useQuery({ queryKey: ['paket'], queryFn: langgananApi.paket })
  const tagihan = useQuery({ queryKey: ['tagihan'], queryFn: langgananApi.tagihan })

  const langganan = ringkasan.data?.subscription
  const tagihanTerbuka = ringkasan.data?.open_invoice
  // Konfirmasi terbaru untuk tagihan terbuka: menunggu verifikasi atau ditolak.
  const konfirmasi = ringkasan.data?.payment_claim
  const pengembalian = ringkasan.data?.refund
  // 404 = memang belum pernah berlangganan (paket Gratis). Galat LAIN jangan
  // disamarkan sebagai "Anda memakai paket Gratis" — pemilik yang baru
  // membayar akan mengira pembayarannya hilang.
  const belumBerlangganan =
    (ringkasan.error instanceof GalatAPI && ringkasan.error.status === 404) ||
    (ringkasan.isSuccess && !langganan)
  // Langganan yang sudah dihentikan, atau data lama "masa coba paket Gratis":
  // kartu atas menampilkan paket Gratis apa adanya — bukan lagi masa coba,
  // tanggal berlaku, atau tombol bayar (dulu itulah yang tetap muncul setelah
  // memilih Gratis di tengah masa coba).
  const langgananGratis =
    !!langganan && (paket.data?.find((p) => p.code === langganan.plan_code)?.monthly_price ?? 1) <= 0
  const sudahBerhenti =
    !!langganan &&
    (langganan.status === 'canceled' || langganan.status === 'expired' || langgananGratis)
  // Masih ada yang berjalan (masa coba / berbayar) → kartu Gratis menawarkan
  // "Pindah ke Gratis" (= menghentikannya, dengan pengembalian bila ada).
  const langgananHidup = !!langganan && !sudahBerhenti

  // Paket yang BERLAKU menurut server (GET /me) — bisa berbeda dari paket
  // yang dipilih: masa coba yang habis, atau masa bayar yang lewat tenggang,
  // membuat tenant kembali ke Gratis walau langganannya masih tercatat.
  const { profil } = useSesi()
  const berlaku = profil?.plan
  const qc = useQueryClient()
  const toast = useToast()

  const batalkanGanti = useMutation({
    mutationFn: langgananApi.batalkanGanti,
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['langganan'] })
      qc.invalidateQueries({ queryKey: ['tagihan'] })
      toast.berhasil('Pindah paket dibatalkan. Paket Anda tidak berubah.')
    },
    onError: (e) => toast.gagal(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  // "Bayar sekarang": pakai tagihan terbuka bila ada, kalau belum ada
  // terbitkan dulu — lalu buka dialog bayar.
  const bayarSekarang = useMutation({
    mutationFn: async () => tagihanTerbuka ?? (await langgananApi.buatTagihan()),
    onSuccess: (tagihanBaru) => {
      qc.invalidateQueries({ queryKey: ['langganan'] })
      qc.invalidateQueries({ queryKey: ['tagihan'] })
      setBayarUntuk(tagihanBaru)
    },
    onError: (e) => toast.gagal(e instanceof GalatAPI ? e.pesan : 'Tagihan belum bisa dibuat.'),
  })

  // Semua kunci fitur di katalog, dalam urutan tetap — setiap kartu menulis
  // baris yang SAMA (termasuk yang tidak ada di paketnya), supaya paket bisa
  // dibandingkan baris demi baris.
  const kunciFitur = urutkanFitur(paket.data ?? [])

  return (
    <div className="flex w-full flex-col gap-4">
      <h1 className="text-judul font-bold text-teks-utama">Langganan</h1>

      {/* Status & tagihan tetap sempit (dibaca, bukan dibandingkan); katalog
          paket di bawahnya memakai lebar penuh. */}
      <div className="flex w-full max-w-2xl flex-col gap-4">

        {ringkasan.isLoading ? (
          <KerangkaKartuAngka />
        ) : belumBerlangganan || sudahBerhenti ? (
          <Kartu className="flex flex-col items-center gap-2 p-6 text-center">
            <Sparkles className="h-10 w-10 text-jingga-600" aria-hidden />
            <p className="text-judul-kartu font-semibold text-teks-utama">
              Anda memakai paket {berlaku?.name ?? 'Gratis'}
            </p>
            <p className="text-isi text-teks-sekunder">
              Gratis selamanya, tanpa batas barang maupun transaksi. Yang terkunci
              hanya QRIS, kanal online, CRM, sales lapangan, dan cabang tambahan.
              {langganan?.trial_ends_at && new Date(langganan.trial_ends_at) > new Date()
                ? ` Sisa masa coba Anda sampai ${formatTanggal(langganan.trial_ends_at)} bisa dilanjutkan dengan memilih paket berbayar.`
                : ' Pilih paket berbayar di bawah kapan saja.'}
            </p>
            {pengembalian && (
              <div className="w-full text-left">
                <StatusPengembalian pengembalian={pengembalian} />
              </div>
            )}
          </Kartu>
        ) : !langganan ? (
          <KeadaanGagal
            pesan={
              ringkasan.error instanceof GalatAPI
                ? ringkasan.error.pesan
                : 'Status langganan belum bisa dimuat.'
            }
            onCobaLagi={() => ringkasan.refetch()}
          />
        ) : berlaku && berlaku.code !== langganan.plan_code ? (
          // Langganan tercatat tapi TIDAK berlaku lagi (masa coba habis atau
          // lewat tenggang): katakan terus terang, dan beri jalan kembalinya di
          // tempat yang sama — bayar, atau tetap di Gratis.
          <Kartu className="flex flex-col gap-3 border-jingga-600 bg-permukaan-2 p-4">
            <div className="flex items-start gap-3">
              <Lock className="mt-0.5 h-5 w-5 shrink-0 text-jingga-700" aria-hidden />
              <div>
                <p className="font-semibold text-teks-utama">
                  {langganan.status === 'trial'
                    ? `Masa coba paket ${langganan.plan_name} sudah berakhir`
                    : `Masa langganan paket ${langganan.plan_name} sudah habis`}
                </p>
                <p className="text-label text-teks-sekunder">
                  Sekarang memakai paket {berlaku.name} — fitur berbayar terkunci sampai
                  tagihannya dibayar. Penjualan tetap berjalan seperti biasa.
                </p>
              </div>
            </div>
            <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
              {!tagihanTerbuka && (
                <Tombol memuat={bayarSekarang.isPending} onClick={() => bayarSekarang.mutate()}>
                  <CreditCard className="h-5 w-5" aria-hidden />
                  Bayar paket {langganan.plan_name}
                </Tombol>
              )}
              <button
                type="button"
                onClick={() => setBerhenti(true)}
                className="text-label font-medium text-teks-sekunder underline underline-offset-4 hover:text-teks-utama focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-utama"
              >
                Tetap pakai Gratis
              </button>
            </div>
          </Kartu>
        ) : (
          <Kartu className="p-4">
            <div className="flex items-start justify-between gap-3">
              <div>
                <p className="text-label font-medium text-teks-sekunder">Paket Anda</p>
                <p className="text-judul font-bold text-teks-utama">
                  {langganan.plan_name}
                </p>
              </div>
              <StatusLangganan status={langganan.status} />
            </div>
            <p className="mt-2 text-isi text-teks-sekunder">
              Aktif sampai{' '}
              <strong className="text-teks-utama">
                {formatTanggal(langganan.current_period_end)}
              </strong>
              {langganan.auto_renew
                ? ' · diperpanjang otomatis'
                : ' · tidak diperpanjang otomatis'}
            </p>
            {langganan.trial_ends_at && langganan.status === 'trial' && (
              <p className="mt-1 text-keterangan text-teks-redup">
                Masa coba gratis berakhir {formatTanggal(langganan.trial_ends_at)}. Bayar
                kapan saja sebelum itu supaya fitur tidak terkunci — masa berbayar baru
                dimulai setelah masa coba berakhir, jadi sisa masa coba tidak hangus.
              </p>
            )}
            {/* Dibayar di tengah masa coba: masa berbayarnya belum mulai. */}
            {langganan.status === 'active' &&
              new Date(langganan.current_period_start) > new Date() && (
                <p className="mt-1 text-keterangan text-teks-redup">
                  Sudah dibayar. Masa berbayar mulai{' '}
                  {formatTanggal(langganan.current_period_start)} — sampai itu sisa masa coba
                  Anda tetap berjalan.
                </p>
              )}
            {/* Dalam tenggang: masa bayar sudah lewat, fitur masih jalan. */}
            {langganan.status !== 'trial' &&
              berlaku?.active_until &&
              new Date(langganan.current_period_end) < new Date() && (
                <p className="mt-1 text-keterangan font-medium text-jingga-700">
                  Masa langganan sudah habis. Fitur tetap aktif sampai{' '}
                  {formatTanggal(berlaku.active_until)} — bayar sebelum itu.
                </p>
              )}
            {!tagihanTerbuka &&
              (langganan.status === 'trial' ||
                new Date(langganan.current_period_end) < new Date()) && (
                <Tombol
                  className="mt-3"
                  jenis="kedua"
                  memuat={bayarSekarang.isPending}
                  onClick={() => bayarSekarang.mutate()}
                >
                  <CreditCard className="h-5 w-5" aria-hidden />
                  Bayar sekarang
                </Tombol>
              )}
            {/* Sengaja kecil dan di pojok: jalan keluar harus ADA dan jujur,
                tapi bukan tombol yang ditekan tidak sengaja. */}
            <button
              type="button"
              onClick={() => setBerhenti(true)}
              className="mt-3 self-start text-label font-medium text-teks-sekunder underline underline-offset-4 hover:text-bahaya-teks focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-utama"
            >
              Berhenti berlangganan
            </button>
          </Kartu>
        )}

        {tagihanTerbuka && (
          <Kartu className="border-jingga-600 bg-permukaan-2 p-4">
            <p className="font-semibold text-jingga-700">
              {tagihanTerbuka.kind === 'plan_change'
                ? `Tagihan pindah ke paket ${tagihanTerbuka.plan_name ?? ''}`
                : 'Ada tagihan yang belum dibayar'}
            </p>
            <p className="mt-1 text-angka font-extrabold tabular-nums text-teks-utama">
              {formatRupiah(tagihanTerbuka.total_amount - tagihanTerbuka.paid_amount)}
            </p>
            <p className="mt-1 text-keterangan text-jingga-700">
              Nomor {tagihanTerbuka.number} · untuk{' '}
              {formatTanggal(tagihanTerbuka.period_start)} –{' '}
              {formatTanggal(tagihanTerbuka.period_end)} · jatuh tempo{' '}
              {formatTanggal(tagihanTerbuka.due_date)}
            </p>
            {tagihanTerbuka.kind === 'plan_change' && (
              // Paket BARU berpindah saat lunas — katakan, supaya pemilik tidak
              // mengira fitur paket barunya rusak.
              <p className="mt-1 text-label text-teks-sekunder">
                Paket berpindah setelah pembayarannya diverifikasi; sampai itu paket{' '}
                {langganan?.plan_name} tetap berjalan.
                {tagihanTerbuka.credit_amount > 0 &&
                  ` Sudah dipotong sisa masa paket lama ${formatRupiah(tagihanTerbuka.credit_amount)}.`}
              </p>
            )}
            {konfirmasi?.status === 'pending' ? (
              // Sudah dikonfirmasi: jangan tawarkan membayar lagi — tombol
              // "Bayar" di sini mengundang transfer dua kali.
              <div className="mt-3 flex items-start gap-2 rounded-kontrol border border-garis bg-permukaan px-3 py-2">
                <Clock className="mt-0.5 h-4 w-4 shrink-0 text-jingga-700" aria-hidden />
                <p className="text-label text-teks-sekunder">
                  <strong className="text-teks-utama">Menunggu verifikasi.</strong> Konfirmasi{' '}
                  {formatRupiah(konfirmasi.amount)} ({namaCara(konfirmasi.method)}) terkirim{' '}
                  {formatTanggal(konfirmasi.created_at)}. Paket aktif setelah pembayarannya kami
                  verifikasi.
                </p>
              </div>
            ) : (
              <>
                {konfirmasi?.status === 'rejected' && (
                  <p className="mt-3 rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
                    Konfirmasi sebelumnya belum bisa diterima: {konfirmasi.reject_reason}
                  </p>
                )}
                <div className="mt-3 flex flex-wrap gap-2">
                  <Tombol onClick={() => setBayarUntuk(tagihanTerbuka)}>
                    <CreditCard className="h-5 w-5" aria-hidden />
                    {konfirmasi?.status === 'rejected' ? 'Kirim Konfirmasi Baru' : 'Bayar Sekarang'}
                  </Tombol>
                  {tagihanTerbuka.kind === 'plan_change' && (
                    <Tombol
                      jenis="kedua"
                      memuat={batalkanGanti.isPending}
                      onClick={() => batalkanGanti.mutate(tagihanTerbuka.id)}
                    >
                      Batalkan Pindah Paket
                    </Tombol>
                  )}
                </div>
              </>
            )}
          </Kartu>
        )}

      </div>

      <section className="flex flex-col gap-2">
        <h2 className="text-judul-kartu font-semibold text-teks-utama">Pilihan paket</h2>
        {paket.isLoading ? (
          <KerangkaBaris jumlah={3} />
        ) : (
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            {paket.data?.map((p) => (
              <KartuPaket
                key={p.code}
                paket={p}
                kunciFitur={kunciFitur}
                // "Dipakai" = paket yang BERLAKU, bukan sekadar yang tercatat.
                sedangDipakai={p.code === (berlaku?.code ?? langganan?.plan_code)}
                // Ganti paket (prorata) hanya untuk langganan berbayar yang
                // berjalan; masa coba & yang belum/berhenti memakai "mulai".
                sudahBerlangganan={
                  langganan?.status === 'active' || langganan?.status === 'past_due'
                }
                // Tagihan pindah ke paket ini sudah terbit tapi belum lunas.
                menungguBayar={
                  tagihanTerbuka?.kind === 'plan_change' && tagihanTerbuka.plan_code === p.code
                }
                onPindahGratis={langgananHidup ? () => setBerhenti(true) : undefined}
              />
            ))}
          </div>
        )}
      </section>

      {(tagihan.data?.data.length ?? 0) > 0 && (
        <section className="flex w-full max-w-2xl flex-col gap-2">
          <h2 className="text-judul-kartu font-semibold text-teks-utama">
            Riwayat tagihan
          </h2>
          <Kartu className="divide-y divide-garis">
            {tagihan.data?.data.map((t) => (
              <div key={t.id} className="flex items-center justify-between gap-3 p-4">
                <div className="min-w-0">
                  <p className="font-medium tabular-nums text-teks-utama">{t.number}</p>
                  <p className="text-keterangan text-teks-redup">
                    {t.plan_name && `${t.plan_name}${t.kind === 'plan_change' ? ' (pindah paket)' : ''} · `}
                    {formatTanggal(t.period_start)} – {formatTanggal(t.period_end)}
                  </p>
                </div>
                <div className="flex shrink-0 items-center gap-3">
                  <p className="font-bold tabular-nums text-teks-utama">
                    {formatRupiah(t.total_amount)}
                  </p>
                  {t.status === 'paid' ? (
                    <LencanaStatus nada="berhasil" anak="Lunas" />
                  ) : t.status === 'void' ? (
                    <LencanaStatus nada="netral" anak="Dibatalkan" />
                  ) : t.status === 'refunded' ? (
                    <LencanaStatus nada="netral" anak="Dikembalikan" />
                  ) : (
                    <LencanaStatus nada="menunggu" anak="Belum lunas" />
                  )}
                </div>
              </div>
            ))}
          </Kartu>
        </section>
      )}

      {berhenti && langganan && (
        <DialogBerhenti langganan={langganan} onTutup={() => setBerhenti(false)} />
      )}

      {bayarUntuk && (
        <DialogKonfirmasi
          tagihan={bayarUntuk}
          info={ringkasan.data?.payment_instructions}
          onTutup={() => setBayarUntuk(null)}
        />
      )}
    </div>
  )
}

/**
 * Nama fitur paket dalam bahasa pemilik warung. Kunci yang belum dikenal di
 * sini tetap tampil (dari kuncinya sendiri) — katalog diubah admin platform
 * tanpa rilis aplikasi, dan fitur baru tidak boleh hilang dari kartu.
 */
const NAMA_FITUR: Record<string, string> = {
  qris: 'Terima pembayaran QRIS',
  online_channel: 'Pesanan dari kanal online',
  crm_freelance: 'CRM: penawaran, proyek & invoice',
  crm_sales: 'Sales lapangan: kunjungan, target & komisi',
  multi_outlet: 'Kelola banyak cabang',
}

/** Kalimat pendek di bawah nama paket — untuk siapa paket itu. */
const UNTUK_SIAPA: Record<string, string> = {
  free: 'Gratis selamanya untuk usaha yang baru mulai',
  basic: 'Untuk warung dan toko satu kasir',
  pro: 'Untuk toko yang juga berjualan online',
  multi: 'Untuk usaha dengan beberapa cabang',
}

function fiturDari(p: Paket): Record<string, boolean> {
  const f = p.features
  return f && typeof f === 'object' ? (f as Record<string, boolean>) : {}
}

function namaFitur(kunci: string): string {
  return NAMA_FITUR[kunci] ?? kunci.replace(/_/g, ' ')
}

/**
 * Kunci fitur seluruh katalog: yang dikenal dulu (urutan NAMA_FITUR), sisanya
 * menyusul menurut abjad.
 */
function urutkanFitur(daftar: Paket[]): string[] {
  const semua = new Set(daftar.flatMap((p) => Object.keys(fiturDari(p))))
  const dikenal = Object.keys(NAMA_FITUR).filter((k) => semua.has(k))
  const lain = [...semua].filter((k) => !(k in NAMA_FITUR)).sort()
  return [...dikenal, ...lain]
}

/**
 * Satu kartu paket.
 *
 * Dulu setiap kartu hanya menulis batas pemakaian — dan karena semua paket
 * katalog "tanpa batas", keempat kartu berisi empat baris yang PERSIS sama:
 * tidak ada alasan memilih paket yang lebih mahal. Pembeda sebenarnya ada di
 * `features` (QRIS, kanal online, CRM, banyak cabang), yang tidak pernah
 * ditampilkan. Kini setiap kartu menulis semua fitur katalog: ✓ bila
 * termasuk, redup bila tidak.
 */
function KartuPaket({
  paket,
  kunciFitur,
  sedangDipakai,
  sudahBerlangganan,
  menungguBayar,
  onPindahGratis,
}: {
  paket: Paket
  kunciFitur: string[]
  sedangDipakai: boolean
  sudahBerlangganan: boolean
  menungguBayar: boolean
  /** Ada masa coba / langganan berjalan: pindah ke Gratis = menghentikannya. */
  onPindahGratis?: () => void
}) {
  const toast = useToast()
  const qc = useQueryClient()
  const [masa, setMasa] = useState(paket.term_prices[0]?.term_months ?? 1)
  const [galat, setGalat] = useState<string | null>(null)

  const harga = paket.term_prices.find((h) => h.term_months === masa)

  // Mulai berlangganan dan ganti paket mengembalikan bentuk berbeda (langganan
  // vs tagihan prorata); layar ini hanya perlu tahu keduanya berhasil, lalu
  // membaca ulang ringkasannya.
  // Hasilnya dijadikan satu kalimat yang BENAR tentang apa yang terjadi.
  // Dulu "mulai" selalu diumumkan "Paket X aktif. Tagihannya sudah dibuat." —
  // padahal memulai hanya membuka masa coba, tanpa tagihan apa pun.
  const ambil = useMutation<string>({
    mutationFn: async () => {
      if (sudahBerlangganan) {
        // Paket BARU berpindah saat tagihannya lunas — kecuali biayanya habis
        // tertutup sisa masa paket lama (langsung lunas).
        const t = await langgananApi.gantiPaket(paket.code, masa)
        return t.status === 'paid'
          ? `Pindah ke paket ${paket.name}. Biayanya tertutup sisa masa paket lama.`
          : `Tagihan pindah ke paket ${paket.name} sudah dibuat. Paket berpindah setelah dibayar.`
      }
      const l = await langgananApi.mulai(paket.code, masa)
      const akhir = l.trial_ends_at ? new Date(l.trial_ends_at) : null
      return akhir && akhir > new Date()
        ? `Masa coba paket ${paket.name} berjalan sampai ${formatTanggal(l.trial_ends_at!)}.`
        : `Paket ${paket.name} dipilih. Bayar tagihannya untuk membuka fiturnya.`
    },
    onSuccess: (pesan) => {
      qc.invalidateQueries({ queryKey: ['langganan'] })
      qc.invalidateQueries({ queryKey: ['tagihan'] })
      // Profil membawa paket yang berlaku — tanpa ini gembok di menu & QRIS
      // di kasir baru berubah setelah aplikasi dimuat ulang.
      qc.invalidateQueries({ queryKey: ['me'] })
      toast.berhasil(pesan)
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  const fitur = fiturDari(paket)
  const batas = [
    { label: 'toko', nilai: paket.max_outlets },
    { label: 'pengguna', nilai: paket.max_users },
    { label: 'barang', nilai: paket.max_products },
    { label: 'transaksi per bulan', nilai: paket.max_monthly_transactions },
  ]
  const semuaTanpaBatas = batas.every((b) => b.nilai === null)

  return (
    <Kartu
      className={cn(
        'flex flex-col gap-4 p-5',
        sedangDipakai && 'border-2 border-utama bg-sorot',
      )}
    >
      <div>
        <div className="flex items-center justify-between gap-2">
          <p className="text-judul-kartu font-bold text-teks-utama">{paket.name}</p>
          {sedangDipakai && <LencanaStatus nada="berhasil" anak="Dipakai" />}
        </div>
        {/* Tinggi dua baris tetap SAAT BERSEBELAHAN (≥640px): kalimat yang
            pendek dan yang membungkus membuat harga di kartu-kartu sebelahnya
            tidak sejajar. Di HP kartunya bertumpuk, jadi tidak perlu. */}
        <p className="mt-0.5 text-keterangan text-teks-sekunder sm:min-h-9">
          {UNTUK_SIAPA[paket.code] ?? ''}
        </p>
      </div>

      <p className="flex items-baseline gap-1">
        <span className="text-judul font-extrabold tabular-nums text-teks-utama">
          {formatRupiah(paket.monthly_price)}
        </span>
        <span className="text-label text-teks-sekunder">/bulan</span>
      </p>

      <ul className="flex flex-col gap-2 border-t border-garis pt-4 text-label">
        {kunciFitur.map((k) =>
          fitur[k] ? (
            <li key={k} className="flex items-start gap-2 text-teks-utama">
              <Check className="mt-0.5 h-4 w-4 shrink-0 text-hijau-700" aria-hidden />
              {namaFitur(k)}
            </li>
          ) : (
            <li key={k} className="flex items-start gap-2 text-teks-redup">
              <Minus className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
              <span>
                <span className="sr-only">Tidak termasuk: </span>
                {namaFitur(k)}
              </span>
            </li>
          ),
        )}
        {/* Batas pemakaian: satu baris bila semuanya tanpa batas — empat baris
            "tanpa batas" yang sama di setiap kartu hanya menenggelamkan
            pembeda yang sebenarnya. */}
        {semuaTanpaBatas ? (
          <li className="flex items-start gap-2 text-teks-utama">
            <Check className="mt-0.5 h-4 w-4 shrink-0 text-hijau-700" aria-hidden />
            Barang, pengguna & transaksi tanpa batas
          </li>
        ) : (
          batas.map((b) => <Batas key={b.label} label={b.label} nilai={b.nilai} />)
        )}
      </ul>

      <div className="mt-auto flex flex-col gap-3">

        {/* Paket gratis tidak punya masa bayar yang perlu dipilih. */}
        {paket.monthly_price > 0 && paket.term_prices.length > 1 && (
          <Pilihan
            label="Masa langganan"
            value={String(masa)}
            onChange={(e) => setMasa(Number(e.target.value))}
          >
            {paket.term_prices.map((h) => (
              <option key={h.term_months} value={h.term_months}>
                {h.term_months} bulan
                {Number(h.discount_rate) > 0 &&
                  ` — hemat ${Math.round(Number(h.discount_rate) * 100)}%`}
              </option>
            ))}
          </Pilihan>
        )}

        {harga && paket.monthly_price > 0 && (
          <p className="text-isi font-bold tabular-nums text-teks-utama">
            <span className="mr-1 text-label font-normal text-teks-sekunder">Total</span>
            {formatRupiah(harga.total_amount)}
            {harga.discount_amount > 0 && (
              <span className="ml-2 text-keterangan font-normal text-hijau-700">
                hemat {formatRupiah(harga.discount_amount)}
              </span>
            )}
          </p>
        )}

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-keterangan text-bahaya-teks">
            {galat}
          </p>
        )}

        {sedangDipakai ? (
          <p className="flex items-center gap-1.5 text-label font-medium text-hijau-700">
            <Check className="h-5 w-5" aria-hidden />
            Paket yang sedang dipakai
          </p>
        ) : menungguBayar ? (
          <p className="flex items-center gap-1.5 text-label font-medium text-jingga-700">
            <Clock className="h-5 w-5" aria-hidden />
            Menunggu pembayaran — lihat tagihan di atas
          </p>
        ) : paket.monthly_price <= 0 ? (
          // Gratis tidak "dibeli": memilihnya = menghentikan masa coba atau
          // langganan yang berjalan (lewat dialog berhenti, yang juga mengurus
          // pengembalian dana). Dulu tombol ini memulai "masa coba paket
          // Gratis" dan tagihan Rp0 yang tidak bisa dibayar.
          onPindahGratis ? (
            <Tombol jenis="kedua" onClick={onPindahGratis}>
              Pindah ke Gratis
            </Tombol>
          ) : (
            <p className="text-label text-teks-sekunder">
              Toko kembali ke paket ini dengan sendirinya bila masa coba atau langganan tidak
              dibayar.
            </p>
          )
        ) : (
          <Tombol
            jenis={sudahBerlangganan ? 'kedua' : 'utama'}
            memuat={ambil.isPending}
            onClick={() => {
              setGalat(null)
              ambil.mutate()
            }}
          >
            {sudahBerlangganan ? 'Pindah ke Paket Ini' : 'Pilih Paket Ini'}
          </Tombol>
        )}
      </div>
    </Kartu>
  )
}

function Batas({ label, nilai }: { label: string; nilai: number | null }) {
  return (
    <li className="flex items-start gap-2 text-teks-utama">
      <Check className="mt-0.5 h-4 w-4 shrink-0 text-hijau-700" aria-hidden />
      {/* null berarti tanpa batas — dikatakan dengan kata, bukan tanda "∞". */}
      {nilai === null ? `${label} tanpa batas` : `${nilai} ${label}`}
    </li>
  )
}

/** Nama cara bayar dalam bahasa pemilik warung. */
function namaCara(m: string): string {
  return ({ transfer: 'transfer bank', qris: 'QRIS', ewallet: 'dompet digital', card: 'kartu', cash: 'tunai' } as Record<string, string>)[m] ?? m
}

/**
 * Konfirmasi pembayaran tagihan langganan.
 *
 * Pemilik membayar di luar aplikasi (transfer/QRIS), lalu MENGONFIRMASI di
 * sini: lewat apa, dan nama pengirim / nomor referensi yang akan dicocokkan
 * staf keuangan dengan mutasi rekening. Paket baru aktif setelah
 * diverifikasi. Dulu tombol di dialog ini ("Bayar Rp …") langsung menandai
 * tagihan lunas dan mengaktifkan paket — tanpa uang yang diterima siapa pun.
 */
function DialogKonfirmasi({
  tagihan,
  info,
  onTutup,
}: {
  tagihan: TagihanLangganan
  info?: InfoBayar
  onTutup: () => void
}) {
  const toast = useToast()
  const qc = useQueryClient()
  const [cara, setCara] = useState('transfer')
  const [referensi, setReferensi] = useState('')
  const [catatan, setCatatan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)
  const [galatReferensi, setGalatReferensi] = useState<string | undefined>()

  // Dibuat sekali saat dialog terbuka dan dipertahankan selama percobaan —
  // kiriman ulang tidak boleh membuat dua konfirmasi.
  const kunci = useRef(ulid())
  const sisa = tagihan.total_amount - tagihan.paid_amount

  const kirim = useMutation({
    mutationFn: () =>
      langgananApi.konfirmasi(
        {
          invoice_id: tagihan.id,
          amount: sisa,
          method: cara,
          reference: referensi.trim(),
          note: catatan.trim() || undefined,
        },
        kunci.current,
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['langganan'] })
      toast.berhasil('Konfirmasi terkirim. Paket aktif setelah pembayarannya kami verifikasi.')
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  function salin(teks: string) {
    navigator.clipboard?.writeText(teks).then(
      () => toast.berhasil('Nomor rekening disalin.'),
      () => {},
    )
  }

  return (
    <Dialog open onOpenChange={(o) => !o && !kirim.isPending && onTutup()}>
      <IsiDialog judul={`Konfirmasi pembayaran ${tagihan.number}`}>
        <div className="flex items-baseline justify-between border-b border-garis pb-3">
          <span className="text-isi text-teks-sekunder">Jumlah yang dibayar</span>
          <span className="text-judul font-extrabold tabular-nums text-teks-utama">
            {formatRupiah(sisa)}
          </span>
        </div>

        {info ? (
          <div className="rounded-kontrol border border-garis bg-permukaan-2 p-3 text-label text-teks-sekunder">
            <p>Transfer ke:</p>
            <div className="mt-1 flex items-center justify-between gap-3">
              <p className="font-semibold text-teks-utama">
                {info.bank_name} · <span className="tabular-nums">{info.account_number}</span>
              </p>
              <Tombol jenis="teks" ukuran="padat" onClick={() => salin(info.account_number)}>
                Salin
              </Tombol>
            </div>
            {info.account_holder && <p>a.n. {info.account_holder}</p>}
            <p className="mt-1">
              Tulis <strong className="text-teks-utama">{tagihan.number}</strong> di berita transfer.
            </p>
          </div>
        ) : (
          <p className="rounded-kontrol border border-garis bg-permukaan-2 p-3 text-label text-teks-sekunder">
            {/* Rekening belum diatur di server (SUBSCRIPTION_BANK_*). Pesan
                WhatsApp tagihan TIDAK memuat rekening, jadi jangan menjanjikannya. */}
            Hubungi tim kami untuk nomor rekening tujuan, lalu kirim konfirmasi di sini setelah
            membayar.
          </p>
        )}

        <Pilihan label="Dibayar lewat" value={cara} onChange={(e) => setCara(e.target.value)}>
          <option value="transfer">Transfer bank</option>
          <option value="qris">QRIS</option>
          <option value="ewallet">Dompet digital</option>
        </Pilihan>
        <Kolom
          label="Nama pengirim atau nomor referensi"
          bantuan="Yang tertulis di bukti transfer — kami mencocokkannya dengan mutasi rekening."
          value={referensi}
          onChange={(e) => {
            setReferensi(e.target.value)
            setGalatReferensi(undefined)
          }}
          galat={galatReferensi}
          required
        />
        <Kolom
          label="Catatan (boleh kosong)"
          placeholder="Mis. transfer dari BCA pukul 14.05"
          value={catatan}
          onChange={(e) => setCatatan(e.target.value)}
        />

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <AksiDialog>
          <Tombol
            memuat={kirim.isPending}
            labelMemuat="Mengirim…"
            onClick={() => {
              setGalat(null)
              if (referensi.trim().length < 2) {
                setGalatReferensi('Isi nama pengirim atau nomor referensi transfer.')
                return
              }
              kirim.mutate()
            }}
          >
            Kirim Konfirmasi
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={kirim.isPending}>
            Nanti saja
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}

/** Status pengembalian dana setelah berhenti — uang kembali lewat transfer staf. */
function StatusPengembalian({ pengembalian: r }: { pengembalian: Pengembalian }) {
  const tujuan = [r.destination_bank, r.destination_account].filter(Boolean).join(' ')
  return r.status === 'paid' ? (
    <p className="flex items-start gap-2 rounded-kontrol border border-garis bg-permukaan px-3 py-2 text-label text-teks-sekunder">
      <Check className="mt-0.5 h-4 w-4 shrink-0 text-hijau-700" aria-hidden />
      <span>
        Pengembalian <strong className="text-teks-utama">{formatRupiah(r.amount)}</strong> sudah
        ditransfer{tujuan && ` ke ${tujuan}`}
        {r.paid_at && ` pada ${formatTanggal(r.paid_at)}`}
        {r.payout_reference && ` (referensi ${r.payout_reference})`}.
      </span>
    </p>
  ) : (
    <p className="flex items-start gap-2 rounded-kontrol border border-garis bg-permukaan px-3 py-2 text-label text-teks-sekunder">
      <Clock className="mt-0.5 h-4 w-4 shrink-0 text-jingga-700" aria-hidden />
      <span>
        Pengembalian <strong className="text-teks-utama">{formatRupiah(r.amount)}</strong>
        {tujuan && ` ke ${tujuan}`}
        {r.destination_holder && ` a.n. ${r.destination_holder}`} sedang kami proses. Kami kabari
        lewat WhatsApp begitu uangnya terkirim.
      </span>
    </p>
  )
}

/**
 * Berhenti berlangganan. Angkanya dari server (pratinjau), bukan dihitung di
 * sini — rumus pengembaliannya (bulan terpakai dihitung harga normal) mudah
 * salah dan pemilik berhak tahu angka pastinya sebelum menekan.
 */
function DialogBerhenti({ langganan, onTutup }: { langganan: Langganan; onTutup: () => void }) {
  const toast = useToast()
  const qc = useQueryClient()
  const pratinjau = useQuery({
    queryKey: ['langganan', 'pratinjau-berhenti'],
    queryFn: langgananApi.pratinjauBerhenti,
    gcTime: 0,
  })
  const [alasan, setAlasan] = useState('')
  const [bank, setBank] = useState('')
  const [rekening, setRekening] = useState('')
  const [pemilik, setPemilik] = useState('')
  const [galat, setGalat] = useState<string | null>(null)

  const refund = pratinjau.data?.refund_amount ?? 0
  const masaCoba = langganan.status === 'trial'
  const sisaCoba =
    masaCoba && langganan.trial_ends_at && new Date(langganan.trial_ends_at) > new Date()
      ? langganan.trial_ends_at
      : undefined
  const perluRekening = refund > 0
  const lengkap = !perluRekening || (bank.trim() && rekening.trim() && pemilik.trim())

  const hentikan = useMutation({
    mutationFn: () =>
      langgananApi.berhenti({
        reason: alasan.trim() || undefined,
        ...(perluRekening && {
          refund_bank: bank.trim(),
          refund_account: rekening.trim(),
          refund_holder: pemilik.trim(),
        }),
      }),
    onSuccess: (r) => {
      qc.invalidateQueries({ queryKey: ['langganan'] })
      qc.invalidateQueries({ queryKey: ['tagihan'] })
      qc.invalidateQueries({ queryKey: ['me'] })
      toast.berhasil(
        r.refund_amount > 0
          ? `Langganan dihentikan. Pengembalian ${formatRupiah(r.refund_amount)} sedang diproses.`
          : masaCoba
            ? 'Toko kembali ke paket Gratis.'
            : 'Langganan dihentikan. Toko kembali ke paket Gratis.',
      )
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !hentikan.isPending && onTutup()}>
      <IsiDialog
        judul={masaCoba ? 'Pindah ke paket Gratis?' : `Berhenti berlangganan paket ${langganan.plan_name}?`}
      >
        <p className="text-isi text-teks-sekunder">
          {masaCoba ? `Masa coba paket ${langganan.plan_name}` : 'Paket'} berhenti{' '}
          <strong className="text-teks-utama">sekarang</strong> dan toko kembali ke paket Gratis:
          QRIS, kanal online, CRM, dan cabang tambahan terkunci. Penjualan, barang, dan data Anda
          tetap aman.
        </p>
        {sisaCoba && (
          <p className="text-label text-teks-sekunder">
            Berubah pikiran? Pilih paket berbayar lagi sebelum {formatTanggal(sisaCoba)} untuk
            melanjutkan sisa masa coba.
          </p>
        )}

        {pratinjau.isLoading ? (
          <KerangkaBaris jumlah={1} />
        ) : pratinjau.isError ? (
          <p className="text-label text-bahaya-teks">
            {pratinjau.error instanceof GalatAPI ? pratinjau.error.pesan : 'Perhitungan belum bisa dimuat.'}
          </p>
        ) : perluRekening ? (
          <div className="flex flex-col gap-3 rounded-kontrol border border-garis bg-permukaan-2 p-3">
            <div className="flex items-baseline justify-between gap-3">
              <span className="text-label text-teks-sekunder">Uang kembali</span>
              <span className="text-judul-kartu font-bold tabular-nums text-teks-utama">
                {formatRupiah(refund)}
              </span>
            </div>
            <p className="text-keterangan text-teks-redup">
              Dibayar {formatRupiah(pratinjau.data!.paid_amount)} − {pratinjau.data!.months_used} bulan
              terpakai × harga normal {formatRupiah(pratinjau.data!.monthly_price)}/bulan. Ditransfer
              tim kami ke rekening berikut.
            </p>
            <Kolom label="Bank" placeholder="Mis. BCA" value={bank} onChange={(e) => setBank(e.target.value)} required />
            <Kolom
              label="Nomor rekening"
              inputMode="numeric"
              value={rekening}
              onChange={(e) => setRekening(e.target.value)}
              required
            />
            <Kolom
              label="Nama pemilik rekening"
              value={pemilik}
              onChange={(e) => setPemilik(e.target.value)}
              required
            />
          </div>
        ) : (
          !masaCoba && (
            <p className="text-label text-teks-sekunder">Tidak ada uang yang perlu dikembalikan.</p>
          )
        )}

        <Kolom
          label="Alasan berhenti (boleh kosong)"
          placeholder="Membantu kami memperbaiki aplikasi"
          value={alasan}
          onChange={(e) => setAlasan(e.target.value)}
        />

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <AksiDialog>
          <Tombol
            jenis={masaCoba ? 'utama' : 'bahaya'}
            memuat={hentikan.isPending}
            labelMemuat={masaCoba ? 'Memindahkan…' : 'Menghentikan…'}
            disabled={!pratinjau.isSuccess || !lengkap}
            onClick={() => {
              setGalat(null)
              hentikan.mutate()
            }}
          >
            {masaCoba ? 'Pindah ke Gratis' : 'Hentikan Langganan'}
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={hentikan.isPending}>
            Batal
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}

function StatusLangganan({ status }: { status: string }) {
  switch (status) {
    case 'active':
      return <LencanaStatus nada="berhasil" anak="Aktif" />
    // Nilai dari CHECK subscriptions.status (000015): trial, active,
    // past_due, canceled, expired.
    case 'trial':
      return <LencanaStatus nada="berhasil" anak="Masa coba" />
    case 'past_due':
      return <LencanaStatus nada="menunggu" anak="Lewat jatuh tempo" />
    case 'canceled':
      return <LencanaStatus nada="bahaya" anak="Berhenti" />
    case 'expired':
      return <LencanaStatus nada="bahaya" anak="Berakhir" />
    default:
      return <LencanaStatus nada="netral" anak={status} />
  }
}
