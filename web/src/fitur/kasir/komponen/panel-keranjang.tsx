import { Trash2 } from 'lucide-react'
import { Tombol } from '@/bersama/ui/tombol'
import { StepperJumlah } from '@/bersama/ui/stepper-jumlah'
import { formatRupiah, pratinjauBaris } from '@/bersama/util/uang'
import type { Keranjang } from '../keranjang'

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
}: {
  keranjang: Keranjang
  onBayar: () => void
}) {
  const { baris, pratinjauTotal, ubahQty, hapus } = keranjang
  const kosong = baris.length === 0

  return (
    <section
      aria-label="Keranjang"
      className="flex h-full flex-col border-garis bg-permukaan lg:border-l"
    >
      <h2 className="border-b border-garis px-4 py-3 text-judul-kartu font-semibold text-teks-utama">
        Keranjang
      </h2>

      <div className="flex-1 overflow-y-auto">
        {kosong ? (
          <p className="px-4 py-8 text-center text-isi text-teks-redup">
            Pilih barang dulu untuk mulai.
          </p>
        ) : (
          <ul className="divide-y divide-garis">
            {baris.map((b) => (
              <li key={b.produk.id} className="flex flex-col gap-2 p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="font-semibold text-teks-utama">{b.produk.name}</p>
                    <p className="text-keterangan tabular-nums text-teks-redup">
                      {formatRupiah(b.produk.sell_price)} × {b.qty}
                      {b.diskon > 0 && ` − ${formatRupiah(b.diskon)}`}
                    </p>
                  </div>
                  <p className="shrink-0 font-bold tabular-nums text-teks-utama">
                    {formatRupiah(pratinjauBaris(b.produk.sell_price, b.qty, b.diskon))}
                  </p>
                </div>

                <div className="flex items-center justify-between gap-2">
                  <StepperJumlah
                    nilai={b.qty}
                    onNilai={(q) => ubahQty(b.produk.id, q)}
                    satuan={b.produk.unit_name}
                    label={`Jumlah ${b.produk.name}`}
                  />
                  {/* Tombol hapus sengaja berjarak dari tombol +/−. */}
                  <button
                    type="button"
                    onClick={() => hapus(b.produk.id)}
                    aria-label={`Hapus ${b.produk.name} dari keranjang`}
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
