import { useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArrowDownLeft, ArrowUpRight, Info, Wallet } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { Kolom } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris, KerangkaKartuAngka } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatJam, formatTanggalAkrab, tanggalISO } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import type { GerakanKas, Shift } from '@/bersama/tipe/pos'
import { rincianShift, useGerakanKas, useShiftAktif } from '../hooks'

type Arah = 'in' | 'out'

/** Alasan yang paling sering — satu ketukan, dan penulisannya seragam di laporan. */
const ALASAN_UMUM: Record<Arah, string[]> = {
  out: ['Setor ke pemilik', 'Belanja bahan', 'Beli es batu', 'Bayar parkir'],
  in: ['Tambah uang kembalian', 'Modal tambahan dari pemilik', 'Titipan uang'],
}

/**
 * Uang masuk & keluar laci di luar penjualan — ambil kembalian, bayar parkir,
 * setor ke pemilik.
 *
 * Angka ini ikut membentuk "uang yang seharusnya ada di laci" saat tutup shift,
 * jadi keterangannya wajib diisi: tanpa alasan, selisih kas jadi tidak bisa
 * ditelusuri.
 *
 * Yang ditambahkan dibanding versi pertama (yang hanya formulir + daftar):
 * posisi laci SEKARANG dan SESUDAH dicatat — orang mengambil uang dari laci
 * justru untuk tahu "masih cukup untuk kembalian?" — siapa yang mencatat tiap
 * baris, dan alasan sekali ketuk. Formulir dan tombolnya muat satu layar HP;
 * hanya daftarnya yang boleh memanjang ke bawah.
 */
