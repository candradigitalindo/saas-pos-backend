import { useDeferredValue, useState } from 'react'
import { Link, useNavigate, useSearchParams } from 'react-router-dom'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import * as Popover from '@radix-ui/react-popover'
import {
  ArrowDownUp,
  Boxes,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Hourglass,
  PackagePlus,
  Search,
  SlidersHorizontal,
  X,
} from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { FotoBarang } from '@/bersama/komponen/foto-barang'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka, KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useKategori } from '@/bersama/hooks/use-katalog'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQtySatuan } from '@/bersama/util/desimal'
import { kelasPetak, petaWarnaKategori } from '@/bersama/util/warna-kategori'
import { cn } from '@/bersama/util/cn'
import type { SaldoStok } from '@/bersama/tipe/katalog'
import { stokApi, type KeadaanStok, type RingkasanStok, type UrutanStok } from '../api'
import { HARI_JENDELA, keadaanSaldo, type Keadaan } from '../keadaan-stok'
import { KEADAAN, Laku, LencanaKeadaan, Meter, teksSisa } from '../komponen/bagian-saldo'

/** Baris per halaman — batas atas server. */
const PER_HALAMAN = 100

/**
 * Stok per toko: "apa yang perlu saya urus hari ini, dan berapa uang saya
 * yang ada di rak".
 *
 * - KARTU KESEHATAN di atas: nilai stok (harga modal), bilah proporsi empat
 *   keadaan, dan tiap keadaan sekaligus tombol saring. Dulu lima kartu angka
 *   terpisah yang memakan separuh layar HP dan tidak memperlihatkan
 *   perbandingannya.
 * - Modal yang TERTAHAN di barang yang tidak laku 30 hari disebut terang —
 *   bagi warung, itu uang yang bisa dipakai belanja barang yang laku.
 * - Daftar diurut PALING MENDESAK lebih dulu (minus → habis → hampir habis),
 *   bukan abjad: halaman pertama selalu berisi yang perlu diurus.
 * - Tiap baris: foto, kategori, sisa terhadap batas (meteran), laku 30 hari
 *   dan perkiraan "cukup berapa hari lagi", serta nilai stoknya.
 *
 * Saringan, urutan, pencarian, dan halaman disimpan di URL: kembali dari
 * riwayat satu barang tidak mengembalikan daftar ke awal.
 *
 * Semua angka ringkasan dihitung server atas SELURUH barang (GET
 * /stocks/summary), dan batas ?status= sama persis dengannya — "Habis 3" yang
 * ditekan selalu menampilkan tiga baris.
 */
