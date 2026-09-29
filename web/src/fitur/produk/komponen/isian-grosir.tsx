import { Plus, X } from 'lucide-react'
import type { TingkatGrosir } from '@/bersama/tipe/katalog'
import { formatAngka, parseRupiah } from '@/bersama/util/uang'
import { cn } from '@/bersama/util/cn'

const MAKS_TINGKAT = 5

/**
 * Harga grosir per jumlah: "beli ≥ 12 pcs → Rp 9.000 per pcs". Berlaku
 * otomatis di kasir untuk semua pembeli, dihitung dari TOTAL barang ini dalam
 * satu transaksi (semua varian dijumlah).
 *
 * Tersembunyi di balik "+ Harga grosir" selama kosong — kebanyakan barang
 * tidak memakainya, dan form barang harus tetap pendek.
 */
export function IsianGrosir({
  nilai,
  onNilai,
  hargaJual,
  hargaBeli,
  satuan,
}: {
  nilai: TingkatGrosir[]
  onNilai: (v: TingkatGrosir[]) => void
  hargaJual: number
  hargaBeli: number
  satuan?: string
}) {
  const ubah = (i: number, isi: Partial<TingkatGrosir>) => onNilai(nilai.map((t, j) => (j === i ? { ...t, ...isi } : t)))
  const tambah = () => onNilai([...nilai, { min_qty: '', price: 0 }])

  if (nilai.length === 0) {
    return (
      <button
        type="button"
        onClick={tambah}
        className="-mx-2 flex min-h-11 items-center gap-1 self-start rounded-kontrol px-2 text-label font-medium text-utama hover:bg-sorot"
      >
        <Plus className="h-4 w-4" aria-hidden />
        Harga grosir (beli banyak lebih murah)
      </button>
    )
  }

  return (
    // min-w-0: <fieldset> bawaan peramban ber-min-width min-content, jadi tanpa
    // ini barisnya mendorong halaman melebar di HP.
    <fieldset className="flex min-w-0 flex-col gap-2 rounded-kontrol border border-garis p-3">
      <legend className="px-1 text-label font-medium text-teks-sekunder">Harga grosir</legend>
      <p className="-mt-1 text-keterangan text-teks-redup">
        Beli minimal sekian → harga per {satuan ?? 'pcs'} turun. Otomatis di kasir bila jumlah barang ini dalam
        satu transaksi mencapai batasnya.
      </p>
      {nilai.map((t, i) => {
        const peringatan =
          t.price > 0 && hargaBeli > 0 && t.price < hargaBeli
            ? { teks: 'Di bawah harga beli — rugi.', bahaya: true }
            : t.price > 0 && hargaJual > 0 && t.price >= hargaJual
              ? { teks: 'Tidak lebih murah dari harga jual.', bahaya: false }
              : null
        return (
          <div key={i} className="flex flex-col gap-1">
            <div className="flex items-center gap-1.5 text-label text-teks-sekunder">
              <span className="shrink-0" aria-hidden>
                ≥
              </span>
              <input
                inputMode="decimal"
                value={t.min_qty}
                onChange={(e) => ubah(i, { min_qty: e.target.value.replace(/[^\d.,]/g, '') })}
                placeholder="12"
                aria-label={`Jumlah minimal tingkat ${i + 1}`}
                className="h-12 w-14 min-w-0 shrink-0 rounded-kontrol border border-garis bg-permukaan px-2 text-center text-isi tabular-nums text-teks-utama focus:outline focus:outline-2 focus:outline-offset-2 focus:outline-utama"
              />
              {/* Satuannya sudah disebut di teks bantuan; di HP sempit ruangnya
                  lebih berguna untuk kotak harga. */}
              <span className="hidden shrink-0 truncate min-[400px]:inline">{satuan ?? 'pcs'}</span>
              <label className="flex h-12 min-w-0 flex-1 items-center gap-1 rounded-kontrol border border-garis bg-permukaan px-2 focus-within:outline focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-utama">
                <span className="text-teks-redup" aria-hidden>
                  Rp
                </span>
                <input
                  inputMode="numeric"
                  value={t.price === 0 ? '' : formatAngka(t.price)}
                  onChange={(e) => ubah(i, { price: parseRupiah(e.target.value) })}
                  placeholder="0"
                  aria-label={`Harga satuan tingkat ${i + 1}`}
                  className="h-full min-w-0 flex-1 bg-transparent text-isi tabular-nums text-teks-utama outline-none"
                />
              </label>
              <button
                type="button"
                onClick={() => onNilai(nilai.filter((_, j) => j !== i))}
                aria-label={`Hapus tingkat ${i + 1}`}
                className="-mr-1 flex h-11 w-11 shrink-0 items-center justify-center rounded-kontrol text-teks-redup hover:bg-permukaan-2"
              >
                <X className="h-4 w-4" aria-hidden />
              </button>
            </div>
            {peringatan && (
              <p className={cn('text-keterangan', peringatan.bahaya ? 'text-bahaya-teks' : 'text-jingga-700')}>
                {peringatan.teks}
              </p>
            )}
          </div>
        )
      })}
      {nilai.length < MAKS_TINGKAT && (
        <button
          type="button"
          onClick={tambah}
          className="-mx-1 flex min-h-11 items-center gap-1 self-start rounded-kontrol px-1 text-label font-medium text-utama hover:bg-sorot"
        >
          <Plus className="h-4 w-4" aria-hidden />
          Tambah tingkat
        </button>
      )}
    </fieldset>
  )
}

/** Tingkat yang siap dikirim (baris kosong dibuang), atau pesan galat. */
export function siapkanGrosir(nilai: TingkatGrosir[]): { tingkat: TingkatGrosir[] } | { galat: string } {
  const isi = nilai
    .map((t) => ({ min_qty: t.min_qty.trim().replace(',', '.'), price: t.price }))
    .filter((t) => t.min_qty !== '' || t.price > 0)
  const sudah = new Set<number>()
  for (const t of isi) {
    const q = Number(t.min_qty)
    if (!Number.isFinite(q) || q <= 1) return { galat: 'Jumlah minimal harga grosir harus lebih dari 1.' }
    if (t.price <= 0) return { galat: `Isi harga untuk pembelian ≥ ${t.min_qty}.` }
    if (sudah.has(q)) return { galat: `Jumlah minimal ${t.min_qty} tertulis dua kali.` }
    sudah.add(q)
  }
  return { tingkat: isi }
}
