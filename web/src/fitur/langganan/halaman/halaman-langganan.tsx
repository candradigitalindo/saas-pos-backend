import { useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ulid } from 'ulid'
import { Check, CreditCard, Sparkles } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Pilihan } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KerangkaBaris, KerangkaKartuAngka } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
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

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-4">
      <h1 className="text-judul font-bold text-teks-utama">Langganan</h1>

      {ringkasan.isLoading ? (
        <KerangkaKartuAngka />
      ) : belumBerlangganan ? (
        <Kartu className="flex flex-col items-center gap-2 p-6 text-center">
          <Sparkles className="h-10 w-10 text-jingga-600" aria-hidden />
          <p className="text-judul-kartu font-semibold text-teks-utama">
            Belum berlangganan
          </p>
          <p className="text-isi text-teks-sekunder">
            Pilih paket di bawah untuk mulai memakai fitur berbayar.
          </p>
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
          {langganan.trial_ends_at && (
            <p className="mt-1 text-keterangan text-teks-redup">
              Masa coba gratis berakhir {formatTanggal(langganan.trial_ends_at)}.
            </p>
          )}
        </Kartu>
      )}

      {tagihanTerbuka && (
        <Kartu className="border-jingga-600 bg-jingga-100 p-4">
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

      <section className="flex flex-col gap-2">
        <h2 className="text-judul-kartu font-semibold text-teks-utama">Pilihan paket</h2>
        {paket.isLoading ? (
          <KerangkaBaris jumlah={3} />
        ) : (
          <div className="grid gap-3 sm:grid-cols-2">
            {paket.data?.map((p) => (
              <KartuPaket
                key={p.code}
                paket={p}
                sedangDipakai={p.code === langganan?.plan_code}
                sudahBerlangganan={!belumBerlangganan}
              />
            ))}
          </div>
        )}
      </section>

      {(tagihan.data?.data.length ?? 0) > 0 && (
        <section className="flex flex-col gap-2">
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

function KartuPaket({
  paket,
  sedangDipakai,
  sudahBerlangganan,
}: {
  paket: Paket
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
  const ambil = useMutation<unknown>({
    mutationFn: () =>
      sudahBerlangganan
        ? langgananApi.gantiPaket(paket.code, masa)
        : langgananApi.mulai(paket.code, masa),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['langganan'] })
      qc.invalidateQueries({ queryKey: ['tagihan'] })
      toast.berhasil(
        sudahBerlangganan
          ? `Pindah ke paket ${paket.name}. Tagihan penyesuaiannya sudah dibuat.`
          : `Paket ${paket.name} aktif. Tagihannya sudah dibuat.`,
      )
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  return (
    <Kartu className={cn('flex flex-col gap-3 p-4', sedangDipakai && 'border-utama bg-sorot')}>
      <div>
        <p className="text-judul-kartu font-semibold text-teks-utama">{paket.name}</p>
        <p className="text-keterangan text-teks-redup">
          {formatRupiah(paket.monthly_price)} per bulan
        </p>
      </div>

      <ul className="flex flex-col gap-1 text-keterangan text-teks-sekunder">
        <Batas label="toko" nilai={paket.max_outlets} />
        <Batas label="pengguna" nilai={paket.max_users} />
        <Batas label="barang" nilai={paket.max_products} />
        <Batas label="transaksi per bulan" nilai={paket.max_monthly_transactions} />
      </ul>

      {paket.term_prices.length > 1 && (
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

      {harga && (
        <p className="text-isi font-bold tabular-nums text-teks-utama">
          {formatRupiah(harga.total_amount)}
          {harga.discount_amount > 0 && (
            <span className="ml-2 text-keterangan font-normal text-hijau-700">
              hemat {formatRupiah(harga.discount_amount)}
            </span>
          )}
        </p>
      )}

      {galat && (
        <p className="rounded-kontrol border border-bahaya bg-red-50 px-3 py-2 text-keterangan text-bahaya-teks">
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
    </Kartu>
  )
}

function Batas({ label, nilai }: { label: string; nilai: number | null }) {
  return (
    <li className="flex items-center gap-1.5">
      <Check className="h-4 w-4 shrink-0 text-hijau-600" aria-hidden />
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
          <p className="rounded-kontrol border border-bahaya bg-red-50 px-3 py-2 text-label text-bahaya-teks">
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
