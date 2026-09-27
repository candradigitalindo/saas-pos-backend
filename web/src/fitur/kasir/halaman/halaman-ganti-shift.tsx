import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  ArrowRight,
  ArrowRightLeft,
  Calculator,
  Check,
  ChevronRight,
  CircleCheck,
  Plus,
  RotateCcw,
  TriangleAlert,
  UserRoundCog,
  X,
} from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { Kolom } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka, KerangkaKartuAngka } from '@/bersama/komponen/kerangka'
import { GalatAdaAntrean, useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import {
  formatJam,
  formatTanggalAkrab,
  formatTanggalJam,
  keDate,
  tanggalISO,
} from '@/bersama/util/tanggal'
import { inisialNama } from '@/bersama/util/inisial'
import { cn } from '@/bersama/util/cn'
import type { Shift } from '@/bersama/tipe/pos'
import { rincianShift, useGerakanKas, useSerahTerimaShift, useShiftAktif } from '../hooks'
import { DaftarCaraBayar } from '../komponen/daftar-cara-bayar'
import { DialogPecahan, HITUNGAN_KOSONG, type HitunganPecahan } from '../komponen/penghitung-laci'

/**
 * Ganti shift — kasir berganti orang tanpa menutup kasirnya.
 *
 * Sebelum layar ini ada, pergantian dilakukan dengan Tutup Shift lalu Buka
 * Shift lagi. Tiga langkah, dan yang paling merugikan: uang laci yang baru
 * saja dihitung TIDAK terbawa, jadi kasir berikutnya mengetik ulang modal
 * awalnya. Angka yang diketik ulang adalah angka yang bisa salah ketik, dan
 * salah ketiknya baru ketahuan saat tutup buku malam hari.
 *
 * Serah terima adalah momen DUA ORANG, jadi layarnya menyebut siapa
 * menyerahkan ke siapa — dan bahwa shift baru dicatat atas nama AKUN YANG
 * SEDANG MASUK. Dulu hal terakhir itu tidak disebut, sehingga kasir berikutnya
 * lazim berjualan di akun kasir sebelumnya tanpa sadar.
 *
 * SATU LAYAR, TANPA MENGGULIR. Yang dikerjakan di sini hanya dua angka (hasil
 * hitung dan yang ditinggal); rincian lain — penjualan per cara bayar, rumus
 * laci di HP, hitung per pecahan, catatan — dibuka bila diminta. Versi
 * sebelumnya menumpuk semuanya dan tombol Serahkan ada di bawah tiga layar HP.
 *
 * NADA BAHASANYA SAMA dengan Tutup Shift: selisih kecil itu wajar, dan UI yang
 * menuduh membuat kasir berhenti jujur (ui/05-ALUR-UTAMA.md §3).
 */
export function HalamanGantiShift() {
  const navigate = useNavigate()
  const { profil } = useSesi()
  const { shift, memuat } = useShiftAktif()
  const serah = useSerahTerimaShift()

  const [dihitung, setDihitung] = useState(0)
  const [sudahIsi, setSudahIsi] = useState(false)
  const [pecahan, setPecahan] = useState<HitunganPecahan>(HITUNGAN_KOSONG)
  const [dialog, setDialog] = useState<'pecahan' | 'rincian' | 'akun' | null>(null)
  const [sisakan, setSisakan] = useState<'semua' | 'sebagian'>('semua')
  const [ditinggal, setDitinggal] = useState(0)
  const [bukaCatatan, setBukaCatatan] = useState(false)
  const [catatan, setCatatan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)
  const [hasil, setHasil] = useState<{ ditutup: Shift; dibuka: Shift; kasirLama: string } | null>(
    null,
  )

  if (hasil) {
    return <HasilSerahTerima {...hasil} kasirBaru={profil?.user.name ?? ''} />
  }

  if (memuat) {
    return (
      <div className="flex w-full max-w-5xl flex-col gap-3">
        <Kerangka className="h-8 w-48" />
        <div className="grid gap-4 lg:grid-cols-2">
          <KerangkaKartuAngka />
          <KerangkaKartuAngka />
        </div>
      </div>
    )
  }

  if (!shift) {
    return (
      <KeadaanKosong
        ikon={ArrowRightLeft}
        judul="Kasir sedang tidak dibuka"
        penjelasan="Tidak ada shift berjalan yang bisa diserahterimakan. Buka kasir dulu."
        aksi={{ label: 'Buka Kasir', onKlik: () => navigate('/kasir') }}
      />
    )
  }

  const { seharusnya, modalAwal } = rincianShift(shift)
  const selisih = dihitung - seharusnya
  const modalBerikutnya = sisakan === 'sebagian' ? ditinggal : dihitung
  const disetor = dihitung - modalBerikutnya
  const modalKebanyakan = sisakan === 'sebagian' && ditinggal > dihitung
  const kasirLama = shift.opened_by_name || 'Kasir sebelumnya'
  const kasirBaru = profil?.user.name ?? 'Anda'
  const orangSama = !!profil && profil.user.id === shift.opened_by

  const isiHitung = (n: number) => {
    setDihitung(n)
    setSudahIsi(true)
  }

  async function kirim(e: React.FormEvent) {
    e.preventDefault()
    if (!shift) return
    setGalat(null)
    try {
      const r = await serah.mutateAsync({
        id: shift.id,
        uangDihitung: dihitung,
        modalDitinggal: sisakan === 'sebagian' ? ditinggal : undefined,
        catatan: catatan.trim() || undefined,
      })
      setHasil({ ...r, kasirLama })
      window.scrollTo({ top: 0 })
    } catch (e) {
      setGalat(
        e instanceof GalatAPI
          ? e.status === 409
            ? 'Shift ini sudah ditutup orang lain. Muat ulang halaman.'
            : e.pesan
          : 'Serah terima gagal. Coba lagi.',
      )
    }
  }

  return (
    <div className="flex w-full max-w-5xl flex-col gap-3 lg:gap-4">
      {/* HP & tablet: siapa-ke-siapa jadi anak judul, bukan kartu sendiri —
          supaya seluruh layar ini (sampai tombol Serahkan) muat tanpa digulir. */}
      <header className="flex flex-col gap-1">
        <div className="flex items-center justify-between gap-3">
          <h1 className="text-judul font-bold text-teks-utama">Ganti Shift</h1>
          <TombolRincian onKlik={() => setDialog('rincian')} className="lg:hidden" />
        </div>
        <p className="hidden text-label text-teks-sekunder lg:block">
          Hitung uang laci bersama, lalu kasir berikutnya langsung melanjutkan — kasir tidak perlu
          ditutup.
        </p>
        <div className="flex flex-col gap-1 lg:hidden">
          <p className="text-label text-teks-sekunder">
            <strong className="whitespace-nowrap font-semibold text-teks-utama">{kasirLama}</strong>
            <ArrowRight
              className="mx-1 inline h-4 w-4 align-[-3px] text-utama"
              aria-label="menyerahkan ke"
            />
            <strong className="whitespace-nowrap font-semibold text-teks-utama">{kasirBaru}</strong>
            <span>
              {' '}
              · sejak {waktuMulai(shift)}
              {shift.sales && ` · ${shift.sales.sales_count} transaksi`}
            </span>
          </p>
          {orangSama && <SaranGantiAkun kasir={kasirBaru} onGanti={() => setDialog('akun')} />}
        </div>
      </header>

      {/* Layar lebar: yang diserahkan (siapa & rumus laci) di kiri, formulirnya di kanan. */}
      <div className="flex flex-col gap-3 lg:grid lg:grid-cols-[minmax(0,5fr)_minmax(0,6fr)] lg:items-start lg:gap-6">
        <div className="hidden flex-col gap-4 lg:flex">
          <PitaSerahTerima
            shift={shift}
            kasirLama={kasirLama}
            kasirBaru={kasirBaru}
            orangSama={orangSama}
            onRincian={() => setDialog('rincian')}
            onGantiAkun={() => setDialog('akun')}
          />
          <Kartu className="flex flex-col gap-3 p-5">
            <h2 className="text-judul-kartu font-semibold text-teks-utama">
              Uang yang seharusnya ada di laci
            </h2>
            <RumusLaci shift={shift} />
          </Kartu>
        </div>

        <form onSubmit={kirim}>
          <Kartu className="flex flex-col gap-2.5 p-4 sm:gap-3 sm:p-5">
            <KolomUang
              label="Hasil hitung laci"
              // Pembandingnya di baris label: di layar lebar angka ini sudah ada
              // di kolom kiri lengkap dengan rumusnya.
              labelKanan={
                <span className="text-label text-teks-sekunder lg:hidden">
                  Seharusnya{' '}
                  <strong className="font-bold tabular-nums text-teks-utama">
                    {formatRupiah(seharusnya)}
                  </strong>
                </span>
              }
              nilai={dihitung}
              onNilai={isiHitung}
              // Hanya di layar lebar, tempat kolomnya terlihat tanpa menggulir.
              // Di HP fokus otomatis memunculkan papan tik yang menutupi
              // siapa-ke-siapa sebelum sempat dibaca.
              autoFocus={layarLebar()}
              bantuan={sudahIsi ? undefined : 'Hitung berdua dengan kasir berikutnya.'}
              sisipanAkhir={
                // Area ketuk 44px setinggi kotaknya; pil 36px di dalamnya
                // hanya tampilan.
                <button
                  type="button"
                  onClick={() => setDialog('pecahan')}
                  className="group -mr-1.5 flex h-11 shrink-0 items-center"
                >
                  <span className="flex h-9 items-center gap-1.5 rounded-full bg-sorot px-3 text-keterangan font-semibold text-hijau-800 group-hover:brightness-95">
                    <Calculator className="h-4 w-4" aria-hidden />
                    Per pecahan
                  </span>
                </button>
              }
            />
            {sudahIsi && <HasilHitung selisih={selisih} />}

            <div className="flex flex-col gap-1">
              {/* "Catatan" menumpang di baris label ini — baris sendiri untuk
                  tautan yang jarang dipakai membuat tombol Serahkan terdorong
                  keluar layar HP. */}
              <div className="flex items-center justify-between gap-3">
                <p id="label-sisa" className="text-label font-medium text-teks-sekunder">
                  Untuk kasir berikutnya
                </p>
                {!bukaCatatan && (
                  <button
                    type="button"
                    onClick={() => setBukaCatatan(true)}
                    className="-mr-1 flex min-h-11 shrink-0 items-center gap-1 px-1 text-label font-medium text-utama hover:underline"
                  >
                    <Plus className="h-4 w-4" aria-hidden />
                    Catatan
                  </button>
                )}
              </div>
              <div
                role="radiogroup"
                aria-labelledby="label-sisa"
                className="grid grid-cols-2 gap-1 rounded-full bg-permukaan-2 p-1"
              >
                {(
                  [
                    ['semua', 'Tinggal semua'],
                    ['sebagian', 'Setor sebagian'],
                  ] as const
                ).map(([v, teks]) => (
                  <button
                    key={v}
                    type="button"
                    role="radio"
                    aria-checked={sisakan === v}
                    onClick={() => setSisakan(v)}
                    className={cn(
                      'h-11 rounded-full px-2 text-label font-semibold transition-colors',
                      'focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-utama',
                      sisakan === v
                        ? 'bg-permukaan text-utama shadow-kartu'
                        : 'text-teks-sekunder hover:text-teks-utama',
                    )}
                  >
                    {teks}
                  </button>
                ))}
              </div>
            </div>

            {/* Hasil akhirnya dikatakan SEBELUM tombol ditekan: serah terima
                memindahkan tanggung jawab uang ke orang lain. Saat setor
                sebagian, kolom yang ditinggal INILAH modal berikutnya — tidak
                diulang di kotak terpisah. */}
            {sisakan === 'sebagian' ? (
              <KolomUang
                label={`Ditinggal untuk ${kasirBaru}`}
                nilai={ditinggal}
                onNilai={setDitinggal}
                galat={
                  modalKebanyakan
                    ? `Tidak bisa melebihi uang di laci (${formatRupiah(dihitung)}).`
                    : undefined
                }
                bantuan={
                  modalKebanyakan
                    ? undefined
                    : sudahIsi
                      ? `Jadi modal awalnya. Disetor: ${formatRupiah(Math.max(0, disetor))}`
                      : 'Jadi modal awalnya; sisanya disetor.'
                }
                sisipanAkhir={
                  modalAwal > 0 && ditinggal !== modalAwal ? (
                    <button
                      type="button"
                      onClick={() => setDitinggal(modalAwal)}
                      aria-label={`Sama seperti modal awal tadi, ${formatRupiah(modalAwal)}`}
                      className="group -mr-1.5 flex h-11 shrink-0 items-center"
                    >
                      <span className="flex h-9 items-center gap-1 rounded-full bg-permukaan-2 px-3 text-keterangan font-semibold text-teks-sekunder group-hover:text-teks-utama">
                        <RotateCcw className="h-3.5 w-3.5" aria-hidden />
                        Seperti tadi
                      </span>
                    </button>
                  ) : undefined
                }
              />
            ) : (
              <div className="flex items-center justify-between gap-3 rounded-kontrol bg-sorot px-3 py-2.5">
                <p className="min-w-0 truncate text-label font-semibold text-hijau-800">
                  Modal awal {kasirBaru}
                </p>
                <p className="shrink-0 text-judul-kartu font-bold tabular-nums text-hijau-800">
                  {sudahIsi ? formatRupiah(modalBerikutnya) : '—'}
                </p>
              </div>
            )}

            {bukaCatatan && (
              <Kolom
                label="Catatan"
                value={catatan}
                onChange={(e) => setCatatan(e.target.value)}
                maxLength={255}
                autoFocus
                placeholder={
                  sudahIsi && selisih !== 0
                    ? 'mis. kurang karena salah kembalian'
                    : 'mis. disetor ke Bu Sari'
                }
              />
            )}

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
              ukuran="kasir"
              lebarPenuh
              disabled={!sudahIsi || modalKebanyakan}
              memuat={serah.isPending}
              labelMemuat="Menyerahkan…"
            >
              {orangSama ? 'Serahkan & Mulai Shift Baru' : `Serahkan ke ${kasirBaru}`}
            </Tombol>
          </Kartu>
        </form>
      </div>

      {dialog === 'pecahan' && (
        <DialogPecahan
          awal={pecahan}
          onTutup={() => setDialog(null)}
          onPakai={(total, h) => {
            setPecahan(h)
            isiHitung(total)
            setDialog(null)
          }}
        />
      )}
      {dialog === 'rincian' && (
        <DialogRincianShift shift={shift} kasirLama={kasirLama} onTutup={() => setDialog(null)} />
      )}
      {dialog === 'akun' && <DialogGantiAkun kasir={kasirBaru} onTutup={() => setDialog(null)} />}
    </div>
  )
}

// ── Siapa menyerahkan ke siapa ──────────────────────────────────────────────

function PitaSerahTerima({
  shift,
  kasirLama,
  kasirBaru,
  orangSama,
  onRincian,
  onGantiAkun,
}: {
  shift: Shift
  kasirLama: string
  kasirBaru: string
  orangSama: boolean
  onRincian: () => void
  onGantiAkun: () => void
}) {
  const p = shift.sales
  return (
    <Kartu className="flex flex-col gap-3 p-4">
      <div className="flex items-center gap-3">
        {orangSama ? (
          <Avatar nama={kasirLama} className="shrink-0" />
        ) : (
          <span className="flex shrink-0 -space-x-2" aria-hidden>
            <Avatar nama={kasirLama} className="ring-2 ring-permukaan" />
            <Avatar nama={kasirBaru} className="ring-2 ring-permukaan" />
          </span>
        )}
        <div className="min-w-0 flex-1">
          {/* Patah baris di antara nama, bukan di tengah nama. */}
          <p className="font-semibold text-teks-utama">
            <span className="whitespace-nowrap">{kasirLama}</span>
            <ArrowRight
              className="mx-1.5 inline h-4 w-4 align-[-2px] text-utama"
              aria-label="menyerahkan ke"
            />
            <span className="whitespace-nowrap">{kasirBaru}</span>
          </p>
          <p className="text-keterangan text-teks-sekunder">
            Sejak {waktuMulai(shift)} · {lamaSejak(shift.opened_at)}
          </p>
        </div>
        <TombolRincian onKlik={onRincian} />
      </div>
      {p && (
        <p className="text-label text-teks-sekunder">
          <strong className="font-semibold text-teks-utama">{p.sales_count} transaksi</strong> ·
          penjualan{' '}
          <strong className="font-semibold tabular-nums text-teks-utama">
            {formatRupiah(p.sales_total)}
          </strong>
        </p>
      )}
      {orangSama && <SaranGantiAkun kasir={kasirBaru} onGanti={onGantiAkun} />}
    </Kartu>
  )
}

function TombolRincian({ onKlik, className }: { onKlik: () => void; className?: string }) {
  return (
    <button
      type="button"
      onClick={onKlik}
      className={cn(
        '-mr-2 flex min-h-11 shrink-0 items-center gap-0.5 rounded-kontrol px-2 text-label font-medium text-utama hover:bg-sorot',
        className,
      )}
    >
      Rincian shift
      <ChevronRight className="h-4 w-4" aria-hidden />
    </button>
  )
}

/**
 * Akun yang masuk = kasir yang menyerahkan. Bisa memang disengaja (orang yang
 * sama lanjut, atau toko memakai satu akun bersama), jadi hanya disarankan —
 * tidak dipaksa.
 */
function SaranGantiAkun({ kasir, onGanti }: { kasir: string; onGanti: () => void }) {
  return (
    <p className="flex flex-wrap items-center gap-x-1.5 text-keterangan text-teks-sekunder">
      <UserRoundCog className="h-4 w-4 shrink-0 text-jingga-700" aria-hidden />
      <span>Shift baru tetap atas nama {kasir}.</span>
      <button
        type="button"
        onClick={onGanti}
        // Area ketuk 44px; margin negatif menjaga tinggi barisnya tetap satu baris teks.
        className="-my-3 min-h-11 font-semibold text-utama hover:underline"
      >
        Masuk sebagai kasir lain
      </button>
    </p>
  )
}

function Avatar({ nama, className }: { nama: string; className?: string }) {
  return (
    <span
      aria-hidden
      className={cn(
        'flex h-11 w-11 items-center justify-center rounded-full bg-sorot text-label font-bold text-hijau-800',
        className,
      )}
    >
      {inisialNama(nama)}
    </span>
  )
}

/** Keluar dari akun, lalu kembali ke layar ini setelah kasir berikutnya masuk. */
function DialogGantiAkun({ kasir, onTutup }: { kasir: string; onTutup: () => void }) {
  const { keluar } = useSesi()
  const navigate = useNavigate()
  const [sedang, setSedang] = useState(false)
  const [galat, setGalat] = useState<string | null>(null)

  const keMasuk = () =>
    navigate('/masuk', { replace: true, state: { dari: '/kasir/ganti-shift' } })

  async function gantiAkun() {
    setSedang(true)
    setGalat(null)
    try {
      await keluar()
      keMasuk()
    } catch (e) {
      if (e instanceof GalatAdaAntrean) {
        setGalat(
          `Masih ada ${e.jumlah} transaksi yang belum terkirim. Tunggu sampai tersambung ke internet, lalu coba lagi.`,
        )
      } else {
        keMasuk()
      }
    } finally {
      setSedang(false)
    }
  }

  return (
    <Dialog open onOpenChange={(o) => !o && !sedang && onTutup()}>
      <IsiDialog judul={`Keluar dari akun ${kasir}?`}>
        <ol className="flex list-decimal flex-col gap-1.5 pl-5 text-isi text-teks-sekunder">
          <li>Anda keluar dari akun ini.</li>
          <li>Kasir berikutnya masuk dengan akunnya sendiri.</li>
          <li>Layar Ganti Shift terbuka lagi — hitung laci bersama di sana.</li>
        </ol>
        <p className="text-label text-teks-redup">
          Shift yang berjalan tidak berubah sampai serah terima selesai.
        </p>
        {galat && (
          <p role="alert" className="text-label text-bahaya-teks">
            {galat}
          </p>
        )}
        <AksiDialog>
          <Tombol onClick={() => void gantiAkun()} memuat={sedang} labelMemuat="Keluar…">
            Keluar &amp; Ganti Akun
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={sedang}>
            Batal
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}

// ── Rincian (dibuka bila diminta) ───────────────────────────────────────────

function DialogRincianShift({
  shift,
  kasirLama,
  onTutup,
}: {
  shift: Shift
  kasirLama: string
  onTutup: () => void
}) {
  const p = shift.sales
  return (
    <Dialog open onOpenChange={(o) => !o && onTutup()}>
      <IsiDialog
        judul={`Shift ${kasirLama}`}
        keterangan={`Sejak ${formatTanggalJam(shift.opened_at)} · ${lamaSejak(shift.opened_at)}`}
      >
        {p && (
          <section className="flex flex-col gap-3">
            <dl className="grid grid-cols-2 gap-2">
              <div className="rounded-kontrol bg-permukaan-2 p-3">
                <dt className="text-keterangan text-teks-redup">Penjualan</dt>
                <dd className="text-judul-kartu font-bold tabular-nums text-teks-utama">
                  {formatRupiah(p.sales_total)}
                </dd>
              </div>
              <div className="rounded-kontrol bg-permukaan-2 p-3">
                <dt className="text-keterangan text-teks-redup">Transaksi</dt>
                <dd className="text-judul-kartu font-bold tabular-nums text-teks-utama">
                  {p.sales_count}
                </dd>
                {p.sales_count > 0 && (
                  <dd className="text-keterangan tabular-nums text-teks-redup">
                    rata-rata {formatRupiah(p.average_sale)}
                  </dd>
                )}
              </div>
            </dl>
            {p.sales_count > 0 ? (
              <DaftarCaraBayar data={p.by_method} />
            ) : (
              <p className="text-label text-teks-redup">Belum ada penjualan di shift ini.</p>
            )}
            {(p.returns_count > 0 || p.canceled_count > 0) && (
              <p className="flex flex-wrap gap-x-4 gap-y-1 text-label text-teks-sekunder">
                {p.returns_count > 0 && (
                  <span className="inline-flex items-center gap-1.5">
                    <RotateCcw className="h-4 w-4" aria-hidden />
                    Retur {p.returns_count}× · −{formatRupiah(p.returns_total)}
                  </span>
                )}
                {p.canceled_count > 0 && (
                  <span className="inline-flex items-center gap-1.5">
                    <X className="h-4 w-4" aria-hidden />
                    Dibatalkan {p.canceled_count}× · {formatRupiah(p.canceled_total)}
                  </span>
                )}
              </p>
            )}
          </section>
        )}

        <section className="flex flex-col gap-2 border-t border-garis pt-3">
          <h3 className="font-semibold text-teks-utama">Uang yang seharusnya ada di laci</h3>
          <RumusLaci shift={shift} />
          {p?.by_method.some((m) => m.method !== 'cash') && (
            <p className="text-keterangan text-teks-redup">
              Hanya uang tunai yang masuk laci; QRIS, transfer, dan kasbon tidak ikut.
            </p>
          )}
        </section>

        <AksiDialog>
          <Tombol jenis="kedua" onClick={onTutup}>
            Tutup
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}

/**
 * Rumus dibuka apa adanya — kasir paham dari mana angkanya, bukan disuruh
 * percaya. Alasan uang masuk/keluar ditulis di bawah angkanya: "keluar
 * Rp 75.000" saja memancing pertanyaan yang tidak bisa dijawab kasir berikutnya.
 */
function RumusLaci({ shift }: { shift: Shift }) {
  const { boleh } = useSesi()
  const { modalAwal, penjualanTunai, kasMasuk, kasKeluar, seharusnya } = rincianShift(shift)
  const adaGerakan = kasMasuk > 0 || kasKeluar > 0
  const { daftar } = useGerakanKas(adaGerakan && boleh(IZIN.cashMovement) ? shift.id : undefined)
  const alasan = (arah: 'in' | 'out') =>
    (daftar.data?.data ?? [])
      .filter((g) => g.direction === arah)
      .map((g) => g.reason)
      .join(', ')

  return (
    <dl className="flex flex-col gap-2 text-label">
      <BarisRumus label="Modal awal" nilai={formatRupiah(modalAwal)} />
      <BarisRumus label="Penjualan tunai" nilai={formatRupiah(penjualanTunai)} />
      {kasMasuk > 0 && (
        <BarisRumus label="Uang masuk lain" nilai={formatRupiah(kasMasuk)} alasan={alasan('in')} />
      )}
      {kasKeluar > 0 && (
        <BarisRumus
          label="Uang keluar"
          nilai={`−${formatRupiah(kasKeluar)}`}
          alasan={alasan('out')}
        />
      )}
      <div className="mt-1 flex items-baseline justify-between gap-3 border-t border-garis pt-3">
        <dt className="font-semibold text-teks-utama">Seharusnya</dt>
        <dd className="text-judul font-bold tabular-nums text-teks-utama">
          {formatRupiah(seharusnya)}
        </dd>
      </div>
    </dl>
  )
}

function BarisRumus({ label, nilai, alasan }: { label: string; nilai: string; alasan?: string }) {
  return (
    <div>
      <div className="flex items-baseline justify-between gap-3">
        <dt className="text-teks-sekunder">{label}</dt>
        <dd className="shrink-0 tabular-nums text-teks-utama">{nilai}</dd>
      </div>
      {alasan && <p className="line-clamp-2 text-keterangan text-teks-redup">{alasan}</p>}
    </div>
  )
}

// ── Potongan formulir ───────────────────────────────────────────────────────

/**
 * Pas → hijau. Selisih → jingga yang menenangkan, bukan merah yang menuduh.
 * Satu baris di bawah kolom (menggantikan kalimat bantuannya), bukan kotak
 * tiga baris.
 */
function HasilHitung({ selisih }: { selisih: number }) {
  return (
    <p aria-live="polite" className="-mt-1.5 flex items-start gap-1.5 text-keterangan">
      {selisih === 0 ? (
        <>
          <Check className="h-4 w-4 shrink-0 text-hijau-800" aria-hidden />
          <span className="font-semibold text-hijau-800">Pas — sama dengan yang seharusnya.</span>
        </>
      ) : (
        <>
          <TriangleAlert className="h-4 w-4 shrink-0 text-jingga-700" aria-hidden />
          <span className="text-teks-sekunder">
            <strong className="font-semibold text-jingga-700">
              {selisih > 0 ? 'Lebih' : 'Kurang'} {formatRupiah(Math.abs(selisih))}
            </strong>{' '}
            · selisih kecil itu wajar.
          </span>
        </>
      )}
    </p>
  )
}

function BarisRingkas({
  label,
  nilai,
  nada,
}: {
  label: string
  nilai: string
  nada?: 'pas' | 'selisih'
}) {
  return (
    <div className="flex items-baseline justify-between gap-3">
      <dt className="text-teks-sekunder">{label}</dt>
      <dd
        className={cn(
          'shrink-0 tabular-nums',
          nada === 'pas'
            ? 'font-semibold text-hijau-800'
            : nada === 'selisih'
              ? 'font-semibold text-jingga-700'
              : 'text-teks-utama',
        )}
      >
        {nilai}
      </dd>
    </div>
  )
}

// ── Selesai ─────────────────────────────────────────────────────────────────

/**
 * Hasil serah terima ditampilkan sebagai layar, bukan toast dua detik: dua
 * orang yang baru bertukar uang perlu melihat angka yang tercatat, bukan
 * sekadar "berhasil".
 */
function HasilSerahTerima({
  ditutup,
  dibuka,
  kasirLama,
  kasirBaru,
}: {
  ditutup: Shift
  dibuka: Shift
  kasirLama: string
  kasirBaru: string
}) {
  const navigate = useNavigate()
  const selisih = ditutup.difference ?? 0
  const dihitung = ditutup.counted_cash ?? 0
  const disetor = dihitung - dibuka.opening_cash
  return (
    <div className="mx-auto flex w-full max-w-lg flex-col gap-4">
      <Kartu className="flex flex-col items-center gap-2 p-6 text-center">
        <span className="flex h-14 w-14 items-center justify-center rounded-full bg-sorot text-hijau-800">
          <CircleCheck className="h-8 w-8" aria-hidden />
        </span>
        <h1 className="text-judul font-bold text-teks-utama">Serah terima selesai</h1>
        <p className="text-label text-teks-sekunder">
          Shift <strong className="text-teks-utama">{kasirLama}</strong> ditutup pukul{' '}
          {formatJam(ditutup.closed_at ?? dibuka.opened_at)}. Shift baru atas nama{' '}
          <strong className="text-teks-utama">{kasirBaru}</strong> sudah berjalan.
        </p>
      </Kartu>

      <Kartu className="p-4 sm:p-5">
        <dl className="flex flex-col gap-2 text-label">
          <BarisRingkas label="Uang di laci (dihitung)" nilai={formatRupiah(dihitung)} />
          <BarisRingkas
            label="Selisih"
            nilai={
              selisih === 0
                ? 'Pas'
                : `${selisih > 0 ? 'Lebih' : 'Kurang'} ${formatRupiah(Math.abs(selisih))}`
            }
            nada={selisih === 0 ? 'pas' : 'selisih'}
          />
          {disetor > 0 && <BarisRingkas label="Disetor" nilai={formatRupiah(disetor)} />}
          <div className="mt-1 flex items-baseline justify-between gap-3 rounded-kontrol bg-sorot px-3 py-3">
            <dt className="font-semibold text-hijau-800">Modal awal shift baru</dt>
            <dd className="text-judul-kartu font-bold tabular-nums text-hijau-800">
              {formatRupiah(dibuka.opening_cash)}
            </dd>
          </div>
        </dl>
      </Kartu>

      <div className="flex flex-col gap-2 sm:flex-row-reverse">
        <Tombol ukuran="kasir" lebarPenuh onClick={() => navigate('/kasir', { replace: true })}>
          Mulai Jualan
        </Tombol>
        <Tombol jenis="kedua" ukuran="kasir" lebarPenuh onClick={() => navigate('/kasir/riwayat')}>
          Lihat Riwayat
        </Tombol>
      </div>
    </div>
  )
}

/** "5 jam 12 menit", "2 hari 3 jam" — sudah berapa lama shift berjalan. */
function lamaSejak(iso: string, kini = Date.now()): string {
  const menit = Math.max(0, Math.floor((kini - keDate(iso).getTime()) / 60_000))
  const jam = Math.floor(menit / 60)
  if (jam >= 24) return `${Math.floor(jam / 24)} hari ${jam % 24} jam`
  if (jam > 0) return `${jam} jam ${menit % 60} menit`
  return `${menit} menit`
}

const layarLebar = () =>
  typeof window !== 'undefined' && !!window.matchMedia?.('(min-width: 1024px)').matches

/** "09.22" bila hari ini, "Kemarin 21.10" / "18 Sep 2026 09.22" bila lebih lama. */
function waktuMulai(shift: Shift): string {
  if (shift.business_date === tanggalISO()) return formatJam(shift.opened_at)
  return `${formatTanggalAkrab(shift.opened_at)} ${formatJam(shift.opened_at)}`
}
