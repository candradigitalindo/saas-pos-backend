import { useMemo, useRef, useState } from 'react'
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import {
  Bike,
  ChefHat,
  Download,
  FileUp,
  Info,
  MessageCircle,
  Percent,
  Plug,
  PlugZap,
  Plus,
  ReceiptText,
  ShoppingBag,
  Trash2,
  TriangleAlert,
  Wallet,
  Zap,
  type LucideIcon,
} from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom } from '@/bersama/ui/kolom'
import { StepperJumlah } from '@/bersama/ui/stepper-jumlah'
import { Tombol } from '@/bersama/ui/tombol'
import { SegmenPilihan } from '@/bersama/ui/segmen'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KartuAngka } from '@/bersama/komponen/kartu-angka'
import { KeadaanGagal } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris, KerangkaKartuAngka } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useDaftarProduk } from '@/bersama/hooks/use-katalog'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { FITUR } from '@/lib/fitur'
import { BannerKunciFitur } from '@/bersama/komponen/kunci-fitur'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQty } from '@/bersama/util/desimal'
import { formatJam, formatTanggalAkrab, formatTanggalJam } from '@/bersama/util/tanggal'
import { inisialNama } from '@/bersama/util/inisial'
import { cn } from '@/bersama/util/cn'
import type { Produk, VarianProduk } from '@/bersama/tipe/katalog'
import { kanalApi, type HasilImporKanal, type Kanal, type PesananKanal } from '../api'
import { DialogSambungan, kodePenyedia } from '../komponen/dialog-sambungan'

type JenisKanal = 'delivery_app' | 'marketplace' | 'conversation'

/**
 * Tiap jenis kanal punya warna & ikon sendiri — di daftar pesanan campuran,
 * mata menemukan "yang dari GoFood" lebih cepat dari warnanya daripada dari
 * namanya. Contoh nama hanya saran isian; komisi bawaannya bukan klaim tarif
 * resmi (tiap toko punya kontrak sendiri) — hanya titik awal yang bisa diubah.
 */
const JENIS: Record<
  JenisKanal,
  { label: string; ikon: LucideIcon; warna: string; contoh: string[]; komisi: string }
> = {
  delivery_app: {
    label: 'Aplikasi antar',
    ikon: Bike,
    warna: 'bg-jingga-400/25 text-jingga-700',
    contoh: ['GoFood', 'GrabFood', 'ShopeeFood'],
    komisi: '20',
  },
  marketplace: {
    label: 'Marketplace',
    ikon: ShoppingBag,
    warna: 'bg-info-teks/10 text-info-teks',
    contoh: ['Shopee', 'Tokopedia & Shop', 'Lazada'],
    komisi: '0',
  },
  conversation: {
    label: 'Chat langsung',
    ikon: MessageCircle,
    warna: 'bg-sorot text-hijau-800',
    contoh: ['WhatsApp', 'Instagram'],
    komisi: '0',
  },
}
/** "hari ini" / "kemarin" di tengah kalimat; tanggal ("10 Sep 2026") apa adanya. */
const hariAkrab = (iso: string) => {
  const t = formatTanggalAkrab(iso)
  return t === 'Hari ini' || t === 'Kemarin' ? t.toLowerCase() : t
}
const jenisDari = (k: Kanal) => JENIS[k.kind as JenisKanal] ?? JENIS.conversation
const persenKomisi = (k: Kanal) => Math.round(Number(k.commission_rate) * 1000) / 10

/**
 * Kanal penjualan online.
 *
 * Sambungan API memakai KREDENSIAL MILIK TENANT (akun developer toko sendiri
 * di Meta/GoBiz/Shopee...) — platform hanya menyambungkan, tidak mengurus izin
 * (komponen/dialog-sambungan.tsx). Penyedia yang adaptornya belum ada tetap
 * dicatat manual atau diimpor CSV, dan layar ini mengatakannya terang-terangan.
 *
 * Pertanyaan yang dijawab layar ini: kanal mana yang ramai, berapa yang
 * benar-benar diterima setelah komisi, dan pesanan apa saja yang masuk. Versi
 * sebelumnya tidak menjawab satu pun — daftar kanalnya selalu kosong (kontrak
 * API salah baca) dan setiap pesanan tertulis Rp 0.
 */
