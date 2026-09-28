import { Printer } from 'lucide-react'
import { Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { Tombol } from '@/bersama/ui/tombol'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQty } from '@/bersama/util/desimal'
import { formatTanggalJam } from '@/bersama/util/tanggal'
import type { Transaksi } from '@/bersama/tipe/pos'
import { NAMA_METODE } from '../label-transaksi'


/**
 * Struk.
 *
 * SELURUH angka di sini berasal dari balasan server — tidak satu pun dihitung
 * ulang di layar. Kalau frontend ikut menghitung, totalnya bisa beda beberapa
 * rupiah dari yang tersimpan, dan itu menghancurkan kepercayaan.
 */
export function Struk({
  transaksi,
  terbuka,
  onTutup,
  onTransaksiBaru,
  menungguDikirim,
}: {
  transaksi: Transaksi
  terbuka: boolean
  onTutup: () => void
  onTransaksiBaru: () => void
  /** true bila transaksi masih di antrean offline. */
  menungguDikirim?: boolean
}) {
  return (
    <Dialog open={terbuka} onOpenChange={(o) => !o && onTutup()}>
      <IsiDialog judul="Transaksi tersimpan" className="sm:max-w-md">
        <div id="area-struk" className="flex flex-col gap-3">
          <div className="flex items-center justify-between gap-2">
            <div>
              <p className="font-bold tabular-nums text-teks-utama">
                {transaksi.receipt_no}
              </p>
              <p className="text-keterangan text-teks-redup">
                {formatTanggalJam(transaksi.occurred_at)}
              </p>
            </div>
            {menungguDikirim ? (
              <LencanaStatus nada="menunggu" anak="Menunggu dikirim" />
            ) : (
              <LencanaStatus nada="berhasil" anak="Tersimpan" />
            )}
          </div>

          <ul className="divide-y divide-garis border-y border-garis">
            {transaksi.items?.map((i) => (
              <li key={i.id} className="flex justify-between gap-3 py-2">
                <span className="min-w-0">
                  <span className="block text-label text-teks-utama">{i.product_name}</span>
                  <span className="block text-keterangan tabular-nums text-teks-redup">
                    {formatQty(i.qty)} {i.unit_name} × {formatRupiah(i.unit_price)}
                    {i.discount_amount > 0 && ` − ${formatRupiah(i.discount_amount)}`}
                  </span>
                  {i.note && (
                    <span className="block break-words text-keterangan italic text-teks-sekunder">“{i.note}”</span>
                  )}
                </span>
                <span className="shrink-0 tabular-nums text-teks-utama">
                  {formatRupiah(i.line_total)}
                </span>
              </li>
            ))}
          </ul>

          <dl className="flex flex-col gap-1 text-label">
            <BarisAngka label="Subtotal" nilai={transaksi.subtotal} />
            {transaksi.discount_amount > 0 && (
              <BarisAngka label="Diskon" nilai={-transaksi.discount_amount} />
            )}
            {transaksi.tax_amount > 0 && (
              <BarisAngka label="Pajak" nilai={transaksi.tax_amount} />
            )}
            {transaksi.service_amount > 0 && (
              <BarisAngka label="Biaya layanan" nilai={transaksi.service_amount} />
            )}
            {transaksi.rounding_amount !== 0 && (
              <BarisAngka label="Pembulatan" nilai={transaksi.rounding_amount} />
            )}

            <div className="mt-1 flex items-baseline justify-between border-t border-garis pt-2">
              <dt className="text-judul-kartu font-semibold text-teks-utama">Total</dt>
              <dd className="text-judul font-extrabold tabular-nums text-teks-utama">
                {formatRupiah(transaksi.total)}
              </dd>
            </div>

            {transaksi.payments?.map((p) => (
              <BarisAngka
                key={p.id}
                label={NAMA_METODE[p.method] ?? p.method}
                nilai={p.amount}
              />
            ))}
            {transaksi.change_amount > 0 && (
              <div className="flex items-baseline justify-between">
                <dt className="font-semibold text-teks-utama">Kembalian</dt>
                <dd className="font-bold tabular-nums text-teks-utama">
                  {formatRupiah(transaksi.change_amount)}
                </dd>
              </div>
            )}
          </dl>
        </div>

        <div className="mt-2 flex flex-col gap-2">
          <Tombol ukuran="kasir" lebarPenuh onClick={onTransaksiBaru}>
            Transaksi Baru
          </Tombol>
          <Tombol jenis="kedua" lebarPenuh onClick={() => window.print()}>
            <Printer className="h-5 w-5" aria-hidden />
            Cetak Struk
          </Tombol>
        </div>
      </IsiDialog>
    </Dialog>
  )
}

function BarisAngka({ label, nilai }: { label: string; nilai: number }) {
  return (
    <div className="flex items-baseline justify-between">
      <dt className="text-teks-sekunder">{label}</dt>
      <dd className="tabular-nums text-teks-utama">{formatRupiah(nilai)}</dd>
    </div>
  )
}
