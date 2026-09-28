import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useLiveQuery } from 'dexie-react-hooks'
import { MessageCircle } from 'lucide-react'
import { Dialog, AksiDialog, IsiDialog } from '@/bersama/ui/dialog'
import { Kolom } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useOnline } from '@/bersama/hooks/use-online'
import { db } from '@/lib/offline/db'
import type { Transaksi } from '@/bersama/tipe/pos'
import { kasirApi } from '../api'
import { alamatStruk, nomorWA, tautanWA, teksStrukWA } from '../struk-wa'

/**
 * Kirim struk lewat WhatsApp TOKO di perangkat ini: aplikasi menyiapkan pesan
 * (teks struk + tautan struk digital), WhatsApp terbuka, kasir menekan kirim.
 *
 * Nomor terisi otomatis bila transaksinya atas nama pelanggan. Dikosongkan
 * pun boleh — WhatsApp lalu meminta kasir memilih kontak.
 *
 * Tautan disiapkan SAAT DIALOG DIBUKA, bukan saat tombol ditekan: jendela
 * WhatsApp harus dibuka langsung dari ketukan, atau peramban HP memblokirnya
 * sebagai munculan.
 */
export function DialogKirimWA({
  transaksi,
  diantre = false,
  onTutup,
}: {
  transaksi: Transaksi
  /** Transaksi masih di antrean offline — belum ada di server, jadi belum bertautan. */
  diantre?: boolean
  onTutup: () => void
}) {
  const { rincianToko } = useSesi()
  const online = useOnline()
  const pelanggan = useLiveQuery(
    async () => (transaksi.customer_id ? await db.pelanggan.get(transaksi.customer_id) : undefined),
    [transaksi.customer_id],
  )
  // null = belum disentuh kasir → pakai nomor pelanggan bila ada.
  const [ketikan, setKetikan] = useState<string | null>(null)
  const isian = ketikan ?? pelanggan?.phone ?? ''
  const nomor = nomorWA(isian)

  const tautan = useQuery({
    queryKey: ['tautan-struk', transaksi.id],
    queryFn: () => kasirApi.tautanStruk(transaksi.id),
    enabled: !diantre && online,
    staleTime: Infinity,
    retry: 1,
  })
  const alamat = tautan.data ? alamatStruk(tautan.data.token) : undefined

  const keterangan = diantre
    ? 'Transaksi belum terkirim ke server, jadi pesannya berisi teks struk saja — tautan struk digital menyusul setelah terkirim.'
    : !online
      ? 'Sedang offline: pesannya berisi teks struk saja, tanpa tautan struk digital.'
      : tautan.isError
        ? 'Tautan struk digital gagal disiapkan; pesannya berisi teks struk saja.'
        : 'Berisi rincian struk dan tautan struk digital yang bisa dibuka pembeli kapan saja.'

  return (
    <Dialog open onOpenChange={(o) => !o && onTutup()}>
      <IsiDialog judul="Kirim Struk lewat WhatsApp" keterangan={keterangan}>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            if (nomor === null) return
            const teks = teksStrukWA(transaksi, rincianToko?.name ?? 'Struk', alamat)
            window.open(tautanWA(nomor, teks), '_blank', 'noopener')
            onTutup()
          }}
          className="flex flex-col gap-4"
          noValidate
        >
          <Kolom
            label="Nomor WhatsApp pembeli"
            type="tel"
            inputMode="tel"
            placeholder="0812 3456 7890"
            value={isian}
            onChange={(e) => setKetikan(e.target.value)}
            galat={nomor === null ? 'Nomor tidak dikenali — contoh 0812 3456 7890.' : undefined}
            bantuan={
              pelanggan?.phone && ketikan === null
                ? `Nomor ${pelanggan.name}.`
                : 'Boleh dikosongkan: pilih kontaknya nanti di WhatsApp.'
            }
            autoFocus={!pelanggan?.phone}
          />
          <AksiDialog>
            <Tombol type="submit" disabled={nomor === null || tautan.isFetching} memuat={tautan.isFetching}>
              <MessageCircle className="h-5 w-5" aria-hidden />
              Buka WhatsApp
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