export function HalamanKanal() {
  const { boleh, punyaFitur } = useSesi()
  // Kunci paket: menambah kanal & mencatat pesanan BARU butuh fitur kanal
  // online. Pesanan yang sudah masuk tetap bisa dilihat & dibatalkan.
  const fiturKanal = punyaFitur(FITUR.kanalOnline)
  const bolehKelola = boleh(IZIN.channelManage)
  const bolehPesanan = boleh(IZIN.channelOrderAccept, IZIN.channelManage)

  const [dialog, setDialog] = useState<
    | { jenis: 'kanal' }
    | { jenis: 'pesanan'; kanal?: Kanal }
    | { jenis: 'impor'; kanal: Kanal }
    | { jenis: 'api'; kanal: Kanal }
    | { jenis: 'detail'; pesanan: PesananKanal }
    | null
  >(null)

  const kanal = useQuery({
    queryKey: ['kanal'],
    queryFn: () => kanalApi.daftar(),
    enabled: bolehKelola,
  })
  const daftar = kanal.data ?? []
  const aktif = daftar.filter((k) => k.is_active)

  const total = daftar.reduce(
    (t, k) => ({
      pesanan: t.pesanan + (k.stats?.order_count ?? 0),
      kotor: t.kotor + (k.stats?.gross_amount ?? 0),
      komisi: t.komisi + (k.stats?.fee_amount ?? 0),
      bersih: t.bersih + (k.stats?.net_amount ?? 0),
      batal: t.batal + (k.stats?.canceled_count ?? 0),
    }),
    { pesanan: 0, kotor: 0, komisi: 0, bersih: 0, batal: 0 },
  )
  const persenPotong = total.kotor > 0 ? Math.round((total.komisi / total.kotor) * 100) : 0

  return (
    <div className="flex w-full max-w-6xl flex-col gap-4">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <div className="min-w-0">
          <h1 className="text-judul font-bold text-teks-utama">Kanal Online</h1>
          <p className="text-label text-teks-sekunder">
            Pesanan dari aplikasi antar, marketplace, dan chat — stok ikut terpotong dan untungnya
            dihitung setelah komisi.
          </p>
        </div>
        {fiturKanal && (
          // HP: dua tombol sama lebar dalam satu baris — berdampingan bebas
          // tiap labelnya terlipat dua baris.
          <div className="grid w-full auto-cols-fr grid-flow-col gap-2 sm:flex sm:w-auto">
            {bolehKelola && (
              <Tombol
                jenis="kedua"
                className="whitespace-nowrap px-3 sm:px-5"
                onClick={() => setDialog({ jenis: 'kanal' })}
              >
                <Plus className="hidden h-5 w-5 sm:block" aria-hidden />
                Tambah Kanal
              </Tombol>
            )}
            {bolehPesanan && aktif.length > 0 && (
              <Tombol
                className="whitespace-nowrap px-3 sm:px-5"
                onClick={() => setDialog({ jenis: 'pesanan' })}
              >
                Catat Pesanan
              </Tombol>
            )}
          </div>
        )}
      </header>

      <BannerKunciFitur
        fitur={FITUR.kanalOnline}
        nama="Kanal online"
        penjelasan="Kanal dan pesanan yang sudah tercatat tetap bisa dilihat dan dibatalkan — hanya menambah kanal atau pesanan baru yang terkunci."
      />

      {bolehKelola &&
        (kanal.isLoading ? (
          <div className="grid gap-3 lg:grid-cols-3">
            <KerangkaKartuAngka />
            <KerangkaKartuAngka />
            <KerangkaKartuAngka />
          </div>
        ) : kanal.isError ? (
          <KeadaanGagal pesan="Daftar kanal gagal dimuat." onCobaLagi={() => kanal.refetch()} />
        ) : daftar.length === 0 ? (
          <KanalKosong
            bisaTambah={bolehKelola && fiturKanal}
            onTambah={() => setDialog({ jenis: 'kanal' })}
          />
        ) : (
          <>
            {/* Empat angka setara, bukan satu kartu sorotan: di kolom sepertiga
                layar, rincian "Rp 292.000" terpotong jadi "Rp 292.…". */}
            <div className="grid grid-cols-2 gap-3 lg:grid-cols-4">
              <KartuAngka
                label="Diterima bersih"
                nilai={total.bersih}
                keterangan="30 hari · setelah komisi"
                ikon={Wallet}
                ringkas
                className="border-utama/30 bg-sorot/40"
              />
              <KartuAngka
                label="Pesanan"
                nilai={total.pesanan}
                uang={false}
                keterangan={total.batal > 0 ? `+${total.batal} dibatalkan` : 'selesai, 30 hari'}
                ikon={ReceiptText}
                ringkas
              />
              <KartuAngka
                label="Penjualan"
                nilai={total.kotor}
                keterangan="sebelum komisi"
                ikon={ShoppingBag}
                ringkas
              />
              <KartuAngka
                label="Komisi kanal"
                nilai={total.komisi}
                keterangan={total.kotor > 0 ? `${persenPotong}% dari penjualan` : 'belum ada potongan'}
                ikon={Percent}
                ringkas
              />
            </div>
            {/* grid-cols-1 eksplisit: kolom implisit "auto" melebar mengikuti
                kartu terlebar (nama panjang) sampai halaman bergulir mendatar. */}
            <ul className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
              {daftar.map((k) => (
                <li key={k.id}>
                  <KartuKanal
                    kanal={k}
                    bisaTulis={fiturKanal}
                    onCatat={() => setDialog({ jenis: 'pesanan', kanal: k })}
                    onImpor={() => setDialog({ jenis: 'impor', kanal: k })}
                    onApi={() => setDialog({ jenis: 'api', kanal: k })}
                  />
                </li>
              ))}
            </ul>
          </>
        ))}

      <p className="flex items-start gap-2 text-keterangan text-teks-redup">
        <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden />
        Kanal yang disambungkan ke API penyedia memakai akun developer milik toko Anda sendiri, dan
        pesanannya masuk otomatis (WhatsApp, GoFood, GrabFood, Shopee, Tokopedia & Shop, dan Lazada). Kanal lain dicatat
        manual atau diimpor dari laporan harian (CSV).
      </p>

      {bolehPesanan && (
        <DaftarPesanan
          kanal={daftar}
          onBuka={(p) => setDialog({ jenis: 'detail', pesanan: p })}
        />
      )}

      {dialog?.jenis === 'kanal' && <DialogKanal onTutup={() => setDialog(null)} />}
      {dialog?.jenis === 'pesanan' && (
        <DialogPesanan kanalAwal={dialog.kanal} pilihan={aktif} onTutup={() => setDialog(null)} />
      )}
      {dialog?.jenis === 'impor' && <DialogImpor kanal={dialog.kanal} onTutup={() => setDialog(null)} />}
      {dialog?.jenis === 'api' && <DialogSambungan kanal={dialog.kanal} onTutup={() => setDialog(null)} />}
      {dialog?.jenis === 'detail' && (
        <DialogDetailPesanan
          pesanan={dialog.pesanan}
          kanal={daftar.find((k) => k.id === dialog.pesanan.channel_id)}
          onTutup={() => setDialog(null)}
        />
      )}
    </div>
  )
}

// ── Kanal ───────────────────────────────────────────────────────────────────

function KanalKosong({ bisaTambah, onTambah }: { bisaTambah: boolean; onTambah: () => void }) {
  return (
    <Kartu className="flex flex-col items-center gap-4 px-6 py-10 text-center">
      <div className="flex -space-x-2" aria-hidden>
        {(Object.keys(JENIS) as JenisKanal[]).map((j) => {
          const { ikon: Ikon, warna } = JENIS[j]
          return (
            <span
              key={j}
              className={cn('flex h-12 w-12 items-center justify-center rounded-full ring-4 ring-permukaan', warna)}
            >
              <Ikon className="h-5 w-5" />
            </span>
          )
        })}
      </div>
      <div className="flex max-w-md flex-col gap-1">
        <p className="text-judul-kartu font-semibold text-teks-utama">Belum ada kanal online</p>
        <p className="text-label text-teks-sekunder">
          Tambahkan GoFood, Shopee, atau WhatsApp. Pesanan dari sana memotong stok yang sama dengan
          kasir, dan untungnya dihitung setelah komisi — jadi terlihat kanal mana yang benar-benar
          menguntungkan.
        </p>
      </div>
      {bisaTambah && (
        <Tombol onClick={onTambah}>
          <Plus className="h-5 w-5" aria-hidden />
          Tambah Kanal
        </Tombol>
      )}
    </Kartu>
  )
}

function AvatarKanal({ kanal, besar }: { kanal: Kanal; besar?: boolean }) {
  const { warna } = jenisDari(kanal)
  return (
    <span
      className={cn(
        'flex shrink-0 items-center justify-center rounded-kontrol font-bold',
        besar ? 'h-11 w-11 text-label' : 'h-9 w-9 text-keterangan',
        warna,
      )}
      aria-hidden
    >
      {inisialNama(kanal.name)}
    </span>
  )
}

