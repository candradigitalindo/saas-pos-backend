import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Check, TriangleAlert } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { Kolom } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatJam } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { rincianShift, useShiftAktif, useTutupShift } from '../hooks'

/**
 * Tutup shift — momen paling rawan salah.
 *
 * NADA BAHASANYA SENGAJA MENENANGKAN. Selisih kecil itu wajar; kalau UI-nya
 * menuduh (merah, "SELISIH!"), kasir akan berhenti jujur dan mulai memaksakan
 * angka agar pas. Itu jauh lebih merugikan daripada selisih Rp 5.000
 * (ui/05-ALUR-UTAMA.md §3).
 */
export function HalamanTutupShift() {
  const navigate = useNavigate()
  const toast = useToast()
  const { shift, memuat } = useShiftAktif()
  const tutup = useTutupShift()

  const [dihitung, setDihitung] = useState(0)
  const [catatan, setCatatan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)
  const [sudahIsi, setSudahIsi] = useState(false)

  if (memuat) return <KerangkaBaris jumlah={4} />

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

  const { modalAwal, penjualanTunai, kasMasuk, kasKeluar, seharusnya } = rincianShift(shift)
  const selisih = dihitung - seharusnya
  const pas = selisih === 0

  async function kirim(e: React.FormEvent) {
    e.preventDefault()
    if (!shift) return
    setGalat(null)
    try {
      await tutup.mutateAsync({
        id: shift.id,
        uangDihitung: dihitung,
        catatan: catatan.trim() || undefined,
      })
      toast.berhasil('Shift ditutup. Terima kasih sudah jualan hari ini.')
      navigate('/', { replace: true })
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
    <div className="mx-auto flex w-full max-w-lg flex-col gap-4 p-4">
      <header>
        <h1 className="text-judul font-bold text-teks-utama">Tutup Shift</h1>
        <p className="text-label text-teks-sekunder">
          Dibuka {formatJam(shift.opened_at)} · {shift.business_date}
        </p>
      </header>

      {/* Rumusnya ditulis terbuka supaya kasir paham dari mana angkanya —
          bukan disuruh percaya pada satu angka akhir. */}
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
              pas ? 'border-hijau-600 bg-hijau-50' : 'border-jingga-600 bg-jingga-100',
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

            {!pas && (
              <>
                <p className="mt-1 text-label text-jingga-700">
                  Tidak apa-apa — selisih kecil biasa terjadi. Beri catatan bila Anda
                  tahu sebabnya.
                </p>
                <div className="mt-3">
                  <Kolom
                    label="Catatan"
                    placeholder="Boleh dikosongkan"
                    value={catatan}
                    onChange={(e) => setCatatan(e.target.value)}
                    maxLength={255}
                  />
                </div>
              </>
            )}
          </Kartu>
        )}

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-red-50 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <Tombol
          type="submit"
          ukuran="kasir"
          lebarPenuh
          memuat={tutup.isPending}
          labelMemuat="Menutup shift…"
          disabled={!sudahIsi}
        >
          Tutup Shift
        </Tombol>
      </form>
    </div>
  )
}

function Baris({ label, nilai }: { label: string; nilai: number }) {
  return (
    <div className="flex items-baseline justify-between">
      <dt className="text-teks-sekunder">{label}</dt>
      <dd className="tabular-nums text-teks-utama">{formatRupiah(nilai)}</dd>
    </div>
  )
}
