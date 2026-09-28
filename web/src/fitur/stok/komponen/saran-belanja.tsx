import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { Check, ChevronDown, Plus, ShoppingBasket } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { FotoBarang } from '@/bersama/komponen/foto-barang'
import { Kerangka } from '@/bersama/komponen/kerangka'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useKategori } from '@/bersama/hooks/use-katalog'
import { formatQtySatuan } from '@/bersama/util/desimal'
import { kelasPetak, petaWarnaKategori } from '@/bersama/util/warna-kategori'
import { cn } from '@/bersama/util/cn'
import type { SaldoStok } from '@/bersama/tipe/katalog'
import { stokApi } from '../api'
import { cukupHari, keadaanSaldo, saranBeli, teksCukup } from '../keadaan-stok'

/** Baris yang tampil sebelum "Lihat semua". */
const RINGKAS = 5

/**
 * Saran belanja di layar Barang Masuk: barang yang sudah di bawah batas
 * minimum, ATAU yang menurut laju jualnya habis dalam seminggu (GET
 * /stocks?status=restock) — lengkap dengan saran jumlah beli untuk dua minggu.
 *
 * Dipakai dua arah: sebelum ke pasar sebagai daftar belanja, dan sepulang
 * dari pasar sebagai jalan pintas mengisi formulir (sekali ketuk = barang
 * masuk daftar dengan jumlah sarannya, tinggal disesuaikan).
 *
 * `terlipat`: saran menyusut jadi satu baris supaya tombol Simpan tidak
 * terdorong jauh ke bawah — pemanggil melipatnya bila formulir diisi lewat
 * pemilih barang, TIDAK saat orang sedang mengambil dari saran ini. Pengguna
 * tetap bisa membuka/menyembunyikannya sendiri.
 */
