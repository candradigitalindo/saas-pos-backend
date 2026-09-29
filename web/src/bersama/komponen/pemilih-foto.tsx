import { useRef, useState } from 'react'
import { Camera, Trash2 } from 'lucide-react'
import { FotoBarang } from '@/bersama/komponen/foto-barang'
import { Tombol } from '@/bersama/ui/tombol'
import { cn } from '@/bersama/util/cn'

/** Sisi terpanjang foto setelah dikecilkan. */
const SISI_MAKS = 800
/** Mutu JPEG hasil pengecilan. */
const MUTU = 0.8

/**
 * Mengecilkan foto DI PERANGKAT sebelum dikirim.
 *
 * Foto kamera HP hari ini 3–8 MB. Warung yang mendaftarkan seratus barang
 * lewat paket data akan membayar ratusan megabita untuk gambar yang di layar
 * kasir tidak pernah lebih besar dari beberapa ratus piksel. Dikecilkan lebih
 * dulu, satu foto turun ke puluhan kilobita — dan unggahannya selesai walau
 * sinyalnya satu bar.
 *
 * Hasilnya selalu JPEG: transparansi tidak berguna untuk foto barang, dan PNG
 * dari kamera berukuran berkali lipat untuk gambar yang sama.
 */
export async function kecilkanFoto(berkas: File): Promise<Blob> {
  const bitmap = await createImageBitmap(berkas)
  const skala = Math.min(1, SISI_MAKS / Math.max(bitmap.width, bitmap.height))
  const l = Math.round(bitmap.width * skala)
  const t = Math.round(bitmap.height * skala)

  const kanvas = document.createElement('canvas')
  kanvas.width = l
  kanvas.height = t
  const ctx = kanvas.getContext('2d')
  if (!ctx) return berkas
  ctx.drawImage(bitmap, 0, 0, l, t)
  bitmap.close()

  const blob = await new Promise<Blob | null>((selesai) =>
    kanvas.toBlob(selesai, 'image/jpeg', MUTU),
  )
  // Gagal mengecilkan bukan alasan membatalkan unggahan — kirim aslinya.
  return blob ?? berkas
}

/**
 * Pemilih foto barang: pratinjau, ambil dari kamera/galeri, hapus.
 *
 * Tidak ada tombol "unggah" terpisah. Memilih foto SUDAH berarti memakainya;
 * langkah konfirmasi tambahan hanya menambah satu ketukan yang selalu dijawab
 * sama (ui/01 §6).
 */
export function PemilihFoto({
  nama,
  url,
  onPilih,
  onHapus,
  sedangUnggah,
  className,
}: {
  nama: string
  url?: string | null
  onPilih: (berkas: Blob) => void
  onHapus: () => void
  sedangUnggah?: boolean
  className?: string
}) {
  const input = useRef<HTMLInputElement>(null)
  const [galat, setGalat] = useState<string | null>(null)

  return (
    <div className={cn('flex flex-col gap-2', className)}>
      <span className="text-label font-medium text-teks-sekunder">Foto barang</span>

      <div className="flex items-center gap-3">
        <FotoBarang nama={nama || '?'} url={url} className="w-28 shrink-0" />

        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <input
            ref={input}
            type="file"
            accept="image/jpeg,image/png,image/webp"
            aria-label="Pilih foto barang"
            // Dikeluarkan dari urutan tab: yang dioperasikan pengguna adalah
            // tombol di sebelahnya, dan `sr-only` TIDAK mengeluarkan elemen
            // dari fokus keyboard — tanpa ini, pengguna keyboard mendarat di
            // kendali tak terlihat yang tak punya nama.
            tabIndex={-1}
            className="sr-only"
            onChange={async (e) => {
              const berkas = e.target.files?.[0]
              // Input direset supaya memilih BERKAS YANG SAMA dua kali tetap
              // memicu onChange — jalur yang wajar setelah unggahan gagal.
              e.target.value = ''
              if (!berkas) return
              setGalat(null)

              // Dua kegagalan yang BERBEDA, dan sempat saya satukan dalam satu
              // catch: gambar yang tidak bisa dibaca perangkat, dan unggahan
              // yang gagal sampai ke server. Digabung, unggahan yang gagal
              // dilaporkan sebagai "foto tidak bisa dibaca" — pengguna disuruh
              // mengganti berkas padahal berkasnya tidak salah apa-apa.
              let siap: Blob
              try {
                siap = await kecilkanFoto(berkas)
              } catch {
                setGalat('Foto tidak bisa dibaca. Coba pilih berkas lain.')
                return
              }
              onPilih(siap)
            }}
          />
          <Tombol
            jenis="kedua"
            ukuran="padat"
            onClick={() => input.current?.click()}
            memuat={sedangUnggah}
            labelMemuat="Mengunggah…"
          >
            <Camera className="h-5 w-5" aria-hidden />
            {url ? 'Ganti foto' : 'Ambil foto'}
          </Tombol>

          {url && (
            <Tombol jenis="teks" ukuran="padat" onClick={onHapus} disabled={sedangUnggah}>
              <Trash2 className="h-4 w-4" aria-hidden />
              Hapus foto
            </Tombol>
          )}

          <p className="text-keterangan text-teks-redup">
            Tanpa foto, kartu barang memakai singkatan namanya.
          </p>
        </div>
      </div>

      {galat && <p className="text-keterangan text-bahaya-teks">{galat}</p>}
    </div>
  )
}
