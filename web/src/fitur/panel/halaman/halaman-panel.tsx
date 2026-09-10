import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { AlertTriangle, Coins, LogOut, RefreshCw, ShieldCheck, Users } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { adaSesi, hapusSesi, langganSesi, simpanSesi } from '@/lib/penyimpanan-sesi'
import { GalatAPI } from '@/lib/api-client'
import { formatTanggalJam, tanggalISO } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { KEMAMPUAN, panelApi, type AdminPanel } from '../api'

/**
 * Panel internal penyedia SaaS.
 *
 * Bukan bagian dari urutan U0–U6: penggunanya staf kita sendiri, bukan
 * pelanggan. Karena itu (ui/06-ROADMAP-UI.md §UX):
 *
 *   - Desktop saja, tanpa offline dan tanpa PWA.
 *   - Menu dibentuk dari `capabilities` yang dikembalikan GET /platform/me,
 *     tidak pernah dari nama peran yang di-hardcode.
 *   - Nadanya alat kerja: tanpa sapaan, tanpa kalimat menyemangati.
 */

// ── Sesi panel ─────────────────────────────────────────────────────────────

interface NilaiSesiPanel {
  admin: AdminPanel | undefined
  memuat: boolean
  sudahMasuk: boolean
  bisa: (kemampuan: string) => boolean
  masuk: (email: string, sandi: string) => Promise<void>
  keluar: () => void
}

const KonteksPanel = createContext<NilaiSesiPanel | null>(null)

export function HalamanPanel() {
  return (
    <PenyediaSesiPanel>
      <IsiPanel />
    </PenyediaSesiPanel>
  )
}

function PenyediaSesiPanel({ children }: { children: ReactNode }) {
  const qc = useQueryClient()
  const [punyaSesi, setPunyaSesi] = useState(() => adaSesi('platform'))

  useEffect(() => langganSesi(() => setPunyaSesi(adaSesi('platform'))), [])

  const { data, isLoading } = useQuery({
    queryKey: ['panel-me'],
    queryFn: panelApi.saya,
    enabled: punyaSesi,
    retry: false,
    staleTime: 5 * 60_000,
  })

  const masuk = useCallback(
    async (email: string, sandi: string) => {
      const hasil = await panelApi.masuk(email, sandi)
      simpanSesi('platform', {
        access_token: hasil.access_token,
        refresh_token: '',
        kedaluwarsa: new Date(hasil.expires_at).getTime(),
      })
      setPunyaSesi(true)
      await qc.invalidateQueries({ queryKey: ['panel-me'] })
    },
    [qc],
  )

  const keluar = useCallback(() => {
    hapusSesi('platform')
    setPunyaSesi(false)
    qc.removeQueries({ queryKey: ['panel-me'] })
  }, [qc])

  const nilai = useMemo<NilaiSesiPanel>(
    () => ({
      admin: data,
      memuat: punyaSesi && isLoading,
      sudahMasuk: punyaSesi && !!data,
      bisa: (k) => !!data?.capabilities.includes(k),
      masuk,
      keluar,
    }),
    [data, punyaSesi, isLoading, masuk, keluar],
  )

  return <KonteksPanel.Provider value={nilai}>{children}</KonteksPanel.Provider>
}

function usePanel(): NilaiSesiPanel {
  const v = useContext(KonteksPanel)
  if (!v) throw new Error('usePanel dipakai di luar PenyediaSesiPanel')
  return v
}

// ── Kerangka ───────────────────────────────────────────────────────────────

type Tab = 'mitra' | 'komisi' | 'outbox'

