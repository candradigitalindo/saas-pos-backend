import { useEffect, useRef, useState } from 'react'
import { Link, NavLink, useLocation } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import {
  ChevronDown,
  Lock,
  PanelLeftClose,
  PanelLeftOpen,
  Receipt,
  Search,
  Sparkles,
} from 'lucide-react'
import {
  MENU_ATAS,
  MENU_BAWAH,
  menuAktif,
  saringKelompok,
  saringMenu,
  tujuanSamping,
  type ItemMenu,
} from '@/app/navigasi'
import { labelPintasan } from '@/app/layouts/palet-perintah'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { PemilihToko } from '@/bersama/komponen/pemilih-toko'
import { StatusKoneksi } from '@/bersama/komponen/status-koneksi'
import { TombolKeluar } from '@/bersama/komponen/tombol-keluar'
import { useSinkron } from '@/lib/offline/mesin'
import { IZIN } from '@/lib/izin'
import { inisialNama } from '@/bersama/util/inisial'
import { cn } from '@/bersama/util/cn'
import { stokApi } from '@/fitur/stok/api'
import { useRingkasanUtang } from '@/fitur/stok/komponen/peringatan-utang'
import { useRingkasanKasbon } from '@/fitur/pelanggan/komponen/peringatan-kasbon'

/** Kunci localStorage: judul kelompok yang sedang dilipat. */
const KUNCI_TERLIPAT = 'navigasi.terlipat'

function bacaTerlipat(): string[] {
  try {
    const v = JSON.parse(localStorage.getItem(KUNCI_TERLIPAT) ?? '[]')
    return Array.isArray(v) ? v.filter((x) => typeof x === 'string') : []
  } catch {
    return []
  }
}

/**
 * Navigasi samping (layar ≥1024px), disusun seperti aplikasi SaaS pada
 * umumnya (Linear, Vercel, Stripe, Shopify admin):
 *
 *   - kepala: pemilih toko, tombol utama "Buka Kasir", dan kotak "Cari…"
 *     yang membuka palet perintah (Ctrl/⌘ K);
 *   - daftar: Beranda & Laporan tanpa judul, lalu kelompok yang BISA DILIPAT
 *     (kelompok berisi halaman aktif selalu terbuka), baris ringkas dengan
 *     garis aksen pada menu aktif, dan lencana jumlah pada Stok;
 *   - kaki: Langganan & Pengaturan, kartu paket (Gratis / masa coba), status
 *     sinkron, dan profil.
 *
 * Bisa DICIUTKAN menjadi lajur ikon 72px (diingat per perangkat): layar
 * 1024px memberi isi halaman 256px lebih banyak.
 *
 * Tinggi baris 48px di layar sentuh, menyusut ke 40px hanya pada penunjuk
 * halus — aturan target sentuh yang sama dengan Tombol "padat" (ui/01 §3).
 * Tablet lanskap 1024px adalah layar kasir utama, dan juga mendapat navigasi
 * ini.
 */