export function HalamanKas() {
  const navigate = useNavigate()
  const { tokoAktif } = useSesi()
  const toast = useToast()
  const { shift, memuat } = useShiftAktif()
  const { daftar, catat } = useGerakanKas(shift?.id)
  const kolomNominal = useRef<HTMLInputElement>(null)

  const [arah, setArah] = useState<Arah>('out')
  const [nominal, setNominal] = useState(0)
  const [alasan, setAlasan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)

  if (memuat) {
    return (
      <div className="flex w-full max-w-5xl flex-col gap-3">
        <KerangkaKartuAngka />
        <KerangkaBaris jumlah={3} />
      </div>
    )
  }

  if (!shift) {
    return (
      <KeadaanKosong
        ikon={Wallet}
        judul="Kasir belum dibuka"
        penjelasan="Uang masuk dan keluar laci hanya bisa dicatat saat kasir sedang dibuka."
        aksi={{ label: 'Buka Kasir', onKlik: () => navigate('/kasir') }}
      />
    )
  }

  const gerakan = daftar.data?.data ?? []
  const { kasMasuk, kasKeluar, seharusnya } = rincianShift(shift)
  const sesudah = seharusnya + (arah === 'in' ? nominal : -nominal)
  const melebihiLaci = arah === 'out' && nominal > seharusnya
  const saran = saranAlasan(arah, gerakan)

  async function kirim(e: React.FormEvent) {
    e.preventDefault()
    if (!tokoAktif || !shift) return
    setGalat(null)
    try {
      await catat.mutateAsync({
        outlet_id: tokoAktif,
        shift_id: shift.id,
        direction: arah,
        amount: nominal,
        reason: alasan.trim(),
      })
      toast.berhasil(
        `${arah === 'in' ? 'Uang masuk' : 'Uang keluar'} ${formatRupiah(nominal)} tercatat. Laci seharusnya ${formatRupiah(sesudah)}.`,
      )
      setNominal(0)
      setAlasan('')
    } catch (e) {
      setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.')
    }
  }

  return (
    <div className="flex w-full max-w-5xl flex-col gap-3 lg:gap-4">
      <header>
        <h1 className="text-judul font-bold text-teks-utama">Uang Masuk & Keluar</h1>
        <p className="text-label text-teks-sekunder">
          Di luar penjualan · shift{' '}
          <strong className="font-semibold text-teks-utama">{shift.opened_by_name ?? 'berjalan'}</strong>{' '}
          sejak {waktuMulai(shift)}
        </p>
      </header>

      <RingkasanLaci
        masuk={kasMasuk}
        keluar={kasKeluar}
        seharusnya={seharusnya}
        jumlahMasuk={gerakan.filter((g) => g.direction === 'in').length}
        jumlahKeluar={gerakan.filter((g) => g.direction === 'out').length}
      />

      {/* ≥1280px: formulir di kiri, catatan shift ini di kanan. Di 1024px
          kolom kanannya tinggal ±290px dan tiap baris terlipat tiga — di
          sana keduanya bertumpuk (formulir tetap di layar pertama). */}
      <div className="flex flex-col gap-3 lg:gap-4 xl:grid xl:grid-cols-[minmax(0,26rem)_minmax(0,1fr)] xl:items-start xl:gap-6">
        <Kartu className="p-4 sm:p-5">
          <form onSubmit={kirim} className="flex flex-col gap-3">
            <div
              role="radiogroup"
              aria-label="Jenis"
              className="grid grid-cols-2 gap-1 rounded-kontrol bg-permukaan-2 p-1"
            >
              <PilihArah
                aktif={arah === 'in'}
                onKlik={() => setArah('in')}
                ikon={ArrowDownLeft}
                label="Uang masuk"
              />
              <PilihArah
                aktif={arah === 'out'}
                onKlik={() => setArah('out')}
                ikon={ArrowUpRight}
                label="Uang keluar"
              />
            </div>

            <KolomUang
              ref={kolomNominal}
              label="Nominal"
              nilai={nominal}
              onNilai={setNominal}
              // Akibatnya pada laci dikatakan SEBELUM dicatat.
              bantuan={
                nominal <= 0
                  ? arah === 'in'
                    ? 'Uang yang dimasukkan ke laci.'
                    : 'Uang yang diambil dari laci.'
                  : melebihiLaci
                    ? `Lebih dari uang di laci (${formatRupiah(seharusnya)}) — pastikan nominalnya benar.`
                    : `Laci seharusnya jadi ${formatRupiah(sesudah)}.`
              }
            />

            <div className="flex flex-col gap-2">
              <Kolom
                label="Keterangan"
                placeholder={arah === 'in' ? 'Contoh: tambah uang kembalian' : 'Contoh: beli plastik kresek'}
                value={alasan}
                onChange={(e) => setAlasan(e.target.value)}
                maxLength={200}
                required
              />
              {/* Satu baris yang bisa digeser — membungkus ke baris kedua
                  mendorong tombol Catat keluar layar HP. */}
              <div
                className="-mx-4 flex gap-1.5 overflow-x-auto px-4 [-ms-overflow-style:none] scrollbar-none sm:-mx-5 sm:px-5 lg:mx-0 lg:flex-wrap lg:px-0 xl:flex-nowrap [&::-webkit-scrollbar]:hidden"
                role="group"
                aria-label="Keterangan yang sering dipakai"
              >
                {saran.map((s) => (
                  <button
                    key={s}
                    type="button"
                    onClick={() => {
                      setAlasan(s)
                      if (nominal <= 0) kolomNominal.current?.focus()
                    }}
                    aria-pressed={alasan === s}
                    className={cn(
                      'flex min-h-11 shrink-0 items-center rounded-full border px-3 text-label whitespace-nowrap',
                      alasan === s
                        ? 'border-utama bg-sorot font-semibold text-utama'
                        : 'border-garis bg-permukaan text-teks-sekunder hover:text-teks-utama',
                    )}
                  >
                    {s}
                  </button>
                ))}
              </div>
            </div>

            {galat && (
              <p
                role="alert"
                className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks"
              >
                {galat}
              </p>
            )}

            <Tombol
              type="submit"
              lebarPenuh
              memuat={catat.isPending}
              labelMemuat="Menyimpan…"
              disabled={nominal <= 0 || alasan.trim().length === 0}
            >
              Catat {arah === 'in' ? 'Uang Masuk' : 'Uang Keluar'}
              {nominal > 0 && ` · ${formatRupiah(nominal)}`}
            </Tombol>
          </form>
        </Kartu>

        <DaftarGerakan gerakan={gerakan} memuat={daftar.isLoading} />
      </div>
    </div>
  )
}