function IsiPanel() {
  const { sudahMasuk, memuat, admin, keluar, bisa } = usePanel()
  const [tab, setTab] = useState<Tab>('mitra')

  const outbox = useQuery({
    queryKey: ['panel-outbox-mati'],
    queryFn: () => panelApi.outbox('dead'),
    enabled: sudahMasuk,
    refetchInterval: 60_000,
  })

  if (memuat) {
    return (
      <div className="mx-auto max-w-4xl p-6">
        <KerangkaBaris jumlah={4} />
      </div>
    )
  }

  if (!sudahMasuk) return <LayarMasukPanel />

  const mati = outbox.data?.length ?? 0

  const menu: { kunci: Tab; label: string; ikon: typeof Users; lencana?: number }[] = [
    { kunci: 'mitra', label: 'Mitra', ikon: Users },
    ...(bisa(KEMAMPUAN.keuanganMitra)
      ? [{ kunci: 'komisi' as const, label: 'Komisi & Pencairan', ikon: Coins }]
      : []),
    { kunci: 'outbox', label: 'Antrean Notifikasi', ikon: AlertTriangle, lencana: mati },
  ]

  return (
    <div className="min-h-dvh bg-latar">
      <header className="border-b border-garis bg-permukaan">
        <div className="mx-auto flex w-full max-w-5xl items-center justify-between gap-4 px-6 py-3">
          <div>
            <p className="text-keterangan font-semibold uppercase tracking-wide text-teks-redup">
              Panel Internal
            </p>
            {/* Peran ditampilkan permanen: pemisahan wewenang bukan sekadar
                teknis. Staf yang tidak menemukan tombol harus bisa melihat
                sendiri kenapa — bukan mengira aplikasinya rusak. */}
            <p className="text-label text-teks-sekunder">
              {admin?.name} · peran <strong className="text-teks-utama">{admin?.role}</strong>
            </p>
          </div>
          <button
            type="button"
            onClick={keluar}
            className="flex h-10 items-center gap-2 rounded-kontrol px-3 text-label font-medium text-teks-sekunder hover:bg-permukaan-2"
          >
            <LogOut className="h-5 w-5" aria-hidden />
            Keluar
          </button>
        </div>
      </header>

      <div className="mx-auto flex w-full max-w-5xl gap-6 px-6 py-6">
        <nav aria-label="Menu panel" className="w-56 shrink-0">
          <ul className="flex flex-col gap-1">
            {menu.map((m) => (
              <li key={m.kunci}>
                <button
                  type="button"
                  onClick={() => setTab(m.kunci)}
                  aria-current={tab === m.kunci ? 'page' : undefined}
                  className={cn(
                    'flex h-11 w-full items-center gap-2 rounded-kontrol px-3 text-label font-medium',
                    tab === m.kunci
                      ? 'bg-sorot text-utama'
                      : 'text-teks-sekunder hover:bg-permukaan-2',
                  )}
                >
                  <m.ikon className="h-5 w-5 shrink-0" aria-hidden />
                  <span className="flex-1 text-left">{m.label}</span>
                  {/* Antrean mati wajib mencolok — hanya di sini ia terlihat. */}
                  {!!m.lencana && m.lencana > 0 && (
                    <span className="rounded-full bg-bahaya px-1.5 text-keterangan font-bold text-white">
                      {m.lencana}
                    </span>
                  )}
                </button>
              </li>
            ))}
          </ul>

          <p className="mt-4 px-3 text-keterangan text-teks-redup">
            Menu disusun dari kemampuan peran Anda, bukan daftar tetap.
          </p>
        </nav>

        <main className="min-w-0 flex-1">
          {tab === 'mitra' && <PanelMitra />}
          {tab === 'komisi' && <PanelKomisi />}
          {tab === 'outbox' && <PanelOutbox />}
        </main>
      </div>
    </div>
  )
}

function LayarMasukPanel() {
  const { masuk } = usePanel()
  const [email, setEmail] = useState('')
  const [sandi, setSandi] = useState('')
  const [galat, setGalat] = useState<string | null>(null)
  const [mengirim, setMengirim] = useState(false)

  return (
    <div className="flex min-h-dvh items-center justify-center bg-latar px-4">
      <div className="w-full max-w-sm">
        <div className="mb-6 text-center">
          <ShieldCheck className="mx-auto h-10 w-10 text-teks-sekunder" aria-hidden />
          <h1 className="mt-3 text-judul font-bold text-teks-utama">Panel Internal</h1>
          <p className="mt-1 text-label text-teks-sekunder">
            Khusus staf. Bukan pintu masuk pemilik toko atau mitra.
          </p>
        </div>

        <form
          onSubmit={async (e) => {
            e.preventDefault()
            setGalat(null)
            setMengirim(true)
            try {
              await masuk(email.trim(), sandi)
            } catch (e) {
              setGalat(
                e instanceof GalatAPI && e.status === 401
                  ? 'Email atau kata sandi salah.'
                  : e instanceof GalatAPI
                    ? e.pesan
                    : 'Terjadi kesalahan.',
              )
            } finally {
              setMengirim(false)
            }
          }}
          className="flex flex-col gap-4"
          noValidate
        >
          <Kolom
            label="Email"
            type="email"
            autoCapitalize="none"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            required
          />
          <Kolom
            label="Kata sandi"
            type="password"
            value={sandi}
            onChange={(e) => setSandi(e.target.value)}
            required
          />
          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-red-50 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}
          <Tombol type="submit" lebarPenuh memuat={mengirim} disabled={!email || !sandi}>
            Masuk
          </Tombol>
        </form>
      </div>
    </div>
  )
}

// ── Mitra ──────────────────────────────────────────────────────────────────