export function NavigasiSamping({
  ringkas,
  onUbahRingkas,
  onBukaPalet,
}: {
  ringkas: boolean
  onUbahRingkas: () => void
  onBukaPalet: () => void
}) {
  const { profil, boleh } = useSesi()
  const sinkron = useSinkron()
  const { pathname } = useLocation()

  const atas = saringMenu(MENU_ATAS, boleh)
  const kelompok = saringKelompok(boleh)
  const bawah = saringMenu(MENU_BAWAH, boleh)
  // Tepat satu menu menyala: yang jalurnya paling spesifik (lihat menuAktif).
  const aktif = menuAktif(pathname, tujuanSamping(boleh))

  // Menu aktif selalu terlihat: di layar pendek daftarnya bergulir, dan menu
  // yang menyala di luar pandangan sama saja dengan tidak memberi tahu
  // "Anda di sini". Digulir seperlunya ('nearest'), tidak melompat bila sudah
  // terlihat.
  const refDaftar = useRef<HTMLElement>(null)
  useEffect(() => {
    const el = refDaftar.current?.querySelector<HTMLElement>('[aria-current="page"]')
    el?.scrollIntoView?.({ block: 'nearest' })
  }, [aktif, ringkas])

  const [terlipat, setTerlipat] = useState<string[]>(bacaTerlipat)
  const lipat = (judul: string) => {
    const baru = terlipat.includes(judul) ? terlipat.filter((j) => j !== judul) : [...terlipat, judul]
    setTerlipat(baru)
    try {
      localStorage.setItem(KUNCI_TERLIPAT, JSON.stringify(baru))
    } catch {
      /* hanya kenyamanan — tetap berlaku sampai halaman ditutup */
    }
  }

  return (
    <aside
      className={cn(
        'fixed inset-y-0 left-0 z-30 hidden flex-col border-r border-garis bg-permukaan lg:flex',
        'transition-[width] duration-200 ease-out motion-reduce:transition-none',
        ringkas ? 'w-[72px]' : 'w-64',
      )}
    >
      <div className="flex flex-col gap-2 p-2">
        <PemilihToko ringkas={ringkas} />

        {boleh(IZIN.saleCreate) && (
          <Link
            to="/kasir"
            title={ringkas ? 'Buka Kasir' : undefined}
            className={cn(
              'flex min-h-12 items-center justify-center gap-2 rounded-kontrol bg-utama text-label font-semibold text-utama-teks',
              'shadow-kartu transition-colors hover:bg-hijau-800 pointer-fine:min-h-10',
              'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-utama focus-visible:ring-offset-2',
            )}
          >
            <Receipt className="h-[18px] w-[18px] shrink-0" aria-hidden />
            <span className={cn(ringkas && 'sr-only')}>Buka Kasir</span>
          </Link>
        )}

        <button
          type="button"
          onClick={onBukaPalet}
          title={ringkas ? `Cari (${labelPintasan()})` : undefined}
          aria-label={`Cari halaman atau aksi (${labelPintasan()})`}
          className={cn(
            'flex min-h-12 items-center gap-2 rounded-kontrol border border-garis bg-latar text-label text-teks-redup pointer-fine:min-h-10',
            'hover:border-teks-redup/40 hover:text-teks-sekunder',
            'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-utama',
            ringkas ? 'justify-center' : 'px-3',
          )}
        >
          <Search className="h-4 w-4 shrink-0" aria-hidden />
          {!ringkas && (
            <>
              <span className="flex-1 text-left">Cari…</span>
              <kbd className="rounded border border-garis bg-permukaan px-1.5 font-sans text-keterangan text-teks-redup">
                {labelPintasan()}
              </kbd>
            </>
          )}
        </button>
      </div>

      <nav
        ref={refDaftar}
        aria-label="Navigasi samping"
        className="flex-1 overflow-y-auto overscroll-contain px-2 pb-2"
      >
        <ul className="flex flex-col gap-0.5">
          {atas.map((m) => (
            <li key={m.ke}>
              <TautanSamping item={m} aktif={aktif === m.ke} ringkas={ringkas} />
            </li>
          ))}
        </ul>

        {kelompok.map((k) => {
          const berisiAktif = k.item.some((m) => m.ke === aktif)
          // Kelompok berisi halaman aktif tidak pernah terlipat: menu yang
          // menyala tersembunyi di balik lipatan sama saja dengan tidak ada.
          const terbuka = ringkas || berisiAktif || !terlipat.includes(k.judul)
          const idDaftar = `kelompok-${k.judul.replace(/\W+/g, '-').toLowerCase()}`
          return (
            <div key={k.judul} className="mt-2">
              {ringkas ? (
                <div className="mx-3 mb-2 h-px bg-garis" aria-hidden />
              ) : (
                <button
                  type="button"
                  onClick={() => lipat(k.judul)}
                  disabled={berisiAktif}
                  aria-expanded={terbuka}
                  aria-controls={idDaftar}
                  className={cn(
                    'group flex min-h-12 w-full items-center gap-1 rounded-kontrol px-2.5 text-keterangan font-semibold text-teks-redup pointer-fine:min-h-8',
                    'hover:text-teks-sekunder disabled:cursor-default disabled:hover:text-teks-redup',
                  )}
                >
                  <span className="flex-1 text-left">{k.judul}</span>
                  <ChevronDown
                    className={cn(
                      'h-3.5 w-3.5 transition-transform duration-150 motion-reduce:transition-none',
                      !terbuka && '-rotate-90',
                      berisiAktif && 'opacity-0',
                    )}
                    aria-hidden
                  />
                </button>
              )}
              {terbuka && (
                <ul id={idDaftar} className="flex flex-col gap-0.5">
                  {k.item.map((m) => (
                    <li key={m.ke}>
                      <TautanSamping item={m} aktif={aktif === m.ke} ringkas={ringkas} />
                    </li>
                  ))}
                </ul>
              )}
            </div>
          )
        })}

        {/* Langganan & Pengaturan di UJUNG daftar, bukan di kaki yang tetap:
            di tablet 1024×768 kaki tetap yang memuatnya ikut memakan hampir
            separuh tinggi layar dan memeras daftar menu jadi beberapa baris. */}
        {bawah.length > 0 && (
          <>
            <div className="mx-2.5 my-2 h-px bg-garis" aria-hidden />
            <ul className="flex flex-col gap-0.5">
              {bawah.map((m) => (
                <li key={m.ke}>
                  <TautanSamping item={m} aktif={aktif === m.ke} ringkas={ringkas} />
                </li>
              ))}
            </ul>
          </>
        )}
      </nav>

      <div className="flex flex-col gap-2 border-t border-garis p-2">
        {!ringkas && <KartuPaketSamping />}

        {/* Antrean yang macet hanya terlihat di sini — beri jumlahnya, jangan
            biarkan penjualan menggantung tanpa ada yang tahu. */}
        {sinkron.perluDiperiksa > 0 && (
          <NavLink
            to="/kasir/belum-terkirim"
            title={ringkas ? `${sinkron.perluDiperiksa} transaksi perlu diperiksa` : undefined}
            className="flex min-h-10 items-center justify-center gap-2 rounded-kontrol bg-permukaan-2 px-3 py-2 text-keterangan font-medium text-jingga-700"
          >
            {ringkas ? sinkron.perluDiperiksa : `${sinkron.perluDiperiksa} transaksi perlu diperiksa`}
          </NavLink>
        )}
        {!ringkas && <StatusKoneksi menunggu={sinkron.menunggu} className="px-2" />}

        {/* Siapa yang sedang masuk — avatar + nama + peran, pola kaki
            navigasi yang lazim di aplikasi SaaS. Di toko yang HP/tabletnya
            dipakai bergantian, ini jawaban cepat untuk "akun siapa ini?". */}
        <div className={cn('flex items-center gap-1', ringkas ? 'flex-col' : 'pl-1')}>
          <span
            className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-sorot text-keterangan font-bold text-hijau-800"
            title={ringkas ? `${profil?.user.name ?? ''} · ${profil?.user.role_name ?? ''}` : undefined}
            aria-hidden
          >
            {inisialNama(profil?.user.name ?? '')}
          </span>
          {!ringkas && (
            <div className="ml-1 min-w-0 flex-1">
              <p className="truncate text-label font-semibold text-teks-utama">{profil?.user.name}</p>
              <p className="truncate text-keterangan text-teks-redup">{profil?.user.role_name}</p>
            </div>
          )}
          <button
            type="button"
            onClick={onUbahRingkas}
            aria-label={ringkas ? 'Lebarkan menu' : 'Ciutkan menu'}
            title={ringkas ? 'Lebarkan menu' : 'Ciutkan menu'}
            className="flex h-12 w-12 shrink-0 items-center justify-center rounded-kontrol text-teks-sekunder hover:bg-permukaan-2 hover:text-teks-utama pointer-fine:h-10 pointer-fine:w-10"
          >
            {ringkas ? (
              <PanelLeftOpen className="h-5 w-5" aria-hidden />
            ) : (
              <PanelLeftClose className="h-5 w-5" aria-hidden />
            )}
          </button>
          <TombolKeluar ringkas className="pointer-fine:h-10 pointer-fine:w-10" />
        </div>
      </div>
    </aside>
  )
}

