import { useDeferredValue, useState } from 'react'
import { Link } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { ChevronRight, MessageCircle, Plus, Search, ShoppingBasket, Truck } from 'lucide-react'
import { Tombol } from '@/bersama/ui/tombol'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { IZIN } from '@/lib/izin'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatLaluHari } from '@/bersama/util/tanggal'
import { inisialNama } from '@/bersama/util/inisial'
import { kelasAvatar } from '@/bersama/util/warna-kategori'
import { cn } from '@/bersama/util/cn'
import type { Pemasok, SaldoStok } from '@/bersama/tipe/katalog'
import { nomorWA } from '@/fitur/kasir/struk-wa'
import { stokApi } from '@/fitur/stok/api'
import { Kartu } from '@/bersama/ui/kartu'
import { pemasokApi, type StatPemasok } from '../api'
import { DialogPemasok } from '../komponen/dialog-pemasok'

/**
 * Daftar pemasok: kontak dan angka belanja di toko aktif — belanja 30 hari,
 * utang (merah bila ada yang lewat jatuh tempo), kapan terakhir belanja.
 * Yang terakhir dibeli di atas: itulah yang paling mungkin dihubungi lagi.
 *
 * Dulu pemasok hanya bisa dibuat dengan NAMA saja di Kategori & Satuan;
 * nomor WhatsApp, alamat, dan catatan tidak bisa diisi dari layar mana pun.
 */
export function HalamanPemasok() {
  const { tokoAktif, boleh } = useSesi()
  const [cari, setCari] = useState('')
  const cariTunda = useDeferredValue(cari.trim())
  const [tambah, setTambah] = useState(false)

  const q = useQuery({
    queryKey: ['pemasok', 'daftar', cariTunda],
    queryFn: () => pemasokApi.daftar(cariTunda),
  })
  const stat = useQuery({
    queryKey: ['pemasok', 'statistik', tokoAktif],
    queryFn: () => pemasokApi.statistik(tokoAktif!),
    enabled: !!tokoAktif && boleh(IZIN.stockView),
  })
  const perStat = new Map((stat.data ?? []).map((s) => [s.supplier_id, s]))

  // Terakhir dibeli dulu; yang belum pernah dibeli di belakang, urut nama.
  const daftar = [...(q.data?.data ?? [])].sort((a, b) => {
    const ta = perStat.get(a.id)?.last_purchase_at ?? ''
    const tb = perStat.get(b.id)?.last_purchase_at ?? ''
    return tb.localeCompare(ta) || a.name.localeCompare(b.name)
  })

  return (
    <div className="flex w-full max-w-4xl flex-col gap-4">
      <header className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-judul font-bold text-teks-utama">Pemasok</h1>
          <p className="text-label text-teks-sekunder">Kontak, belanja, dan utang ke tiap pemasok.</p>
        </div>
        {boleh(IZIN.productEdit) && (
          <Tombol onClick={() => setTambah(true)} className="shrink-0 px-4">
            <Plus className="h-5 w-5" aria-hidden />
            Pemasok
          </Tombol>
        )}
      </header>

      <div className="relative">
        <Search
          className="pointer-events-none absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-teks-redup"
          aria-hidden
        />
        <input
          type="search"
          value={cari}
          onChange={(e) => setCari(e.target.value)}
          placeholder="Cari nama atau nomor…"
          aria-label="Cari pemasok"
          className="h-12 w-full rounded-kontrol border border-garis bg-permukaan pl-10 pr-3 text-isi text-teks-utama placeholder:text-teks-redup"
        />
      </div>

      {!cari && boleh(IZIN.stockView) && <PerluDipesan />}

      {q.isLoading ? (
        <KerangkaBaris jumlah={4} />
      ) : q.isError ? (
        <KeadaanGagal
          pesan={q.error instanceof GalatAPI ? q.error.pesan : 'Pemasok belum bisa dimuat.'}
          onCobaLagi={() => q.refetch()}
        />
      ) : daftar.length === 0 ? (
        <KeadaanKosong
          ikon={Truck}
          judul={cari ? 'Pemasok tidak ditemukan' : 'Belum ada pemasok'}
          penjelasan={
            cari
              ? `Tidak ada pemasok bernama "${cari}".`
              : 'Catat pemasok beserta nomor WhatsApp-nya supaya bisa memesan ulang dan melacak utang.'
          }
          aksi={boleh(IZIN.productEdit) && !cari ? { label: 'Tambah Pemasok', onKlik: () => setTambah(true) } : undefined}
        />
      ) : (
        <ul className="grid gap-2 sm:grid-cols-2">
          {daftar.map((p) => (
            <li key={p.id}>
              <KartuPemasok p={p} s={perStat.get(p.id)} />
            </li>
          ))}
        </ul>
      )}

      <DialogPemasok terbuka={tambah} onTutup={() => setTambah(false)} />
    </div>
  )
}