function PanelMitra() {
  const { bisa } = usePanel()
  const toast = useToast()
  const qc = useQueryClient()

  const mitra = useQuery({ queryKey: ['panel-mitra'], queryFn: panelApi.daftarMitra })

  const ubah = useMutation({
    mutationFn: ({ id, aksi }: { id: string; aksi: 'setujui' | 'bekukan' }) =>
      aksi === 'setujui' ? panelApi.setujuiMitra(id) : panelApi.bekukanMitra(id),
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: ['panel-mitra'] })
      toast.berhasil(v.aksi === 'setujui' ? 'Mitra diverifikasi.' : 'Mitra dibekukan.')
    },
    onError: (e) => toast.gagal(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  const bolehVerifikasi = bisa(KEMAMPUAN.verifikasiMitra)

  return (
    <section className="flex flex-col gap-3">
      <h2 className="text-judul font-bold text-teks-utama">Mitra</h2>

      {!bolehVerifikasi && (
        <p className="rounded-kontrol bg-permukaan-2 px-3 py-2 text-keterangan text-teks-sekunder">
          Peran Anda hanya bisa membaca daftar ini. Verifikasi dan pembekuan
          mitra dikerjakan peran operator atau superadmin.
        </p>
      )}

      {mitra.isLoading ? (
        <KerangkaBaris jumlah={4} />
      ) : (mitra.data?.data.length ?? 0) === 0 ? (
        <p className="text-isi text-teks-redup">Belum ada mitra terdaftar.</p>
      ) : (
        <Kartu className="divide-y divide-garis">
          {mitra.data?.data.map((m) => (
            <div key={m.id} className="flex items-center justify-between gap-3 p-4">
              <div className="min-w-0">
                <p className="truncate font-medium text-teks-utama">{m.name}</p>
                <p className="text-keterangan text-teks-redup">
                  {m.tier_name} · {m.referral_code}
                  {m.region && ` · ${m.region}`}
                </p>
              </div>
              <div className="flex shrink-0 items-center gap-2">
                <StatusMitra status={m.status} />
                {bolehVerifikasi && m.status !== 'active' && (
                  <Tombol
                    ukuran="padat"
                    memuat={ubah.isPending}
                    onClick={() => ubah.mutate({ id: m.id, aksi: 'setujui' })}
                  >
                    Verifikasi
                  </Tombol>
                )}
                {bolehVerifikasi && m.status === 'active' && (
                  <Tombol
                    jenis="kedua"
                    ukuran="padat"
                    memuat={ubah.isPending}
                    onClick={() => ubah.mutate({ id: m.id, aksi: 'bekukan' })}
                  >
                    Bekukan
                  </Tombol>
                )}
              </div>
            </div>
          ))}
        </Kartu>
      )}
    </section>
  )
}

// ── Komisi ─────────────────────────────────────────────────────────────────