/**
 * Satu baris menu: ikon, label, gembok bila fiturnya terkunci paket, dan
 * lencana jumlah (Stok). Menu aktif: latar `sorot` + garis aksen hijau di
 * tepi kiri — dua penanda, bukan warna saja.
 */
function TautanSamping({ item, aktif, ringkas }: { item: ItemMenu; aktif: boolean; ringkas: boolean }) {
  const { punyaFitur } = useSesi()
  const terkunci = !!item.fitur && !punyaFitur(item.fitur)
  const lencana = useLencana(item.ke)

  return (
    <Link
      to={item.ke}
      aria-current={aktif ? 'page' : undefined}
      title={ringkas ? item.label : undefined}
      className={cn(
        'relative flex min-h-12 items-center gap-3 rounded-kontrol text-label font-medium pointer-fine:min-h-10',
        ringkas ? 'justify-center' : 'px-2.5',
        aktif
          ? 'bg-sorot text-utama before:absolute before:inset-y-2 before:left-0 before:w-[3px] before:rounded-full before:bg-utama'
          : 'text-teks-sekunder hover:bg-permukaan-2 hover:text-teks-utama',
      )}
    >
      <item.ikon
        className={cn('h-[18px] w-[18px] shrink-0', aktif ? 'text-utama' : 'text-teks-redup')}
        aria-hidden
      />
      <span className={cn('min-w-0 flex-1 truncate', ringkas && 'sr-only')}>{item.label}</span>
      {terkunci && (
        <>
          {!ringkas && <Lock className="h-3.5 w-3.5 shrink-0 text-teks-redup" aria-hidden />}
          <span className="sr-only">(terkunci paket)</span>
        </>
      )}
      {lencana > 0 &&
        (ringkas ? (
          <span className="absolute right-2 top-2 h-2 w-2 rounded-full bg-jingga-600" aria-hidden />
        ) : (
          <span className="rounded-full bg-permukaan-2 px-1.5 text-keterangan font-semibold tabular-nums text-jingga-700">
            {lencana}
          </span>
        ))}
      {lencana > 0 && <span className="sr-only">, {lencana} perlu perhatian</span>}
    </Link>
  )
}

