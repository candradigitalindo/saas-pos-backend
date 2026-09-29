import { useEffect, useRef, useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowLeft, Copy, MessageCircle, PackagePlus, Pencil, Phone, Send, Trash2 } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { StepperJumlah } from '@/bersama/ui/stepper-jumlah'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { KeadaanGagal } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka, KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { IZIN } from '@/lib/izin'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatQty, formatQtySatuan } from '@/bersama/util/desimal'
import { formatLaluHari } from '@/bersama/util/tanggal'
import { inisialNama } from '@/bersama/util/inisial'
import { kelasAvatar } from '@/bersama/util/warna-kategori'
import { cn } from '@/bersama/util/cn'
import type { Pemasok, SaldoStok } from '@/bersama/tipe/katalog'
import { nomorWA, tautanWA } from '@/fitur/kasir/struk-wa'
import { stokApi } from '@/fitur/stok/api'
import { cukupHari, keadaanSaldo, saranBeli, teksCukup } from '@/fitur/stok/keadaan-stok'
import { BarisNota } from '@/fitur/stok/komponen/dialog-riwayat-masuk'
import { pemasokApi, type BarangPemasok } from '../api'
import { DialogPemasok } from '../komponen/dialog-pemasok'
import { jumlahSatuanBeli, teksPesanan } from '../pesanan'

/** Barang dianggap perlu dipesan bila di bawah batas / habis dalam seminggu. */
const HARI_PESAN = 7

/**
 * Satu pemasok: kontak (WhatsApp, telepon), angka belanja di toko ini, PESAN
 * LAGI, dan riwayat belanja.
 *
 * "Pesan lagi" berisi barang yang biasa dibeli dari pemasok ini. Yang perlu
 * dibeli (di bawah batas minimum atau habis dalam seminggu menurut laju jual)
 * sudah tercentang dengan jumlah sarannya — dalam SATUAN BELI terakhir
 * (dus bila biasa per dus, dibulatkan ke dus utuh). Pesanannya dikirim
 * sebagai teks WhatsApp ke nomor pemasok; tanpa nomor, teksnya bisa disalin.
 */