export function HalamanStok() {
  const { tokoAktif, boleh } = useSesi()
  const [params, setParams] = useSearchParams()
  const keadaan = bacaKeadaan(params.get('keadaan'))
  const urut = bacaUrut(params.get('urut'))
  const cari = params.get('q') ?? ''
  const cariTunda = useDeferredValue(cari.trim())
  const halaman = Math.max(1, Number(params.get('hal')) || 1)

  /** Ubah parameter URL; selain pindah halaman, halaman kembali ke 1. */
  const ubah = (isi: Record<string, string | null>) =>
    setParams(
      (lama) => {
        const baru = new URLSearchParams(lama)
        for (const [k, v] of Object.entries(isi)) {
          if (v) baru.set(k, v)
          else baru.delete(k)
        }
        if (!('hal' in isi)) baru.delete('hal')
        return baru
      },
      { replace: true },
    )

  const ringkasan = useQuery({
    queryKey: ['stok-ringkasan', tokoAktif],
    queryFn: () => stokApi.ringkasan(tokoAktif!),
    enabled: !!tokoAktif,
    staleTime: 30_000,
  })

  const q = useQuery({
    queryKey: ['stok', tokoAktif, keadaan, urut, cariTunda, halaman],
    queryFn: () =>
      stokApi.saldo(tokoAktif!, false, halaman, PER_HALAMAN, {
        cari: cariTunda,
        keadaan: keadaan ?? undefined,
        urut,
      }),
    enabled: !!tokoAktif,
    staleTime: 30_000,
    placeholderData: keepPreviousData,
  })

  const kat = useKategori()
  const warna = petaWarnaKategori(kat.data?.data ?? [])

  const daftar = q.data?.data ?? []
  const total = q.data?.total ?? 0
  const jumlahHalaman = Math.max(1, Math.ceil(total / PER_HALAMAN))
  const bolehKoreksi = boleh(IZIN.stockAdjust)

  return (
    <div className="flex flex-col gap-4">
      <header className="flex items-center justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-judul font-bold text-teks-utama">Stok</h1>
          <p className="hidden text-label text-teks-sekunder sm:block">
            Sisa barang, yang perlu dibeli, dan modal yang ada di rak.
          </p>
        </div>
        {bolehKoreksi && (
          <div className="flex shrink-0 gap-2">
            {/* HP: Koreksi cukup ikon supaya judul & kedua tombol muat satu
                baris — Barang Masuk lebih sering dipakai, labelnya tetap. */}
            <Tombol jenis="kedua" asChild className="px-3 sm:px-5">
              <Link to="/stok/koreksi" aria-label="Koreksi stok">
                <SlidersHorizontal className="h-5 w-5" aria-hidden />
                <span className="hidden sm:inline">Koreksi</span>
              </Link>
            </Tombol>
            <Tombol asChild className="px-4 sm:px-5">
              <Link to="/stok/masuk">
                <PackagePlus className="h-5 w-5" aria-hidden />
                Barang Masuk
              </Link>
            </Tombol>
          </div>
        )}
      </header>

      <KesehatanStok
        data={ringkasan.data}
        memuat={ringkasan.isLoading}
        aktif={keadaan}
        onPilih={(k) => ubah({ keadaan: k === keadaan ? null : k })}
      />

      <div className="flex gap-2">
        <div className="relative min-w-0 flex-1">
          <Search
            className="pointer-events-none absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-teks-redup"
            aria-hidden
          />
          <input
            type="search"
            value={cari}
            onChange={(e) => ubah({ q: e.target.value || null })}
            placeholder="Cari nama, SKU, atau barcode…"
            aria-label="Cari barang di stok"
            className="h-12 w-full rounded-kontrol border border-garis bg-permukaan pl-10 pr-3 text-isi text-teks-utama placeholder:text-teks-redup"
          />
        </div>
        <PilihUrutan nilai={urut} onPilih={(u) => ubah({ urut: u === 'urgent' ? null : u })} />
      </div>

      {keadaan && (
        <div className="-mt-1 flex flex-wrap items-center gap-2 text-label text-teks-sekunder">
          <span>Menampilkan</span>
          <button
            type="button"
            onClick={() => ubah({ keadaan: null })}
            aria-label={`Hapus saringan ${KEADAAN[keadaan].label}`}
            className="inline-flex min-h-9 items-center gap-1.5 rounded-full bg-sorot px-3 font-semibold text-hijau-800 hover:brightness-95"
          >
            {KEADAAN[keadaan].label}
            <X className="h-4 w-4" aria-hidden />
          </button>
          {q.data && <span className="tabular-nums">· {total.toLocaleString('id-ID')} barang</span>}
        </div>
      )}

      {q.isLoading ? (
        <KerangkaBaris jumlah={6} />
      ) : q.isError ? (
        <KeadaanGagal
          pesan={q.error instanceof GalatAPI ? q.error.pesan : 'Saldo stok belum bisa dimuat.'}
          onCobaLagi={() => q.refetch()}
        />
      ) : daftar.length === 0 ? (
        <KeadaanKosong
          ikon={Boxes}
          judul={cari ? 'Barang tidak ditemukan' : keadaan ? KEADAAN[keadaan].kosong[0] : 'Belum ada stok tercatat'}
          penjelasan={
            cari
              ? `Tidak ada barang bernama "${cari}"${keadaan ? ` yang ${KEADAAN[keadaan].label.toLowerCase()}` : ''}.`
              : keadaan
                ? KEADAAN[keadaan].kosong[1]
                : 'Catat barang masuk atau isi stok awal supaya saldonya muncul di sini.'
          }
          aksi={
            cari || keadaan
              ? { label: 'Lihat semua barang', onKlik: () => ubah({ q: null, keadaan: null }) }
              : undefined
          }
        />
      ) : (
        <>
          {/* HP & tablet: kartu bertumpuk; seluruh kartu membuka riwayatnya. */}
          <ul className={cn('flex flex-col gap-2 lg:hidden', q.isPlaceholderData && 'opacity-60')}>
            {daftar.map((s) => (
              <li key={`${s.product_id}-${s.variant_id ?? ''}`}>
                <KartuStok saldo={s} kelasWarna={kelasPetak(warna, s.category_id)} />
              </li>
            ))}
          </ul>

          {/* Layar lebar: tabel. */}
          <TabelStok
            daftar={daftar}
            warna={warna}
            bolehKoreksi={bolehKoreksi}
            redup={q.isPlaceholderData}
          />

          {jumlahHalaman > 1 && (
            <nav aria-label="Halaman daftar stok" className="flex flex-wrap items-center justify-between gap-3">
              <p className="text-label tabular-nums text-teks-sekunder">
                Halaman {halaman} dari {jumlahHalaman} · {total.toLocaleString('id-ID')} barang
              </p>
              <div className="flex gap-2">
                <Tombol
                  jenis="kedua"
                  ukuran="padat"
                  disabled={halaman <= 1}
                  onClick={() => ubah({ hal: String(halaman - 1) })}
                >
                  <ChevronLeft className="h-4 w-4" aria-hidden />
                  Sebelumnya
                </Tombol>
                <Tombol
                  jenis="kedua"
                  ukuran="padat"
                  disabled={halaman >= jumlahHalaman}
                  onClick={() => ubah({ hal: String(halaman + 1) })}
                >
                  Berikutnya
                  <ChevronRight className="h-4 w-4" aria-hidden />
                </Tombol>
              </div>
            </nav>
          )}
        </>
      )}
    </div>
  )
}

