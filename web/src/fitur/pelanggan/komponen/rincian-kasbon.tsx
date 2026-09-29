import { useEffect, useState } from 'react'
import { Link } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { CalendarClock, ChevronRight, Copy, MessageCircle, Wallet } from 'lucide-react'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { Kolom } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatTanggal, formatTanggalJam } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { nomorWA, tautanWA } from '@/fitur/kasir/struk-wa'
import { statusJatuhTempo, tambahHari, type NadaJatuhTempo } from '@/fitur/stok/utang'
import { pelangganApi, type Kasbon } from '../api'
import { teksTagihan } from '../tagihan'
import { DialogSetoran } from './dialog-setoran'

export const WARNA_TEMPO: Record<NadaJatuhTempo, string> = {
  lewat: 'font-semibold text-bahaya-teks',
  hariIni: 'font-semibold text-jingga-700',
  dekat: 'text-jingga-700',
  nanti: 'text-teks-redup',
  tanpa: 'text-teks-redup',
}

const CARA: Record<string, string> = { cash: 'Tunai', qris: 'QRIS', transfer: 'Transfer', card: 'Kartu', ewallet: 'E-wallet' }

/** Nama toko untuk teks tagihan. */
export function useNamaToko(): string {
  const { rincianToko, profil } = useSesi()
  return rincianToko?.name || profil?.tenant.business_name || 'toko kami'
}

/**
 * Kasbon belum lunas SEORANG pelanggan: tiap nota (terlama dulu — urutan
 * pelunasan) dengan jatuh temponya, tombol Terima setoran, Tagih lewat
 * WhatsApp (teks merinci nota & jatuh tempo), dan riwayat setorannya.
 * Dipakai di dialog rincian (layar Kasbon) dan di halaman pelanggan.
 */
