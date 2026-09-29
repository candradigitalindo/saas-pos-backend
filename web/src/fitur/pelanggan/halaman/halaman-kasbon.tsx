import { useDeferredValue, useState } from 'react'
import { MessageCircle, NotebookPen, Search, TriangleAlert, Wallet } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka } from '@/bersama/komponen/kerangka'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatLaluHari, formatTanggal } from '@/bersama/util/tanggal'
import { inisialNama } from '@/bersama/util/inisial'
import { kelasAvatar } from '@/bersama/util/warna-kategori'
import { cn } from '@/bersama/util/cn'
import { nomorWA, tautanWA } from '@/fitur/kasir/struk-wa'
import { statusJatuhTempo } from '@/fitur/stok/utang'
import type { KasbonPelanggan } from '../api'
import { teksTagihan } from '../tagihan'
import { DialogRincianKasbon, WARNA_TEMPO, useNamaToko } from '../komponen/rincian-kasbon'
import { DialogSetoran } from '../komponen/dialog-setoran'
import { useRingkasanKasbon } from '../komponen/peringatan-kasbon'

/** Kotak cari muncul bila pelanggan berutang sebanyak ini atau lebih. */
const CARI_MULAI = 8

/**
 * Kasbon — utang pelanggan, PER PELANGGAN.
 *
 * Yang dicari pemilik: siapa yang masih berutang, berapa, dan siapa yang
 * harus ditagih lebih dulu. Karena itu yang lewat jatuh tempo di atas (merah),
 * lalu sisa terbesar. Setiap baris langsung punya "Tagih" (WhatsApp dari nomor
 * toko, teks sudah jadi) dan "Terima" (setoran — dibagi ke kasbon terlama).
 * Rincian nota, janji bayar, dan riwayat setoran ada di balik ketukan baris.
 *
 * Dulu layar ini daftar per NOTA: 25 teratas saja, total dihitung dari 25 itu,
 * nama pelanggan di luar 25 pertama tertulis "Pelanggan", dan kasbon yang
 * sudah dicicil hilang dari saringan "Belum lunas".
 */
export function HalamanKasbon() {
  const namaToko = useNamaToko()
  const [cari, setCari] = useState('')
  const cariTunda = useDeferredValue(cari.trim().toLowerCase())
  const [rinci, setRinci] = useState<KasbonPelanggan | null>(null)
  const [setor, setSetor] = useState<KasbonPelanggan | null>(null)

  const q = useRingkasanKasbon()
  const r = q.data
  const daftar = (r?.customers ?? []).filter(
    (k) => !cariTunda || k.customer_name.toLowerCase().includes(cariTunda) || (k.phone ?? '').includes(cariTunda),
  )

  return (
    <div className="flex w-full max-w-4xl flex-col gap-4">
      <header className="min-w-0">
        <h1 className="text-judul font-bold text-teks-utama">Kasbon</h1>
        <p className="text-label text-teks-sekunder">Siapa yang masih berutang, dan siapa yang perlu ditagih dulu.</p>
      </header>

      {q.isLoading ? (
        <Kerangka className="h-64 w-full rounded-kartu" />
      ) : q.isError ? (
        <KeadaanGagal
          pesan={q.error instanceof GalatAPI ? q.error.pesan : 'Kasbon belum bisa dimuat.'}
          onCobaLagi={() => q.refetch()}
        />
      ) : !r || r.customers.length === 0 ? (
        <KeadaanKosong
          ikon={NotebookPen}
          judul="Tidak ada kasbon yang belum lunas"
          penjelasan='Kasbon muncul di sini saat Anda menjual dengan cara bayar "Kasbon" di kasir, sampai pelanggannya melunasi.'
        />
      ) : (
        <>
          <Kartu className="flex flex-wrap items-end justify-between gap-3 p-4 sm:p-5">
            <div>
              <p className="text-keterangan font-medium text-teks-sekunder">Total belum dibayar</p>
              <p className="text-judul font-extrabold tabular-nums text-teks-utama">{formatRupiah(r.outstanding)}</p>
              <p className="text-keterangan text-teks-redup">
                {r.customers.length} pelanggan · {r.count} nota
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              {r.overdue_count > 0 && (
                <p className="flex items-center gap-1.5 rounded-full bg-bahaya-teks/10 px-3 py-1.5 text-label font-semibold text-bahaya-teks">
                  <TriangleAlert className="h-4 w-4" aria-hidden />
                  {r.overdue_count} pelanggan lewat jatuh tempo · {formatRupiah(r.overdue_amount)}
                </p>
              )}
              {r.due_soon_count > 0 && (
                <p className="rounded-full bg-permukaan-2 px-3 py-1.5 text-label font-semibold text-jingga-700">
                  {r.due_soon_count} pelanggan jatuh tempo ≤ {r.due_soon_days} hari · {formatRupiah(r.due_soon_amount)}
                </p>
              )}
            </div>
          </Kartu>

          {r.customers.length >= CARI_MULAI && (
            <div className="relative">
              <Search
                className="pointer-events-none absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-teks-redup"
                aria-hidden
              />
              <input
                type="search"
                value={cari}
                onChange={(e) => setCari(e.target.value)}
                placeholder="Cari nama atau nomor…"
                aria-label="Cari pelanggan berkasbon"
                className="h-12 w-full rounded-kontrol border border-garis bg-permukaan pl-10 pr-3 text-isi text-teks-utama placeholder:text-teks-redup"
              />
            </div>
          )}

          {daftar.length === 0 ? (
            <p className="py-6 text-center text-label text-teks-redup">Tidak ada pelanggan berkasbon bernama "{cari}".</p>
          ) : (
            <ul className="grid gap-2 md:grid-cols-2">
              {daftar.map((k) => (
                <li key={k.customer_id}>
                  <BarisKasbon
                    k={k}
                    hariIni={r.today}
                    namaToko={namaToko}
                    onRinci={() => setRinci(k)}
                    onSetor={() => setSetor(k)}
                  />
                </li>
              ))}
            </ul>
          )}
        </>
      )}

      <DialogRincianKasbon
        pelanggan={rinci ? { id: rinci.customer_id, name: rinci.customer_name, phone: rinci.phone } : null}
        hariIni={r?.today ?? ''}
        onTutup={() => setRinci(null)}
      />
      <DialogSetoran
        pelanggan={setor ? { id: setor.customer_id, name: setor.customer_name, outstanding: setor.outstanding } : null}
        onTutup={() => setSetor(null)}
      />
    </div>
  )
}