/**
 * Jumlah yang perlu perhatian untuk satu menu (dihitung server atas SELURUH
 * data toko aktif):
 * - Stok: barang hampir habis + habis + minus (GET /stocks/summary);
 * - Utang Pemasok: nota lewat jatuh tempo + segera jatuh tempo
 *   (GET /payables/summary — batas hari sama dengan pengingat WhatsApp).
 * - Kasbon: pelanggan yang kasbonnya lewat jatuh tempo (GET /receivable-summary).
 * Menu lain 0.
 */
function useLencana(ke: string): number {
  const { tokoAktif, boleh } = useSesi()
  const aktif = ke === '/stok' && boleh(IZIN.stockView) && !!tokoAktif
  const q = useQuery({
    queryKey: ['stok-ringkasan', tokoAktif],
    queryFn: () => stokApi.ringkasan(tokoAktif!),
    enabled: aktif,
    staleTime: 60_000,
  })
  const utang = useRingkasanUtang(ke === '/stok/utang')
  const kasbon = useRingkasanKasbon(ke === '/kasbon')
  if (ke === '/stok/utang') return utang.data ? utang.data.overdue_count + utang.data.due_soon_count : 0
  if (ke === '/kasbon') return kasbon.data?.overdue_count ?? 0
  if (!aktif || !q.data) return 0
  return q.data.low + q.data.out + q.data.negative
}

/**
 * Kartu paket di kaki navigasi — hanya untuk yang mengurus langganan
 * (billing.manage), dan hanya saat ada yang perlu diketahui: memakai paket
 * Gratis, atau sedang masa coba (berapa hari lagi). Paket berbayar yang
 * berjalan normal tidak diberi kartu — tidak ada yang perlu dikerjakan.
 */
function KartuPaketSamping() {
  const { profil, boleh } = useSesi()
  const plan = profil?.plan
  if (!plan || !boleh(IZIN.billingManage)) return null

  let judul: string
  let isi: string
  if (plan.status === 'none') {
    judul = `Paket ${plan.name}`
    isi = 'Buka QRIS & lainnya'
  } else if (plan.status === 'trial' && plan.active_until) {
    const sisa = Math.max(0, Math.ceil((new Date(plan.active_until).getTime() - Date.now()) / 86_400_000))
    judul = `Masa coba ${plan.name}`
    isi = sisa === 0 ? 'Berakhir hari ini' : `Berakhir ${sisa} hari lagi`
  } else {
    return null
  }

  // Dua baris pendek, ±56px: kartu yang lebih tinggi ikut memeras daftar menu
  // di layar pendek (tablet 1024×768), padahal ia hanya pengingat.
  return (
    <Link
      to="/langganan"
      className="flex min-h-12 items-center gap-2.5 rounded-kontrol border border-garis bg-latar px-3 py-2 hover:border-utama/40"
    >
      <Sparkles className="h-4 w-4 shrink-0 text-jingga-600" aria-hidden />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-label font-semibold text-teks-utama">{judul}</span>
        <span className="block truncate text-keterangan text-teks-sekunder">{isi}</span>
      </span>
      <span className="shrink-0 text-keterangan font-semibold text-utama">Lihat →</span>
    </Link>
  )
}
