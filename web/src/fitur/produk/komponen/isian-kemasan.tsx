import { Plus, X } from 'lucide-react'
import type { Satuan } from '@/bersama/tipe/katalog'
import { formatAngka, formatRupiah, parseRupiah } from '@/bersama/util/uang'

const MAKS_KEMASAN = 5

/** Satu baris isian kemasan (belum divalidasi). */
export interface BarisKemasan {
  unit_id: string
  /** Isi dalam satuan dasar, apa adanya dari kolom. */
  conversion: string
  /** 0 = otomatis (isi × harga jual). */
  sell_price: number
  barcode: string
}

const kelasKotak =
  'h-12 min-w-0 rounded-kontrol border border-garis bg-permukaan px-2 text-isi text-teks-utama focus:outline focus:outline-2 focus:outline-offset-2 focus:outline-utama'

/**
 * Kemasan barang: "dus isi 40". Dipakai kasir (jual per dus) dan barang masuk
 * (beli per dus); stok & laporan tetap dalam satuan dasar. Tersembunyi di
 * balik "+ Kemasan" selama kosong.
 */
export function IsianKemasan({
  nilai,
  onNilai,
  satuan,
  satuanDasar,
  hargaJual,
}: {
  nilai: BarisKemasan[]
  onNilai: (v: BarisKemasan[]) => void
  /** Semua satuan toko. */
  satuan: Satuan[]
  /** id satuan dasar barang ini (tidak boleh jadi kemasan). */
  satuanDasar: string
  hargaJual: number
}) {
  const namaDasar = satuan.find((s) => s.id === satuanDasar)?.name ?? 'satuan'
  const pilihan = satuan.filter((s) => s.id !== satuanDasar)
  const ubah = (i: number, isi: Partial<BarisKemasan>) => onNilai(nilai.map((k, j) => (j === i ? { ...k, ...isi } : k)))
  const tambah = () => onNilai([...nilai, { unit_id: '', conversion: '', sell_price: 0, barcode: '' }])

  if (nilai.length === 0) {
    return (
      <button
        type="button"
        onClick={tambah}
        className="-mx-2 flex min-h-11 items-center gap-1 self-start rounded-kontrol px-2 text-label font-medium text-utama hover:bg-sorot"
      >
        <Plus className="h-4 w-4" aria-hidden />
        Kemasan (dus, pak) — jual & beli per kemasan
      </button>
    )
  }

  return (
    // min-w-0: <fieldset> bawaan ber-min-width min-content (lihat isian-grosir).
    <fieldset className="flex min-w-0 flex-col gap-3 rounded-kontrol border border-garis p-3">
      <legend className="px-1 text-label font-medium text-teks-sekunder">Kemasan</legend>
      <p className="-mt-2 text-keterangan text-teks-redup">
        Jual & beli per kemasan; stok tetap dihitung dalam {namaDasar}.
        {pilihan.length === 0 && ' Buat dulu satuan kemasannya (mis. dus) di Barang › Master › Satuan.'}
      </p>
      {nilai.map((k, i) => {
        const isi = Number(k.conversion.replace(',', '.'))
        const otomatis = Number.isFinite(isi) && isi > 1 && hargaJual > 0 ? Math.round(isi * hargaJual) : 0
        return (
          <div key={i} className="flex flex-col gap-2 border-b border-garis pb-3 last:border-0 last:pb-0">
            <div className="flex items-center gap-2 text-label text-teks-sekunder">
              <select
                value={k.unit_id}
                onChange={(e) => ubah(i, { unit_id: e.target.value })}
                aria-label={`Satuan kemasan ${i + 1}`}
                className={`${kelasKotak} w-24 shrink-0`}
              >
                <option value="">Satuan…</option>
                {pilihan.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </select>
              <span className="shrink-0">isi</span>
              <input
                inputMode="decimal"
                value={k.conversion}
                onChange={(e) => ubah(i, { conversion: e.target.value.replace(/[^\d.,]/g, '') })}
                placeholder="40"
                aria-label={`Isi kemasan ${i + 1} dalam ${namaDasar}`}
                className={`${kelasKotak} w-16 shrink-0 text-center tabular-nums`}
              />
              <span className="min-w-0 flex-1 truncate">{namaDasar}</span>
              <button
                type="button"
                onClick={() => onNilai(nilai.filter((_, j) => j !== i))}
                aria-label={`Hapus kemasan ${i + 1}`}
                className="-mr-1 flex h-11 w-11 shrink-0 items-center justify-center rounded-kontrol text-teks-redup hover:bg-permukaan-2"
              >
                <X className="h-4 w-4" aria-hidden />
              </button>
            </div>
            <div className="flex items-center gap-2">
              <label className="flex h-12 min-w-0 flex-1 items-center gap-1 rounded-kontrol border border-garis bg-permukaan px-2 focus-within:outline focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-utama">
                <span className="text-teks-redup" aria-hidden>
                  Rp
                </span>
                <input
                  inputMode="numeric"
                  value={k.sell_price === 0 ? '' : formatAngka(k.sell_price)}
                  onChange={(e) => ubah(i, { sell_price: parseRupiah(e.target.value) })}
                  placeholder={otomatis > 0 ? formatAngka(otomatis) : 'harga'}
                  aria-label={`Harga jual kemasan ${i + 1}`}
                  className="h-full min-w-0 flex-1 bg-transparent text-isi tabular-nums text-teks-utama outline-none placeholder:text-teks-redup"
                />
              </label>
              <input
                value={k.barcode}
                onChange={(e) => ubah(i, { barcode: e.target.value })}
                placeholder="Barcode (boleh kosong)"
                aria-label={`Barcode kemasan ${i + 1}`}
                className={`${kelasKotak} w-0 flex-1`}
              />
            </div>
            {k.sell_price === 0 && otomatis > 0 && (
              <p className="-mt-1 text-keterangan text-teks-redup">
                Kosong = {formatRupiah(otomatis)} (isi × harga jual).
              </p>
            )}
          </div>
        )
      })}
      {nilai.length < MAKS_KEMASAN && (
        <button
          type="button"
          onClick={tambah}
          className="-mx-1 flex min-h-11 items-center gap-1 self-start rounded-kontrol px-1 text-label font-medium text-utama hover:bg-sorot"
        >
          <Plus className="h-4 w-4" aria-hidden />
          Tambah kemasan
        </button>
      )}
    </fieldset>
  )
}

/** Kemasan siap kirim, atau pesan galat. Baris yang masih kosong dibuang. */
export function siapkanKemasan(
  nilai: BarisKemasan[],
): { kemasan: { unit_id: string; conversion: string; sell_price?: number; barcode?: string }[] } | { galat: string } {
  const isi = nilai.filter((k) => k.unit_id || k.conversion.trim() || k.sell_price > 0 || k.barcode.trim())
  const sudah = new Set<string>()
  const keluar = []
  for (const k of isi) {
    if (!k.unit_id) return { galat: 'Pilih satuan untuk setiap kemasan.' }
    if (sudah.has(k.unit_id)) return { galat: 'Satu satuan kemasan diisi dua kali.' }
    sudah.add(k.unit_id)
    const conv = k.conversion.trim().replace(',', '.')
    if (!(Number(conv) > 1)) return { galat: 'Isi kemasan harus lebih dari 1.' }
    keluar.push({
      unit_id: k.unit_id,
      conversion: conv,
      ...(k.sell_price > 0 ? { sell_price: k.sell_price } : {}),
      ...(k.barcode.trim() ? { barcode: k.barcode.trim() } : {}),
    })
  }
  return { kemasan: keluar }
}
