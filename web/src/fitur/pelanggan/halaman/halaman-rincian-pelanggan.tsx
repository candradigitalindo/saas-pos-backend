import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, MessageCircle, NotebookPen, Pencil, Phone, ShoppingBag, Trash2 } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { KeadaanGagal } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka, KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { IZIN } from '@/lib/izin'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQty, formatQtySatuan } from '@/bersama/util/desimal'
import { formatLaluHari, formatTanggalJam, tanggalISO } from '@/bersama/util/tanggal'
import { inisialNama } from '@/bersama/util/inisial'
import { kelasAvatar } from '@/bersama/util/warna-kategori'
import { cn } from '@/bersama/util/cn'
import type { Transaksi } from '@/bersama/tipe/pos'
import { nomorWA } from '@/fitur/kasir/struk-wa'
import { namaMetode } from '@/fitur/kasir/label-transaksi'
import { pelangganApi } from '../api'
import { DialogPelanggan } from '../komponen/dialog-pelanggan'
import { RincianKasbon } from '../komponen/rincian-kasbon'

/**
 * Satu pelanggan: kontak, angka belanjanya di toko ini (total, kunjungan,
 * rata-rata, terakhir datang), barang yang sering dibeli, KASBON (tagih lewat
 * WhatsApp, terima setoran, janji bayar), dan riwayat belanja.
 *
 * Pola yang sama dengan halaman pemasok: yang dicari pemilik tentang seorang
 * pelanggan ada di satu tempat, bukan tersebar di Pelanggan, Kasbon, dan
 * Riwayat Penjualan.
 */
