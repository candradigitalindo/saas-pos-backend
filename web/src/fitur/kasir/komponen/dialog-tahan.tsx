import { useState } from 'react'
import { Dialog, AksiDialog, IsiDialog } from '@/bersama/ui/dialog'
import { Kolom } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'

/**
 * Menamai keranjang yang ditahan sebagai tagihan terbuka. Nama bebas —
 * "Meja 5", "Pak Budi", "Gojek 2" — cukup untuk menemukannya lagi di daftar.
 */
export function DialogTahan({
  onSimpan,
  onTutup,
}: {
  /** Melempar galat bila gagal; pesannya ditampilkan di dialog. */
  onSimpan: (label: string) => Promise<void>
  onTutup: () => void
}) {
  const [label, setLabel] = useState('')
  const [menyimpan, setMenyimpan] = useState(false)
  const [galat, setGalat] = useState<string | null>(null)

  return (
    <Dialog open onOpenChange={(o) => !o && !menyimpan && onTutup()}>
      <IsiDialog judul="Tahan Tagihan" keterangan="Pesanan disimpan dan bisa dibuka lagi dari perangkat mana pun di toko ini.">
        <form
          onSubmit={async (e) => {
            e.preventDefault()
            if (!label.trim()) return
            setMenyimpan(true)
            setGalat(null)
            try {
              await onSimpan(label.trim())
            } catch (err) {
              setGalat(err instanceof Error ? err.message : 'Gagal menyimpan tagihan.')
              setMenyimpan(false)
            }
          }}
          className="flex flex-col gap-4"
          noValidate
        >
          <Kolom
            label="Nama tagihan"
            placeholder="mis. Meja 5, Pak Budi"
            value={label}
            onChange={(e) => setLabel(e.target.value)}
            maxLength={60}
            autoFocus
            galat={galat ?? undefined}
          />
          <AksiDialog>
            <Tombol type="submit" memuat={menyimpan} disabled={!label.trim()}>
              Simpan Tagihan
            </Tombol>
            <Tombol type="button" jenis="kedua" onClick={onTutup} disabled={menyimpan}>
              Batal
            </Tombol>
          </AksiDialog>
        </form>
      </IsiDialog>
    </Dialog>
  )
}