// ── Keadaan & urutan ─────────────────────────────────────────────────────────

const URUTAN: { nilai: UrutanStok; label: string; penjelasan: string }[] = [
  { nilai: 'urgent', label: 'Paling mendesak', penjelasan: 'Perlu dicocokkan, habis, lalu hampir habis' },
  { nilai: 'sold', label: 'Paling laku', penjelasan: `Terbanyak keluar ${HARI_JENDELA} hari terakhir` },
  { nilai: 'value', label: 'Nilai terbesar', penjelasan: 'Modal paling banyak di rak' },
  { nilai: 'name', label: 'Nama A–Z', penjelasan: 'Urut abjad' },
]

function bacaKeadaan(v: string | null): KeadaanStok | null {
  return v && v in KEADAAN ? (v as KeadaanStok) : null
}

function bacaUrut(v: string | null): UrutanStok {
  return URUTAN.find((u) => u.nilai === v)?.nilai ?? 'urgent'
}

// ── Kartu kesehatan stok ─────────────────────────────────────────────────────

const EMPAT: Keadaan[] = ['safe', 'low', 'out', 'negative']

function KesehatanStok({
  data,
  memuat,
  aktif,
  onPilih,
}: {
  data?: RingkasanStok
  memuat: boolean
  aktif: KeadaanStok | null
  onPilih: (k: KeadaanStok) => void
}) {
  if (memuat) return <Kerangka className="h-44 w-full rounded-kartu lg:h-32" />
  if (!data || data.total === 0) return null

  const jumlah: Record<Keadaan, number> = {
    safe: data.safe,
    low: data.low,
    out: data.out,
    negative: data.negative,
  }

  return (
    <Kartu className="flex flex-col gap-4 p-4 sm:p-5">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:gap-8">
        <div className="shrink-0 lg:w-60">
          <p className="text-keterangan font-medium text-teks-sekunder">Nilai stok · harga modal</p>
          <p className="text-judul font-extrabold tabular-nums text-teks-utama">{formatRupiah(data.stock_value)}</p>
          <p className="text-keterangan text-teks-redup">
            {data.total.toLocaleString('id-ID')} barang dicatat stoknya
          </p>
        </div>

        <div className="flex min-w-0 flex-1 flex-col gap-3">
          {/* Bilah proporsi — pelengkap visual; angkanya tertulis di bawah. */}
          <div className="flex h-2.5 gap-0.5 overflow-hidden rounded-full bg-permukaan-2" aria-hidden>
            {EMPAT.map((k) =>
              jumlah[k] > 0 ? (
                <span
                  key={k}
                  className={cn('h-full first:rounded-l-full last:rounded-r-full', KEADAAN[k].bilah)}
                  style={{ flexGrow: jumlah[k] }}
                />
              ) : null,
            )}
          </div>
          <div className="grid grid-cols-2 gap-2 sm:grid-cols-4" role="group" aria-label="Saring menurut keadaan">
            {EMPAT.map((k) => (
              <TombolKeadaan
                key={k}
                keadaan={k}
                jumlah={jumlah[k]}
                aktif={aktif === k}
                onKlik={() => onPilih(k)}
              />
            ))}
          </div>
        </div>
      </div>

      {data.idle > 0 && (
        // Garis pemisah di pembungkus, bukan di tombol: tombol berujung
        // bulat membuat garis atasnya ikut melengkung.
        <div className="border-t border-garis pt-2">
          <button
            type="button"
            onClick={() => onPilih('idle')}
            aria-pressed={aktif === 'idle'}
            className={cn(
              '-mx-2 flex min-h-12 w-[calc(100%+1rem)] items-center gap-3 rounded-kontrol px-2 py-1.5 text-left',
              'hover:bg-permukaan-2/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-utama',
              aktif === 'idle' && 'bg-sorot hover:bg-sorot',
            )}
          >
            <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-permukaan-2 text-teks-sekunder">
              <Hourglass className="h-4.5 w-4.5" aria-hidden />
            </span>
            <span className="min-w-0 flex-1 text-label text-teks-sekunder">
              <strong className="font-bold tabular-nums text-teks-utama">{formatRupiah(data.idle_value)}</strong> modal
              tertahan di{' '}
              <strong className="font-semibold text-teks-utama">{data.idle.toLocaleString('id-ID')} barang</strong> yang
              tidak laku {HARI_JENDELA} hari.
            </span>
            <ChevronRight className="h-5 w-5 shrink-0 text-teks-redup" aria-hidden />
          </button>
        </div>
      )}
    </Kartu>
  )
}