export function HalamanRincianPelanggan() {
  const { id } = useParams()
  const { tokoAktif, boleh, rincianToko } = useSesi()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const toast = useToast()
  const [ubah, setUbah] = useState(false)
  const [hapus, setHapus] = useState(false)
  const zona = rincianToko?.timezone
  const hariIni = tanggalISO(undefined, zona)

  const q = useQuery({
    queryKey: ['pelanggan', 'satu', id, tokoAktif],
    queryFn: () => pelangganApi.satu(id!, tokoAktif ?? undefined),
    enabled: !!id,
  })

  const hapusM = useMutation({
    mutationFn: () => pelangganApi.hapus(id!),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pelanggan'] })
      toast.berhasil(`${q.data?.name} dihapus. Riwayat belanja & kasbonnya tetap tersimpan.`)
      navigate('/pelanggan', { replace: true })
    },
    onError: (e) => toast.gagal(e instanceof GalatAPI ? e.pesan : 'Pelanggan belum bisa dihapus.'),
  })

  if (q.isLoading) return <Kerangka className="h-64 w-full max-w-4xl rounded-kartu" />
  if (q.isError || !q.data) {
    return (
      <KeadaanGagal
        pesan={q.error instanceof GalatAPI ? q.error.pesan : 'Pelanggan tidak ditemukan.'}
        onCobaLagi={() => q.refetch()}
      />
    )
  }
  const p = q.data
  const st = p.stats
  const wa = p.phone ? nomorWA(p.phone) : null
  const bolehKasbon = boleh(IZIN.receivableManage)
  const kasbon = st?.receivable_outstanding ?? 0
  const syarat = [
    p.credit_term_days ? `tempo kasbon ${p.credit_term_days} hari` : null,
    p.credit_limit ? `batas ${formatRupiah(p.credit_limit)}` : null,
  ].filter(Boolean)

  return (
    <div className="flex w-full max-w-4xl flex-col gap-4">
      <Link to="/pelanggan" className="flex min-h-9 items-center gap-1 self-start text-label font-medium text-utama hover:underline">
        <ArrowLeft className="h-4 w-4" aria-hidden />
        Pelanggan
      </Link>

      <Kartu className="flex flex-col gap-4 p-4 sm:p-5">
        <div className="flex items-start gap-3">
          <span
            className={cn('flex h-14 w-14 shrink-0 items-center justify-center rounded-full text-judul-kartu font-bold', kelasAvatar(p.id))}
            aria-hidden
          >
            {inisialNama(p.name)}
          </span>
          <div className="min-w-0 flex-1">
            <h1 className="text-judul font-bold text-teks-utama">{p.name}</h1>
            <p className="text-label text-teks-sekunder">{p.phone || 'Belum ada nomor'}</p>
            {p.address && <p className="text-keterangan text-teks-redup">{p.address}</p>}
            {syarat.length > 0 && <p className="text-keterangan text-teks-redup">{syarat.join(' · ')}</p>}
            {p.note && <p className="mt-1 text-keterangan italic text-teks-sekunder">“{p.note}”</p>}
          </div>
          {boleh(IZIN.customerEdit) && (
            <div className="-mr-2 -mt-2 flex shrink-0">
              <Tombol jenis="teks" ukuran="ikon" onClick={() => setUbah(true)} aria-label={`Ubah ${p.name}`}>
                <Pencil className="h-5 w-5" aria-hidden />
              </Tombol>
              <Tombol
                jenis="teks"
                ukuran="ikon"
                onClick={() => setHapus(true)}
                aria-label={`Hapus ${p.name}`}
                className="text-bahaya-teks"
              >
                <Trash2 className="h-5 w-5" aria-hidden />
              </Tombol>
            </div>
          )}
        </div>
        {(wa || p.phone) && (
          <div className="flex flex-wrap gap-2">
            {wa && (
              <Tombol jenis="kedua" ukuran="padat" asChild>
                <a href={`https://wa.me/${wa}`} target="_blank" rel="noreferrer">
                  <MessageCircle className="h-4 w-4" aria-hidden />
                  WhatsApp
                </a>
              </Tombol>
            )}
            {p.phone && (
              <Tombol jenis="kedua" ukuran="padat" asChild>
                <a href={`tel:${p.phone.replace(/[^\d+]/g, '')}`}>
                  <Phone className="h-4 w-4" aria-hidden />
                  Telepon
                </a>
              </Tombol>
            )}
          </div>
        )}
      </Kartu>

      <dl className="grid grid-cols-2 gap-2 sm:grid-cols-4">
        <Angka label="Total belanja" nilai={st?.visit_count ? formatRupiah(st.total_spent) : '—'} />
        <Angka label="Datang" nilai={st?.visit_count ? `${st.visit_count.toLocaleString('id-ID')}×` : 'Belum pernah'} />
        <Angka
          label="Rata-rata"
          nilai={st?.visit_count ? formatRupiah(Math.round(st.total_spent / st.visit_count)) : '—'}
        />
        <Angka
          label="Terakhir datang"
          nilai={st?.last_visit_at ? formatLaluHari(st.last_visit_at, undefined, zona) : '—'}
        />
      </dl>

      {bolehKasbon && (
        <Kartu className="flex flex-col gap-3 p-4 sm:p-5">
          <h2 className="flex items-center gap-2 text-judul-kartu font-semibold text-teks-utama">
            <NotebookPen className="h-5 w-5 text-utama" aria-hidden />
            Kasbon
            {(st?.receivable_overdue ?? 0) > 0 && (
              <span className="rounded-full bg-bahaya-teks/10 px-2 py-0.5 text-keterangan font-semibold text-bahaya-teks">
                lewat jatuh tempo
              </span>
            )}
          </h2>
          {kasbon > 0 ? (
            <RincianKasbon pelanggan={p} hariIni={hariIni} />
          ) : (
            <p className="text-label text-teks-redup">Tidak ada kasbon yang belum lunas.</p>
          )}
        </Kartu>
      )}

      <SeringDibeli id={p.id} />
      {boleh(IZIN.saleCreate) && <RiwayatBelanja id={p.id} />}

      {ubah && <DialogPelanggan awal={p} onTutup={() => setUbah(false)} />}
      <Dialog open={hapus} onOpenChange={(o) => !o && setHapus(false)}>
        <IsiDialog
          judul={`Hapus ${p.name}?`}
          keterangan={
            kasbon > 0
              ? `${p.name} masih punya kasbon ${formatRupiah(kasbon)}. Kasbonnya tetap tercatat dan tetap bisa ditagih dari layar Kasbon.`
              : 'Riwayat belanjanya tetap tersimpan; pelanggan ini tidak bisa dipilih lagi di kasir.'
          }
        >
          <AksiDialog>
            <Tombol jenis="bahaya" memuat={hapusM.isPending} onClick={() => hapusM.mutate()}>
              Ya, Hapus
            </Tombol>
            <Tombol jenis="kedua" onClick={() => setHapus(false)}>
              Batal
            </Tombol>
          </AksiDialog>
        </IsiDialog>
      </Dialog>
    </div>
  )
}

function Angka({ label, nilai }: { label: string; nilai: string }) {
  return (
    <div className="h-full rounded-kartu border border-garis bg-permukaan p-3 shadow-kartu">
      <dt className="text-keterangan text-teks-sekunder">{label}</dt>
      <dd className="mt-0.5 font-bold tabular-nums text-teks-utama">{nilai}</dd>
    </div>
  )
}