export function HalamanRincianPemasok() {
  const { id } = useParams()
  const { tokoAktif, boleh } = useSesi()
  const navigate = useNavigate()
  const qc = useQueryClient()
  const toast = useToast()
  const [ubah, setUbah] = useState(false)
  const [hapus, setHapus] = useState(false)

  const q = useQuery({ queryKey: ['pemasok', id], queryFn: () => pemasokApi.satu(id!), enabled: !!id })
  const stat = useQuery({
    queryKey: ['pemasok', 'statistik', tokoAktif],
    queryFn: () => pemasokApi.statistik(tokoAktif!),
    enabled: !!tokoAktif && boleh(IZIN.stockView),
  })
  const s = stat.data?.find((x) => x.supplier_id === id)

  const hapusM = useMutation({
    mutationFn: () => pemasokApi.hapus(id!),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pemasok'] })
      qc.invalidateQueries({ queryKey: ['katalog-pemasok'] })
      toast.berhasil(`${q.data?.name} dihapus. Riwayat belanjanya tetap tersimpan.`)
      navigate('/pemasok', { replace: true })
    },
    onError: (e) => toast.gagal(e instanceof GalatAPI ? e.pesan : 'Pemasok belum bisa dihapus.'),
  })

  if (q.isLoading) return <Kerangka className="h-64 w-full max-w-4xl rounded-kartu" />
  if (q.isError || !q.data) {
    return (
      <KeadaanGagal
        pesan={q.error instanceof GalatAPI ? q.error.pesan : 'Pemasok tidak ditemukan.'}
        onCobaLagi={() => q.refetch()}
      />
    )
  }
  const p = q.data
  const wa = p.phone ? nomorWA(p.phone) : null

  return (
    <div className="flex w-full max-w-4xl flex-col gap-4">
      <Link to="/pemasok" className="flex min-h-9 items-center gap-1 self-start text-label font-medium text-utama hover:underline">
        <ArrowLeft className="h-4 w-4" aria-hidden />
        Pemasok
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
            {p.note && <p className="mt-1 text-keterangan italic text-teks-sekunder">“{p.note}”</p>}
          </div>
          {boleh(IZIN.productEdit) && (
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
          {boleh(IZIN.stockAdjust) && (
            <Tombol jenis="kedua" ukuran="padat" asChild>
              <Link to={`/stok/masuk?pemasok=${p.id}`}>
                <PackagePlus className="h-4 w-4" aria-hidden />
                Catat barang masuk
              </Link>
            </Tombol>
          )}
        </div>
      </Kartu>

      {boleh(IZIN.stockView) && (
        <dl className="grid grid-cols-3 gap-2">
          <Angka label="Belanja 30 hari" nilai={s ? formatRupiah(s.spent_30d) : '—'} />
          <Angka label="Nota belanja" nilai={s ? s.purchase_count.toLocaleString('id-ID') : '0'} />
          <Link to="/stok/utang" className="rounded-kartu focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-utama">
            <Angka
              label="Utang"
              nilai={s?.outstanding ? formatRupiah(s.outstanding) : 'Lunas'}
              nada={!s?.outstanding ? undefined : s.overdue_count > 0 ? 'bahaya' : 'jingga'}
            />
          </Link>
        </dl>
      )}

      {boleh(IZIN.stockView) && <PesanLagi pemasok={p} />}
      {boleh(IZIN.stockView) && <RiwayatBelanja id={p.id} />}

      <DialogPemasok terbuka={ubah} pemasok={p} onTutup={() => setUbah(false)} />
      <Dialog open={hapus} onOpenChange={(o) => !o && setHapus(false)}>
        <IsiDialog judul={`Hapus ${p.name}?`} keterangan="Riwayat belanja & utangnya tetap tersimpan; pemasok ini tidak bisa dipilih lagi di Barang Masuk.">
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

function Angka({ label, nilai, nada }: { label: string; nilai: string; nada?: 'bahaya' | 'jingga' }) {
  return (
    <div className="h-full rounded-kartu border border-garis bg-permukaan p-3 shadow-kartu">
      <dt className="text-keterangan text-teks-sekunder">{label}</dt>
      <dd
        className={cn(
          'mt-0.5 font-bold tabular-nums',
          nada === 'bahaya' ? 'text-bahaya-teks' : nada === 'jingga' ? 'text-jingga-700' : 'text-teks-utama',
        )}
      >
        {nilai}
      </dd>
    </div>
  )
}

/** Satu barang di "Pesan lagi", lengkap dengan saldo & saran jumlah. */
interface Kandidat {
  b: BarangPemasok
  s?: SaldoStok
  isi: number
  perlu: boolean
  saran: number // satuan beli
}

function PesanLagi({ pemasok: p }: { pemasok: Pemasok }) {
  const { tokoAktif, rincianToko, profil } = useSesi()
  const toast = useToast()
  const barang = useQuery({
    queryKey: ['pemasok', p.id, 'barang', tokoAktif],
    queryFn: () => pemasokApi.barang(p.id, tokoAktif!),
    enabled: !!tokoAktif,
  })
  const ids = (barang.data ?? []).map((b) => b.product_id)
  const saldo = useQuery({
    queryKey: ['stok', tokoAktif, 'pemasok', ids.join(',')],
    queryFn: () => stokApi.saldo(tokoAktif!, false, 1, 100, { produk: ids }),
    enabled: !!tokoAktif && ids.length > 0,
  })
  const perSaldo = new Map((saldo.data?.data ?? []).map((x) => [x.product_id, x]))

  const kandidat: Kandidat[] = (barang.data ?? []).map((b) => {
    const s = perSaldo.get(b.product_id)
    const isi = Number.parseFloat(b.unit_conversion) || 1
    const hari = s ? cukupHari(s) : null
    const perlu = !!s && (keadaanSaldo(s) !== 'safe' || (hari !== null && hari <= HARI_PESAN))
    return { b, s, isi, perlu, saran: s ? jumlahSatuanBeli(saranBeli(s), isi) : 1 }
  })

  // Pilihan: product_id → jumlah (satuan beli). Diisi SEKALI saat data siap:
  // yang perlu dibeli tercentang dengan jumlah sarannya.
  const [pilih, setPilih] = useState<Map<string, number>>(new Map())
  const sudahDiisi = useRef(false)
  useEffect(() => {
    if (sudahDiisi.current || !barang.data || (ids.length > 0 && !saldo.data)) return
    sudahDiisi.current = true
    setPilih(new Map(kandidat.filter((k) => k.perlu).map((k) => [k.b.product_id, k.saran])))
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [barang.data, saldo.data])

  if (barang.isLoading) return <Kerangka className="h-48 w-full rounded-kartu" />
  if (!kandidat.length) return null

  const namaToko = rincianToko?.name || profil?.tenant.business_name || 'toko kami'
  const dipesan = kandidat.filter((k) => pilih.has(k.b.product_id))
  const teks = teksPesanan(
    p.name,
    namaToko,
    dipesan.map((k) => ({
      nama: k.b.product_name,
      jumlah: pilih.get(k.b.product_id)!,
      satuan: k.b.unit_name || k.b.base_unit_name,
      isi: k.isi,
      satuanDasar: k.b.base_unit_name,
    })),
  )
  const wa = p.phone ? nomorWA(p.phone) : null

  return (
    <Kartu className="flex flex-col gap-3 p-4 sm:p-5">
      <div>
        <h2 className="flex items-center gap-2 text-judul-kartu font-semibold text-teks-utama">
          <Send className="h-5 w-5 text-utama" aria-hidden />
          Pesan lagi
        </h2>
        <p className="text-keterangan text-teks-redup">
          Barang yang biasa dibeli dari {p.name}. Yang perlu dibeli sudah tercentang dengan jumlah saran untuk ±2 minggu.
        </p>
      </div>

      <ul className="-mx-1 divide-y divide-garis">
        {kandidat.map((k) => {
          const dipilih = pilih.has(k.b.product_id)
          const satuan = k.b.unit_name || k.b.base_unit_name
          const kd = k.s ? keadaanSaldo(k.s) : null
          const hari = k.s ? cukupHari(k.s) : null
          return (
            <li key={k.b.product_id} className={cn('flex flex-col gap-2 px-1 py-2.5', dipilih && 'bg-sorot/40')}>
              <label className="flex cursor-pointer items-start gap-3">
                <input
                  type="checkbox"
                  checked={dipilih}
                  onChange={(e) =>
                    setPilih((m) => {
                      const baru = new Map(m)
                      if (e.target.checked) baru.set(k.b.product_id, k.saran)
                      else baru.delete(k.b.product_id)
                      return baru
                    })
                  }
                  className="mt-0.5 h-5 w-5 shrink-0 accent-utama"
                />
                <span className="min-w-0 flex-1">
                  <span className="block font-medium text-teks-utama">{k.b.product_name}</span>
                  <span className="block text-keterangan text-teks-redup">
                    {k.s && (
                      <span
                        className={cn(
                          kd === 'negative' || kd === 'out'
                            ? 'font-semibold text-bahaya-teks'
                            : k.perlu
                              ? 'font-semibold text-jingga-700'
                              : 'text-teks-sekunder',
                        )}
                      >
                        {kd === 'negative' || kd === 'out' ? 'habis' : `sisa ${formatQtySatuan(k.s.qty, k.b.base_unit_name)}`}
                        {kd === 'safe' && hari !== null && hari <= HARI_PESAN && ` · ${teksCukup(hari)}`}
                      </span>
                    )}
                    {k.s && ' · '}
                    terakhir {formatRupiah(k.b.unit_cost)}/{satuan} · {formatLaluHari(k.b.last_bought_at).toLowerCase()}
                  </span>
                </span>
              </label>
              {dipilih && (
                <div className="pl-8">
                  <StepperJumlah
                    nilai={String(pilih.get(k.b.product_id))}
                    onNilai={(q) => setPilih((m) => new Map(m).set(k.b.product_id, Number(q)))}
                    satuan={k.isi > 1 ? `${satuan} (isi ${formatQty(String(k.isi))})` : satuan}
                    minimal="1"
                    label={`Jumlah pesan ${k.b.product_name}`}
                  />
                </div>
              )}
            </li>
          )
        })}
      </ul>

      {dipesan.length > 0 && (
        <details className="rounded-kontrol bg-permukaan-2/60 px-3 py-2 text-label">
          <summary className="cursor-pointer font-medium text-teks-sekunder">Lihat teks pesanan</summary>
          <pre className="mt-2 whitespace-pre-wrap font-sans text-teks-utama">{teks}</pre>
        </details>
      )}

      <div className="flex flex-col gap-2 sm:flex-row">
        {wa ? (
          <Tombol asChild className={cn(!dipesan.length && 'pointer-events-none opacity-50')} lebarPenuh>
            <a href={tautanWA(wa, teks)} target="_blank" rel="noreferrer" aria-disabled={!dipesan.length}>
              <MessageCircle className="h-5 w-5" aria-hidden />
              Kirim lewat WhatsApp ({dipesan.length})
            </a>
          </Tombol>
        ) : (
          <p className="flex-1 rounded-kontrol bg-permukaan-2 px-3 py-2 text-keterangan text-teks-sekunder">
            Tambahkan nomor WhatsApp {p.name} (tombol Ubah) untuk mengirim pesanan langsung. Sementara, salin teksnya.
          </p>
        )}
        <Tombol
          jenis="kedua"
          disabled={!dipesan.length}
          onClick={() =>
            navigator.clipboard
              .writeText(teks)
              .then(() => toast.berhasil('Teks pesanan disalin.'))
              .catch(() => toast.gagal('Tidak bisa menyalin — buka "Lihat teks pesanan".'))
          }
          className="shrink-0"
        >
          <Copy className="h-5 w-5" aria-hidden />
          Salin teks
        </Tombol>
      </div>
    </Kartu>
  )
}

const PER_HALAMAN = 20

function RiwayatBelanja({ id }: { id: string }) {
  const { tokoAktif } = useSesi()
  const [buka, setBuka] = useState<string | null>(null)
  const q = useInfiniteQuery({
    queryKey: ['pembelian', tokoAktif, 'pemasok', id],
    queryFn: ({ pageParam }) => stokApi.daftarPembelian(tokoAktif!, pageParam, PER_HALAMAN, { pemasok: id }),
    initialPageParam: 1,
    getNextPageParam: (akhir, semua) => (semua.length * PER_HALAMAN < akhir.total ? semua.length + 1 : undefined),
    enabled: !!tokoAktif,
  })
  const daftar = q.data?.pages.flatMap((h) => h.data) ?? []
  return (
    <Kartu className="flex flex-col gap-1 p-4 sm:p-5">
      <h2 className="text-judul-kartu font-semibold text-teks-utama">Riwayat belanja</h2>
      {q.isLoading ? (
        <KerangkaBaris jumlah={3} />
      ) : daftar.length === 0 ? (
        <p className="py-4 text-label text-teks-redup">Belum ada belanja dari pemasok ini di toko ini.</p>
      ) : (
        <>
          <ul className="-mx-1 divide-y divide-garis">
            {daftar.map((n) => (
              <li key={n.id}>
                <BarisNota p={n} terbuka={buka === n.id} onKlik={() => setBuka(buka === n.id ? null : n.id)} tanpaPemasok />
              </li>
            ))}
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