function TombolKeadaan({
  keadaan,
  jumlah,
  aktif,
  onKlik,
}: {
  keadaan: Keadaan
  jumlah: number
  aktif: boolean
  onKlik: () => void
}) {
  const { label, ikon: Ikon, teks, bilah } = KEADAAN[keadaan]
  const kosong = jumlah === 0
  return (
    <button
      type="button"
      onClick={onKlik}
      // Nol tidak bisa disaring (daftarnya pasti kosong), tapi tetap
      // ditampilkan: "0 habis" adalah kabar baik.
      disabled={kosong && !aktif}
      aria-pressed={aktif}
      aria-label={`${label}: ${jumlah} barang`}
      className={cn(
        'relative flex min-h-12 items-center gap-2 overflow-hidden rounded-kontrol border py-2 pl-3.5 pr-3 text-left transition-colors',
        // HP 320px: "Perlu dicocokkan" tidak muat bersebelahan dengan angkanya
        // — angka turun ke bawah label.
        'max-[359px]:flex-col max-[359px]:items-start max-[359px]:gap-0.5',
        'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-utama',
        aktif
          ? 'border-utama bg-sorot'
          : 'border-garis bg-permukaan enabled:hover:border-utama/40 enabled:hover:bg-permukaan-2/50',
      )}
    >
      {/* Jalur warna di tepi kiri = warna potongannya di bilah. */}
      <span className={cn('absolute inset-y-0 left-0 w-1', kosong ? 'bg-garis' : bilah)} aria-hidden />
      <span className="flex min-w-0 flex-1 items-center gap-2">
        <Ikon className={cn('h-4 w-4 shrink-0', kosong ? 'text-teks-redup' : teks)} aria-hidden />
        <span className="min-w-0 text-keterangan font-medium leading-tight text-teks-sekunder">{label}</span>
      </span>
      <span
        className={cn(
          'text-judul-kartu font-extrabold tabular-nums',
          kosong ? 'text-teks-redup' : 'text-teks-utama',
        )}
      >
        {jumlah.toLocaleString('id-ID')}
      </span>
    </button>
  )
}

// ── Urutan ───────────────────────────────────────────────────────────────────

