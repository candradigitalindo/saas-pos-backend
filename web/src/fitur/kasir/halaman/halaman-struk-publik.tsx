import { useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { Printer } from 'lucide-react'
import { api, GalatAPI } from '@/lib/api-client'
import { Tombol } from '@/bersama/ui/tombol'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQty } from '@/bersama/util/desimal'
import { formatTanggalJam } from '@/bersama/util/tanggal'
import { namaMetode } from '../label-transaksi'

/** Isi struk publik dari GET /public/receipts/:token (hanya yang tercetak di struk). */
interface StrukPublik {
  store_name: string
  address?: string
  phone?: string
  header?: string
  footer?: string
  timezone: string
  receipt_no: string
  status: string
  occurred_at: string
  items: { name: string; qty: string; unit: string; unit_price: number; discount: number; line_total: number; note?: string }[]
  subtotal: number
  discount: number
  tax: number
  service: number
  rounding: number
  total: number
  paid: number
  change: number
  payments: { method: string; amount: number }[]
}

/**
 * Struk digital — dibuka pembeli dari tautan WhatsApp, TANPA akun. Tampil
 * seperti struk kertas (satu kolom sempit) dan bisa dicetak / disimpan PDF
 * lewat menu cetak peramban; tombol cetaknya sendiri tidak ikut tercetak.
 */
export function HalamanStrukPublik() {
  const { token = '' } = useParams()
  const q = useQuery({
    queryKey: ['struk-publik', token],
    queryFn: () => api.get<StrukPublik>(`/public/receipts/${encodeURIComponent(token)}`),
    retry: (n, e) => !(e instanceof GalatAPI && e.status === 404) && n < 2,
  })

  return (
    <main className="flex min-h-dvh justify-center bg-latar px-4 py-6 print:bg-white print:p-0">
      <div className="flex w-full max-w-sm flex-col gap-4">
        {q.isLoading ? (
          <div className="rounded-kartu bg-permukaan p-5 shadow-kartu">
            <KerangkaBaris jumlah={5} />
          </div>
        ) : q.isError || !q.data ? (
          <div className="rounded-kartu bg-permukaan p-6 text-center shadow-kartu">
            <p className="text-judul-kartu font-semibold text-teks-utama">Struk tidak ditemukan</p>
            <p className="mt-1 text-label text-teks-sekunder">
              Tautannya mungkin terpotong saat disalin. Minta toko mengirim ulang struknya.
            </p>
          </div>
        ) : (
          <IsiStruk s={q.data} />
        )}
        {q.data && (
          <Tombol jenis="kedua" lebarPenuh onClick={() => window.print()} className="print:hidden">
            <Printer className="h-5 w-5" aria-hidden />
            Cetak / Simpan PDF
          </Tombol>
        )}
      </div>
    </main>
  )
}

function IsiStruk({ s }: { s: StrukPublik }) {
  const batal = s.status === 'canceled'
  const angka = (label: string, nilai: number, tebal = false) =>
    nilai !== 0 || tebal ? (
      <div className={`flex items-baseline justify-between gap-3 ${tebal ? 'text-isi font-bold text-teks-utama' : ''}`}>
        <dt className={tebal ? '' : 'text-teks-sekunder'}>{label}</dt>
        <dd className="tabular-nums">{formatRupiah(nilai)}</dd>
      </div>
    ) : null

  return (
    <article className="flex flex-col gap-3 rounded-kartu bg-permukaan p-5 text-label shadow-kartu print:shadow-none">
      <header className="text-center">
        <h1 className="text-judul-kartu font-bold text-teks-utama">{s.store_name}</h1>
        {s.header && <p className="whitespace-pre-line text-teks-sekunder">{s.header}</p>}
        {s.address && <p className="text-keterangan text-teks-redup">{s.address}</p>}
        {s.phone && <p className="text-keterangan text-teks-redup">{s.phone}</p>}
      </header>

      <div className="flex items-baseline justify-between gap-3 border-y border-dashed border-garis py-2">
        <span className="font-semibold tabular-nums text-teks-utama">{s.receipt_no}</span>
        <span className="text-keterangan text-teks-redup">{formatTanggalJam(s.occurred_at, s.timezone)}</span>
      </div>

      {batal && (
        <p className="rounded-kontrol border border-bahaya px-3 py-2 text-center font-semibold text-bahaya-teks">
          Transaksi ini DIBATALKAN
        </p>
      )}

      <ul className="flex flex-col gap-2">
        {s.items.map((i, n) => (
          <li key={n} className="flex items-start justify-between gap-3">
            <span className="min-w-0">
              <span className="block text-teks-utama">{i.name}</span>
              <span className="block text-keterangan tabular-nums text-teks-redup">
                {formatQty(i.qty)} {i.unit} × {formatRupiah(i.unit_price)}
                {i.discount > 0 && ` − ${formatRupiah(i.discount)}`}
              </span>
              {i.note && <span className="block text-keterangan italic text-teks-sekunder">“{i.note}”</span>}
            </span>
            <span className="shrink-0 tabular-nums text-teks-utama">{formatRupiah(i.line_total)}</span>
          </li>
        ))}
      </ul>

      <dl className="flex flex-col gap-1 border-t border-dashed border-garis pt-2">
        {(s.discount > 0 || s.tax > 0 || s.service > 0 || s.rounding !== 0) && angka('Subtotal', s.subtotal)}
        {angka('Diskon', -s.discount)}
        {angka('Pajak', s.tax)}
        {angka('Biaya layanan', s.service)}
        {angka('Pembulatan', s.rounding)}
        {angka('Total', s.total, true)}
        {s.payments.map((p, n) => (
          <div key={n} className="flex items-baseline justify-between gap-3">
            <dt className="text-teks-sekunder">{namaMetode(p.method)}</dt>
            <dd className="tabular-nums text-teks-sekunder">{formatRupiah(p.amount)}</dd>
          </div>
        ))}
        {angka('Kembalian', s.change)}
      </dl>

      <footer className="border-t border-dashed border-garis pt-2 text-center text-keterangan text-teks-redup">
        {s.footer ? <p className="whitespace-pre-line">{s.footer}</p> : <p>Terima kasih!</p>}
      </footer>
    </article>
  )
}