export function RincianKasbon({
  pelanggan: p,
  hariIni,
}: {
  pelanggan: { id: string; name: string; phone?: string }
  hariIni: string
}) {
  const toast = useToast()
  const namaToko = useNamaToko()
  const [setor, setSetor] = useState(false)
  const [aturTempo, setAturTempo] = useState<Kasbon | null>(null)

  const q = useQuery({
    queryKey: ['kasbon', 'pelanggan', p.id],
    queryFn: () => pelangganApi.daftarKasbon(p.id, 'unpaid', 1, 100),
  })
  const daftar = q.data?.data ?? []
  const total = daftar.reduce((j, k) => j + k.outstanding, 0)
  const lewat = daftar.filter((k) => statusJatuhTempo(k.due_date, hariIni).nada === 'lewat')

  const teks = teksTagihan({
    nama: p.name,
    toko: namaToko,
    total,
    hariIni,
    rincian: daftar.map((k) => ({ tanggal: k.business_date, nota: k.receipt_no, sisa: k.outstanding, jatuhTempo: k.due_date })),
  })
  const wa = p.phone ? nomorWA(p.phone) : null

  if (q.isLoading) return <KerangkaBaris jumlah={3} />

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-end justify-between gap-2">
        <div>
          <p className="text-keterangan font-medium text-teks-sekunder">Sisa kasbon</p>
          <p className={cn('text-judul font-extrabold tabular-nums', total > 0 ? 'text-teks-utama' : 'text-hijau-700')}>
            {total > 0 ? formatRupiah(total) : 'Lunas'}
          </p>
          {total > 0 && (
            <p className="text-keterangan text-teks-redup">
              {daftar.length} nota
              {lewat.length > 0 && (
                <span className="font-semibold text-bahaya-teks">
                  {' '}
                  · {formatRupiah(lewat.reduce((j, k) => j + k.outstanding, 0))} lewat jatuh tempo
                </span>
              )}
            </p>
          )}
        </div>
      </div>

      {/* Bertumpuk di HP: berdampingan, label keduanya terbungkus dua baris. */}
      {total > 0 && (
        <div className="flex flex-col gap-2 sm:grid sm:grid-cols-2">
          <Tombol onClick={() => setSetor(true)}>
            <Wallet className="h-5 w-5" aria-hidden />
            Terima setoran
          </Tombol>
          {wa ? (
            <Tombol jenis="kedua" asChild>
              <a href={tautanWA(wa, teks)} target="_blank" rel="noreferrer">
                <MessageCircle className="h-5 w-5" aria-hidden />
                Tagih via WA
              </a>
            </Tombol>
          ) : (
            <Tombol
              jenis="kedua"
              onClick={() =>
                navigator.clipboard
                  .writeText(teks)
                  .then(() => toast.berhasil('Teks tagihan disalin — belum ada nomor WhatsApp.'))
                  .catch(() => toast.gagal('Tidak bisa menyalin teks.'))
              }
            >
              <Copy className="h-5 w-5" aria-hidden />
              Salin tagihan
            </Tombol>
          )}
        </div>
      )}

      {daftar.length > 0 && (
        <ul className="-mx-1 divide-y divide-garis">
          {daftar.map((k) => {
            const tempo = statusJatuhTempo(k.due_date, hariIni)
            return (
              <li key={k.id} className="flex items-center gap-3 px-1 py-2.5">
                {/* Nomor nota panjang ("01M2-260917-0010") di barisnya sendiri —
                    disambung ke tanggal, ia terpotong di tengah di HP. */}
                <div className="min-w-0 flex-1">
                  <p className="text-label font-medium text-teks-utama">
                    {k.business_date ? formatTanggal(k.business_date + 'T12:00:00Z') : formatTanggal(k.created_at)}
                    <span className={cn('text-keterangan', WARNA_TEMPO[tempo.nada])}> · {tempo.teks}</span>
                  </p>
                  {k.receipt_no && <p className="truncate text-keterangan tabular-nums text-teks-redup">{k.receipt_no}</p>}
                  {k.paid_amount > 0 && (
                    <p className="text-keterangan text-teks-redup">
                      dicicil {formatRupiah(k.paid_amount)} dari {formatRupiah(k.amount)}
                    </p>
                  )}
                </div>
                <span className="shrink-0 font-semibold tabular-nums text-teks-utama">{formatRupiah(k.outstanding)}</span>
                <Tombol
                  jenis="teks"
                  ukuran="ikon"
                  onClick={() => setAturTempo(k)}
                  aria-label={`Atur jatuh tempo ${k.receipt_no ?? ''}`.trim()}
                  className="-mr-2 shrink-0"
                >
                  <CalendarClock className="h-5 w-5" aria-hidden />
                </Tombol>
              </li>
            )
          })}
        </ul>
      )}

      <RiwayatSetoran pelangganId={p.id} />

      <DialogSetoran
        pelanggan={setor ? { id: p.id, name: p.name, outstanding: total } : null}
        onTutup={() => setSetor(false)}
      />
      <DialogJatuhTempo kasbon={aturTempo} hariIni={hariIni} onTutup={() => setAturTempo(null)} />
    </div>
  )
}

/** Setoran terakhir, dilipat — jejak "sudah bayar kapan & ke siapa". */
function RiwayatSetoran({ pelangganId }: { pelangganId: string }) {
  const [buka, setBuka] = useState(false)
  const q = useQuery({
    queryKey: ['kasbon', 'setoran', pelangganId],
    queryFn: () => pelangganApi.riwayatSetoran(pelangganId),
    enabled: buka,
  })
  return (
    <details className="rounded-kontrol bg-permukaan-2/60 px-3 py-2" onToggle={(e) => setBuka(e.currentTarget.open)}>
      <summary className="cursor-pointer text-label font-medium text-teks-sekunder">Riwayat setoran</summary>
      {q.isLoading ? (
        <KerangkaBaris jumlah={2} />
      ) : !q.data?.length ? (
        <p className="py-2 text-keterangan text-teks-redup">Belum ada setoran.</p>
      ) : (
        <ul className="mt-1 divide-y divide-garis">
          {q.data.map((s) => (
            <li key={s.id} className="flex items-baseline justify-between gap-3 py-2 text-keterangan">
              <span className="min-w-0">
                <span className="block text-teks-utama">{formatTanggalJam(s.paid_at)}</span>
                <span className="block text-teks-redup">
                  {[CARA[s.method] ?? s.method, s.to_drawer && 'masuk laci', s.receipt_no, s.collected_name, s.note]
                    .filter(Boolean)
                    .join(' · ')}
                </span>
              </span>
              <span className="shrink-0 font-semibold tabular-nums text-hijau-700">{formatRupiah(s.amount)}</span>
            </li>
          ))}
        </ul>
      )}
    </details>
  )
}