function PilihUrutan({ nilai, onPilih }: { nilai: UrutanStok; onPilih: (u: UrutanStok) => void }) {
  const [buka, setBuka] = useState(false)
  const terpilih = URUTAN.find((u) => u.nilai === nilai) ?? URUTAN[0]!
  return (
    <Popover.Root open={buka} onOpenChange={setBuka}>
      <Popover.Trigger asChild>
        <button
          type="button"
          aria-label={`Urutkan: ${terpilih.label}`}
          className="flex h-12 shrink-0 items-center gap-2 rounded-kontrol border border-garis bg-permukaan px-3 text-label font-medium text-teks-utama hover:bg-permukaan-2 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-utama"
        >
          <ArrowDownUp className="h-4 w-4 text-teks-sekunder" aria-hidden />
          {/* HP: ikon saja; urutannya tetap disebut pada aria-label. */}
          <span className="hidden sm:inline">{terpilih.label}</span>
          <ChevronDown className="hidden h-4 w-4 text-teks-redup sm:block" aria-hidden />
        </button>
      </Popover.Trigger>
      <Popover.Portal>
        <Popover.Content
          align="end"
          sideOffset={6}
          className="gerak-lapis z-60 w-72 max-w-[calc(100vw-2rem)] rounded-kontrol border border-garis bg-permukaan p-1 shadow-melayang"
        >
          <p className="px-3 pb-1 pt-2 text-keterangan font-semibold text-teks-sekunder">Urutkan</p>
          <ul role="listbox" aria-label="Urutkan daftar stok">
            {URUTAN.map((u) => {
              const pilih = u.nilai === nilai
              return (
                <li key={u.nilai} role="option" aria-selected={pilih}>
                  <button
                    type="button"
                    onClick={() => {
                      onPilih(u.nilai)
                      setBuka(false)
                    }}
                    className={cn(
                      'flex w-full items-start gap-2 rounded-kontrol px-3 py-2.5 text-left hover:bg-permukaan-2',
                      pilih && 'bg-sorot hover:bg-sorot',
                    )}
                  >
                    <Check className={cn('mt-0.5 h-4 w-4 shrink-0 text-utama', !pilih && 'invisible')} aria-hidden />
                    <span className="min-w-0">
                      <span className="block text-label font-semibold text-teks-utama">{u.label}</span>
                      <span className="block text-keterangan text-teks-redup">{u.penjelasan}</span>
                    </span>
                  </button>
                </li>
              )
            })}
          </ul>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}

// ── Baris ────────────────────────────────────────────────────────────────────

function KartuStok({ saldo: s, kelasWarna }: { saldo: SaldoStok; kelasWarna: string }) {
  const k = keadaanSaldo(s)
  const min = Number.parseFloat(s.min_stock) > 0
  return (
    <Link
      to={`/stok/kartu/${s.product_id}`}
      aria-label={`${s.product_name}: ${teksSisa(s, k)}. Lihat riwayat stok`}
      className="flex items-start gap-3 rounded-kartu border border-garis bg-permukaan p-3 shadow-kartu transition-colors hover:bg-permukaan-2/50 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-utama"
    >
      <FotoBarang nama={s.product_name} url={s.image_url} kecil kelasWarna={kelasWarna} className="w-11" />
      <div className="min-w-0 flex-1">
        <div className="flex items-baseline justify-between gap-2">
          <p className="truncate font-semibold text-teks-utama">{s.product_name}</p>
          <p
            className={cn(
              'shrink-0 font-bold tabular-nums',
              k === 'out' || k === 'negative' ? 'text-bahaya-teks' : 'text-teks-utama',
            )}
          >
            {teksSisa(s, k)}
          </p>
        </div>
        <div className="mt-1 flex items-center justify-between gap-2">
          <div className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-1">
            <LencanaKeadaan keadaan={k} />
            {min && (
              <span className="text-keterangan tabular-nums text-teks-redup">
                batas {formatQtySatuan(s.min_stock, s.unit_name)}
              </span>
            )}
          </div>
          <Meter s={s} k={k} className="w-14 shrink-0" />
        </div>
        <p className="mt-1 text-keterangan text-teks-redup">
          <Laku s={s} k={k} sebaris />
        </p>
      </div>
    </Link>
  )
}

function TabelStok({
  daftar,
  warna,
  bolehKoreksi,
  redup,
}: {
  daftar: SaldoStok[]
  warna: Map<string, string>
  bolehKoreksi: boolean
  redup: boolean
}) {
  const navigate = useNavigate()
  return (
    <div
      className={cn(
        'hidden overflow-hidden rounded-kartu border border-garis bg-permukaan shadow-kartu lg:block',
        redup && 'opacity-60',
      )}
    >
      <table className="w-full">
        <thead>
          <tr className="border-b border-garis bg-permukaan-2/60 text-left text-label text-teks-sekunder">
            <th scope="col" className="px-4 py-3 font-medium">Barang</th>
            <th scope="col" className="px-3 py-3 font-medium">Keadaan</th>
            <th scope="col" className="px-3 py-3 text-right font-medium">Sisa</th>
            <th scope="col" className="px-3 py-3 text-right font-medium">Laku {HARI_JENDELA} hari</th>
            <th scope="col" className="px-3 py-3 text-right font-medium">Nilai stok</th>
            <th scope="col" className="px-4 py-3">
              <span className="sr-only">Aksi</span>
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-garis">
          {daftar.map((s) => {
            const k = keadaanSaldo(s)
            const ke = `/stok/kartu/${s.product_id}`
            const sub = [s.category_name, s.sku && `SKU ${s.sku}`].filter(Boolean).join(' · ')
            return (
              <tr
                key={`${s.product_id}-${s.variant_id ?? ''}`}
                // Seluruh baris bisa diklik tetikus; papan ketik & pembaca
                // layar memakai tautan pada nama barang.
                onClick={() => navigate(ke)}
                className="cursor-pointer hover:bg-permukaan-2/50"
              >
                <td className="px-4 py-2.5">
                  <div className="flex items-center gap-3">
                    <FotoBarang
                      nama={s.product_name}
                      url={s.image_url}
                      kecil
                      kelasWarna={kelasPetak(warna, s.category_id)}
                      className="w-10"
                    />
                    <div className="min-w-0">
                      <Link
                        to={ke}
                        onClick={(e) => e.stopPropagation()}
                        className="font-medium text-teks-utama hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-utama"
                      >
                        {s.product_name}
                      </Link>
                      {sub && <p className="text-keterangan text-teks-redup">{sub}</p>}
                    </div>
                  </div>
                </td>
                <td className="px-3 py-2.5">
                  <div className="flex flex-col items-start gap-1.5">
                    <LencanaKeadaan keadaan={k} />
                    <Meter s={s} k={k} className="w-24" />
                  </div>
                </td>
                <td className="px-3 py-2.5 text-right tabular-nums">
                  <span
                    className={cn(
                      'block font-semibold',
                      k === 'out' || k === 'negative' ? 'text-bahaya-teks' : 'text-teks-utama',
                    )}
                  >
                    {k === 'negative' ? formatQtySatuan(s.qty, s.unit_name).replace('-', '−') : teksSisa(s, k)}
                  </span>
                  <span className="block text-keterangan text-teks-redup">
                    {Number.parseFloat(s.min_stock) > 0
                      ? `batas ${formatQtySatuan(s.min_stock, s.unit_name)}`
                      : 'tanpa batas'}
                  </span>
                </td>
                <td className="px-3 py-2.5 text-right text-keterangan text-teks-redup">
                  {k === 'negative' ? <span className="text-bahaya-teks">perlu dihitung ulang</span> : <Laku s={s} k={k} />}
                </td>
                <td className="px-3 py-2.5 text-right tabular-nums">
                  <span className="block font-medium text-teks-utama">{formatRupiah(s.stock_value ?? 0)}</span>
                  {!!s.cost_price && (
                    <span className="block text-keterangan text-teks-redup">
                      @ {formatRupiah(s.cost_price)}
                    </span>
                  )}
                </td>
                <td className="px-4 py-2.5 text-right">
                  {bolehKoreksi && (
                    <Tombol jenis={k === 'negative' ? 'kedua' : 'teks'} ukuran="padat" asChild>
                      <Link
                        to={`/stok/koreksi?product_id=${s.product_id}`}
                        onClick={(e) => e.stopPropagation()}
                        aria-label={`${k === 'negative' ? 'Cocokkan' : 'Koreksi'} stok ${s.product_name}`}
                      >
                        {k === 'negative' ? 'Cocokkan' : 'Koreksi'}
                      </Link>
                    </Tombol>
                  )}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