function KartuKanal({
  kanal,
  bisaTulis,
  onCatat,
  onImpor,
  onApi,
}: {
  kanal: Kanal
  bisaTulis: boolean
  onCatat: () => void
  onImpor: () => void
  onApi: () => void
}) {
  const { boleh } = useSesi()
  const toast = useToast()
  const qc = useQueryClient()
  const s = kanal.stats
  const jenis = jenisDari(kanal)
  const komisi = persenKomisi(kanal)

  // DELETE /channels/:id di server hanya MENONAKTIFKAN (pesanan lamanya tetap
  // tercatat). Dulu tombolnya bergambar tong sampah dan berkata "dihapus".
  const ubahAktif = useMutation({
    mutationFn: (aktif: boolean) => kanalApi.ubah(kanal.id, { is_active: aktif }),
    onSuccess: (k) => {
      qc.invalidateQueries({ queryKey: ['kanal'] })
      toast.berhasil(k.is_active ? `${k.name} diaktifkan lagi.` : `${k.name} dinonaktifkan.`)
    },
    onError: (e) => toast.gagal(e instanceof GalatAPI ? e.pesan : 'Gagal mengubah kanal.'),
  })

  return (
    <Kartu className={cn('flex h-full flex-col gap-3 p-4', !kanal.is_active && 'opacity-70')}>
      <div className="flex items-start gap-3">
        <AvatarKanal kanal={kanal} besar />
        <div className="min-w-0 flex-1">
          {/* Dua baris, bukan terpotong: "Tokopedia & S…" tidak lagi terbaca. */}
          <p className="line-clamp-2 font-semibold break-words text-teks-utama">{kanal.name}</p>
          <p className="text-keterangan text-teks-redup">
            {jenis.label} ·{' '}
            <span className="whitespace-nowrap">komisi {komisi.toLocaleString('id-ID')}%</span>
          </p>
        </div>
        {/* Status & aksinya satu kendali. Dulu lencana "Aktif" di atas dan
            tombol tong sampah di bawah — yang di server pun hanya menonaktifkan. */}
        {bisaTulis && boleh(IZIN.channelManage) ? (
          <button
            type="button"
            role="switch"
            aria-checked={kanal.is_active}
            aria-label={`Kanal ${kanal.name} aktif`}
            onClick={() => ubahAktif.mutate(!kanal.is_active)}
            disabled={ubahAktif.isPending}
            className="-my-2 -mr-2 flex min-h-11 shrink-0 items-center gap-2 rounded-kontrol px-2 text-keterangan font-medium text-teks-sekunder focus-visible:outline-2 focus-visible:outline-utama"
          >
            {kanal.is_active ? 'Aktif' : 'Nonaktif'}
            <span
              className={cn(
                'relative h-6 w-10 rounded-full transition-colors',
                kanal.is_active ? 'bg-utama' : 'bg-garis',
              )}
              aria-hidden
            >
              <span
                className={cn(
                  'absolute left-0.5 top-0.5 h-5 w-5 rounded-full bg-white shadow-kartu transition-transform',
                  kanal.is_active && 'translate-x-4',
                )}
              />
            </span>
          </button>
        ) : kanal.is_active ? (
          <LencanaStatus nada="berhasil" anak="Aktif" />
        ) : (
          <LencanaStatus nada="netral" anak="Nonaktif" />
        )}
      </div>

      <dl className="grid grid-cols-2 gap-2">
        <div className="rounded-kontrol bg-permukaan-2 px-3 py-2">
          <dt className="text-keterangan text-teks-redup">Pesanan · {s?.days ?? 30} hari</dt>
          <dd className="text-judul-kartu font-bold tabular-nums text-teks-utama">{s?.order_count ?? 0}</dd>
        </div>
        <div className="rounded-kontrol bg-permukaan-2 px-3 py-2">
          <dt className="text-keterangan text-teks-redup">Diterima</dt>
          <dd className="truncate text-judul-kartu font-bold tabular-nums text-teks-utama">
            {formatRupiah(s?.net_amount ?? 0)}
          </dd>
        </div>
      </dl>
      <p className="text-keterangan text-teks-sekunder">
        {s && s.fee_amount > 0 && <>Komisi {formatRupiah(s.fee_amount)} · </>}
        {s?.canceled_count ? <>{s.canceled_count} dibatalkan · </> : null}
        {s?.last_order_at
          ? `Terakhir ${hariAkrab(s.last_order_at)} ${formatJam(s.last_order_at)}`
          : 'Belum ada pesanan'}
      </p>

      {kanal.is_active && (
        <PitaSambungan kanal={kanal} bisaAtur={bisaTulis && boleh(IZIN.channelManage)} onApi={onApi} />
      )}

      {bisaTulis && kanal.is_active && (
        <div className="mt-auto flex flex-wrap items-center gap-2 border-t border-garis pt-3">
          {boleh(IZIN.channelOrderAccept, IZIN.channelManage) && (
            <Tombol ukuran="padat" onClick={onCatat}>
              Catat pesanan
            </Tombol>
          )}
          {boleh(IZIN.channelManage) && (
            <Tombol jenis="kedua" ukuran="padat" onClick={onImpor}>
              <FileUp className="h-4 w-4" aria-hidden />
              Impor CSV
            </Tombol>
          )}
        </div>
      )}
      {!kanal.is_active && (
        <p className="mt-auto border-t border-garis pt-3 text-keterangan text-teks-redup">
          Nonaktif: tidak ditawarkan saat mencatat pesanan. Pesanan lamanya tetap tercatat.
        </p>
      )}
    </Kartu>
  )
}

/**
 * Status sambungan otomatis kanal, satu baris. Pesanan otomatis atau manual
 * adalah hal pertama yang ingin diketahui pemilik dari sebuah kanal.
 */
function PitaSambungan({ kanal, bisaAtur, onApi }: { kanal: Kanal; bisaAtur: boolean; onApi: () => void }) {
  const status = kanal.integration_mode === 'api' ? kanal.connection_status : 'manual'
  const isi = {
    connected: { ikon: Zap, warna: 'text-hijau-800', teks: 'Otomatis lewat API', aksi: 'Pengaturan' },
    error: { ikon: TriangleAlert, warna: 'text-jingga-700', teks: 'API bermasalah', aksi: 'Perbaiki' },
    none: { ikon: PlugZap, warna: 'text-teks-sekunder', teks: 'API tersimpan, belum dites', aksi: 'Tes' },
    manual: { ikon: Plug, warna: 'text-teks-redup', teks: 'Dicatat manual / CSV', aksi: 'Hubungkan API' },
  }[status]
  const Ikon = isi.ikon
  return (
    <div className="flex items-center justify-between gap-2 rounded-kontrol bg-permukaan-2 py-1 pl-3 pr-1">
      <span className={cn('flex min-w-0 items-center gap-1.5 text-keterangan font-medium', isi.warna)}>
        <Ikon className="h-4 w-4 shrink-0" aria-hidden />
        <span className="truncate">{isi.teks}</span>
      </span>
      {bisaAtur && (
        <button
          type="button"
          onClick={onApi}
          className="min-h-11 shrink-0 rounded-kontrol px-2 text-keterangan font-semibold text-utama hover:bg-sorot"
        >
          {isi.aksi}
        </button>
      )}
    </div>
  )
}

// ── Pesanan ─────────────────────────────────────────────────────────────────

const statusPesanan = (p: PesananKanal) =>
  p.sale_status === 'canceled' || p.external_status === 'canceled' ? 'canceled' : 'completed'

/**
 * Nomor pesanan dari API bisa sangat panjang (WhatsApp: "wamid.HBgN…" ±60
 * karakter) — di daftar cukup awal & ujungnya; utuhnya di rincian.
 */
const nomorPendek = (id: string) => (id.length > 18 ? `${id.slice(0, 8)}…${id.slice(-5)}` : id)

/** Nomor pesanan hanya diulang di baris kecil bila judulnya nama pembeli. */
const metaPesanan = (p: PesananKanal, k?: Kanal) =>
  [k?.name ?? 'Kanal', p.buyer_name ? nomorPendek(p.external_order_id) : null, formatTanggalJam(p.occurred_at ?? p.created_at)]
    .filter(Boolean)
    .join(' · ')

const ringkasIsi = (p: PesananKanal) =>
  (p.items ?? []).map((i) => `${formatQty(i.qty)}× ${i.product_name}`).join(', ') || '—'

