import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ReceiptText } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { Kolom } from '@/bersama/ui/kolom'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { KeadaanKosong, KeadaanGagal } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { LencanaTransaksi } from '@/bersama/komponen/lencana-status'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQty } from '@/bersama/util/desimal'
import { formatJam, tanggalISO } from '@/bersama/util/tanggal'
import type { Transaksi } from '@/bersama/tipe/pos'
import { kasirApi } from '../api'
import { useRiwayatTransaksi } from '../hooks'

/** Riwayat transaksi hari ini, dengan pembatalan dan retur. */
export function HalamanRiwayat() {
  const { tokoAktif, boleh } = useSesi()
  const [tanggal, setTanggal] = useState(() => tanggalISO())
  const [dibatalkan, setDibatalkan] = useState<Transaksi | null>(null)

  const q = useRiwayatTransaksi({ outlet_id: tokoAktif, business_date: tanggal })

  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-end justify-between gap-3">
        <h1 className="text-judul font-bold text-teks-utama">Riwayat Penjualan</h1>
        <div className="w-44">
          <Kolom
            label="Tanggal"
            type="date"
            value={tanggal}
            onChange={(e) => setTanggal(e.target.value)}
          />
        </div>
      </header>

      {q.isLoading ? (
        <KerangkaBaris jumlah={5} />
      ) : q.isError ? (
        <KeadaanGagal
          pesan={
            q.error instanceof GalatAPI
              ? q.error.pesan
              : 'Daftar transaksi belum bisa dimuat.'
          }
          onCobaLagi={() => q.refetch()}
        />
      ) : (q.data?.data.length ?? 0) === 0 ? (
        <KeadaanKosong
          ikon={ReceiptText}
          judul="Belum ada transaksi"
          penjelasan={
            tanggal === tanggalISO()
              ? 'Transaksi hari ini akan muncul di sini begitu ada penjualan.'
              : 'Tidak ada penjualan pada tanggal itu.'
          }
        />
      ) : (
        <ul className="flex flex-col gap-2">
          {q.data?.data.map((t) => (
            <li key={t.id}>
              <BarisTransaksi
                transaksi={t}
                bolehBatalkan={boleh(IZIN.saleVoid)}
                onBatalkan={() => setDibatalkan(t)}
              />
            </li>
          ))}
        </ul>
      )}

      {dibatalkan && (
        <DialogBatalkan transaksi={dibatalkan} onTutup={() => setDibatalkan(null)} />
      )}
    </div>
  )
}

function BarisTransaksi({
  transaksi,
  bolehBatalkan,
  onBatalkan,
}: {
  transaksi: Transaksi
  bolehBatalkan: boolean
  onBatalkan: () => void
}) {
  const sudahBatal = !!transaksi.voided_at

  return (
    <Kartu className="flex items-center justify-between gap-3 p-4">
      <div className="min-w-0">
        <div className="flex flex-wrap items-center gap-2">
          <p className="font-semibold tabular-nums text-teks-utama">
            {transaksi.receipt_no}
          </p>
          <LencanaTransaksi status={sudahBatal ? 'void' : transaksi.status} />
        </div>
        <p className="text-keterangan text-teks-redup">
          {formatJam(transaksi.occurred_at)}
          {transaksi.void_reason && ` · ${transaksi.void_reason}`}
        </p>
      </div>

      <div className="flex shrink-0 items-center gap-3">
        <p className="text-judul-kartu font-bold tabular-nums text-teks-utama">
          {formatRupiah(transaksi.total)}
        </p>
        {/* Izin menyembunyikan: kalau tidak berhak, tombolnya tidak ada. */}
        {bolehBatalkan && !sudahBatal && (
          <Tombol jenis="kedua" ukuran="padat" onClick={onBatalkan}>
            Batalkan
          </Tombol>
        )}
      </div>
    </Kartu>
  )
}

/**
 * Pembatalan transaksi adalah aksi berat dan tidak bisa dibalik, jadi dialognya
 * MENJELASKAN AKIBATNYA DENGAN ANGKA — bukan sekadar bertanya "Anda yakin?".
 * Dialog yang cuma bertanya akan diklik "Ya" tanpa dibaca setelah kali kelima.
 */
function DialogBatalkan({
  transaksi,
  onTutup,
}: {
  transaksi: Transaksi
  onTutup: () => void
}) {
  const qc = useQueryClient()
  const toast = useToast()
  const [alasan, setAlasan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)

  const batal = useMutation({
    mutationFn: () => kasirApi.batalkan(transaksi.id, alasan.trim()),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['riwayat-transaksi'] })
      qc.invalidateQueries({ queryKey: ['shift-aktif'] })
      qc.invalidateQueries({ queryKey: ['stok-kasir'] })
      toast.berhasil(`Transaksi ${transaksi.receipt_no} dibatalkan.`)
      onTutup()
    },
    onError: (e) =>
      setGalat(
        e instanceof GalatAPI
          ? e.status === 409
            ? 'Transaksi ini sudah dibatalkan sebelumnya.'
            : e.pesan
          : 'Terjadi kesalahan. Coba lagi.',
      ),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !batal.isPending && onTutup()}>
      <IsiDialog judul={`Batalkan transaksi ${transaksi.receipt_no}?`}>
        <div className="flex flex-col gap-2 text-isi text-teks-sekunder">
          {transaksi.items && transaksi.items.length > 0 && (
            <p>
              Stok{' '}
              {transaksi.items
                .map((i) => `${formatQty(i.qty)} ${i.unit_name} ${i.product_name}`)
                .join(', ')}{' '}
              akan dikembalikan.
            </p>
          )}
          <p>
            Omzet hari ini turun{' '}
            <strong className="tabular-nums text-teks-utama">
              {formatRupiah(transaksi.total)}
            </strong>
            .
          </p>
          <p>
            Transaksi ini tetap tercatat di riwayat sebagai &ldquo;dibatalkan&rdquo;.
          </p>
        </div>

        <Kolom
          label="Alasan pembatalan"
          placeholder="Contoh: pembeli batal beli"
          value={alasan}
          onChange={(e) => setAlasan(e.target.value)}
          bantuan="Minimal 3 huruf. Dicatat supaya bisa ditelusuri nanti."
          required
        />

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <AksiDialog>
          {/* Tombol berbahaya diberi jarak dari tombol batal (flex-row-reverse
              di AksiDialog menaruhnya di kanan, jauh dari ibu jari yang refleks). */}
          <Tombol
            jenis="bahaya"
            memuat={batal.isPending}
            labelMemuat="Membatalkan…"
            disabled={alasan.trim().length < 3}
            onClick={() => batal.mutate()}
          >
            Ya, Batalkan Transaksi
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={batal.isPending}>
            Jangan jadi
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}