/** Lima barang yang paling sering dibeli — bekal menawarkan, dan tanda selera. */
function SeringDibeli({ id }: { id: string }) {
  const q = useQuery({ queryKey: ['pelanggan', id, 'sering-dibeli'], queryFn: () => pelangganApi.seringDibeli(id) })
  if (!q.data?.length) return null
  return (
    <Kartu className="flex flex-col gap-1 p-4 sm:p-5">
      <h2 className="flex items-center gap-2 text-judul-kartu font-semibold text-teks-utama">
        <ShoppingBag className="h-5 w-5 text-utama" aria-hidden />
        Sering dibeli
      </h2>
      <ul className="-mx-1 divide-y divide-garis">
        {q.data.map((b) => (
          <li key={b.product_id} className="flex items-baseline justify-between gap-3 px-1 py-2">
            <span className="min-w-0">
              <span className="block truncate font-medium text-teks-utama">{b.product_name}</span>
              <span className="block text-keterangan text-teks-redup">
                total {formatQtySatuan(b.qty, b.unit_name)} · terakhir {formatLaluHari(b.last_at).toLowerCase()}
              </span>
            </span>
            <span className="shrink-0 text-label font-semibold tabular-nums text-teks-sekunder">{b.times}× beli</span>
          </li>
        ))}
      </ul>
    </Kartu>
  )
}

const PER_HALAMAN = 20

/** "2× Kopi Hitam, 1× Gula Pasir +2 lainnya". */
function ringkasBarang(t: Transaksi): string {
  const items = t.items ?? []
  if (items.length === 0) return '—'
  const tampil = items.slice(0, 2).map((i) => `${formatQty(i.qty.replace('-', ''))}× ${i.product_name}`)
  return items.length > 2 ? `${tampil.join(', ')} +${items.length - 2} lainnya` : tampil.join(', ')
}

/** Riwayat belanja pelanggan (semua toko yang boleh dilihat), terbaru dulu. */
function RiwayatBelanja({ id }: { id: string }) {
  const q = useInfiniteQuery({
    queryKey: ['pelanggan', id, 'belanja'],
    queryFn: ({ pageParam }) => pelangganApi.riwayatBelanja(id, pageParam, PER_HALAMAN),
    initialPageParam: 1,
    getNextPageParam: (akhir, semua) => (semua.length * PER_HALAMAN < akhir.total ? semua.length + 1 : undefined),
  })
  const daftar = q.data?.pages.flatMap((h) => h.data) ?? []
  return (
    <Kartu className="flex flex-col gap-1 p-4 sm:p-5">
      <h2 className="text-judul-kartu font-semibold text-teks-utama">Riwayat belanja</h2>
      {q.isLoading ? (
        <KerangkaBaris jumlah={3} />
      ) : daftar.length === 0 ? (
        <p className="py-4 text-label text-teks-redup">Belum pernah belanja sebagai pelanggan ini.</p>
      ) : (
        <>
          <ul className="-mx-1 divide-y divide-garis">
            {daftar.map((t) => {
              const kasbon = (t.payments ?? []).some((b) => b.method === 'credit')
              const batal = t.status === 'canceled' || !!t.voided_at
              const retur = t.status === 'returned'
              return (
                <li key={t.id}>
                  <Link
                    to={`/kasir/riwayat?tanggal=${t.business_date}&cari=${encodeURIComponent(t.receipt_no)}`}
                    className="flex min-h-12 items-center gap-3 rounded-kontrol px-1 py-2.5 hover:bg-permukaan-2/60"
                  >
                    <span className="min-w-0 flex-1">
                      <span className="flex flex-wrap items-center gap-1.5 text-label font-medium text-teks-utama">
                        {formatTanggalJam(t.occurred_at)}
                        <span className="font-normal tabular-nums text-teks-redup">{t.receipt_no}</span>
                        {kasbon && !batal && (
                          <span className="rounded-full bg-permukaan-2 px-1.5 text-keterangan font-semibold text-jingga-700">Kasbon</span>
                        )}
                        {retur && (
                          <span className="rounded-full bg-permukaan-2 px-1.5 text-keterangan font-semibold text-teks-sekunder">Retur</span>
                        )}
                        {batal && (
                          <span className="rounded-full bg-bahaya-teks/10 px-1.5 text-keterangan font-semibold text-bahaya-teks">Batal</span>
                        )}
                      </span>
                      <span className="block truncate text-keterangan text-teks-redup">
                        {ringkasBarang(t)}
                        {!kasbon && t.payments?.[0] ? ` · ${namaMetode(t.payments[0].method)}` : ''}
                      </span>
                    </span>
                    <span className={cn('shrink-0 font-semibold tabular-nums', batal ? 'text-teks-redup line-through' : 'text-teks-utama')}>
                      {formatRupiah(t.total)}
                    </span>
                  </Link>
                </li>
              )
            })}
          </ul>
          {q.hasNextPage && (
            <Tombol jenis="kedua" lebarPenuh memuat={q.isFetchingNextPage} onClick={() => q.fetchNextPage()}>
              Muat lebih banyak
            </Tombol>
          )}
        </>
      )}
    </Kartu>
  )
}