/**
 * Tiga angka laci shift ini. "Seharusnya di laci" adalah angka yang sama
 * dengan yang nanti dicocokkan saat tutup/ganti shift — dari server, bukan
 * dijumlah ulang di sini.
 */
function RingkasanLaci({
  masuk,
  keluar,
  seharusnya,
  jumlahMasuk,
  jumlahKeluar,
}: {
  masuk: number
  keluar: number
  seharusnya: number
  jumlahMasuk: number
  jumlahKeluar: number
}) {
  return (
    <>
      {/* HP: tiga kolom sama lebar memotong "Rp 180.0…". Angka utamanya
          dibuat besar, masuk/keluar bertumpuk kecil di sebelahnya. */}
      <Kartu className="flex items-center justify-between gap-3 px-4 py-3 sm:hidden">
        <div className="min-w-0">
          <p className="flex items-center gap-1.5 text-keterangan text-teks-redup">
            <Wallet className="h-4 w-4 text-utama" aria-hidden />
            Seharusnya di laci
          </p>
          <p className="text-judul-kartu font-bold tabular-nums text-teks-utama">
            {formatRupiah(seharusnya)}
          </p>
        </div>
        <dl className="flex shrink-0 flex-col items-end gap-0.5 text-keterangan tabular-nums">
          <div className="flex items-center gap-1">
            <dt className="sr-only">Masuk</dt>
            <ArrowDownLeft className="h-3.5 w-3.5 text-hijau-800" aria-hidden />
            <dd className="font-semibold text-hijau-800">+{formatRupiah(masuk)}</dd>
          </div>
          <div className="flex items-center gap-1">
            <dt className="sr-only">Keluar</dt>
            <ArrowUpRight className="h-3.5 w-3.5 text-jingga-700" aria-hidden />
            <dd className="font-semibold text-teks-utama">
              {keluar > 0 ? `−${formatRupiah(keluar)}` : formatRupiah(0)}
            </dd>
          </div>
        </dl>
      </Kartu>

      <Kartu className="hidden grid-cols-3 divide-x divide-garis sm:grid">
        <AngkaLaci
          label={`Masuk · ${jumlahMasuk}×`}
          nilai={`+${formatRupiah(masuk)}`}
          ikon={<ArrowDownLeft className="h-4 w-4 text-hijau-800" aria-hidden />}
        />
        <AngkaLaci
          label={`Keluar · ${jumlahKeluar}×`}
          nilai={keluar > 0 ? `−${formatRupiah(keluar)}` : formatRupiah(0)}
          ikon={<ArrowUpRight className="h-4 w-4 text-jingga-700" aria-hidden />}
        />
        <AngkaLaci
          label="Seharusnya di laci"
          nilai={formatRupiah(seharusnya)}
          ikon={<Wallet className="h-4 w-4 text-utama" aria-hidden />}
          tebal
        />
      </Kartu>
    </>
  )
}

function AngkaLaci({
  label,
  nilai,
  ikon,
  tebal,
}: {
  label: string
  nilai: string
  ikon: React.ReactNode
  tebal?: boolean
}) {
  return (
    <div className="flex min-w-0 flex-col gap-0.5 px-4 py-3">
      <p className="flex items-center gap-1.5 text-keterangan text-teks-redup">
        {ikon}
        <span className="truncate">{label}</span>
      </p>
      <p
        className={cn(
          'truncate tabular-nums text-teks-utama',
          tebal ? 'text-judul-kartu font-bold' : 'text-isi font-semibold',
        )}
      >
        {nilai}
      </p>
    </div>
  )
}