function DaftarPesanan({
  kanal,
  onBuka,
}: {
  kanal: Kanal[]
  onBuka: (p: PesananKanal) => void
}) {
  const [saring, setSaring] = useState<string>('semua')
  const kanalId = saring === 'semua' ? undefined : saring
  const q = useInfiniteQuery({
    queryKey: ['pesanan-kanal', kanalId],
    queryFn: ({ pageParam }) => kanalApi.daftarPesanan(kanalId, pageParam),
    initialPageParam: 1,
    getNextPageParam: (h) => (h.current_page < h.last_page ? h.current_page + 1 : undefined),
  })
  const baris = useMemo(() => q.data?.pages.flatMap((h) => h.data) ?? [], [q.data])
  const jumlah = q.data?.pages[0]?.total ?? 0
  const peta = useMemo(() => new Map(kanal.map((k) => [k.id, k])), [kanal])

  return (
    <section className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-judul-kartu font-semibold text-teks-utama">
          Pesanan
          {jumlah > 0 && <span className="ml-2 text-label font-normal text-teks-redup">{jumlah}</span>}
        </h2>
        {kanal.length > 1 && (
          <SegmenPilihan
            label="Saring per kanal"
            pilihan={[['semua', 'Semua'], ...kanal.map((k) => [k.id, k.name] as [string, string])]}
            nilai={saring}
            onPilih={setSaring}
          />
        )}
      </div>

      {q.isLoading ? (
        <KerangkaBaris jumlah={3} />
      ) : q.isError ? (
        <KeadaanGagal pesan="Pesanan gagal dimuat." onCobaLagi={() => q.refetch()} />
      ) : baris.length === 0 ? (
        <div className="rounded-kartu border border-dashed border-garis px-6 py-8 text-center">
          <p className="font-medium text-teks-utama">Belum ada pesanan</p>
          <p className="text-label text-teks-redup">
            Catat pesanan dari chat, atau impor laporan harian marketplace.
          </p>
        </div>
      ) : (
        <>
          {/* HP: kartu. Layar lebar: tabel dengan kotor, komisi, diterima. */}
          <ul className="flex flex-col gap-2 lg:hidden">
            {baris.map((p) => (
              <li key={p.id}>
                <KartuPesanan p={p} kanal={peta.get(p.channel_id)} onBuka={() => onBuka(p)} />
              </li>
            ))}
          </ul>
          <TabelPesanan baris={baris} peta={peta} onBuka={onBuka} />
          {q.hasNextPage && (
            <Tombol
              jenis="kedua"
              lebarPenuh
              onClick={() => q.fetchNextPage()}
              memuat={q.isFetchingNextPage}
              labelMemuat="Memuat…"
            >
              Muat {Math.min(20, jumlah - baris.length)} pesanan lagi
            </Tombol>
          )}
        </>
      )}
    </section>
  )
}

function KartuPesanan({ p, kanal, onBuka }: { p: PesananKanal; kanal?: Kanal; onBuka: () => void }) {
  const batal = statusPesanan(p) === 'canceled'
  return (
    <button
      type="button"
      onClick={onBuka}
      className="flex w-full items-start gap-3 rounded-kartu border border-garis bg-permukaan p-4 text-left shadow-kartu hover:border-utama/40 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-utama"
    >
      {kanal && <AvatarKanal kanal={kanal} />}
      <div className="min-w-0 flex-1">
        <div className="flex items-start justify-between gap-2">
          <p className="min-w-0 truncate font-semibold text-teks-utama">
            {p.buyer_name || nomorPendek(p.external_order_id)}
          </p>
          <p
            className={cn(
              'shrink-0 font-bold tabular-nums',
              batal ? 'text-teks-redup line-through' : 'text-teks-utama',
            )}
          >
            {formatRupiah(p.net_amount)}
          </p>
        </div>
        <p className="line-clamp-2 text-label text-teks-sekunder">{ringkasIsi(p)}</p>
        <p className="mt-1 flex flex-wrap items-center gap-x-2 gap-y-1 text-keterangan text-teks-redup">
          <span>{metaPesanan(p, kanal)}</span>
          {batal && <LencanaStatus nada="bahaya" anak="Dibatalkan" />}
          {!batal && p.ready_at && <LencanaStatus nada="berhasil" anak={`Siap ${formatJam(p.ready_at)}`} />}
          {!batal && p.fee_amount > 0 && <span>komisi {formatRupiah(p.fee_amount)}</span>}
        </p>
      </div>
    </button>
  )
}

