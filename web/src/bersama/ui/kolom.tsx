import { forwardRef, useId } from 'react'
import { AlertCircle, type LucideIcon } from 'lucide-react'
import { cn } from '@/bersama/util/cn'

/**
 * Kolom isian (ui/02-SISTEM-DESAIN.md).
 *
 *   - Label SELALU di atas, tidak pernah cuma placeholder: placeholder hilang
 *     begitu diketik, dan pengguna lupa kolom ini untuk apa.
 *   - Pesan salah muncul DI BAWAH kolomnya, bukan sebagai spanduk di atas layar.
 *   - Kolom uang punya awalan "Rp" yang menyatu di dalam kotak.
 */
export interface PropKolom extends React.InputHTMLAttributes<HTMLInputElement> {
  label: string
  /** Kalimat bantuan singkat. Wajib untuk kolom uang. */
  bantuan?: string
  galat?: string
  /** Teks awalan satuan di dalam kotak, mis. "Rp". */
  awalan?: string
  akhiran?: string
  /** Ikon penanda di awal kotak (dekoratif — labelnya tetap yang menjelaskan). */
  ikon?: LucideIcon
  /**
   * Kendali di ujung kanan kotak, mis. tombol "lihat sandi". Berbeda dari
   * `akhiran` yang hanya teks satuan: ini boleh interaktif, jadi tidak
   * disembunyikan dari pembaca layar.
   */
  sisipanAkhir?: React.ReactNode
}

export const Kolom = forwardRef<HTMLInputElement, PropKolom>(function Kolom(
  {
    label,
    bantuan,
    galat,
    awalan,
    akhiran,
    ikon: Ikon,
    sisipanAkhir,
    className,
    id,
    required,
    ...sisa
  },
  ref,
) {
  const otomatis = useId()
  const idKolom = id ?? otomatis
  const idBantuan = `${idKolom}-bantuan`
  const idGalat = `${idKolom}-galat`

  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={idKolom} className="text-label font-medium text-teks-sekunder">
        {label}
        {required && <span className="text-bahaya-teks"> *</span>}
      </label>

      <div
        className={cn(
          'flex items-center gap-2 rounded-kontrol border bg-permukaan px-3',
          'h-12 focus-within:outline focus-within:outline-2 focus-within:outline-utama',
          'focus-within:outline-offset-2',
          galat ? 'border-bahaya' : 'border-garis',
        )}
      >
        {Ikon && <Ikon className="h-5 w-5 shrink-0 text-teks-redup" aria-hidden />}
        {awalan && (
          <span className="shrink-0 text-teks-redup" aria-hidden>
            {awalan}
          </span>
        )}
        <input
          ref={ref}
          id={idKolom}
          required={required}
          aria-invalid={!!galat}
          aria-describedby={cn(galat ? idGalat : undefined, bantuan ? idBantuan : undefined)}
          className={cn(
            // h-full: seluruh tinggi kotak jadi area ketuk. Tanpa ini kotaknya
            // terlihat 48px tapi hanya ~24px di tengah yang benar-benar
            // memfokuskan isian — jari yang mengenai tepinya tidak berbuat apa-apa.
            'h-full min-w-0 flex-1 bg-transparent text-isi text-teks-utama outline-none',
            'placeholder:text-teks-redup',
            className,
          )}
          {...sisa}
        />
        {akhiran && (
          <span className="shrink-0 text-teks-redup" aria-hidden>
            {akhiran}
          </span>
        )}
        {sisipanAkhir}
      </div>

      {galat ? (
        <p id={idGalat} className="flex items-start gap-1 text-keterangan text-bahaya-teks">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
          {galat}
        </p>
      ) : (
        bantuan && (
          <p id={idBantuan} className="text-keterangan text-teks-redup">
            {bantuan}
          </p>
        )
      )}
    </div>
  )
})

/** Kolom pilihan. Bentuk visualnya sama persis dengan Kolom agar form rapi. */
export interface PropPilihan extends React.SelectHTMLAttributes<HTMLSelectElement> {
  label: string
  bantuan?: string
  galat?: string
}

export const Pilihan = forwardRef<HTMLSelectElement, PropPilihan>(function Pilihan(
  { label, bantuan, galat, className, id, required, children, ...sisa },
  ref,
) {
  const otomatis = useId()
  const idKolom = id ?? otomatis

  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={idKolom} className="text-label font-medium text-teks-sekunder">
        {label}
        {required && <span className="text-bahaya-teks"> *</span>}
      </label>
      <select
        ref={ref}
        id={idKolom}
        required={required}
        aria-invalid={!!galat}
        className={cn(
          'h-12 rounded-kontrol border bg-permukaan px-3 text-isi text-teks-utama',
          galat ? 'border-bahaya' : 'border-garis',
          className,
        )}
        {...sisa}
      >
        {children}
      </select>
      {galat ? (
        <p className="flex items-start gap-1 text-keterangan text-bahaya-teks">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
          {galat}
        </p>
      ) : (
        bantuan && <p className="text-keterangan text-teks-redup">{bantuan}</p>
      )}
    </div>
  )
})
