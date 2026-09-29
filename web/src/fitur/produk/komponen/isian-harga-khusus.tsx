import { useState } from 'react'
import { Plus } from 'lucide-react'
import type { DaftarHarga } from '@/bersama/tipe/katalog'
import { formatAngka, parseRupiah } from '@/bersama/util/uang'

/**
 * Harga khusus per daftar harga (Member, Reseller): satu harga per daftar;
 * kosong = pelanggan daftar itu membayar harga umum. Tidak tampil sama sekali
 * selama toko belum membuat daftar harga (Barang › Master › Daftar Harga).
 */
export function IsianHargaKhusus({
  daftar,
  nilai,
  onNilai,
  hargaBeli,
}: {
  daftar: DaftarHarga[]
  /** id daftar harga → harga (0 = tidak diisi). */
  nilai: Record<string, number>
  onNilai: (v: Record<string, number>) => void
  hargaBeli: number
}) {
  const adaIsi = Object.values(nilai).some((n) => n > 0)
  const [buka, setBuka] = useState(false)
  if (daftar.length === 0) return null

  if (!adaIsi && !buka) {
    return (
      <button
        type="button"
        onClick={() => setBuka(true)}
        className="-mx-2 flex min-h-11 items-center gap-1 self-start rounded-kontrol px-2 text-label font-medium text-utama hover:bg-sorot"
      >
        <Plus className="h-4 w-4" aria-hidden />
        Harga khusus ({daftar.map((d) => d.name).join(', ')})
      </button>
    )
  }

  return (
    // min-w-0: <fieldset> bawaan ber-min-width min-content (lihat isian-grosir).
    <fieldset className="flex min-w-0 flex-col gap-2 rounded-kontrol border border-garis p-3">
      <legend className="px-1 text-label font-medium text-teks-sekunder">Harga khusus</legend>
      <p className="-mt-1 text-keterangan text-teks-redup">
        Berlaku bila transaksinya atas nama pelanggan daftar itu. Kosongkan = harga umum.
      </p>
      {daftar.map((d) => {
        const n = nilai[d.id] ?? 0
        return (
          <div key={d.id} className="flex flex-col gap-1">
            <div className="flex items-center gap-2">
              <span className="w-24 shrink-0 truncate text-label text-teks-sekunder">{d.name}</span>
              <label className="flex h-12 min-w-0 flex-1 items-center gap-1 rounded-kontrol border border-garis bg-permukaan px-3 focus-within:outline focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-utama">
                <span className="text-teks-redup" aria-hidden>
                  Rp
                </span>
                <input
                  inputMode="numeric"
                  value={n === 0 ? '' : formatAngka(n)}
                  onChange={(e) => onNilai({ ...nilai, [d.id]: parseRupiah(e.target.value) })}
                  placeholder="harga umum"
                  aria-label={`Harga khusus ${d.name}`}
                  className="h-full min-w-0 flex-1 bg-transparent text-isi tabular-nums text-teks-utama outline-none placeholder:text-teks-redup"
                />
              </label>
            </div>
            {n > 0 && hargaBeli > 0 && n < hargaBeli && (
              <p className="text-keterangan text-bahaya-teks">Di bawah harga beli — rugi.</p>
            )}
          </div>
        )
      })}
    </fieldset>
  )
}
