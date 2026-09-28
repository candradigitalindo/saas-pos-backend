import { Trash2 } from 'lucide-react'
import { Tombol } from '@/bersama/ui/tombol'
import { StepperJumlah } from '@/bersama/ui/stepper-jumlah'
import { formatRupiah, pratinjauBaris } from '@/bersama/util/uang'
import { hargaBaris, namaBaris, type Keranjang } from '../keranjang'

/**
 * Keranjang. SELALU TERLIHAT di layar kasir — tidak pernah bersembunyi di balik
 * ikon, karena kasir harus bisa membaca ulang isinya sebelum menekan Bayar.
 *
 * Angka di sini adalah PRATINJAU. Total yang tercetak di struk diambil dari
 * balasan server.
 */
export function PanelKeranjang({
  keranjang,
  onBayar,
  tanpaJudul = false,
}: {
  keranjang: Keranjang
  onBayar: () => void
  /** Di lembar keranjang HP kepalanya sudah bertuliskan "Keranjang". */
  tanpaJudul?: boolean
}) {
  const { baris, pratinjauTotal, rincian, ubahQty, hapus } = keranjang
  // Pajak eksklusif MENAMBAH total (total = subtotal − diskon + pajak +
  // layanan); pajak inklusif sudah ada di dalam harga dan hanya disebutkan.
  const pajakEksklusif =
    rincian.tax_amount > 0 &&
    rincian.total ===
      rincian.subtotal - rincian.discount_amount + rincian.tax_amount + rincian.service_amount
  const kosong = baris.length === 0

  return (
    <section
      aria-label="Keranjang"
      className="flex h-full flex-col border-garis bg-permukaan lg:border-l"
    >
      {!tanpaJudul && (
        <h2 className="border-b border-garis px-4 py-3 text-judul-kartu font-semibold text-teks-utama">
          Keranjang
        </h2>
      )}

      <div className="flex-1 overflow-y-auto">
        {kosong ? (
          <p className="px-4 py-8 text-center text-isi text-teks-redup">
            Pilih barang dulu untuk mulai.
          </p>
        ) : (
          <ul className="divide-y divide-garis">
            {baris.map((b) => (
              <li key={b.kunci} className="flex flex-col gap-2 p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="font-semibold text-teks-utama">{namaBaris(b)}</p>
                    <p className="text-keterangan tabular-nums text-teks-redup">
                      {formatRupiah(hargaBaris(b))} × {b.qty}
                      {b.diskon > 0 && ` − ${formatRupiah(b.diskon)}`}
                    </p>
                  </div>
                  <p className="shrink-0 font-bold tabular-nums text-teks-utama">
                    {formatRupiah(pratinjauBaris(hargaBaris(b), b.qty, b.diskon))}
                  </p>
                </div>

                <div className="flex items-center justify-between gap-2">
                  <StepperJumlah
                    nilai={b.qty}
                    onNilai={(q) => ubahQty(b.kunci, q)}
                    satuan={b.produk.unit_name}
                    label={`Jumlah ${namaBaris(b)}`}
                  />
                  {/* Tombol hapus sengaja berjarak dari tombol +/−. */}
                  <button
                    type="button"
                    onClick={() => hapus(b.kunci)}
                    aria-label={`Hapus ${namaBaris(b)} dari keranjang`}
                    className="ml-2 flex h-12 w-12 shrink-0 items-center justify-center rounded-kontrol text-bahaya-teks hover:bg-bahaya-teks/10"
                  >
                    <Trash2 className="h-5 w-5" aria-hidden />
                  </button>
                </div>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="border-t border-garis p-4">
        {/* Pajak & biaya layanan ditampilkan bila ada, supaya kasir bisa
            menjelaskan ke pembeli kenapa total lebih besar dari harga barang. */}
        {(rincian.tax_amount > 0 || rincian.service_amount > 0) && (
          <dl className="mb-2 flex flex-col gap-1 text-label text-teks-sekunder">
            {pajakEksklusif && (
              <div className="flex justify-between">
                <dt>Pajak</dt>
                <dd className="tabular-nums">{formatRupiah(rincian.tax_amount)}</dd>
              </div>
            )}
            {rincian.service_amount > 0 && (
              <div className="flex justify-between">
                <dt>Biaya layanan</dt>
                <dd className="tabular-nums">{formatRupiah(rincian.service_amount)}</dd>
              </div>
            )}
            {rincian.tax_amount > 0 && !pajakEksklusif && (
              <p className="text-keterangan text-teks-redup">
                Sudah termasuk pajak {formatRupiah(rincian.tax_amount)}
              </p>
            )}
          </dl>
        )}
        <div className="mb-3 flex items-baseline justify-between">
          <span className="text-isi text-teks-sekunder">Total</span>
          <span className="text-judul font-extrabold tabular-nums text-teks-utama">
            {formatRupiah(pratinjauTotal)}
          </span>
        </div>

        {/* Tombol Bayar MENAMPILKAN NOMINALNYA — kasir membaca ulang sebelum menekan. */}
        <Tombol ukuran="kasir" lebarPenuh disabled={kosong} onClick={onBayar}>
          BAYAR {formatRupiah(pratinjauTotal)}
        </Tombol>
      </div>
    </section>
  )
}
