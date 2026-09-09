import type { LucideIcon } from 'lucide-react'
import { Tombol } from '@/bersama/ui/tombol'

/**
 * Tidak ada jalan buntu (ui/01-PRINSIP-DESAIN.md §7).
 *
 * Setiap layar kosong WAJIB punya jalan keluar: satu tombol aksi utama, dan
 * kalimat yang menjelaskan apa yang harus dilakukan — bukan sekadar
 * "Tidak ada data".
 */
export function KeadaanKosong({
  ikon: Ikon,
  judul,
  penjelasan,
  aksi,
  aksiKedua,
}: {
  ikon: LucideIcon
  judul: string
  penjelasan: string
  aksi?: { label: string; onKlik: () => void }
  aksiKedua?: { label: string; onKlik: () => void }
}) {
  return (
    <div className="flex flex-col items-center gap-3 px-6 py-12 text-center">
      <Ikon className="h-12 w-12 text-teks-redup" aria-hidden strokeWidth={1.5} />
      <h3 className="text-judul-kartu font-semibold text-teks-utama">{judul}</h3>
      <p className="max-w-sm text-isi text-teks-sekunder">{penjelasan}</p>
      {aksi && (
        <div className="mt-2 flex flex-col items-center gap-2">
          <Tombol onClick={aksi.onKlik}>{aksi.label}</Tombol>
          {aksiKedua && (
            <Tombol jenis="teks" onClick={aksiKedua.onKlik}>
              {aksiKedua.label}
            </Tombol>
          )}
        </div>
      )}
    </div>
  )
}

/**
 * Kegagalan memuat. Sama seperti keadaan kosong: sebutkan apa yang bisa
 * dilakukan, bukan kode error.
 */
export function KeadaanGagal({
  pesan,
  onCobaLagi,
}: {
  pesan: string
  onCobaLagi?: () => void
}) {
  return (
    <div className="flex flex-col items-center gap-3 px-6 py-12 text-center">
      <p className="max-w-sm text-isi text-teks-sekunder">{pesan}</p>
      {onCobaLagi && (
        <Tombol jenis="kedua" onClick={onCobaLagi}>
          Coba lagi
        </Tombol>
      )}
    </div>
  )
}
