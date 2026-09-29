import { forwardRef, useState } from 'react'
import { Eye, EyeOff, KeyRound } from 'lucide-react'
import { Kolom, type PropKolom } from './kolom'

/**
 * Kolom kata sandi dengan tombol "lihat sandi".
 *
 * Kasir dan pemilik warung mengetik sandi dengan jempol di HP; salah ketik yang
 * tidak terlihat adalah alasan terbanyak "sandi salah". Dipakai halaman masuk
 * dan daftar supaya perilakunya sama persis.
 */
export const KolomSandi = forwardRef<HTMLInputElement, Omit<PropKolom, 'type' | 'sisipanAkhir'>>(
  function KolomSandi(props, ref) {
    const [lihat, setLihat] = useState(false)
    return (
      <Kolom
        ref={ref}
        ikon={KeyRound}
        {...props}
        type={lihat ? 'text' : 'password'}
        sisipanAkhir={
          <button
            type="button"
            onClick={() => setLihat((v) => !v)}
            aria-label={lihat ? 'Sembunyikan sandi' : 'Lihat sandi'}
            aria-pressed={lihat}
            // Target sentuh 48×48 (ui/01 §3) di dalam kotak setinggi 48px:
            // -my-px menimpa garis tepi 1px atas-bawah, -mr-3 sampai ke tepi
            // kanan. Tanpa latar saat disorot supaya garis tepi tetap utuh.
            className="-my-px -mr-3 flex h-12 w-12 shrink-0 items-center justify-center text-teks-redup hover:text-teks-utama"
          >
            {lihat ? <EyeOff className="h-5 w-5" aria-hidden /> : <Eye className="h-5 w-5" aria-hidden />}
          </button>
        }
      />
    )
  },
)