function BarisKasbon({
  k,
  hariIni,
  namaToko,
  onRinci,
  onSetor,
}: {
  k: KasbonPelanggan
  hariIni: string
  namaToko: string
  onRinci: () => void
  onSetor: () => void
}) {
  const tempo = statusJatuhTempo(k.nearest_due, hariIni)
  const lewat = k.overdue_count > 0
  const wa = k.phone ? nomorWA(k.phone) : null
  const teks = teksTagihan({
    nama: k.customer_name,
    toko: namaToko,
    total: k.outstanding,
    hariIni,
    jumlahNota: k.count,
    jatuhTempo: k.nearest_due,
  })
  const keterangan = [
    tempo.nada === 'tanpa' ? `sejak ${formatTanggal(k.oldest_at)}` : null,
    k.last_paid_at ? `terakhir bayar ${formatLaluHari(k.last_paid_at).toLowerCase()}` : 'belum pernah bayar',
  ]
    .filter(Boolean)
    .join(' · ')

  return (
    <Kartu
      className={cn(
        'relative flex h-full flex-col justify-between gap-3 p-4 transition-colors hover:border-utama/40',
        lewat && 'border-bahaya-teks/40',
      )}
    >
      <div className="flex items-start gap-3">
        <span
          className={cn(
            'flex h-11 w-11 shrink-0 items-center justify-center rounded-full text-label font-bold',
            kelasAvatar(k.customer_id),
          )}
          aria-hidden
        >
          {inisialNama(k.customer_name)}
        </span>
        <div className="min-w-0 flex-1">
          {/* Seluruh kartu membuka rinciannya (tombol terentang). */}
          <button
            type="button"
            onClick={onRinci}
            className="block max-w-full truncate text-left font-semibold text-teks-utama after:absolute after:inset-0 after:rounded-kartu focus-visible:outline-none focus-visible:after:ring-2 focus-visible:after:ring-utama"
          >
            {k.customer_name}
          </button>
          <p className="text-keterangan">
            <span className={WARNA_TEMPO[tempo.nada]}>{tempo.nada === 'tanpa' ? `${k.count} nota` : tempo.teks}</span>
            {tempo.nada !== 'tanpa' && <span className="text-teks-redup"> · {k.count} nota</span>}
          </p>
          <p className="text-keterangan text-teks-redup">{keterangan}</p>
        </div>
        <div className="shrink-0 text-right">
          <p className={cn('font-bold tabular-nums', lewat ? 'text-bahaya-teks' : 'text-jingga-700')}>
            {formatRupiah(k.outstanding)}
          </p>
          {k.credit_limit > 0 && (
            <p className="text-keterangan tabular-nums text-teks-redup">batas {formatRupiah(k.credit_limit)}</p>
          )}
        </div>
      </div>
      <div className="relative z-10 grid grid-cols-2 gap-2">
        {wa ? (
          <Tombol jenis="kedua" ukuran="padat" asChild>
            <a href={tautanWA(wa, teks)} target="_blank" rel="noreferrer" aria-label={`Tagih ${k.customer_name} lewat WhatsApp`}>
              <MessageCircle className="h-4 w-4" aria-hidden />
              Tagih
            </a>
          </Tombol>
        ) : (
          <Tombol jenis="kedua" ukuran="padat" onClick={onRinci} aria-label={`Rincian kasbon ${k.customer_name}`}>
            Rincian
          </Tombol>
        )}
        <Tombol ukuran="padat" onClick={onSetor} aria-label={`Terima setoran ${k.customer_name}`}>
          <Wallet className="h-4 w-4" aria-hidden />
          Terima
        </Tombol>
      </div>
    </Kartu>
  )
}