export function SaranBelanja({
  sudah,
  onTambah,
  terlipat: terlipatAwal = false,
  bisaDilipat = false,
}: {
  /** product_id yang sudah ada di formulir. */
  sudah: Set<string>
  onTambah: (daftar: { saldo: SaldoStok; qty: number }[]) => void
  terlipat?: boolean
  /** Tampilkan tautan "Sembunyikan" (formulir sudah berisi). */
  bisaDilipat?: boolean
}) {
  const { tokoAktif } = useSesi()
  const [lipat, setLipat] = useState<boolean | null>(null)
  const [semua, setSemua] = useState(false)
  const terlipat = lipat ?? terlipatAwal

  const q = useQuery({
    queryKey: ['stok', tokoAktif, 'saran-belanja'],
    queryFn: () => stokApi.saldo(tokoAktif!, false, 1, 100, { keadaan: 'restock', urut: 'urgent' }),
    enabled: !!tokoAktif,
    staleTime: 30_000,
  })
  const kat = useKategori()
  const warna = petaWarnaKategori(kat.data?.data ?? [])

  if (q.isLoading) return <Kerangka className="h-40 w-full rounded-kartu" />
  const daftar = q.data?.data ?? []
  if (daftar.length === 0) return null

  const belum = daftar.filter((s) => !sudah.has(s.product_id))
  const tampil = semua ? daftar : daftar.slice(0, RINGKAS)

  if (terlipat) {
    return (
      <button
        type="button"
        onClick={() => setLipat(false)}
        className="flex min-h-12 items-center gap-2 rounded-kartu border border-dashed border-garis px-4 text-left text-label text-teks-sekunder hover:bg-permukaan-2/50"
      >
        <ShoppingBasket className="h-4 w-4 shrink-0 text-utama" aria-hidden />
        <span className="flex-1">
          Saran belanja ·{' '}
          <strong className="font-semibold text-teks-utama">
            {belum.length ? `${belum.length} barang belum di daftar` : 'semua sudah di daftar'}
          </strong>
        </span>
        <ChevronDown className="h-4 w-4 shrink-0" aria-hidden />
      </button>
    )
  }

  return (
    <Kartu className="flex flex-col gap-1 p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <h2 className="flex items-center gap-2 text-judul-kartu font-semibold text-teks-utama">
            <ShoppingBasket className="h-5 w-5 shrink-0 text-utama" aria-hidden />
            Perlu dibeli
            <span className="text-label font-medium tabular-nums text-teks-redup">{daftar.length}</span>
          </h2>
          <p className="text-keterangan text-teks-redup">
            Di bawah batas minimum, atau habis dalam seminggu. Jumlahnya perkiraan untuk dua minggu.
          </p>
        </div>
        {belum.length > 1 && (
          <Tombol
            jenis="kedua"
            ukuran="padat"
            onClick={() => onTambah(belum.map((s) => ({ saldo: s, qty: saranBeli(s) })))}
          >
            <Plus className="h-4 w-4" aria-hidden />
            Semua
          </Tombol>
        )}
      </div>

      <ul className="-mx-1 divide-y divide-garis">
        {tampil.map((s) => {
          const ada = sudah.has(s.product_id)
          const qty = saranBeli(s)
          return (
            <li key={s.product_id}>
              <button
                type="button"
                disabled={ada}
                onClick={() => onTambah([{ saldo: s, qty }])}
                aria-label={
                  ada
                    ? `${s.product_name} sudah di daftar`
                    : `Tambah ${s.product_name}, ${formatQtySatuan(String(qty), s.unit_name)}`
                }
                className="flex min-h-14 w-full items-center gap-3 rounded-kontrol px-1 py-2 text-left enabled:hover:bg-permukaan-2/60"
              >
                <FotoBarang
                  nama={s.product_name}
                  url={s.image_url}
                  kecil
                  kelasWarna={kelasPetak(warna, s.category_id)}
                  className="w-10"
                />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium text-teks-utama">{s.product_name}</span>
                  <Alasan s={s} />
                </span>
                {ada ? (
                  <span className="flex shrink-0 items-center gap-1 text-keterangan font-medium text-utama">
                    <Check className="h-4 w-4" aria-hidden />
                    Di daftar
                  </span>
                ) : (
                  <span className="flex shrink-0 items-center gap-2">
                    <span className="text-right text-keterangan leading-tight text-teks-redup">
                      beli
                      <span className="block text-label font-semibold tabular-nums text-teks-utama">
                        ±{formatQtySatuan(String(qty), s.unit_name)}
                      </span>
                    </span>
                    <span className="flex h-9 w-9 items-center justify-center rounded-full bg-sorot text-hijau-800">
                      <Plus className="h-5 w-5" aria-hidden />
                    </span>
                  </span>
                )}
              </button>
            </li>
          )
        })}
      </ul>

      <div className="flex flex-wrap items-center justify-between gap-2">
        {daftar.length > RINGKAS ? (
          <button
            type="button"
            onClick={() => setSemua((v) => !v)}
            className="min-h-11 text-label font-medium text-utama hover:underline"
          >
            {semua ? 'Tampilkan lebih sedikit' : `Lihat semua ${daftar.length}`}
          </button>
        ) : (
          <span />
        )}
        {bisaDilipat && (
          <button
            type="button"
            onClick={() => setLipat(true)}
            className="min-h-11 text-label font-medium text-teks-sekunder hover:underline"
          >
            Sembunyikan
          </button>
        )}
      </div>
    </Kartu>
  )
}

/** Kenapa barang ini disarankan — satu baris, bernada sesuai keadaannya. */
function Alasan({ s }: { s: SaldoStok }) {
  const k = keadaanSaldo(s)
  const kelas = 'block truncate text-keterangan'
  if (k === 'negative') return <span className={cn(kelas, 'text-bahaya-teks')}>Perlu dicocokkan</span>
  if (k === 'out') return <span className={cn(kelas, 'font-medium text-bahaya-teks')}>Habis</span>
  const hari = cukupHari(s)
  return (
    <span className={cn(kelas, 'text-jingga-700')}>
      sisa {formatQtySatuan(s.qty, s.unit_name)}
      {k === 'low' && Number.parseFloat(s.min_stock) > 0 && ` · batas ${formatQtySatuan(s.min_stock, s.unit_name)}`}
      {k !== 'low' && hari !== null && ` · ${teksCukup(hari)}`}
    </span>
  )
}