/**
 * Atur jatuh tempo satu kasbon — biasanya "janji bayar": pelanggan bilang
 * akan melunasi tanggal sekian. Pilihan cepat +7/+14/+30 hari dari hari ini.
 */
function DialogJatuhTempo({ kasbon: k, hariIni, onTutup }: { kasbon: Kasbon | null; hariIni: string; onTutup: () => void }) {
  const qc = useQueryClient()
  const toast = useToast()
  const [tgl, setTgl] = useState('')
  const [galat, setGalat] = useState<string | null>(null)
  useEffect(() => {
    if (!k) return
    setTgl(k.due_date ?? '')
    setGalat(null)
  }, [k])

  const simpan = useMutation({
    mutationFn: (due: string) => pelangganApi.aturJatuhTempo(k!.id, due),
    onSuccess: (_, due) => {
      qc.invalidateQueries({ queryKey: ['kasbon'] })
      qc.invalidateQueries({ queryKey: ['pelanggan'] })
      toast.berhasil(due ? `Jatuh tempo diatur ${formatTanggal(due + 'T12:00:00Z')}.` : 'Jatuh tempo dihapus.')
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  return (
    <Dialog open={!!k} onOpenChange={(o) => !o && !simpan.isPending && onTutup()}>
      {k && (
        <IsiDialog
          judul="Jatuh tempo kasbon"
          keterangan={`${k.receipt_no ? `Nota ${k.receipt_no} · ` : ''}sisa ${formatRupiah(k.outstanding)}`}
        >
          <div className="flex flex-col gap-3">
            <div className="flex flex-wrap gap-2">
              {[7, 14, 30].map((n) => {
                const t = tambahHari(hariIni, n)
                return (
                  <Tombol
                    key={n}
                    jenis={tgl === t ? 'utama' : 'kedua'}
                    ukuran="padat"
                    onClick={() => setTgl(t)}
                    aria-pressed={tgl === t}
                  >
                    +{n} hari
                  </Tombol>
                )
              })}
            </div>
            <Kolom
              label="Tanggal janji bayar"
              type="date"
              value={tgl}
              min={hariIni}
              onChange={(e) => setTgl(e.target.value)}
              bantuan="Kasbon yang lewat tanggal ini ditandai merah dan naik ke atas daftar Kasbon."
            />
            {galat && (
              <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
                {galat}
              </p>
            )}
          </div>
          <AksiDialog>
            <Tombol disabled={!tgl} memuat={simpan.isPending} onClick={() => simpan.mutate(tgl)}>
              Simpan
            </Tombol>
            {k.due_date ? (
              <Tombol jenis="kedua" disabled={simpan.isPending} onClick={() => simpan.mutate('')}>
                Tanpa jatuh tempo
              </Tombol>
            ) : (
              <Tombol jenis="kedua" onClick={onTutup} disabled={simpan.isPending}>
                Batal
              </Tombol>
            )}
          </AksiDialog>
        </IsiDialog>
      )}
    </Dialog>
  )
}

/**
 * Dialog rincian kasbon seorang pelanggan (dari layar Kasbon) — plus tautan
 * ke halaman pelanggannya.
 */
export function DialogRincianKasbon({
  pelanggan,
  hariIni,
  onTutup,
}: {
  pelanggan: { id: string; name: string; phone?: string } | null
  hariIni: string
  onTutup: () => void
}) {
  return (
    <Dialog open={!!pelanggan} onOpenChange={(o) => !o && onTutup()}>
      {pelanggan && (
        <IsiDialog judul={`Kasbon ${pelanggan.name}`} keterangan={pelanggan.phone || 'Belum ada nomor WhatsApp'} className="sm:max-w-lg">
          <RincianKasbon pelanggan={pelanggan} hariIni={hariIni} />
          <Link
            to={`/pelanggan/${pelanggan.id}`}
            className="flex min-h-11 items-center justify-center gap-1 text-label font-medium text-utama hover:underline"
          >
            Lihat belanja {pelanggan.name}
            <ChevronRight className="h-4 w-4" aria-hidden />
          </Link>
        </IsiDialog>
      )}
    </Dialog>
  )
}
