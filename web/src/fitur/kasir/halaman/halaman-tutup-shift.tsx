import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Calculator, Check, CircleCheck, Plus } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { Kolom } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka, KerangkaKartuAngka } from '@/bersama/komponen/kerangka'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatJam } from '@/bersama/util/tanggal'
import type { Shift } from '@/bersama/tipe/pos'
import { rincianShift, useShiftAktif, useTutupShift } from '../hooks'
import { DialogPecahan, HITUNGAN_KOSONG, type HitunganPecahan } from '../komponen/penghitung-laci'
import { DaftarCaraBayar } from '../komponen/daftar-cara-bayar'
import {
  BarisRingkas,
  DialogRincianShift,
  HasilHitung,
  PeringatanBelumTerkirim,
  RumusLaci,
  TombolRincian,
  layarLebar,
  useJualanBelumTerkirim,
  waktuMulai,
} from '../komponen/bagian-shift'

/**
 * Tutup shift — momen paling rawan salah.
 *
 * NADA BAHASANYA SENGAJA MENENANGKAN. Selisih kecil itu wajar; kalau UI-nya
 * menuduh (merah, "SELISIH!"), kasir akan berhenti jujur dan mulai memaksakan
 * angka agar pas. Itu jauh lebih merugikan daripada selisih Rp 5.000
 * (ui/05-ALUR-UTAMA.md §3).
 *
 * SATU LAYAR, TANPA MENGGULIR — pola yang sama dengan Ganti Shift: yang
 * dikerjakan hanya SATU angka (hasil hitung laci), pembandingnya di baris
 * label, hitung per pecahan & rincian penjualan di dialog, catatan lewat
 * "+ Catatan". Versi sebelumnya menumpuk rumus di kartu sendiri dan kotak
 * selisih tiga baris, sehingga tombol Tutup Shift terdorong ke bawah layar HP.
 *
 * Penjualan offline yang belum terkirim MENAHAN penutupan: server menolak
 * penjualan untuk shift yang sudah ditutup.
 */
export function HalamanTutupShift() {
  const navigate = useNavigate()
  const { shift, memuat } = useShiftAktif()
  const tutup = useTutupShift()
  const belumTerkirim = useJualanBelumTerkirim()

  const [dihitung, setDihitung] = useState(0)
  const [sudahIsi, setSudahIsi] = useState(false)
  const [pecahan, setPecahan] = useState<HitunganPecahan>(HITUNGAN_KOSONG)
  const [dialog, setDialog] = useState<'pecahan' | 'rincian' | null>(null)
  const [bukaCatatan, setBukaCatatan] = useState(false)
  const [catatan, setCatatan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)
  const [hasil, setHasil] = useState<{ ditutup: Shift; sebelum: Shift } | null>(null)

  if (hasil) return <HasilTutupShift {...hasil} />

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
        ikon={Check}
        judul="Kasir sedang tidak dibuka"
        penjelasan="Tidak ada shift yang perlu ditutup sekarang."
        aksi={{ label: 'Kembali ke Beranda', onKlik: () => navigate('/') }}
      />
    )
  }

  const { seharusnya } = rincianShift(shift)
  const selisih = dihitung - seharusnya
  const kasir = shift.opened_by_name || 'Kasir'

  const isiHitung = (n: number) => {
    setDihitung(n)
    setSudahIsi(true)
  }

  async function kirim(e: React.FormEvent) {
    e.preventDefault()
    // Enter di kolom juga mengirim formulir — penjagaannya di sini, bukan
    // hanya pada tombol yang dinonaktifkan.
    if (!shift || !sudahIsi || belumTerkirim > 0) return
    setGalat(null)
    try {
      const ditutup = await tutup.mutateAsync({
        id: shift.id,
        uangDihitung: dihitung,
        catatan: catatan.trim() || undefined,
      })
      setHasil({ ditutup, sebelum: shift })
      window.scrollTo({ top: 0 })
    } catch (e) {
      setGalat(
        e instanceof GalatAPI
          ? e.status === 409
            ? 'Shift ini sudah ditutup. Muat ulang halaman.'
            : e.pesan
          : 'Terjadi kesalahan. Coba lagi.',
      )
    }
  }

  return (
    <div className="flex w-full max-w-5xl flex-col gap-3 lg:gap-4">
      {/* HP: siapa & sejak kapan jadi anak judul, bukan kartu sendiri. */}
      <header className="flex flex-col gap-1">
        <div className="flex items-center justify-between gap-3">
          <h1 className="text-judul font-bold text-teks-utama">Tutup Shift</h1>
          <TombolRincian onKlik={() => setDialog('rincian')} className="lg:hidden" />
        </div>
        <p className="text-label text-teks-sekunder">
          <strong className="font-semibold text-teks-utama">{kasir}</strong> · sejak {waktuMulai(shift)}
          {shift.sales && ` · ${shift.sales.sales_count} transaksi`}
        </p>
      </header>

      {/* Layar lebar: penjualan & rumus laci di kiri, formulirnya di kanan. */}
      <div className="flex flex-col gap-3 lg:grid lg:grid-cols-[minmax(0,5fr)_minmax(0,6fr)] lg:items-start lg:gap-6">
        <div className="hidden flex-col gap-4 lg:flex">
          {shift.sales && shift.sales.sales_count > 0 && (
            <Kartu className="flex flex-col gap-3 p-5">
              <div className="flex items-baseline justify-between gap-3">
                <h2 className="text-judul-kartu font-semibold text-teks-utama">Penjualan shift ini</h2>
                <span className="text-judul-kartu font-bold tabular-nums text-teks-utama">
                  {formatRupiah(shift.sales.sales_total)}
                </span>
              </div>
              <DaftarCaraBayar data={shift.sales.by_method} />
            </Kartu>
          )}
          <Kartu className="flex flex-col gap-3 p-5">
            <h2 className="text-judul-kartu font-semibold text-teks-utama">Uang yang seharusnya ada di laci</h2>
            <RumusLaci shift={shift} />
          </Kartu>
        </div>

        <form onSubmit={kirim}>
          <Kartu className="flex flex-col gap-2.5 p-4 sm:gap-3 sm:p-5">
            <KolomUang
              label="Hasil hitung laci"
              labelKanan={
                <span className="text-label text-teks-sekunder lg:hidden">
                  Seharusnya{' '}
                  <strong className="font-bold tabular-nums text-teks-utama">{formatRupiah(seharusnya)}</strong>
                </span>
              }
              nilai={dihitung}
              onNilai={isiHitung}
              // Hanya di layar lebar: di HP papan tik menutupi layar sebelum
              // kasir sempat membaca angka pembandingnya.
              autoFocus={layarLebar()}
              bantuan={sudahIsi ? undefined : 'Hitung semua uang tunai di laci, lalu isi apa adanya.'}
              sisipanAkhir={
                <button type="button" onClick={() => setDialog('pecahan')} className="group -mr-1.5 flex h-11 shrink-0 items-center">
                  <span className="flex h-9 items-center gap-1.5 rounded-full bg-sorot px-3 text-keterangan font-semibold text-hijau-800 group-hover:brightness-95">
                    <Calculator className="h-4 w-4" aria-hidden />
                    Per pecahan
                  </span>
                </button>
              }
            />
            {sudahIsi && <HasilHitung selisih={selisih} />}

            {bukaCatatan ? (
              <Kolom
                label="Catatan"
                value={catatan}
                onChange={(e) => setCatatan(e.target.value)}
                maxLength={255}
                autoFocus
                placeholder={sudahIsi && selisih !== 0 ? 'mis. kurang karena salah kembalian' : 'Boleh dikosongkan'}
              />
            ) : (
              <button
                type="button"
                onClick={() => setBukaCatatan(true)}
                className="-mx-1 flex min-h-11 items-center gap-1 self-start px-1 text-label font-medium text-utama hover:underline"
              >
                <Plus className="h-4 w-4" aria-hidden />
                Catatan
              </button>
            )}

            {belumTerkirim > 0 && <PeringatanBelumTerkirim jumlah={belumTerkirim} />}

            {galat && (
              <p role="alert" className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
                {galat}
              </p>
            )}

            <Tombol
              type="submit"
              ukuran="kasir"
              lebarPenuh
              memuat={tutup.isPending}
              labelMemuat="Menutup shift…"
              disabled={!sudahIsi || belumTerkirim > 0}
            >
              Tutup Shift
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
      {dialog === 'rincian' && <DialogRincianShift shift={shift} kasirLama={kasir} onTutup={() => setDialog(null)} />}
    </div>
  )
}

