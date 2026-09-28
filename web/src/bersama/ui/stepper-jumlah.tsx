import { useRef, useState } from 'react'
import { Minus, Plus } from 'lucide-react'
import { cn } from '@/bersama/util/cn'
import { bacaQty, bandingQty, formatQty, kurangQty, tambahQty } from '@/bersama/util/desimal'

/**
 * Pengubah jumlah dengan tombol − dan + besar, dan angka yang BISA DIKETIK.
 *
 * Tombolnya tetap besar: mengubah satu-dua barang sambil berdiri di depan rak
 * lebih aman lewat ketukan daripada papan ketik. Tapi angkanya juga kolom
 * isian — versi sebelumnya hanya punya tombol, sehingga menghitung fisik 146
 * pcs dari 0 berarti mengetuk "+" 146 kali. Ketuk angkanya, papan angka muncul
 * (inputMode="decimal"), isinya terpilih semua; Enter atau meninggalkan kolom
 * menyimpan. Isian yang bukan angka atau di bawah minimum dikembalikan ke
 * nilai lama — jumlah tidak pernah jadi kosong atau NaN.
 */
export function StepperJumlah({
  nilai,
  onNilai,
  langkah = 1,
  minimal = '0',
  satuan,
  besar,
  label = 'Jumlah',
}: {
  nilai: string
  onNilai: (qty: string) => void
  langkah?: number
  minimal?: string
  satuan?: string
  besar?: boolean
  label?: string
}) {
  const ukuran = besar ? 'h-16 w-16' : 'h-12 w-12'
  // null = tidak sedang diketik; angkanya mengikuti `nilai`.
  const [teks, setTeks] = useState<string | null>(null)
  // Escape membatalkan: onBlur yang menyusul masih melihat `teks` lama, jadi
  // pembatalannya ditandai lewat ref, bukan lewat state.
  const batal = useRef(false)
  const tampil = teks ?? formatQty(nilai)

  function simpan() {
    if (batal.current) {
      batal.current = false
      setTeks(null)
      return
    }
    if (teks === null) return
    const baru = bacaQty(teks)
    if (baru !== null && bandingQty(baru, minimal) >= 0 && bandingQty(baru, nilai) !== 0) onNilai(baru)
    setTeks(null)
  }

  return (
    <div className="flex items-center gap-2" role="group" aria-label={label}>
      <button
        type="button"
        onClick={() => onNilai(kurangQty(nilai, langkah, minimal))}
        aria-label={`Kurangi ${label.toLowerCase()}`}
        className={cn(
          ukuran,
          'flex shrink-0 items-center justify-center rounded-kontrol border',
          'border-garis bg-permukaan text-teks-utama hover:bg-permukaan-2',
        )}
      >
        <Minus className="h-5 w-5" aria-hidden />
      </button>

      <label
        className={cn(
          'flex min-w-16 flex-1 cursor-text items-center justify-center gap-1 rounded-kontrol font-bold tabular-nums',
          'focus-within:ring-2 focus-within:ring-utama',
          besar ? 'h-16 text-judul' : 'h-12 text-judul-kartu',
        )}
      >
        <input
          type="text"
          inputMode="decimal"
          value={tampil}
          size={Math.max(1, tampil.length)}
          onFocus={(e) => {
            setTeks(formatQty(nilai))
            e.currentTarget.select()
          }}
          onChange={(e) => setTeks(e.target.value)}
          onBlur={simpan}
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              // Enter menyimpan angka, BUKAN mengirim formulir di sekitarnya.
              e.preventDefault()
              e.currentTarget.blur()
            } else if (e.key === 'Escape') {
              batal.current = true
              e.currentTarget.blur()
            }
          }}
          aria-label={label}
          className="min-w-0 bg-transparent text-center outline-none"
        />
        {satuan && <span className="text-label font-normal text-teks-redup">{satuan}</span>}
      </label>

      <button
        type="button"
        onClick={() => onNilai(tambahQty(nilai, langkah))}
        aria-label={`Tambah ${label.toLowerCase()}`}
        className={cn(
          ukuran,
          'flex shrink-0 items-center justify-center rounded-kontrol border',
          'border-garis bg-permukaan text-teks-utama hover:bg-permukaan-2',
        )}
      >
        <Plus className="h-5 w-5" aria-hidden />
      </button>
    </div>
  )
}