function DaftarGerakan({ gerakan, memuat }: { gerakan: GerakanKas[]; memuat: boolean }) {
  return (
    <section className="flex flex-col gap-2">
      <h2 className="flex items-baseline justify-between gap-3 text-judul-kartu font-semibold text-teks-utama">
        Tercatat di shift ini
        {gerakan.length > 0 && (
          <span className="text-label font-normal text-teks-redup">{gerakan.length} catatan</span>
        )}
      </h2>
      {memuat ? (
        <KerangkaBaris jumlah={2} />
      ) : gerakan.length === 0 ? (
        <div className="flex flex-col items-center gap-1 rounded-kartu border border-dashed border-garis px-6 py-8 text-center">
          <Wallet className="mb-1 h-8 w-8 text-teks-redup" aria-hidden />
          <p className="text-isi font-medium text-teks-utama">Belum ada uang masuk atau keluar</p>
          <p className="text-label text-teks-redup">
            Mis. setor ke pemilik, beli es batu, atau tambah uang kembalian.
          </p>
        </div>
      ) : (
        <Kartu className="overflow-hidden">
          {/* Layar lebar: daftarnya yang menggulir, bukan halamannya. */}
          <ul className="flex flex-col divide-y divide-garis xl:max-h-[calc(100dvh-19rem)] xl:overflow-y-auto">
            {gerakan.map((g) => (
              <BarisGerakan key={g.id} g={g} />
            ))}
          </ul>
          <p className="flex items-start gap-2 border-t border-garis bg-permukaan-2 px-4 py-2.5 text-keterangan text-teks-redup">
            <Info className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden />
            Catatan tidak bisa dihapus supaya laci selalu bisa ditelusuri. Salah catat? Catat
            kebalikannya dengan keterangan &ldquo;koreksi&rdquo;.
          </p>
        </Kartu>
      )}
    </section>
  )
}

function BarisGerakan({ g }: { g: GerakanKas }) {
  const masuk = g.direction === 'in'
  return (
    <li className="flex items-start gap-3 px-4 py-3">
      <span
        className={cn(
          'mt-0.5 flex h-9 w-9 shrink-0 items-center justify-center rounded-full',
          masuk ? 'bg-sorot text-hijau-800' : 'bg-permukaan-2 text-jingga-700',
        )}
        aria-hidden
      >
        {masuk ? <ArrowDownLeft className="h-4 w-4" /> : <ArrowUpRight className="h-4 w-4" />}
      </span>
      <div className="min-w-0 flex-1">
        <p className="text-label font-medium text-teks-utama">{g.reason}</p>
        <p className="text-keterangan text-teks-redup">
          {masuk ? 'Masuk' : 'Keluar'} · {formatJam(g.occurred_at)}
          {g.created_by_name && (
            <>
              {' · '}
              <span className="whitespace-nowrap">{g.created_by_name}</span>
            </>
          )}
        </p>
      </div>
      <p
        className={cn(
          'shrink-0 font-bold tabular-nums',
          masuk ? 'text-hijau-800' : 'text-teks-utama',
        )}
      >
        {masuk ? '+' : '−'}
        {formatRupiah(g.amount)}
      </p>
    </li>
  )
}

function PilihArah({
  aktif,
  onKlik,
  ikon: Ikon,
  label,
}: {
  aktif: boolean
  onKlik: () => void
  ikon: typeof ArrowDownLeft
  label: string
}) {
  return (
    <button
      type="button"
      role="radio"
      aria-checked={aktif}
      onClick={onKlik}
      className={cn(
        'flex h-11 items-center justify-center gap-2 rounded-[calc(var(--radius-kontrol)-2px)] text-label font-semibold transition-colors',
        'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-utama',
        aktif ? 'bg-permukaan text-utama shadow-kartu' : 'text-teks-sekunder hover:text-teks-utama',
      )}
    >
      <Ikon className="h-4 w-4" aria-hidden />
      {label}
    </button>
  )
}

/** Alasan yang baru dipakai di shift ini lebih dulu, lalu yang umum — tanpa ganda. */
function saranAlasan(arah: Arah, gerakan: GerakanKas[]): string[] {
  const hasil: string[] = []
  const sudah = new Set<string>()
  for (const s of [...gerakan.filter((g) => g.direction === arah).map((g) => g.reason), ...ALASAN_UMUM[arah]]) {
    const kunci = s.trim().toLocaleLowerCase('id-ID')
    if (!kunci || sudah.has(kunci)) continue
    sudah.add(kunci)
    hasil.push(s.trim())
  }
  return hasil.slice(0, 6)
}

/** "09.22" bila hari ini, "Kemarin 21.10" / "18 Sep 2026 09.22" bila lebih lama. */
function waktuMulai(shift: Shift): string {
  if (shift.business_date === tanggalISO()) return formatJam(shift.opened_at)
  return `${formatTanggalAkrab(shift.opened_at)} ${formatJam(shift.opened_at)}`
}
