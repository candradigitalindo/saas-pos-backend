import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowRight, ChevronDown, PackageCheck, Plus, Store, Trash2, TriangleAlert, Truck } from 'lucide-react'
import { IZIN } from '@/lib/izin'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { SegmenPilihan } from '@/bersama/ui/segmen'
import { StepperJumlah } from '@/bersama/ui/stepper-jumlah'
import { Tombol } from '@/bersama/ui/tombol'
import { FotoBarang } from '@/bersama/komponen/foto-barang'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka, KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useKategori } from '@/bersama/hooks/use-katalog'
import { api, GalatAPI, type Halaman } from '@/lib/api-client'
import { formatQty, formatQtySatuan } from '@/bersama/util/desimal'
import { formatTanggalJam } from '@/bersama/util/tanggal'
import { kelasPetak, petaWarnaKategori } from '@/bersama/util/warna-kategori'
import { cn } from '@/bersama/util/cn'
import type { Produk } from '@/bersama/tipe/katalog'
import type { Toko } from '@/bersama/tipe/organisasi'
import { PemilihBarang } from '../komponen/pemilih-barang'
import { stokApi, type Transfer } from '../api'

const PER_HALAMAN = 20

interface BarisKirim {
  produk: Produk
  qty: string
}

type Arah = 'semua' | 'keluar' | 'masuk'

/**
 * Kirim barang antar toko.
 *
 * Tiga keadaan yang ditampilkan apa adanya: disiapkan → di jalan → diterima.
 * Stok toko asal berkurang saat barang BERANGKAT, dan toko tujuan baru
 * bertambah saat MENERIMA — barang yang masih di jalan tidak boleh terlihat
 * seolah sudah sampai.
 *
 * - Kiriman yang sedang di jalan KE toko ini tampil paling atas dengan tombol
 *   "Terima Barang": itulah satu-satunya hal yang ditunggu toko tujuan.
 * - Tiap barang yang akan dikirim menyebut sisanya di toko ini, dan memberi
 *   tahu bila jumlah kirim melebihi sisa (stok toko ini akan minus).
 * - Daftar pengiriman menyebut NAMA barang & toko. Versi sebelumnya menulis
 *   potongan id mentah ("3 × 01M3M0PM…") karena baris transfer hanya
 *   menyimpan product_id.
 */