function PanelKomisi() {
  const toast = useToast()
  const kini = new Date()
  const [dari, setDari] = useState(
    `${kini.getFullYear()}-${String(kini.getMonth() + 1).padStart(2, '0')}-01`,
  )
  const [sampai, setSampai] = useState(tanggalISO())
  const [hasil, setHasil] = useState<Awaited<
    ReturnType<typeof panelApi.jalankanKomisi>
  > | null>(null)

  const jalankan = useMutation({
    mutationFn: () => panelApi.jalankanKomisi(dari, sampai),
    onSuccess: (h) => {
      setHasil(h)
      toast.berhasil(`${h.computed} komisi dihitung.`)
    },
    onError: (e) => toast.gagal(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  return (
    <section className="flex flex-col gap-3">
      <h2 className="text-judul font-bold text-teks-utama">Komisi & Pencairan</h2>

      <Kartu className="flex flex-col gap-4 p-4">
        <div className="grid gap-3 sm:grid-cols-2">
          <Kolom label="Dari" type="date" value={dari} onChange={(e) => setDari(e.target.value)} />
          <Kolom
            label="Sampai"
            type="date"
            value={sampai}
            onChange={(e) => setSampai(e.target.value)}
          />
        </div>
        <Tombol memuat={jalankan.isPending} onClick={() => jalankan.mutate()} className="self-start">
          Jalankan Perhitungan Komisi
        </Tombol>
      </Kartu>

      {hasil && (
        <Kartu className="p-4">
          <h3 className="mb-2 text-judul-kartu font-semibold text-teks-utama">Hasil</h3>
          <dl className="grid grid-cols-2 gap-x-6 gap-y-1 text-label">
            <Baris label="Referral diperiksa" nilai={hasil.referrals_seen} />
            <Baris label="Baru aktif" nilai={hasil.activated} />
            <Baris label="Komisi dihitung" nilai={hasil.computed} />
            <Baris label="Ditarik kembali" nilai={hasil.clawed_back} />
            <Baris label="Dilewati (belum aktif)" nilai={hasil.skipped_no_activation} />
          </dl>
        </Kartu>
      )}
    </section>
  )
}

function Baris({ label, nilai }: { label: string; nilai: number }) {
  return (
    <>
      <dt className="text-teks-sekunder">{label}</dt>
      <dd className="text-right tabular-nums text-teks-utama">{nilai}</dd>
    </>
  )
}

// ── Outbox ─────────────────────────────────────────────────────────────────

function PanelOutbox() {
  const { bisa } = usePanel()
  const toast = useToast()
  const qc = useQueryClient()
  const [saring, setSaring] = useState<'dead' | 'pending' | ''>('dead')

  const outbox = useQuery({
    queryKey: ['panel-outbox', saring],
    queryFn: () => panelApi.outbox(saring || undefined),
  })

  const ulangi = useMutation({
    mutationFn: (id: string) => panelApi.ulangiOutbox(id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['panel-outbox'] })
      qc.invalidateQueries({ queryKey: ['panel-outbox-mati'] })
      toast.berhasil('Dijadwalkan untuk dikirim ulang.')
    },
    onError: (e) => toast.gagal(e instanceof GalatAPI ? e.pesan : 'Gagal menjadwalkan.'),
  })

  const bolehUlangi = bisa(KEMAMPUAN.kelolaAdmin)
  const daftar = outbox.data ?? []

  return (
    <section className="flex flex-col gap-3">
      <h2 className="text-judul font-bold text-teks-utama">Antrean Notifikasi</h2>
      <p className="text-label text-teks-sekunder">
        Notifikasi yang gagal berkali-kali hanya terlihat di sini — tidak ada
        tempat lain yang menampilkannya.
      </p>

      <div className="flex gap-2">
        {(
          [
            ['dead', 'Gagal total'],
            ['pending', 'Menunggu'],
            ['', 'Semua'],
          ] as const
        ).map(([nilai, label]) => (
          <button
            key={label}
            type="button"
            onClick={() => setSaring(nilai)}
            aria-pressed={saring === nilai}
            className={cn(
              'h-12 rounded-full border px-4 text-label font-medium',
              saring === nilai
                ? 'border-utama bg-sorot text-utama'
                : 'border-garis bg-permukaan text-teks-sekunder hover:bg-permukaan-2',
            )}
          >
            {label}
          </button>
        ))}
      </div>

      {outbox.isLoading ? (
        <KerangkaBaris jumlah={4} />
      ) : daftar.length === 0 ? (
        <p className="text-isi text-teks-redup">
          {saring === 'dead'
            ? 'Tidak ada notifikasi yang gagal total. Antreannya sehat.'
            : 'Antreannya kosong.'}
        </p>
      ) : (
        <Kartu className="divide-y divide-garis">
          {daftar.map((o) => (
            <div key={o.id} className="flex items-start justify-between gap-3 p-4">
              <div className="min-w-0">
                <p className="font-medium text-teks-utama">{o.topic}</p>
                <p className="text-keterangan text-teks-redup">
                  {formatTanggalJam(o.created_at)} · {o.attempts}× dicoba
                </p>
                {/* Alasan gagal ditampilkan apa adanya — staf butuh sebabnya,
                    bukan sekadar tahu ada yang gagal. */}
                {o.last_error && (
                  <p className="mt-1 break-words rounded-kontrol bg-permukaan-2 px-2 py-1 text-keterangan text-bahaya-teks">
                    {o.last_error}
                  </p>
                )}
              </div>
              <div className="flex shrink-0 items-center gap-2">
                {o.status === 'dead' ? (
                  <LencanaStatus nada="bahaya" anak="Gagal total" />
                ) : o.status === 'sent' ? (
                  <LencanaStatus nada="berhasil" anak="Terkirim" />
                ) : (
                  <LencanaStatus nada="menunggu" anak="Menunggu" />
                )}
                {bolehUlangi && o.status === 'dead' && (
                  <Tombol
                    jenis="kedua"
                    ukuran="padat"
                    memuat={ulangi.isPending}
                    onClick={() => ulangi.mutate(o.id)}
                  >
                    <RefreshCw className="h-4 w-4" aria-hidden />
                    Coba kirim lagi
                  </Tombol>
                )}
              </div>
            </div>
          ))}
        </Kartu>
      )}
    </section>
  )
}

function StatusMitra({ status }: { status: string }) {
  switch (status) {
    case 'active':
      return <LencanaStatus nada="berhasil" anak="Aktif" />
    case 'pending':
    case 'pending_verification':
      return <LencanaStatus nada="menunggu" anak="Menunggu verifikasi" />
    case 'suspended':
      return <LencanaStatus nada="bahaya" anak="Dibekukan" />
    default:
      return <LencanaStatus nada="netral" anak={status} />
  }
}
