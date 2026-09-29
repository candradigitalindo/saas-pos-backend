import { useState } from 'react'
import { Dialog, AksiDialog, IsiDialog } from '@/bersama/ui/dialog'
import { Kolom } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQty } from '@/bersama/util/desimal'
import { diskonBaris, hargaBaris, kotorBaris, namaBaris, type BarisKeranjang } from '../keranjang'
import { KolomDiskon, galatDiskon, type NilaiDiskon } from './kolom-diskon'

/**
 * Catatan & diskon untuk satu baris keranjang.
 *
 * Diskon hanya tampil bagi yang berizin sale.discount (izin menyembunyikan,
 * bukan menonaktifkan); server tetap menegakkan izinnya sendiri. Hasil potongan
 * langsung ditunjukkan dalam rupiah supaya "10%" tidak perlu dihitung di kepala.
 */
export function DialogAturBaris({
  baris,
  qtyProduk,
  bolehDiskon,
  onSimpan,
  onTutup,
}: {
  baris: BarisKeranjang
  /** Total jumlah barang ini di keranjang — penentu harga grosir. */
  qtyProduk?: string
  bolehDiskon: boolean
  onSimpan: (isi: Pick<BarisKeranjang, 'catatan' | 'diskon' | 'diskonPersen'>) => void
  onTutup: () => void
}) {
  const [catatan, setCatatan] = useState(baris.catatan ?? '')
  const [diskon, setDiskon] = useState<NilaiDiskon>(
    baris.diskonPersen !== undefined
      ? { jenis: 'persen', nilai: baris.diskonPersen }
      : { jenis: 'nominal', nilai: baris.diskon },
  )
  const isi = {
    catatan,
    diskon: diskon.jenis === 'nominal' ? diskon.nilai : 0,
    diskonPersen: diskon.jenis === 'persen' && diskon.nilai > 0 ? diskon.nilai : undefined,
  }
  const kotor = kotorBaris(baris, qtyProduk)
  const potongan = diskonBaris({ ...baris, ...isi }, qtyProduk)
  const galat = bolehDiskon ? galatDiskon(diskon, kotor) : undefined

  return (
    <Dialog open onOpenChange={(o) => !o && onTutup()}>
      <IsiDialog
        judul={namaBaris(baris)}
        keterangan={`${formatQty(baris.qty)} ${baris.produk.unit_name ?? ''} × ${formatRupiah(hargaBaris(baris, qtyProduk))}`}
      >
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (galat) return
            onSimpan(bolehDiskon ? isi : { catatan, diskon: baris.diskon, diskonPersen: baris.diskonPersen })
          }}
          className="flex flex-col gap-4"
          noValidate
        >
          <Kolom
            label="Catatan"
            placeholder="mis. tanpa es, pedas sedang"
            value={catatan}
            onChange={(e) => setCatatan(e.target.value)}
            maxLength={200}
            bantuan="Ikut tercetak di struk."
            autoFocus={!bolehDiskon}
          />
          {bolehDiskon && (
            <KolomDiskon
              nilai={diskon}
              onNilai={setDiskon}
              galat={galat}
              bantuan={
                potongan > 0
                  ? `${formatRupiah(kotor)} → ${formatRupiah(kotor - potongan)} (hemat ${formatRupiah(potongan)})`
                  : `Harga baris ${formatRupiah(kotor)}.`
              }
            />
          )}
          <AksiDialog>
            <Tombol type="submit" disabled={!!galat}>
              Simpan
            </Tombol>
            <Tombol type="button" jenis="kedua" onClick={onTutup}>
              Batal
            </Tombol>
          </AksiDialog>
        </form>
      </IsiDialog>
    </Dialog>
  )
}