export function HalamanTransfer() {
  const { tokoAktif, boleh } = useSesi()
  const navigate = useNavigate()
  const toast = useToast()
  const qc = useQueryClient()
  const kat = useKategori()
  const warna = petaWarnaKategori(kat.data?.data ?? [])

  const [keToko, setKeToko] = useState('')
  const [catatan, setCatatan] = useState('')
  const [baris, setBaris] = useState<BarisKirim[]>([])
  const [bukaPemilih, setBukaPemilih] = useState(false)
  const [galat, setGalat] = useState<string | null>(null)

  const toko = useQuery({
    queryKey: ['outlets'],
    queryFn: () => api.get<Halaman<Toko>>('/outlets', { query: { limit: 100 } }),
  })
  const tokoLain = (toko.data?.data ?? []).filter((t) => t.id !== tokoAktif)
  const namaTokoAktif = toko.data?.data.find((t) => t.id === tokoAktif)?.name

  // Sisa di toko ini untuk barang yang akan dikirim.
  const ids = baris.map((b) => b.produk.id)
  const saldo = useQuery({
    queryKey: ['stok', tokoAktif, 'transfer', ids.join(',')],
    queryFn: () => stokApi.saldo(tokoAktif!, false, 1, 100, { produk: ids }),
    enabled: !!tokoAktif && ids.length > 0,
    staleTime: 30_000,
  })
  const sisa = new Map((saldo.data?.data ?? []).map((s) => [s.product_id, s.qty]))

  const segarkan = () => {
    qc.invalidateQueries({ queryKey: ['transfer'] })
    qc.invalidateQueries({ queryKey: ['stok'] })
    qc.invalidateQueries({ queryKey: ['stok-ringkasan'] })
    qc.invalidateQueries({ queryKey: ['stok-kasir'] })
    qc.invalidateQueries({ queryKey: ['kartu-stok'] })
  }

  const buat = useMutation({
    mutationFn: () =>
      stokApi.buatTransfer({
        from_outlet_id: tokoAktif!,
        to_outlet_id: keToko,
        note: catatan.trim() || undefined,
        items: baris.map((b) => ({ product_id: b.produk.id, qty: b.qty })),
      }),
    onSuccess: () => {
      segarkan()
      toast.berhasil('Pengiriman dibuat. Tekan "Barang Sudah Berangkat" saat barangnya dibawa.')
      setBaris([])
      setCatatan('')
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  const ubahStatus = useMutation({
    mutationFn: ({ id, aksi }: { id: string; aksi: 'kirim' | 'terima' }) =>
      aksi === 'kirim' ? stokApi.kirimTransfer(id) : stokApi.terimaTransfer(id),
    onSuccess: (_d, v) => {
      segarkan()
      toast.berhasil(
        v.aksi === 'kirim'
          ? 'Barang ditandai sudah berangkat. Stok toko ini berkurang.'
          : 'Barang diterima. Stok toko ini bertambah.',
      )
    },
    onError: (e) => toast.gagal(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })
  const aksi = {
    sedang: ubahStatus.isPending ? ubahStatus.variables?.id : undefined,
    jalankan: (id: string, a: 'kirim' | 'terima') => ubahStatus.mutate({ id, aksi: a }),
  }

  if (tokoLain.length === 0 && !toko.isLoading) {
    return (
      <KeadaanKosong
        ikon={Store}
        judul="Baru ada satu toko"
        penjelasan="Kirim barang antar toko baru berguna kalau Anda punya lebih dari satu cabang. Tambahkan cabang dulu di Pengaturan."
        aksi={
          boleh(IZIN.outletManage) ? { label: 'Tambah Cabang', onKlik: () => navigate('/pengaturan/toko') } : undefined
        }
      />
    )
  }

  const melebihi = baris.filter((b) => {
    const s = sisa.get(b.produk.id)
    return s !== undefined && Number.parseFloat(b.qty) > Number.parseFloat(s)
  })

  return (
    // Layar lebar: formulir pengiriman di kiri, daftar pengiriman di kanan.
    <div className="flex w-full max-w-lg flex-col gap-4 lg:grid lg:max-w-5xl lg:grid-cols-[minmax(0,26rem)_minmax(0,1fr)] lg:items-start lg:gap-x-6">
      <header className="lg:col-span-2">
        <h1 className="text-judul font-bold text-teks-utama">Kirim Antar Toko</h1>
        <p className="text-label text-teks-sekunder">
          Stok toko asal berkurang saat barang berangkat; toko tujuan bertambah saat menerima.
        </p>
      </header>

      <KirimanMasuk tokoAktif={tokoAktif} aksi={aksi} className="lg:col-span-2" />

      <Kartu className="flex flex-col gap-4 p-4">
        <div className="flex flex-col gap-1.5">
          <span className="text-label font-medium text-teks-sekunder">Dari</span>
          <span className="flex min-h-12 items-center gap-2 rounded-kontrol bg-permukaan-2/60 px-3 font-medium text-teks-utama">
            <Store className="h-4 w-4 text-teks-redup" aria-hidden />
            {namaTokoAktif ?? 'Toko ini'}
          </span>
        </div>

        <Pilihan label="Kirim ke" value={keToko} onChange={(e) => setKeToko(e.target.value)} required>
          <option value="">Pilih toko tujuan</option>
          {tokoLain.map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </Pilihan>

        <div className="flex items-center justify-between">
          <p className="text-label font-medium text-teks-sekunder">
            Barang
            {baris.length > 0 && <span className="ml-1.5 tabular-nums text-teks-redup">{baris.length}</span>}
          </p>
          <Tombol jenis="kedua" ukuran="padat" onClick={() => setBukaPemilih(true)}>
            <Plus className="h-5 w-5" aria-hidden />
            Tambah
          </Tombol>
        </div>

        {baris.length === 0 ? (
          <p className="rounded-kontrol border border-dashed border-garis py-5 text-center text-label text-teks-redup">
            Belum ada barang dipilih.
          </p>
        ) : (
          <ul className="flex flex-col gap-3">
            {baris.map((b, i) => {
              const s = sisa.get(b.produk.id)
              const lebih = s !== undefined && Number.parseFloat(b.qty) > Number.parseFloat(s)
              return (
                <li key={b.produk.id} className="flex flex-col gap-2 border-t border-garis pt-3">
                  <div className="flex items-center gap-3">
                    <FotoBarang
                      nama={b.produk.name}
                      url={b.produk.image_url}
                      kecil
                      kelasWarna={kelasPetak(warna, b.produk.category_id)}
                      className="w-10"
                    />
                    <div className="min-w-0 flex-1">
                      <p className="truncate font-medium text-teks-utama">{b.produk.name}</p>
                      {s !== undefined && (
                        <p
                          className={cn(
                            'text-keterangan tabular-nums',
                            lebih ? 'font-semibold text-bahaya-teks' : 'text-teks-redup',
                          )}
                        >
                          {lebih
                            ? `melebihi sisa di sini (${formatQtySatuan(s, b.produk.unit_name).replace('-', '−')})`
                            : `sisa di sini ${formatQtySatuan(s, b.produk.unit_name)}`}
                        </p>
                      )}
                    </div>
                    <button
                      type="button"
                      onClick={() => setBaris((l) => l.filter((_, j) => j !== i))}
                      aria-label={`Hapus ${b.produk.name}`}
                      className="-m-2 shrink-0 rounded-kontrol p-2 text-bahaya-teks hover:bg-bahaya-teks/10"
                    >
                      <Trash2 className="h-5 w-5" aria-hidden />
                    </button>
                  </div>
                  <StepperJumlah
                    nilai={b.qty}
                    onNilai={(q) => setBaris((l) => l.map((x, j) => (j === i ? { ...x, qty: q } : x)))}
                    satuan={b.produk.unit_name}
                    minimal="1"
                    label={`Jumlah ${b.produk.name}`}
                  />
                </li>
              )
            })}
          </ul>
        )}

        {melebihi.length > 0 && (
          <p className="flex items-start gap-2 rounded-kontrol bg-bahaya-teks/10 px-3 py-2 text-keterangan text-bahaya-teks">
            <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
            {melebihi.length} barang dikirim melebihi sisanya — stok toko ini akan tercatat minus setelah berangkat.
          </p>
        )}

        <Kolom
          label="Catatan"
          value={catatan}
          onChange={(e) => setCatatan(e.target.value)}
          bantuan="Boleh dikosongkan. Mis. nama kurir."
          maxLength={255}
        />

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <Tombol
          lebarPenuh
          memuat={buat.isPending}
          disabled={!keToko || baris.length === 0}
          onClick={() => {
            setGalat(null)
            buat.mutate()
          }}
        >
          Buat Pengiriman
        </Tombol>
      </Kartu>

      <DaftarPengiriman tokoAktif={tokoAktif} aksi={aksi} />

      <PemilihBarang
        terbuka={bukaPemilih}
        onTutup={() => setBukaPemilih(false)}
        onPilih={(p) => {
          setBukaPemilih(false)
          setBaris((l) => (l.some((b) => b.produk.id === p.id) ? l : [...l, { produk: p, qty: '1' }]))
        }}
        judul="Pilih barang yang dikirim"
        sudahDipilih={ids}
      />
    </div>
  )
}

interface Aksi {
  /** id kiriman yang sedang diproses — hanya tombol kiriman itu yang memuat. */
  sedang?: string
  jalankan: (id: string, a: 'kirim' | 'terima') => void
}

/**
 * Kiriman yang sedang di jalan KE toko ini — ditampilkan paling atas karena
 * hanya toko tujuan yang bisa menerimanya, dan stoknya baru bertambah setelah
 * itu.
 */
function KirimanMasuk({ tokoAktif, aksi, className }: { tokoAktif?: string; aksi: Aksi; className?: string }) {
  const q = useQuery({
    queryKey: ['transfer', tokoAktif, 'masuk'],
    queryFn: () => stokApi.daftarTransfer(tokoAktif!, 1, 50),
    enabled: !!tokoAktif,
  })
  const masuk = (q.data?.data ?? []).filter((t) => t.status === 'sent' && t.to_outlet_id === tokoAktif)
  if (!masuk.length) return null
  return (
    <Kartu className={cn('flex flex-col gap-3 border-info-teks/30 bg-info-teks/5 p-4', className)}>
      <h2 className="flex items-center gap-2 text-judul-kartu font-semibold text-teks-utama">
        <Truck className="h-5 w-5 text-info-teks" aria-hidden />
        {masuk.length} kiriman sedang di jalan ke toko ini
      </h2>
      <ul className="grid gap-2 lg:grid-cols-2">
        {masuk.map((t) => (
          <li key={t.id}>
            <KartuTransfer t={t} tokoAktif={tokoAktif} aksi={aksi} ringkas />
          </li>
        ))}
      </ul>
    </Kartu>
  )
}

function DaftarPengiriman({ tokoAktif, aksi }: { tokoAktif?: string; aksi: Aksi }) {
  const [arah, setArah] = useState<Arah>('semua')
  const q = useInfiniteQuery({
    queryKey: ['transfer', tokoAktif, 'daftar'],
    queryFn: ({ pageParam }) => stokApi.daftarTransfer(tokoAktif!, pageParam, PER_HALAMAN),
    initialPageParam: 1,
    getNextPageParam: (akhir, semua) => (semua.length * PER_HALAMAN < akhir.total ? semua.length + 1 : undefined),
    enabled: !!tokoAktif,
  })
  const semua = q.data?.pages.flatMap((h) => h.data) ?? []
  const daftar = semua.filter((t) =>
    arah === 'keluar' ? t.from_outlet_id === tokoAktif : arah === 'masuk' ? t.to_outlet_id === tokoAktif : true,
  )

  return (
    <section className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-judul-kartu font-semibold text-teks-utama">Pengiriman</h2>
        <SegmenPilihan
          label="Arah pengiriman"
          nilai={arah}
          onPilih={setArah}
          pilihan={[
            ['semua', 'Semua'],
            ['keluar', 'Keluar'],
            ['masuk', 'Masuk'],
          ]}
        />
      </div>

      {q.isLoading ? (
        <KerangkaBaris jumlah={3} />
      ) : daftar.length === 0 ? (
        <p className="rounded-kartu border border-dashed border-garis p-6 text-center text-isi text-teks-redup">
          {arah === 'semua' ? 'Belum ada pengiriman antar toko.' : `Belum ada pengiriman ${arah}.`}
        </p>
      ) : (
        <ul className="flex flex-col gap-2">
          {daftar.map((t) => (
            <li key={t.id}>
              <KartuTransfer t={t} tokoAktif={tokoAktif} aksi={aksi} />
            </li>
          ))}
        </ul>
      )}
      {q.hasNextPage && (
        <Tombol jenis="kedua" lebarPenuh memuat={q.isFetchingNextPage} onClick={() => q.fetchNextPage()}>
          Muat lebih banyak
        </Tombol>
      )}
    </section>
  )
}

function KartuTransfer({
  t,
  tokoAktif,
  aksi,
  ringkas = false,
}: {
  t: Transfer
  tokoAktif?: string
  aksi: Aksi
  ringkas?: boolean
}) {
  const [buka, setBuka] = useState(false)
  const dariSini = t.from_outlet_id === tokoAktif
  const keSini = t.to_outlet_id === tokoAktif
  const nama = t.item_names ?? []
  const lebih = t.item_count - nama.length
  const sedang = aksi.sedang === t.id

  return (
    <Kartu className="flex flex-col gap-3 p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="flex flex-wrap items-center gap-1.5 font-semibold text-teks-utama">
            <span className={cn(dariSini && 'text-utama')}>{dariSini ? 'Toko ini' : t.from_outlet_name}</span>
            <ArrowRight className="h-4 w-4 shrink-0 text-teks-redup" aria-hidden />
            <span className={cn(keSini && 'text-utama')}>{keSini ? 'Toko ini' : t.to_outlet_name}</span>
          </p>
          <p className="truncate text-label text-teks-sekunder">
            {nama.join(', ')}
            {lebih > 0 && ` +${lebih} lagi`}
          </p>
          <p className="text-keterangan text-teks-redup">
            {formatTanggalJam(t.sent_at || t.created_at)}
            {t.created_by_name && ` · ${t.created_by_name}`}
          </p>
        </div>
        {!ringkas && <StatusTransfer status={t.status} />}
      </div>

      <button
        type="button"
        onClick={() => setBuka((v) => !v)}
        aria-expanded={buka}
        className="-my-1 flex min-h-9 items-center gap-1 self-start text-label font-medium text-utama hover:underline"
      >
        {buka ? 'Sembunyikan rincian' : `Rincian ${t.item_count} barang`}
        <ChevronDown className={cn('h-4 w-4 transition-transform', buka && 'rotate-180')} aria-hidden />
      </button>
      {buka && <RincianTransfer id={t.id} />}

      {/* Tombolnya hanya muncul untuk toko yang memang berwenang di tahap itu. */}
      {t.status === 'draft' && dariSini && (
        <Tombol ukuran="padat" memuat={sedang} onClick={() => aksi.jalankan(t.id, 'kirim')}>
          <Truck className="h-5 w-5" aria-hidden />
          Barang Sudah Berangkat
        </Tombol>
      )}
      {t.status === 'sent' && keSini && (
        <Tombol ukuran="padat" memuat={sedang} onClick={() => aksi.jalankan(t.id, 'terima')}>
          <PackageCheck className="h-5 w-5" aria-hidden />
          Terima Barang
        </Tombol>
      )}
      {t.status === 'sent' && dariSini && (
        <p className="text-keterangan text-teks-redup">Menunggu {t.to_outlet_name} menerima barangnya.</p>
      )}
    </Kartu>
  )
}

/** Barang, jumlah, dan tiga titik waktu: disiapkan → berangkat → diterima. */
function RincianTransfer({ id }: { id: string }) {
  const q = useQuery({ queryKey: ['transfer', 'rinci', id], queryFn: () => stokApi.transfer(id) })
  if (q.isLoading) return <Kerangka className="h-16 w-full" />
  if (q.isError || !q.data) return <p className="text-label text-bahaya-teks">Rincian belum bisa dimuat.</p>
  const t = q.data
  const waktu: [string, string | undefined][] = [
    ['Disiapkan', t.created_at],
    ['Berangkat', t.sent_at],
    ['Diterima', t.received_at],
  ]
  return (
    <div className="flex flex-col gap-2 rounded-kontrol bg-permukaan-2/60 px-3 py-2.5">
      <ul className="flex flex-col gap-1">
        {(t.items ?? []).map((it) => (
          <li key={it.id} className="flex items-baseline justify-between gap-3 text-label">
            <span className="min-w-0 truncate text-teks-utama">{it.product_name}</span>
            <span className="shrink-0 font-semibold tabular-nums text-teks-utama">
              {formatQty(it.qty)} {it.unit_name}
            </span>
          </li>
        ))}
      </ul>
      {t.note && <p className="text-keterangan text-teks-sekunder">Catatan: {t.note}</p>}
      <ol className="flex flex-wrap gap-x-4 gap-y-1 border-t border-garis pt-2 text-keterangan">
        {waktu.map(([label, iso]) => (
          <li key={label} className={iso ? 'text-teks-sekunder' : 'text-teks-redup'}>
            {label}: {iso ? formatTanggalJam(iso) : '—'}
          </li>
        ))}
      </ol>
    </div>
  )
}

function StatusTransfer({ status }: { status: string }) {
  switch (status) {
    case 'draft':
      return <LencanaStatus nada="netral" anak="Disiapkan" />
    case 'sent':
      return <LencanaStatus nada="menunggu" anak="Di jalan" />
    case 'received':
      return <LencanaStatus nada="berhasil" anak="Diterima" />
    case 'canceled':
      return <LencanaStatus nada="bahaya" anak="Dibatalkan" />
    default:
      return <LencanaStatus nada="netral" anak={status} />
  }
}
