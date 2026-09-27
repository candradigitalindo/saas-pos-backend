import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ulid } from 'ulid'
import { Check, CreditCard, Lock, Minus, Sparkles } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Pilihan } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KerangkaBaris, KerangkaKartuAngka } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { formatRupiah } from '@/bersama/util/uang'
import { formatTanggal } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { langgananApi, type Paket, type TagihanLangganan } from '../api'

/**
 * Langganan aplikasi.
 *
 * Yang perlu diketahui pemilik cuma dua: sampai kapan aktif, dan ada tagihan
 * yang harus dibayar atau tidak. Keduanya di layar pertama; katalog paket
 * menyusul di bawah.
 */
export function HalamanLangganan() {
  const [bayarUntuk, setBayarUntuk] = useState<TagihanLangganan | null>(null)

  const ringkasan = useQuery({
    queryKey: ['langganan'],
    queryFn: langgananApi.ringkasan,
    retry: false,
  })

  const paket = useQuery({ queryKey: ['paket'], queryFn: langgananApi.paket })
  const tagihan = useQuery({ queryKey: ['tagihan'], queryFn: langgananApi.tagihan })

  const langganan = ringkasan.data?.subscription
  const tagihanTerbuka = ringkasan.data?.open_invoice
  const belumBerlangganan = ringkasan.isError || !langganan

  // Paket yang BERLAKU menurut server (GET /me) — bisa berbeda dari paket
  // yang dipilih: masa coba yang habis, atau masa bayar yang lewat tenggang,
  // membuat tenant kembali ke Gratis walau langganannya masih tercatat.
  const { profil } = useSesi()
  const berlaku = profil?.plan
  const qc = useQueryClient()
  const toast = useToast()

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
        ) : belumBerlangganan ? (
          <Kartu className="flex flex-col items-center gap-2 p-6 text-center">
            <Sparkles className="h-10 w-10 text-jingga-600" aria-hidden />
            <p className="text-judul-kartu font-semibold text-teks-utama">
              Anda memakai paket {berlaku?.name ?? 'Gratis'}
            </p>
            <p className="text-isi text-teks-sekunder">
              QRIS, kanal online, CRM, dan cabang tambahan terkunci. Paket berbayar
              dimulai dengan masa coba gratis — pilih di bawah.
            </p>
          </Kartu>
        ) : berlaku && berlaku.code !== langganan.plan_code ? (
          // Langganan tercatat tapi TIDAK berlaku lagi (masa coba habis, lewat
          // tenggang, atau dihentikan): katakan terus terang, dan beri jalan
          // kembalinya di tempat yang sama.
          <Kartu className="flex flex-col gap-3 border-jingga-600 bg-permukaan-2 p-4">
            <div className="flex items-start gap-3">
              <Lock className="mt-0.5 h-5 w-5 shrink-0 text-jingga-700" aria-hidden />
              <div>
                <p className="font-semibold text-teks-utama">
                  {langganan.status === 'trial'
                    ? `Masa coba paket ${langganan.plan_name} sudah berakhir`
                    : langganan.status === 'canceled' || langganan.status === 'expired'
                      ? `Langganan paket ${langganan.plan_name} sudah berhenti`
                      : `Masa langganan paket ${langganan.plan_name} sudah habis`}
                </p>
                <p className="text-label text-teks-sekunder">
                  Sekarang memakai paket {berlaku.name} — fitur berbayar terkunci sampai
                  tagihannya dibayar. Penjualan tetap berjalan seperti biasa.
                </p>
              </div>
            </div>
            {langganan.status !== 'canceled' && langganan.status !== 'expired' && !tagihanTerbuka && (
              <Tombol
                className="self-start"
                memuat={bayarSekarang.isPending}
                onClick={() => bayarSekarang.mutate()}
              >
                <CreditCard className="h-5 w-5" aria-hidden />
                Bayar paket {langganan.plan_name}
              </Tombol>
            )}
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
                sebelum itu supaya fitur tidak terkunci.
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
          </Kartu>
        )}

        {tagihanTerbuka && (
          <Kartu className="border-jingga-600 bg-permukaan-2 p-4">
            <p className="font-semibold text-jingga-700">Ada tagihan yang belum dibayar</p>
            <p className="mt-1 text-angka font-extrabold tabular-nums text-teks-utama">
              {formatRupiah(tagihanTerbuka.total_amount - tagihanTerbuka.paid_amount)}
            </p>
            <p className="mt-1 text-keterangan text-jingga-700">
              Nomor {tagihanTerbuka.number} · jatuh tempo{' '}
              {formatTanggal(tagihanTerbuka.due_date)}
            </p>
            <Tombol className="mt-3" onClick={() => setBayarUntuk(tagihanTerbuka)}>
              <CreditCard className="h-5 w-5" aria-hidden />
              Bayar Sekarang
            </Tombol>
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
                    {formatTanggal(t.period_start)} – {formatTanggal(t.period_end)}
                  </p>
                </div>
                <div className="flex shrink-0 items-center gap-3">
                  <p className="font-bold tabular-nums text-teks-utama">
                    {formatRupiah(t.total_amount)}
                  </p>
                  {t.status === 'paid' ? (
                    <LencanaStatus nada="berhasil" anak="Lunas" />
                  ) : (
                    <LencanaStatus nada="menunggu" anak="Belum lunas" />
                  )}
                </div>
              </div>
            ))}
          </Kartu>
        </section>
      )}

      {bayarUntuk && (
        <DialogBayar tagihan={bayarUntuk} onTutup={() => setBayarUntuk(null)} />
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
  multi_outlet: 'Kelola banyak cabang',
}

/** Kalimat pendek di bawah nama paket — untuk siapa paket itu. */
const UNTUK_SIAPA: Record<string, string> = {
  free: 'Untuk mencoba dan usaha yang baru mulai',
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
}: {
  paket: Paket
  kunciFitur: string[]
  sedangDipakai: boolean
  sudahBerlangganan: boolean
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
        await langgananApi.gantiPaket(paket.code, masa)
        return `Pindah ke paket ${paket.name}. Tagihan penyesuaiannya sudah dibuat.`
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

function DialogBayar({
  tagihan,
  onTutup,
}: {
  tagihan: TagihanLangganan
  onTutup: () => void
}) {
  const toast = useToast()
  const qc = useQueryClient()
  const [cara, setCara] = useState('transfer')
  const [galat, setGalat] = useState<string | null>(null)

  // Dibuat sekali saat dialog terbuka dan dipertahankan selama percobaan —
  // pembayaran langganan tidak boleh tercatat dua kali.
  const kunci = useRef(ulid())
  const sisa = tagihan.total_amount - tagihan.paid_amount

  const bayar = useMutation({
    mutationFn: () =>
      langgananApi.bayar(
        { invoice_id: tagihan.id, amount: sisa, method: cara },
        kunci.current,
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['langganan'] })
      qc.invalidateQueries({ queryKey: ['tagihan'] })
      qc.invalidateQueries({ queryKey: ['me'] })
      toast.berhasil('Pembayaran tercatat. Terima kasih.')
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !bayar.isPending && onTutup()}>
      <IsiDialog judul={`Bayar tagihan ${tagihan.number}`}>
        <div className="flex items-baseline justify-between border-b border-garis pb-3">
          <span className="text-isi text-teks-sekunder">Jumlah yang harus dibayar</span>
          <span className="text-judul font-extrabold tabular-nums text-teks-utama">
            {formatRupiah(sisa)}
          </span>
        </div>

        <Pilihan label="Cara bayar" value={cara} onChange={(e) => setCara(e.target.value)}>
          <option value="transfer">Transfer bank</option>
          <option value="qris">QRIS</option>
          <option value="ewallet">Dompet digital</option>
          <option value="card">Kartu</option>
          <option value="cash">Tunai</option>
        </Pilihan>

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <AksiDialog>
          <Tombol
            memuat={bayar.isPending}
            labelMemuat="Mencatat…"
            onClick={() => {
              setGalat(null)
              bayar.mutate()
            }}
          >
            Bayar {formatRupiah(sisa)}
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={bayar.isPending}>
            Nanti saja
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
    case 'trialing':
      return <LencanaStatus nada="berhasil" anak="Masa coba" />
    case 'past_due':
      return <LencanaStatus nada="menunggu" anak="Lewat jatuh tempo" />
    case 'canceled':
      return <LencanaStatus nada="bahaya" anak="Berhenti" />
    default:
      return <LencanaStatus nada="netral" anak={status} />
  }
}
