import { useState } from 'react'
import { Wallet } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { useBukaShift } from '../hooks'

/**
 * Buka shift.
 *
 * Satu layar, satu pekerjaan: mencatat berapa uang yang ada di laci saat mulai.
 * Tanpa angka ini, tutup shift nanti tidak punya pembanding dan selisihnya
 * tidak berarti apa-apa.
 */
export function HalamanBukaShift() {
  const { tokoAktif, boleh } = useSesi()
  const bukaShift = useBukaShift()
  const [modalAwal, setModalAwal] = useState(0)
  const [galat, setGalat] = useState<string | null>(null)

  if (!boleh(IZIN.shiftOpen)) {
    return (
      <div className="mx-auto max-w-md p-6 text-center">
        <h1 className="text-judul font-bold text-teks-utama">Kasir belum dibuka</h1>
        <p className="mt-2 text-isi text-teks-sekunder">
          Minta pemilik atau kepala toko untuk membuka kasir lebih dulu.
        </p>
      </div>
    )
  }

  async function kirim(e: React.FormEvent) {
    e.preventDefault()
    if (!tokoAktif) return
    setGalat(null)
    try {
      await bukaShift.mutateAsync({ outletId: tokoAktif, modalAwal })
    } catch (e) {
      setGalat(
        e instanceof GalatAPI
          ? // 409 = sudah ada shift terbuka. Ini bukan kesalahan pengguna:
            // biasanya karena tab lain sudah membukanya.
            e.status === 409
            ? 'Kasir sudah dibuka di perangkat lain. Muat ulang halaman ini.'
            : e.pesan
          : 'Terjadi kesalahan. Coba lagi.',
      )
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-md flex-col gap-4 p-4">
      <div className="text-center">
        <Wallet className="mx-auto h-10 w-10 text-utama" aria-hidden />
        <h1 className="mt-2 text-judul font-bold text-teks-utama">Buka Kasir</h1>
        <p className="mt-1 text-isi text-teks-sekunder">
          Hitung uang di laci sekarang, lalu isi jumlahnya.
        </p>
      </div>

      <Kartu className="p-4">
        <form onSubmit={kirim} className="flex flex-col gap-4">
          <KolomUang
            label="Modal awal di laci"
            nilai={modalAwal}
            onNilai={setModalAwal}
            bantuan="Uang kembalian yang sudah ada sebelum mulai jualan. Boleh Rp 0."
            autoFocus
          />

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-red-50 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <Tombol
            type="submit"
            ukuran="kasir"
            lebarPenuh
            memuat={bukaShift.isPending}
            labelMemuat="Membuka kasir…"
          >
            Mulai Jualan
          </Tombol>
        </form>
      </Kartu>
    </div>
  )
}
