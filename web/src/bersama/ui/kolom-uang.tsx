import { forwardRef } from 'react'
import { Kolom, type PropKolom } from './kolom'
import { formatAngka, parseRupiah } from '@/bersama/util/uang'

interface PropKolomUang extends Omit<PropKolom, 'value' | 'onChange' | 'awalan' | 'type'> {
  /** Nilai dalam rupiah BULAT. Tidak pernah pecahan. */
  nilai: number
  onNilai: (rupiah: number) => void
}

/**
 * Kolom uang: papan tik angka di HP dan pemisah ribuan otomatis saat mengetik.
 * Nilai yang keluar selalu bilangan bulat rupiah.
 */
export const KolomUang = forwardRef<HTMLInputElement, PropKolomUang>(function KolomUang(
  { nilai, onNilai, ...sisa },
  ref,
) {
  return (
    <Kolom
      ref={ref}
      awalan="Rp"
      inputMode="numeric"
      autoComplete="off"
      value={nilai === 0 ? '' : formatAngka(nilai)}
      onChange={(e) => onNilai(parseRupiah(e.target.value))}
      placeholder="0"
      {...sisa}
    />
  )
})