/**
 * Hasil tutup shift sebagai LAYAR, bukan toast dua detik: angka yang tercatat
 * (terutama selisihnya) perlu dibaca, dan pemilik sering menanyakannya besok.
 */
function HasilTutupShift({ ditutup, sebelum }: { ditutup: Shift; sebelum: Shift }) {
  const navigate = useNavigate()
  const selisih = ditutup.difference ?? 0
  const penjualan = sebelum.sales
  return (
    <div className="mx-auto flex w-full max-w-lg flex-col gap-4">
      <Kartu className="flex flex-col items-center gap-2 p-6 text-center">
        <span className="flex h-14 w-14 items-center justify-center rounded-full bg-sorot text-hijau-800">
          <CircleCheck className="h-8 w-8" aria-hidden />
        </span>
        <h1 className="text-judul font-bold text-teks-utama">Shift ditutup</h1>
        <p className="text-label text-teks-sekunder">
          Ditutup pukul {formatJam(ditutup.closed_at ?? new Date().toISOString())}. Terima kasih sudah jualan hari ini.
        </p>
      </Kartu>

      <Kartu className="p-4 sm:p-5">
        <dl className="flex flex-col gap-2 text-label">
          {penjualan && (
            <BarisRingkas
              label={`Penjualan · ${penjualan.sales_count} transaksi`}
              nilai={formatRupiah(penjualan.sales_total)}
            />
          )}
          <BarisRingkas label="Uang di laci (dihitung)" nilai={formatRupiah(ditutup.counted_cash ?? 0)} />
          <BarisRingkas label="Seharusnya" nilai={formatRupiah(ditutup.expected_cash ?? 0)} />
          <BarisRingkas
            label="Selisih"
            nilai={selisih === 0 ? 'Pas' : `${selisih > 0 ? 'Lebih' : 'Kurang'} ${formatRupiah(Math.abs(selisih))}`}
            nada={selisih === 0 ? 'pas' : 'selisih'}
          />
        </dl>
      </Kartu>

      <div className="flex flex-col gap-2 sm:flex-row-reverse">
        <Tombol ukuran="kasir" lebarPenuh onClick={() => navigate('/', { replace: true })}>
          Ke Beranda
        </Tombol>
        <Tombol jenis="kedua" ukuran="kasir" lebarPenuh onClick={() => navigate('/kasir/riwayat')}>
          Lihat Riwayat
        </Tombol>
      </div>
    </div>
  )
}
