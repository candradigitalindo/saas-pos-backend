import { useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ulid } from 'ulid'
import { Info, Plus, Trash2 } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { StepperJumlah } from '@/bersama/ui/stepper-jumlah'
import { Tombol } from '@/bersama/ui/tombol'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { usePemasok } from '@/bersama/hooks/use-katalog'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah, pratinjauBaris } from '@/bersama/util/uang'
import type { Produk } from '@/bersama/tipe/katalog'
import { PemilihBarang } from '../komponen/pemilih-barang'
import { stokApi } from '../api'

interface BarisMasuk {
  produk: Produk
  qty: string
  hargaBeli: number
}

/**
 * Barang masuk (ui/05-ALUR-UTAMA.md §4).
 *
 * Setelah tersimpan, pemilik diberi tahu bahwa HARGA MODAL IKUT DIPERBARUI —
 * backend mengambil cost_price dari pembelian terakhir. Tanpa diberi tahu,
 * pemilik akan bingung kenapa angka untungnya tiba-tiba berubah.
 */
export function HalamanBarangMasuk() {
  const { tokoAktif } = useSesi()
  const navigate = useNavigate()
  const toast = useToast()
  const qc = useQueryClient()
  const pemasok = usePemasok()

  const [pemasokId, setPemasokId] = useState('')
  const [noNota, setNoNota] = useState('')
  const [baris, setBaris] = useState<BarisMasuk[]>([])
  const [bukaPemilih, setBukaPemilih] = useState(false)
  const [galat, setGalat] = useState<string | null>(null)

  // Kunci idempotensi dibuat sekali per formulir: barang masuk menciptakan
  // stok DAN utang ke pemasok, jadi tidak boleh tercatat dua kali.
  const kunci = useRef(ulid())

  const total = baris.reduce((j, b) => j + pratinjauBaris(b.hargaBeli, b.qty), 0)

  const simpan = useMutation({
    mutationFn: () =>
      stokApi.barangMasuk(
        {
          outlet_id: tokoAktif!,
          supplier_id: pemasokId || undefined,
          invoice_no: noNota.trim() || undefined,
          items: baris.map((b) => ({
            product_id: b.produk.id,
            qty: b.qty,
            unit_cost: b.hargaBeli,
          })),
        },
        kunci.current,
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['stok'] })
      qc.invalidateQueries({ queryKey: ['stok-kasir'] })
      qc.invalidateQueries({ queryKey: ['kartu-stok'] })
      qc.invalidateQueries({ queryKey: ['produk'] })

      const ringkas = baris
        .map((b) => `${b.qty} ${b.produk.unit_name ?? ''} ${b.produk.name}`.trim())
        .join(', ')
      toast.berhasil(`${ringkas} masuk. Harga modal diperbarui otomatis.`)
      navigate('/stok')
    },
    onError: (e) =>
      setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  function tambahBaris(p: Produk) {
    setBukaPemilih(false)
    setBaris((lama) =>
      lama.some((b) => b.produk.id === p.id)
        ? lama
        : [...lama, { produk: p, qty: '1', hargaBeli: p.cost_price }],
    )
  }

  return (
    <div className="mx-auto flex w-full max-w-lg flex-col gap-4">
      <h1 className="text-judul font-bold text-teks-utama">Barang Masuk</h1>

      <Kartu className="flex flex-col gap-4 p-4">
        <Pilihan
          label="Dari pemasok"
          value={pemasokId}
          onChange={(e) => setPemasokId(e.target.value)}
          bantuan="Boleh dikosongkan bila beli sendiri di pasar."
        >
          <option value="">Tanpa pemasok</option>
          {pemasok.data?.data.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </Pilihan>

        <Kolom
          label="Nomor nota"
          value={noNota}
          onChange={(e) => setNoNota(e.target.value)}
          bantuan="Boleh dikosongkan."
        />
      </Kartu>

      <section className="flex flex-col gap-2">
        <div className="flex items-center justify-between">
          <h2 className="text-judul-kartu font-semibold text-teks-utama">Barang</h2>
          <Tombol jenis="kedua" ukuran="padat" onClick={() => setBukaPemilih(true)}>
            <Plus className="h-5 w-5" aria-hidden />
            Tambah
          </Tombol>
        </div>

        {baris.length === 0 ? (
          <Kartu className="p-6 text-center">
            <p className="text-isi text-teks-redup">
              Belum ada barang. Ketuk &ldquo;Tambah&rdquo; untuk memilih.
            </p>
          </Kartu>
        ) : (
          <ul className="flex flex-col gap-2">
            {baris.map((b, i) => (
              <li key={b.produk.id}>
                <Kartu className="flex flex-col gap-3 p-4">
                  <div className="flex items-start justify-between gap-3">
                    <p className="min-w-0 font-semibold text-teks-utama">
                      {b.produk.name}
                    </p>
                    <button
                      type="button"
                      onClick={() =>
                        setBaris((l) => l.filter((x) => x.produk.id !== b.produk.id))
                      }
                      aria-label={`Hapus ${b.produk.name}`}
                      className="-m-2 shrink-0 rounded-kontrol p-2 text-bahaya-teks hover:bg-red-50"
                    >
                      <Trash2 className="h-5 w-5" aria-hidden />
                    </button>
                  </div>

                  <StepperJumlah
                    nilai={b.qty}
                    onNilai={(q) =>
                      setBaris((l) => l.map((x, j) => (j === i ? { ...x, qty: q } : x)))
                    }
                    satuan={b.produk.unit_name}
                    minimal="1"
                    label={`Jumlah ${b.produk.name}`}
                  />

                  <KolomUang
                    label="Harga beli satuan"
                    nilai={b.hargaBeli}
                    onNilai={(n) =>
                      setBaris((l) => l.map((x, j) => (j === i ? { ...x, hargaBeli: n } : x)))
                    }
                    bantuan="Modal per satuan dari pemasok."
                  />

                  <div className="flex items-baseline justify-between border-t border-garis pt-2">
                    <span className="text-label text-teks-sekunder">Subtotal</span>
                    <span className="font-bold tabular-nums text-teks-utama">
                      {formatRupiah(pratinjauBaris(b.hargaBeli, b.qty))}
                    </span>
                  </div>
                </Kartu>
              </li>
            ))}
          </ul>
        )}
      </section>

      {baris.length > 0 && (
        <Kartu className="flex flex-col gap-3 p-4">
          <div className="flex items-baseline justify-between">
            <span className="text-isi text-teks-sekunder">Total bayar</span>
            <span className="text-judul font-extrabold tabular-nums text-teks-utama">
              {formatRupiah(total)}
            </span>
          </div>

          <p className="flex items-start gap-2 rounded-kontrol bg-blue-50 px-3 py-2 text-keterangan text-info-teks">
            <Info className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
            Harga modal barang ini akan diperbarui otomatis mengikuti harga beli
            di atas. Itu yang dipakai menghitung untung Anda.
          </p>

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-red-50 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <Tombol
            lebarPenuh
            memuat={simpan.isPending}
            labelMemuat="Menyimpan…"
            onClick={() => {
              setGalat(null)
              simpan.mutate()
            }}
          >
            Simpan Barang Masuk
          </Tombol>
        </Kartu>
      )}

      <PemilihBarang
        terbuka={bukaPemilih}
        onTutup={() => setBukaPemilih(false)}
        onPilih={tambahBaris}
        judul="Pilih barang yang masuk"
      />
    </div>
  )
}
