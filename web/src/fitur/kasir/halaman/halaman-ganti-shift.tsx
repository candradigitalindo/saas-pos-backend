import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ArrowRightLeft, Check, TriangleAlert } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { Kolom } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { cn } from '@/bersama/util/cn'
import { rincianShift, useSerahTerimaShift, useShiftAktif } from '../hooks'

/**
 * Ganti shift — kasir berganti orang tanpa menutup kasirnya.
 *
 * Sebelum layar ini ada, pergantian dilakukan dengan Tutup Shift lalu Buka
 * Shift lagi. Tiga langkah, dan yang paling merugikan: uang laci yang baru
 * saja dihitung TIDAK terbawa, jadi kasir berikutnya mengetik ulang modal
 * awalnya. Angka yang diketik ulang adalah angka yang bisa salah ketik, dan
 * salah ketiknya baru ketahuan saat tutup buku malam hari.
 *
 * NADA BAHASANYA SAMA dengan Tutup Shift: selisih kecil itu wajar, dan UI yang
 * menuduh membuat kasir berhenti jujur (ui/05-ALUR-UTAMA.md §3).
 */
export function HalamanGantiShift() {
  const navigate = useNavigate()
  const toast = useToast()
  const { shift, memuat } = useShiftAktif()
  const serah = useSerahTerimaShift()

  const [dihitung, setDihitung] = useState(0)
  const [sudahIsi, setSudahIsi] = useState(false)
  const [setorSebagian, setSetorSebagian] = useState(false)
  const [ditinggal, setDitinggal] = useState(0)
  const [catatan, setCatatan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)

  if (memuat) return <KerangkaBaris jumlah={4} />

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

  const { modalAwal, penjualanTunai, kasMasuk, kasKeluar, seharusnya } = rincianShift(shift)
  const selisih = dihitung - seharusnya
  const pas = selisih === 0
  const modalBerikutnya = setorSebagian ? ditinggal : dihitung
  const disetor = dihitung - modalBerikutnya
  const modalKebanyakan = setorSebagian && ditinggal > dihitung

  async function kirim(e: React.FormEvent) {
    e.preventDefault()
    if (!shift) return
    setGalat(null)
    try {
      const hasil = await serah.mutateAsync({
        id: shift.id,
        uangDihitung: dihitung,
        modalDitinggal: setorSebagian ? ditinggal : undefined,
        catatan: catatan.trim() || undefined,
      })
      toast.berhasil(
        `Shift diserahkan. Modal awal shift baru ${formatRupiah(hasil.dibuka.opening_cash)}.`,
      )
      navigate('/kasir', { replace: true })
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
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-4">
      <header>
        <h1 className="text-judul font-bold text-teks-utama">Ganti Shift</h1>
        <p className="text-label text-teks-sekunder">
          Hitung uang laci bersama-sama, lalu kasir berikutnya melanjutkan.
        </p>
      </header>

      {/* Rumus dibuka apa adanya — sama seperti layar Tutup Shift. */}
      <Kartu className="p-4">
        <h2 className="mb-3 text-judul-kartu font-semibold text-teks-utama">
          Uang yang seharusnya ada di laci
        </h2>
        <dl className="flex flex-col gap-1.5 text-label">
          <Baris label="Modal awal" nilai={modalAwal} />
          <Baris label="Penjualan tunai" nilai={penjualanTunai} />
          {kasMasuk > 0 && <Baris label="Uang masuk lain" nilai={kasMasuk} />}
          {kasKeluar > 0 && <Baris label="Uang keluar" nilai={-kasKeluar} />}
          <div className="mt-1 flex items-baseline justify-between border-t border-garis pt-2">
            <dt className="font-semibold text-teks-utama">Seharusnya</dt>
            <dd className="text-judul-kartu font-bold tabular-nums text-teks-utama">
              {formatRupiah(seharusnya)}
            </dd>
          </div>
        </dl>
      </Kartu>

      <form onSubmit={kirim} className="flex flex-col gap-4">
        <Kartu className="p-4">
          <KolomUang
            label="Hitung uang di laci sekarang, lalu isi"
            nilai={dihitung}
            onNilai={(n) => {
              setDihitung(n)
              setSudahIsi(true)
            }}
            bantuan="Isi apa adanya. Selisih kecil itu biasa dan tidak apa-apa."
            autoFocus
          />
        </Kartu>

        {sudahIsi && (
          <Kartu
            className={cn(
              'p-4',
              pas ? 'border-hijau-600 bg-hijau-700/10' : 'border-jingga-600 bg-permukaan-2',
            )}
          >
            <p
              className={cn(
                'flex items-center gap-2 font-semibold',
                pas ? 'text-hijau-800' : 'text-jingga-700',
              )}
            >
              {pas ? (
                <>
                  <Check className="h-5 w-5" aria-hidden />
                  Pas
                </>
              ) : (
                <>
                  <TriangleAlert className="h-5 w-5" aria-hidden />
                  {selisih > 0 ? 'Lebih' : 'Kurang'} {formatRupiah(Math.abs(selisih))}
                </>
              )}
            </p>
          </Kartu>
        )}

        <Kartu className="flex flex-col gap-3 p-4">
          <label className="flex min-h-12 cursor-pointer items-center gap-3">
            <input
              type="checkbox"
              checked={setorSebagian}
              onChange={(e) => setSetorSebagian(e.target.checked)}
              className="h-5 w-5 shrink-0 accent-[var(--warna-utama)]"
            />
            <span className="text-isi text-teks-utama">
              Sebagian uang disetor, tidak semua ditinggal di laci
            </span>
          </label>

          {setorSebagian && (
            <>
              <KolomUang
                label="Uang yang ditinggal di laci"
                nilai={ditinggal}
                onNilai={setDitinggal}
                bantuan="Sisanya dianggap disetor dan keluar dari laci."
              />
              {modalKebanyakan && (
                <p className="text-keterangan text-bahaya-teks">
                  Tidak bisa meninggalkan lebih banyak daripada yang ada di laci.
                </p>
              )}
            </>
          )}
        </Kartu>

        {/* Hasil akhirnya dikatakan SEBELUM tombol ditekan. Serah terima
            memindahkan tanggung jawab uang ke orang lain; tidak boleh ada
            kejutan setelahnya. */}
        {sudahIsi && !modalKebanyakan && (
          <Kartu className="p-4">
            <dl className="flex flex-col gap-1.5 text-label">
              <Baris label="Modal awal kasir berikutnya" nilai={modalBerikutnya} />
              {disetor > 0 && <Baris label="Disetor / keluar dari laci" nilai={-disetor} />}
            </dl>
          </Kartu>
        )}

        <Kolom
          label="Catatan (boleh dikosongkan)"
          value={catatan}
          onChange={(e) => setCatatan(e.target.value)}
          placeholder="mis. diserahkan ke Budi"
        />

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
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
          Serahkan ke Kasir Berikutnya
        </Tombol>
      </form>
    </div>
  )
}

function Baris({ label, nilai }: { label: string; nilai: number }) {
  return (
    <div className="flex items-baseline justify-between gap-3">
      <dt className="text-teks-sekunder">{label}</dt>
      <dd className="tabular-nums text-teks-utama">{formatRupiah(nilai)}</dd>
    </div>
  )
}
