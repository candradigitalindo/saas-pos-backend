import { useState } from 'react'
import { Dialog, AksiDialog, IsiDialog } from '@/bersama/ui/dialog'
import { Tombol } from '@/bersama/ui/tombol'
import { formatRupiah } from '@/bersama/util/uang'
import { nominalDiskonTransaksi, type DiskonTransaksi } from '../keranjang'
import { KolomDiskon, galatDiskon, type NilaiDiskon } from './kolom-diskon'

/**
 * Diskon untuk seluruh belanja ("potongan 10% hari ini", "genapkan jadi 50
 * ribu"). Persen dihitung dari belanja setelah diskon barang, sebelum pajak &
 * biaya layanan — sama dengan keranjang.
 */
export function DialogDiskonTransaksi({
  sekarang,
  dasar,
  batas,
  onSimpan,
  onTutup,
}: {
  sekarang: DiskonTransaksi | null
  /** Belanja setelah diskon barang (dasar persen). */
  dasar: number
  /** Nominal terbesar yang diterima server (Σ total baris). */
  batas: number
  onSimpan: (d: DiskonTransaksi | null) => void
  onTutup: () => void
}) {
  const [nilai, setNilai] = useState<NilaiDiskon>(sekarang ?? { jenis: 'persen', nilai: 0 })
  const potongan = nominalDiskonTransaksi(
    { subtotal: dasar, discount_amount: 0, tax_amount: 0, service_amount: 0, total: batas },
    nilai,
  )
  const galat = galatDiskon(nilai, batas)

  return (
    <Dialog open onOpenChange={(o) => !o && onTutup()}>
      <IsiDialog judul="Diskon Transaksi" keterangan={`Belanja ${formatRupiah(dasar)}, sebelum pajak & layanan.`}>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (galat) return
            onSimpan(nilai.nilai > 0 ? nilai : null)
          }}
          className="flex flex-col gap-4"
          noValidate
        >
          <KolomDiskon
            nilai={nilai}
            onNilai={setNilai}
            galat={galat}
            bantuan={
              potongan > 0
                ? `${formatRupiah(dasar)} → ${formatRupiah(dasar - potongan)} (hemat ${formatRupiah(potongan)})`
                : undefined
            }
          />
          <AksiDialog>
            <Tombol type="submit" disabled={!!galat}>
              Simpan
            </Tombol>
            {sekarang ? (
              <Tombol type="button" jenis="kedua" onClick={() => onSimpan(null)}>
                Hapus Diskon
              </Tombol>
            ) : (
              <Tombol type="button" jenis="kedua" onClick={onTutup}>
                Batal
              </Tombol>
            )}
          </AksiDialog>
        </form>
      </IsiDialog>
    </Dialog>
  )
}