function TabelPesanan({
  baris,
  peta,
  onBuka,
}: {
  baris: PesananKanal[]
  peta: Map<string, Kanal>
  onBuka: (p: PesananKanal) => void
}) {
  return (
    <Kartu className="hidden overflow-hidden lg:block">
      <table className="w-full text-label">
        <thead className="border-b border-garis bg-permukaan-2 text-left text-keterangan font-semibold uppercase tracking-wide text-teks-redup">
          <tr>
            <th className="px-4 py-3 font-semibold">Pesanan</th>
            <th className="px-4 py-3 font-semibold">Isi</th>
            <th className="px-4 py-3 text-right font-semibold">Penjualan</th>
            <th className="px-4 py-3 text-right font-semibold">Komisi</th>
            <th className="px-4 py-3 text-right font-semibold">Diterima</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-garis">
          {baris.map((p) => {
            const k = peta.get(p.channel_id)
            const batal = statusPesanan(p) === 'canceled'
            return (
              <tr
                key={p.id}
                tabIndex={0}
                onClick={() => onBuka(p)}
                onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && (e.preventDefault(), onBuka(p))}
                aria-label={`Pesanan ${p.external_order_id} — lihat rincian`}
                className="cursor-pointer align-top transition-colors hover:bg-permukaan-2 focus-visible:bg-sorot focus-visible:outline-none"
              >
                <td className="px-4 py-3">
                  <div className="flex items-start gap-3">
                    {k && <AvatarKanal kanal={k} />}
                    <div className="min-w-0">
                      <p className="font-semibold text-teks-utama">
                        {p.buyer_name || nomorPendek(p.external_order_id)}
                      </p>
                      <p className="text-keterangan text-teks-redup">{metaPesanan(p, k)}</p>
                    </div>
                  </div>
                </td>
                <td className="max-w-72 px-4 py-3 text-teks-sekunder">
                  <p className="line-clamp-2">{ringkasIsi(p)}</p>
                  {batal && (
                    <span className="mt-1 inline-block">
                      <LencanaStatus nada="bahaya" anak="Dibatalkan" />
                    </span>
                  )}
                  {!batal && p.ready_at && (
                    <span className="mt-1 inline-block">
                      <LencanaStatus nada="berhasil" anak={`Siap ${formatJam(p.ready_at)}`} />
                    </span>
                  )}
                </td>
                <td className="whitespace-nowrap px-4 py-3 text-right tabular-nums text-teks-sekunder">
                  {formatRupiah(p.gross_amount)}
                </td>
                <td className="whitespace-nowrap px-4 py-3 text-right tabular-nums text-teks-sekunder">
                  {p.fee_amount > 0 ? `−${formatRupiah(p.fee_amount)}` : '—'}
                </td>
                <td
                  className={cn(
                    'whitespace-nowrap px-4 py-3 text-right font-bold tabular-nums',
                    batal ? 'text-teks-redup line-through' : 'text-teks-utama',
                  )}
                >
                  {formatRupiah(p.net_amount)}
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
    </Kartu>
  )
}

function DialogDetailPesanan({
  pesanan: p,
  kanal,
  onTutup,
}: {
  pesanan: PesananKanal
  kanal?: Kanal
  onTutup: () => void
}) {
  const toast = useToast()
  const qc = useQueryClient()
  const [membatalkan, setMembatalkan] = useState(false)
  const [alasan, setAlasan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)
  const batal = statusPesanan(p) === 'canceled'
  // "Tandai siap" hanya untuk aplikasi antar yang tersambung API dan
  // penyedianya menerimanya (katalog: mark_ready).
  const penyedia = useQuery({ queryKey: ['penyedia-kanal'], queryFn: kanalApi.penyedia, staleTime: 5 * 60_000 })
  const kode = kanal ? kodePenyedia(kanal) : undefined
  const bisaSiap =
    !batal &&
    !!kanal &&
    kanal.integration_mode === 'api' &&
    kanal.connection_status === 'connected' &&
    !!penyedia.data?.some((x) => x.code === kode && x.mark_ready)
  const [siapPada, setSiapPada] = useState(p.ready_at)
  const siap = useMutation({
    mutationFn: () => kanalApi.tandaiSiap(p.id),
    onSuccess: (r) => {
      setSiapPada(r.ready_at)
      qc.invalidateQueries({ queryKey: ['pesanan-kanal'] })
      toast.berhasil(`${kanal?.name ?? 'Aplikasi antar'} diberi tahu: pesanan siap diambil.`)
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Gagal menandai siap.'),
  })

  const batalkan = useMutation({
    mutationFn: () => kanalApi.batalkan(p.id, alasan.trim()),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pesanan-kanal'] })
      qc.invalidateQueries({ queryKey: ['kanal'] })
      qc.invalidateQueries({ queryKey: ['stok'] })
      toast.berhasil(`Pesanan ${nomorPendek(p.external_order_id)} dibatalkan. Stoknya dikembalikan.`)
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Gagal membatalkan.'),
  })

  const info: [string, string | undefined][] = [
    ['No. pesanan', p.external_order_id],
    ['Pembeli', p.buyer_name],
    ['Telepon', p.buyer_phone],
    ['Alamat', p.shipping_address],
    ['Kurir', [p.courier, p.tracking_no].filter(Boolean).join(' · ') || undefined],
    ['Nota', p.receipt_no],
  ]

  return (
    <Dialog open onOpenChange={(o) => !o && !batalkan.isPending && !siap.isPending && onTutup()}>
      <IsiDialog
        judul={`Pesanan ${nomorPendek(p.external_order_id)}`}
        keterangan={`${kanal?.name ?? 'Kanal'} · ${formatTanggalJam(p.occurred_at ?? p.created_at)}`}
      >
        {batal && <LencanaStatus nada="bahaya" anak="Dibatalkan — stok sudah dikembalikan" />}
        {!batal && siapPada && (
          <LencanaStatus nada="berhasil" anak={`Siap diambil sejak ${formatJam(siapPada)} — pengemudi sudah diberi tahu`} />
        )}

        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1.5 rounded-kontrol bg-permukaan-2 p-3 text-label">
          {info
            .filter(([, v]) => v)
            .map(([k, v]) => (
              <div key={k} className="contents">
                <dt className="text-teks-redup">{k}</dt>
                <dd className="min-w-0 break-all text-teks-utama">{v}</dd>
              </div>
            ))}
        </dl>

        <ul className="flex flex-col divide-y divide-garis">
          {(p.items ?? []).map((i, n) => (
            <li key={n} className="flex items-baseline justify-between gap-3 py-2">
              <span className="min-w-0 text-teks-utama">
                {formatQty(i.qty)} {i.unit_name} {i.product_name}
              </span>
              <span className="shrink-0 tabular-nums text-teks-sekunder">{formatRupiah(i.line_total)}</span>
            </li>
          ))}
        </ul>

        <dl className="flex flex-col gap-1.5 border-t border-garis pt-3 text-label">
          <div className="flex justify-between gap-3">
            <dt className="text-teks-sekunder">Penjualan</dt>
            <dd className="tabular-nums text-teks-utama">{formatRupiah(p.gross_amount)}</dd>
          </div>
          <div className="flex justify-between gap-3">
            <dt className="text-teks-sekunder">
              Komisi {kanal ? `${persenKomisi(kanal).toLocaleString('id-ID')}%` : 'kanal'}
            </dt>
            <dd className="tabular-nums text-teks-utama">−{formatRupiah(p.fee_amount)}</dd>
          </div>
          <div className="flex justify-between gap-3 font-bold text-teks-utama">
            <dt>Diterima toko</dt>
            <dd className={cn('tabular-nums', batal && 'line-through')}>{formatRupiah(p.net_amount)}</dd>
          </div>
        </dl>

        {membatalkan && (
          <div className="flex flex-col gap-2 rounded-kontrol border border-garis p-3">
            <p className="text-label text-teks-sekunder">
              Penjualannya dibatalkan dan stok barangnya dikembalikan. Tidak bisa diurungkan.
            </p>
            <Kolom
              label="Alasan"
              value={alasan}
              onChange={(e) => setAlasan(e.target.value)}
              placeholder="mis. dibatalkan pembeli di aplikasi"
              autoFocus
              required
            />
          </div>
        )}
        {galat && (
          <p role="alert" className="text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        {/* Aksi utama dapur: lebar penuh, terpisah dari Batalkan/Tutup supaya
            tidak berdesakan (dan tidak salah tekan) di baris yang sama. */}
        {bisaSiap && !siapPada && !membatalkan && (
          <Tombol
            lebarPenuh
            onClick={() => {
              setGalat(null)
              siap.mutate()
            }}
            memuat={siap.isPending}
            labelMemuat="Memberi tahu…"
          >
            <ChefHat className="h-5 w-5" aria-hidden />
            Tandai Siap Diambil
          </Tombol>
        )}

        <AksiDialog>
          {!batal &&
            (membatalkan ? (
              <Tombol
                jenis="bahaya"
                onClick={() => {
                  setGalat(null)
                  batalkan.mutate()
                }}
                memuat={batalkan.isPending}
                disabled={!alasan.trim()}
              >
                Ya, Batalkan Pesanan
              </Tombol>
            ) : (
              <Tombol jenis="kedua" onClick={() => setMembatalkan(true)}>
                Batalkan pesanan
              </Tombol>
            ))}
          <Tombol jenis="kedua" onClick={onTutup} disabled={batalkan.isPending}>
            Tutup
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}

// ── Dialog tambah kanal ─────────────────────────────────────────────────────

function DialogKanal({ onTutup }: { onTutup: () => void }) {
  const { tokoAktif } = useSesi()
  const toast = useToast()
  const qc = useQueryClient()
  const [jenis, setJenis] = useState<JenisKanal>('delivery_app')
  const [penyedia, setPenyedia] = useState('')
  const [nama, setNama] = useState('')
  const [komisi, setKomisi] = useState(JENIS.delivery_app.komisi)
  const [galat, setGalat] = useState<string | null>(null)

  const buat = useMutation({
    mutationFn: () =>
      kanalApi.buat({
        outlet_id: tokoAktif!,
        kind: jenis,
        provider: penyedia.trim() || nama.trim(),
        name: nama.trim(),
        // Pengguna mengetik persen; backend menyimpan pecahan.
        commission_rate: String((Number(komisi.replace(',', '.')) || 0) / 100),
        integration_mode: 'manual',
      }),
    onSuccess: (k) => {
      qc.invalidateQueries({ queryKey: ['kanal'] })
      toast.berhasil(`Kanal ${k.name} ditambahkan.`)
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !buat.isPending && onTutup()}>
      <IsiDialog judul="Tambah Kanal" className="sm:max-w-lg">
        <form
          onSubmit={(e) => {
            e.preventDefault()
            setGalat(null)
            buat.mutate()
          }}
          className="flex flex-col gap-4"
          noValidate
        >
          <fieldset className="flex flex-col gap-1.5">
            <legend className="mb-1.5 text-label font-medium text-teks-sekunder">Jenis kanal</legend>
            <div role="radiogroup" aria-label="Jenis kanal" className="grid grid-cols-3 gap-2">
              {(Object.keys(JENIS) as JenisKanal[]).map((j) => {
                const { label, ikon: Ikon, warna } = JENIS[j]
                const aktif = jenis === j
                return (
                  <button
                    key={j}
                    type="button"
                    role="radio"
                    aria-checked={aktif}
                    onClick={() => {
                      setJenis(j)
                      setKomisi(JENIS[j].komisi)
                    }}
                    className={cn(
                      'flex min-h-20 flex-col items-center justify-center gap-1.5 rounded-kontrol border p-2 text-center text-keterangan font-semibold',
                      aktif ? 'border-utama bg-sorot/60 text-utama' : 'border-garis text-teks-sekunder hover:border-utama/40',
                    )}
                  >
                    <span className={cn('flex h-8 w-8 items-center justify-center rounded-full', warna)}>
                      <Ikon className="h-4 w-4" aria-hidden />
                    </span>
                    {label}
                  </button>
                )
              })}
            </div>
          </fieldset>

          <div className="flex flex-col gap-2">
            <Kolom
              label="Nama kanal"
              placeholder={`Contoh: ${JENIS[jenis].contoh[0]}`}
              value={nama}
              onChange={(e) => setNama(e.target.value)}
              autoFocus
              required
            />
            <div className="flex flex-wrap gap-1.5" role="group" aria-label="Nama yang umum">
              {JENIS[jenis].contoh.map((c) => (
                <button
                  key={c}
                  type="button"
                  onClick={() => setNama(c)}
                  aria-pressed={nama === c}
                  className={cn(
                    'min-h-11 rounded-full border px-3 text-label',
                    nama === c
                      ? 'border-utama bg-sorot font-semibold text-utama'
                      : 'border-garis text-teks-sekunder hover:text-teks-utama',
                  )}
                >
                  {c}
                </button>
              ))}
            </div>
          </div>

          <Kolom
            label="Komisi kanal"
            inputMode="decimal"
            akhiran="%"
            value={komisi}
            onChange={(e) => setKomisi(e.target.value.replace(/[^\d.,]/g, ''))}
            bantuan="Potongan kanal dari tiap penjualan, sesuai kontrak toko Anda. Inilah yang membuat untung kanal online lebih tipis."
          />
          <Kolom
            label="Nama penyedia (boleh dikosongkan)"
            placeholder="Sama dengan nama kanal"
            value={penyedia}
            onChange={(e) => setPenyedia(e.target.value)}
            bantuan="Dipakai untuk mencocokkan laporan dari mereka."
          />

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <AksiDialog>
            <Tombol type="submit" memuat={buat.isPending} disabled={!nama.trim()}>
              Tambah Kanal
            </Tombol>
            <Tombol jenis="kedua" onClick={onTutup} disabled={buat.isPending}>
              Batal
            </Tombol>
          </AksiDialog>
        </form>
      </IsiDialog>
    </Dialog>
  )
}

// ── Dialog catat pesanan ────────────────────────────────────────────────────

/** Satu baris pesanan manual. `pilihan` terisi bila barangnya bervarian. */
interface BarisPesanan {
  kunci: string
  produk: Produk
  pilihan?: VarianProduk[]
  varian?: VarianProduk
  qty: string
}

/** Perkiraan harga satuan: harga jual + selisih varian (server memakai harga kanal bila dipetakan). */
function hargaPesanan(b: BarisPesanan): number {
  return b.produk.sell_price + (b.varian?.price_delta ?? 0)
}

function DialogPesanan({
  kanalAwal,
  pilihan,
  onTutup,
}: {
  kanalAwal?: Kanal
  pilihan: Kanal[]
  onTutup: () => void
}) {
  const toast = useToast()
  const qc = useQueryClient()
  const [kanal, setKanal] = useState<Kanal | undefined>(kanalAwal ?? (pilihan.length === 1 ? pilihan[0] : undefined))
  const [nomor, setNomor] = useState('')
  const [pembeli, setPembeli] = useState('')
  const [cari, setCari] = useState('')
  const [baris, setBaris] = useState<BarisPesanan[]>([])
  const [galat, setGalat] = useState<string | null>(null)

  const produk = useDaftarProduk(cari)
  // Perkiraan — server memakai harga khusus kanal bila barangnya dipetakan.
  const kotor = baris.reduce((t, b) => t + Math.round(hargaPesanan(b) * Number(b.qty)), 0)
  // Barang bervarian wajib dipilih variannya (sama dengan kasir): harganya
  // bergantung pada pilihan itu.
  const belumPilih = baris.find((b) => (b.pilihan?.length ?? 0) > 0 && !b.varian)

  /** Tambah barang; bila bervarian, variannya dimuat lalu wajib dipilih. */
  function tambahBarang(p: Produk) {
    setCari('')
    const kunci = `${p.id}#${Date.now()}`
    setBaris((l) => {
      // Barang tanpa varian cukup satu baris; yang bervarian boleh berulang
      // (satu baris per varian).
      if (l.some((b) => b.produk.id === p.id && !(b.pilihan?.length ?? 0))) return l
      return [...l, { kunci, produk: p, qty: '1' }]
    })
    void kanalApi
      .varianBarang(p.id)
      .then((vs) => {
        const aktif = vs.filter((v) => v.is_active)
        if (aktif.length === 0) return
        setBaris((l) => l.map((b) => (b.kunci === kunci ? { ...b, pilihan: aktif } : b)))
      })
      .catch(() => {})
  }

  function pilihVarian(kunci: string, v: VarianProduk) {
    setBaris((l) => {
      const b = l.find((x) => x.kunci === kunci)
      if (!b) return l
      // Varian yang sama sudah ada di baris lain → digabung, bukan kembar.
      const kembar = l.find((x) => x.kunci !== kunci && x.produk.id === b.produk.id && x.varian?.id === v.id)
      if (kembar) {
        return l
          .filter((x) => x.kunci !== kunci)
          .map((x) => (x.kunci === kembar.kunci ? { ...x, qty: String(Number(x.qty) + Number(b.qty)) } : x))
      }
      return l.map((x) => (x.kunci === kunci ? { ...x, varian: v } : x))
    })
  }
  const komisi = kanal ? Math.round((kotor * persenKomisi(kanal)) / 100) : 0

  const catat = useMutation({
    mutationFn: () =>
      kanalApi.catatPesanan({
        channel_id: kanal!.id,
        external_order_id: nomor.trim(),
        buyer_name: pembeli.trim() || undefined,
        items: baris.map((b) => ({
          product_id: b.produk.id,
          ...(b.varian ? { variant_id: b.varian.id } : {}),
          qty: b.qty,
        })),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pesanan-kanal'] })
      qc.invalidateQueries({ queryKey: ['kanal'] })
      qc.invalidateQueries({ queryKey: ['stok'] })
      // Nomor pesanan yang sudah pernah masuk dibalas 200 oleh backend, bukan
      // galat — jadi kalimat ini sengaja tidak menjanjikan stok ikut berkurang,
      // karena pada kasus itu memang tidak.
      toast.berhasil('Pesanan tercatat.')
      onTutup()
    },
    onError: (e) =>
      setGalat(
        e instanceof GalatAPI
          ? e.status === 409
            ? 'Nomor pesanan ini sudah pernah dicatat.'
            : e.pesan
          : 'Terjadi kesalahan.',
      ),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !catat.isPending && onTutup()}>
      <IsiDialog
        judul={kanal ? `Catat pesanan ${kanal.name}` : 'Catat pesanan'}
        keterangan="Untuk pesanan yang datang lewat chat atau aplikasi yang belum tersambung otomatis."
        className="sm:max-w-lg"
      >
        <div className="flex flex-col gap-4">
          {pilihan.length > 1 && (
            <div className="flex flex-col gap-1.5">
              <p className="text-label font-medium text-teks-sekunder">Kanal</p>
              <div className="flex flex-wrap gap-1.5" role="radiogroup" aria-label="Kanal">
                {pilihan.map((k) => (
                  <button
                    key={k.id}
                    type="button"
                    role="radio"
                    aria-checked={kanal?.id === k.id}
                    onClick={() => setKanal(k)}
                    className={cn(
                      'flex min-h-11 items-center gap-2 rounded-full border py-1 pl-1 pr-3 text-label',
                      kanal?.id === k.id
                        ? 'border-utama bg-sorot font-semibold text-utama'
                        : 'border-garis text-teks-sekunder hover:text-teks-utama',
                    )}
                  >
                    <AvatarKanal kanal={k} />
                    {k.name}
                  </button>
                ))}
              </div>
            </div>
          )}
          <div className="grid gap-3 sm:grid-cols-2">
            <Kolom
              label="Nomor pesanan"
              placeholder="Dari aplikasi kanal"
              value={nomor}
              onChange={(e) => setNomor(e.target.value)}
              bantuan="Supaya pesanan yang sama tidak tercatat dua kali."
              autoFocus
              required
            />
            <Kolom
              label="Nama pembeli"
              value={pembeli}
              onChange={(e) => setPembeli(e.target.value)}
              bantuan="Boleh dikosongkan."
            />
          </div>

          <div className="flex flex-col gap-2">
            <Kolom
              label="Tambah barang"
              type="search"
              value={cari}
              onChange={(e) => setCari(e.target.value)}
              placeholder="Ketik nama barang…"
            />
            {cari && (
              <ul className="max-h-40 divide-y divide-garis overflow-y-auto rounded-kontrol border border-garis">
                {(produk.data?.data ?? []).length === 0 && !produk.isLoading && (
                  <li className="px-3 py-3 text-label text-teks-redup">Barang tidak ditemukan.</li>
                )}
                {produk.data?.data.map((p) => (
                  <li key={p.id}>
                    <button
                      type="button"
                      onClick={() => tambahBarang(p)}
                      className="flex min-h-12 w-full items-center justify-between gap-3 px-3 text-left hover:bg-permukaan-2"
                    >
                      <span className="min-w-0 truncate text-label text-teks-utama">{p.name}</span>
                      <span className="shrink-0 tabular-nums text-keterangan text-teks-redup">
                        {formatRupiah(p.sell_price)}
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>

          {baris.length > 0 && (
            <ul className="flex flex-col divide-y divide-garis rounded-kontrol border border-garis">
              {baris.map((b, i) => (
                <li key={b.kunci} className="flex flex-col gap-2 p-3">
                  <div className="flex items-start justify-between gap-3">
                    <div className="min-w-0">
                      <p className="font-medium text-teks-utama">
                        {b.produk.name}
                        {b.varian && ` (${b.varian.name})`}
                      </p>
                      <p className="text-keterangan tabular-nums text-teks-redup">
                        {formatRupiah(hargaPesanan(b))} · subtotal{' '}
                        {formatRupiah(Math.round(hargaPesanan(b) * Number(b.qty)))}
                      </p>
                    </div>
                    <button
                      type="button"
                      onClick={() => setBaris((l) => l.filter((_, j) => j !== i))}
                      aria-label={`Hapus ${b.produk.name}`}
                      className="-m-2 flex h-11 w-11 shrink-0 items-center justify-center rounded-kontrol text-bahaya-teks hover:bg-bahaya-teks/10"
                    >
                      <Trash2 className="h-5 w-5" aria-hidden />
                    </button>
                  </div>
                  {b.pilihan?.length ? (
                    <SegmenPilihan
                      label={`Varian ${b.produk.name}`}
                      nilai={b.varian?.id ?? ''}
                      onPilih={(id) => {
                        const v = b.pilihan!.find((x) => x.id === id)
                        if (v) pilihVarian(b.kunci, v)
                      }}
                      pilihan={b.pilihan.map((v) => [v.id, v.name] as const)}
                    />
                  ) : null}
                  <StepperJumlah
                    nilai={b.qty}
                    onNilai={(q) => setBaris((l) => l.map((x, j) => (j === i ? { ...x, qty: q } : x)))}
                    satuan={b.produk.unit_name}
                    minimal="1"
                    label={`Jumlah ${b.produk.name}`}
                  />
                </li>
              ))}
            </ul>
          )}

          {baris.length > 0 && kanal && (
            <dl className="flex flex-col gap-1 rounded-kontrol bg-permukaan-2 px-3 py-2.5 text-label">
              <div className="flex justify-between gap-3">
                <dt className="text-teks-sekunder">Penjualan</dt>
                <dd className="tabular-nums text-teks-utama">{formatRupiah(kotor)}</dd>
              </div>
              <div className="flex justify-between gap-3">
                <dt className="text-teks-sekunder">
                  Komisi {persenKomisi(kanal).toLocaleString('id-ID')}%
                </dt>
                <dd className="tabular-nums text-teks-utama">−{formatRupiah(komisi)}</dd>
              </div>
              <div className="flex justify-between gap-3 font-bold text-teks-utama">
                <dt>Perkiraan diterima</dt>
                <dd className="tabular-nums">{formatRupiah(kotor - komisi)}</dd>
              </div>
            </dl>
          )}

          {belumPilih && (
            <p className="text-label text-jingga-700">Pilih varian {belumPilih.produk.name} dulu.</p>
          )}
          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}
        </div>

        <AksiDialog>
          <Tombol
            memuat={catat.isPending}
            disabled={!kanal || !nomor.trim() || baris.length === 0 || !!belumPilih}
            onClick={() => {
              setGalat(null)
              catat.mutate()
            }}
          >
            Catat Pesanan
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={catat.isPending}>
            Batal
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}

// ── Dialog impor CSV ────────────────────────────────────────────────────────

const CONTOH_CSV =
  'external_order_id,date,sku,qty,unit_price,fee_amount\n' +
  'SP-1001,2026-09-27,KOPI-SUSU,2,18000,7200\n' +
  'SP-1001,2026-09-27,AIR-600,1,4000,800\n' +
  'SP-1002,2026-09-27,KOPI-SUSU,1,18000,3600\n'

/**
 * Impor laporan harian. Dulu hanya disebut di layar ("diimpor dari laporan
 * harian marketplace") tanpa tombol apa pun, dan server hanya mengenal ID
 * internal barang yang tidak ada di laporan mana pun — kini kolomnya `sku`.
 */
function DialogImpor({ kanal, onTutup }: { kanal: Kanal; onTutup: () => void }) {
  const qc = useQueryClient()
  const berkas = useRef<HTMLInputElement>(null)
  const [nama, setNama] = useState<string | null>(null)
  const [isi, setIsi] = useState<string | null>(null)
  const [hasil, setHasil] = useState<HasilImporKanal | null>(null)
  const [galat, setGalat] = useState<string | null>(null)

  const impor = useMutation({
    mutationFn: () => kanalApi.imporPesanan(kanal.id, isi!),
    onSuccess: (r) => {
      setHasil(r)
      qc.invalidateQueries({ queryKey: ['pesanan-kanal'] })
      qc.invalidateQueries({ queryKey: ['kanal'] })
      qc.invalidateQueries({ queryKey: ['stok'] })
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Impor gagal.'),
  })

  function unduhContoh() {
    const url = URL.createObjectURL(new Blob([CONTOH_CSV], { type: 'text/csv' }))
    const a = document.createElement('a')
    a.href = url
    a.download = `contoh-impor-${kanal.name.toLowerCase().replace(/\s+/g, '-')}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  return (
    <Dialog open onOpenChange={(o) => !o && !impor.isPending && onTutup()}>
      <IsiDialog
        judul={`Impor laporan ${kanal.name}`}
        keterangan="Satu baris = satu barang dalam pesanan. Baris dengan nomor pesanan sama digabung."
        className="sm:max-w-lg"
      >
        {hasil ? (
          <div className="flex flex-col gap-3">
            <dl className="grid grid-cols-3 gap-2 text-center">
              <AngkaImpor label="Masuk" nilai={hasil.imported} nada="berhasil" />
              <AngkaImpor label="Sudah ada" nilai={hasil.skipped} />
              <AngkaImpor label="Gagal" nilai={hasil.failed} nada={hasil.failed > 0 ? 'bahaya' : undefined} />
            </dl>
            {hasil.skipped > 0 && (
              <p className="text-keterangan text-teks-redup">
                &ldquo;Sudah ada&rdquo; = nomor pesanan yang pernah diimpor; dilewati, tidak dobel.
              </p>
            )}
            {(hasil.errors ?? []).length > 0 && (
              <ul className="max-h-40 overflow-y-auto rounded-kontrol border border-garis text-keterangan">
                {hasil.errors!.map((e, i) => (
                  <li key={i} className="border-b border-garis px-3 py-2 text-bahaya-teks last:border-0">
                    {e}
                  </li>
                ))}
              </ul>
            )}
            <AksiDialog>
              <Tombol onClick={onTutup}>Selesai</Tombol>
            </AksiDialog>
          </div>
        ) : (
          <div className="flex flex-col gap-3">
            <div className="overflow-x-auto rounded-kontrol border border-garis">
              <table className="w-full text-keterangan">
                <thead className="bg-permukaan-2 text-left text-teks-redup">
                  <tr>
                    <th className="px-3 py-2 font-semibold">Kolom</th>
                    <th className="px-3 py-2 font-semibold">Isi</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-garis text-teks-sekunder">
                  {[
                    ['external_order_id', 'Nomor pesanan dari kanal'],
                    ['date', 'Tanggal, TTTT-BB-HH'],
                    ['sku', 'SKU/barcode barang di toko, atau SKU yang dipetakan ke kanal'],
                    ['qty', 'Jumlah'],
                    ['unit_price', 'Harga satuan (rupiah, tanpa titik)'],
                    ['fee_amount', 'Komisi baris itu — boleh kosong'],
                  ].map(([k, v]) => (
                    <tr key={k}>
                      <td className="whitespace-nowrap px-3 py-1.5 font-mono text-teks-utama">{k}</td>
                      <td className="px-3 py-1.5">{v}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <div className="flex flex-wrap gap-2">
              <Tombol jenis="kedua" ukuran="padat" onClick={unduhContoh}>
                <Download className="h-4 w-4" aria-hidden />
                Unduh contoh
              </Tombol>
              <Tombol jenis="kedua" ukuran="padat" onClick={() => berkas.current?.click()}>
                <FileUp className="h-4 w-4" aria-hidden />
                {nama ? 'Ganti berkas' : 'Pilih berkas CSV'}
              </Tombol>
              <input
                ref={berkas}
                type="file"
                accept=".csv,text/csv"
                className="sr-only"
                aria-label="Berkas CSV"
                onChange={async (e) => {
                  const f = e.target.files?.[0]
                  if (!f) return
                  setGalat(null)
                  setNama(f.name)
                  setIsi(await f.text())
                }}
              />
            </div>
            {nama && (
              <p className="text-label text-teks-sekunder">
                Berkas: <strong className="text-teks-utama">{nama}</strong>
                {isi && ` · ${Math.max(0, isi.trim().split(/\r?\n/).length - 1)} baris`}
              </p>
            )}
            <p className="text-keterangan text-teks-redup">
              Aman diimpor ulang: pesanan yang sudah pernah masuk dilewati. Stok barangnya ikut
              terpotong.
            </p>
            {galat && (
              <p role="alert" className="text-label text-bahaya-teks">
                {galat}
              </p>
            )}
            <AksiDialog>
              <Tombol onClick={() => impor.mutate()} disabled={!isi} memuat={impor.isPending} labelMemuat="Mengimpor…">
                Impor
              </Tombol>
              <Tombol jenis="kedua" onClick={onTutup} disabled={impor.isPending}>
                Batal
              </Tombol>
            </AksiDialog>
          </div>
        )}
      </IsiDialog>
    </Dialog>
  )
}

function AngkaImpor({ label, nilai, nada }: { label: string; nilai: number; nada?: 'berhasil' | 'bahaya' }) {
  return (
    <div className="rounded-kontrol bg-permukaan-2 px-2 py-3">
      <dd
        className={cn(
          'text-judul font-bold tabular-nums',
          nada === 'berhasil' ? 'text-hijau-800' : nada === 'bahaya' ? 'text-bahaya-teks' : 'text-teks-utama',
        )}
      >
        {nilai}
      </dd>
      <dt className="text-keterangan text-teks-redup">{label}</dt>
    </div>
  )
}