function KartuPemasok({ p, s }: { p: Pemasok; s?: StatPemasok }) {
  const wa = p.phone ? nomorWA(p.phone) : null
  return (
    <div className="relative flex h-full flex-col gap-3 rounded-kartu border border-garis bg-permukaan p-4 shadow-kartu transition-colors hover:border-utama/40">
      <div className="flex items-start gap-3">
        <span
          className={cn(
            'flex h-11 w-11 shrink-0 items-center justify-center rounded-full text-label font-bold',
            kelasAvatar(p.id),
          )}
          aria-hidden
        >
          {inisialNama(p.name)}
        </span>
        <div className="min-w-0 flex-1">
          {/* Seluruh kartu membuka rinciannya (tautan terentang). */}
          <Link
            to={`/pemasok/${p.id}`}
            className="block truncate font-semibold text-teks-utama after:absolute after:inset-0 after:rounded-kartu focus-visible:outline-none focus-visible:after:ring-2 focus-visible:after:ring-utama"
          >
            {p.name}
          </Link>
          <p className="truncate text-keterangan text-teks-redup">
            {[p.phone, p.address].filter(Boolean).join(' · ') || 'Belum ada kontak'}
          </p>
        </div>
        {wa && (
          <a
            href={`https://wa.me/${wa}`}
            target="_blank"
            rel="noreferrer"
            aria-label={`WhatsApp ${p.name}`}
            className="relative z-10 flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-sorot text-hijau-800 hover:brightness-95"
          >
            <MessageCircle className="h-5 w-5" aria-hidden />
          </a>
        )}
      </div>
      <dl className="grid grid-cols-3 gap-2 border-t border-garis pt-3 text-keterangan">
        <div>
          <dt className="text-teks-redup">Belanja 30 hr</dt>
          <dd className="font-semibold tabular-nums text-teks-utama">{s ? formatRupiah(s.spent_30d) : '—'}</dd>
        </div>
        <div>
          <dt className="text-teks-redup">Utang</dt>
          <dd
            className={cn(
              'font-semibold tabular-nums',
              !s?.outstanding ? 'text-teks-redup' : s.overdue_count > 0 ? 'text-bahaya-teks' : 'text-jingga-700',
            )}
          >
            {s?.outstanding ? formatRupiah(s.outstanding) : 'Lunas'}
          </dd>
        </div>
        <div>
          <dt className="text-teks-redup">Terakhir</dt>
          <dd className="font-semibold text-teks-utama">
            {s?.last_purchase_at ? formatLaluHari(s.last_purchase_at) : 'Belum pernah'}
          </dd>
        </div>
      </dl>
    </div>
  )
}

/**
 * "Perlu dipesan": barang di bawah batas / habis dalam seminggu (saran
 * belanja), dikelompokkan menurut PEMASOK UTAMA-nya — satu ketukan membuka
 * "Pesan lagi" pemasok itu dengan barangnya sudah tercentang. Barang tanpa
 * pemasok utama disebut terpisah, dengan arahan mengaturnya.
 */
function PerluDipesan() {
  const { tokoAktif } = useSesi()
  const q = useQuery({
    queryKey: ['stok', tokoAktif, 'saran-belanja'],
    queryFn: () => stokApi.saldo(tokoAktif!, false, 1, 100, { keadaan: 'restock', urut: 'urgent' }),
    enabled: !!tokoAktif,
    staleTime: 30_000,
  })
  const daftar = q.data?.data ?? []
  if (!daftar.length) return null

  const kelompok = new Map<string, { nama: string; barang: SaldoStok[] }>()
  for (const s of daftar) {
    const k = s.supplier_id ?? ''
    const g = kelompok.get(k) ?? { nama: s.supplier_name || '', barang: [] }
    g.barang.push(s)
    kelompok.set(k, g)
  }
  // Yang berpemasok dulu (bisa langsung dipesan), terbanyak dulu.
  const urut = [...kelompok.entries()].sort(
    ([a, x], [b, y]) => Number(a === '') - Number(b === '') || y.barang.length - x.barang.length,
  )

  return (
    <Kartu className="flex flex-col gap-1 p-4">
      <h2 className="flex items-center gap-2 text-judul-kartu font-semibold text-teks-utama">
        <ShoppingBasket className="h-5 w-5 text-utama" aria-hidden />
        Perlu dipesan
      </h2>
      <p className="text-keterangan text-teks-redup">
        Di bawah batas minimum atau habis dalam seminggu — dikelompokkan menurut pemasok utamanya.
      </p>
      <ul className="-mx-1 mt-1 divide-y divide-garis">
        {urut.map(([id, g]) => {
          const nama = g.barang.slice(0, 3).map((b) => b.product_name).join(', ')
          const lebih = g.barang.length > 3 ? ` +${g.barang.length - 3} lagi` : ''
          const isi = (
            <>
              <span className="min-w-0 flex-1">
                <span className="block font-medium text-teks-utama">{id ? g.nama : 'Tanpa pemasok utama'}</span>
                <span className="block truncate text-keterangan text-teks-sekunder">
                  {g.barang.length} barang: {nama}
                  {lebih}
                </span>
                {!id && (
                  <span className="block text-keterangan text-jingga-700">
                    Atur pemasok utamanya di formulir barang supaya bisa dipesan dari sini.
                  </span>
                )}
              </span>
              {id && (
                <span className="flex shrink-0 items-center gap-0.5 text-label font-semibold text-utama">
                  Pesan
                  <ChevronRight className="h-4 w-4" aria-hidden />
                </span>
              )}
            </>
          )
          return (
            <li key={id || '-'}>
              {id ? (
                <Link to={`/pemasok/${id}`} className="flex min-h-12 items-center gap-3 rounded-kontrol px-1 py-2.5 hover:bg-permukaan-2/60">
                  {isi}
                </Link>
              ) : (
                <div className="flex items-center gap-3 px-1 py-2.5">{isi}</div>
              )}
            </li>
          )
        })}
      </ul>
    </Kartu>
  )
}
