import { useState } from 'react'
import { Link } from 'react-router-dom'
import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight, MessageCircle, Plus, Search, Users } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatLaluHari } from '@/bersama/util/tanggal'
import { inisialNama } from '@/bersama/util/inisial'
import { kelasAvatar } from '@/bersama/util/warna-kategori'
import { cn } from '@/bersama/util/cn'
import type { Pelanggan } from '@/bersama/tipe/pos'
import { nomorWA } from '@/fitur/kasir/struk-wa'
import { pelangganApi } from '../api'
import { DialogPelanggan } from '../komponen/dialog-pelanggan'

/** Pelanggan per halaman. */
const PER_HALAMAN = 25

/**
 * Daftar pelanggan. Dipakai kasir untuk kasbon dan pemilik untuk menagih.
 *
 * Setiap baris kini menjawab pertanyaan yang biasanya ditanyakan tentang
 * seorang pelanggan: seberapa sering ia datang, berapa total belanjanya,
 * kapan terakhir datang, dan berapa kasbonnya yang belum lunas — seperti
 * daftar pelanggan di aplikasi SaaS kasir pada umumnya. Dulu hanya nama,
 * nomor HP, dan batas kasbon.
 *
 * Angka belanja bersih (retur mengurangi, batal tidak ikut) dan dihitung
 * server. Sisa kasbon hanya dikirim kepada yang berhak mengurus kasbon, jadi
 * kolomnya muncul hanya bila datanya ada — merah bila ada yang lewat jatuh
 * tempo. Mengetuk pelanggan membuka halamannya (belanja, kasbon, tagih).
 */
export function HalamanPelanggan() {
  const { boleh, rincianToko, tokoAktif } = useSesi()
  const [cari, setCari] = useState('')
  const [halaman, setHalaman] = useState(1)
  const [formUntuk, setFormUntuk] = useState<Pelanggan | 'baru' | null>(null)

  const q = useQuery({
    queryKey: ['pelanggan', cari, halaman, tokoAktif],
    queryFn: () => pelangganApi.daftar(cari || undefined, halaman, PER_HALAMAN, tokoAktif ?? undefined),
    staleTime: 30_000,
    placeholderData: keepPreviousData,
  })

  const daftar = q.data?.data ?? []
  const total = q.data?.total ?? 0
  const jumlahHalaman = Math.max(1, Math.ceil(total / PER_HALAMAN))
  const adaKasbon = daftar.some((p) => p.stats?.receivable_outstanding !== undefined)
  const bolehUbah = boleh(IZIN.customerEdit)
  const zona = rincianToko?.timezone

  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-judul font-bold text-teks-utama">Pelanggan</h1>
          {q.data && (
            <p className="text-label text-teks-sekunder">
              {total.toLocaleString('id-ID')} pelanggan{cari ? ' cocok' : ''}
            </p>
          )}
        </div>
        {bolehUbah && (
          <Tombol onClick={() => setFormUntuk('baru')}>
            <Plus className="h-5 w-5" aria-hidden />
            Tambah Pelanggan
          </Tombol>
        )}
      </header>

      <div className="relative">
        <Search
          className="pointer-events-none absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-teks-redup"
          aria-hidden
        />
        <input
          type="search"
          value={cari}
          onChange={(e) => {
            setCari(e.target.value)
            setHalaman(1)
          }}
          placeholder="Cari nama atau nomor HP…"
          aria-label="Cari pelanggan"
          className="h-12 w-full rounded-kontrol border border-garis bg-permukaan pl-10 pr-3 text-isi text-teks-utama placeholder:text-teks-redup"
        />
      </div>

      {q.isLoading ? (
        <KerangkaBaris jumlah={5} />
      ) : q.isError ? (
        <KeadaanGagal
          pesan={q.error instanceof GalatAPI ? q.error.pesan : 'Daftar pelanggan belum bisa dimuat.'}
          onCobaLagi={() => q.refetch()}
        />
      ) : daftar.length === 0 ? (
        <KeadaanKosong
          ikon={Users}
          judul={cari ? 'Pelanggan tidak ditemukan' : 'Belum ada pelanggan'}
          penjelasan={
            cari
              ? `Tidak ada pelanggan bernama "${cari}".`
              : 'Simpan data pelanggan langganan supaya bisa mencatat kasbon dan menagihnya nanti.'
          }
          aksi={
            bolehUbah && !cari
              ? { label: 'Tambah Pelanggan', onKlik: () => setFormUntuk('baru') }
              : undefined
          }
        />
      ) : (
        <>
          {/* HP & tablet: kartu bertumpuk. */}
          <ul className="flex flex-col gap-2 lg:hidden">
            {daftar.map((p) => (
              <li key={p.id}>
                <Kartu className="relative flex items-center gap-3 p-4 transition-colors hover:border-utama/40">
                  <Avatar pelanggan={p} />
                  <div className="min-w-0 flex-1">
                    {/* Seluruh kartu membuka halaman pelanggan (tautan terentang). */}
                    <Link
                      to={`/pelanggan/${p.id}`}
                      className="block truncate font-semibold text-teks-utama after:absolute after:inset-0 after:rounded-kartu focus-visible:outline-none focus-visible:after:ring-2 focus-visible:after:ring-utama"
                    >
                      {p.name}
                    </Link>
                    {/* Boleh membungkus: dipotong, "9 hari lalu" jadi "9 har…". */}
                    <p className="text-keterangan text-teks-redup">{ringkasBelanja(p, zona)}</p>
                    <SisaKasbon pelanggan={p} />
                  </div>
                  <TombolWA pelanggan={p} />
                </Kartu>
              </li>
            ))}
          </ul>

          {/* Layar lebar: tabel. */}
          <div className="hidden overflow-hidden rounded-kartu border border-garis bg-permukaan shadow-kartu lg:block">
            <table className="w-full">
              <thead>
                <tr className="border-b border-garis bg-permukaan-2/60 text-left text-label text-teks-sekunder">
                  <th scope="col" className="px-4 py-3 font-medium">Pelanggan</th>
                  <th scope="col" className="px-3 py-3 text-right font-medium">Datang</th>
                  <th scope="col" className="px-3 py-3 text-right font-medium">Total belanja</th>
                  <th scope="col" className="px-3 py-3 font-medium">Terakhir datang</th>
                  {adaKasbon && (
                    <th scope="col" className="px-3 py-3 text-right font-medium">Sisa kasbon</th>
                  )}
                  <th scope="col" className="px-4 py-3">
                    <span className="sr-only">Aksi</span>
                  </th>
                </tr>
              </thead>
              <tbody className="divide-y divide-garis">
                {daftar.map((p) => (
                  <tr key={p.id} className="hover:bg-permukaan-2/50">
                    <td className="px-4 py-2.5">
                      <div className="flex items-center gap-3">
                        <Avatar pelanggan={p} />
                        <div className="min-w-0">
                          <Link
                            to={`/pelanggan/${p.id}`}
                            className="font-medium text-teks-utama underline-offset-4 hover:text-utama hover:underline"
                          >
                            {p.name}
                          </Link>
                          <p className="text-keterangan tabular-nums text-teks-redup">
                            {p.phone || 'Tanpa nomor HP'}
                          </p>
                        </div>
                      </div>
                    </td>
                    {/* Belum pernah belanja: "—", bukan "0×" dan "Rp 0" — dua
                        angka nol per baris hanya menambah derau di tabel. */}
                    <td className="px-3 py-2.5 text-right tabular-nums text-teks-sekunder">
                      {p.stats?.visit_count ? `${p.stats.visit_count.toLocaleString('id-ID')}×` : '—'}
                    </td>
                    <td className="px-3 py-2.5 text-right font-medium tabular-nums text-teks-utama">
                      {p.stats?.visit_count ? formatRupiah(p.stats.total_spent) : <span className="font-normal text-teks-redup">—</span>}
                    </td>
                    <td className="px-3 py-2.5 text-teks-sekunder">
                      {p.stats?.last_visit_at ? formatLaluHari(p.stats.last_visit_at, undefined, zona) : 'Belum pernah'}
                    </td>
                    {adaKasbon && (
                      <td className="px-3 py-2.5 text-right tabular-nums">
                        <KasbonSel pelanggan={p} />
                      </td>
                    )}
                    <td className="px-4 py-2.5">
                      <div className="flex items-center justify-end gap-1">
                        <TombolWA pelanggan={p} />
                        {bolehUbah && (
                          <Tombol jenis="teks" ukuran="padat" onClick={() => setFormUntuk(p)} aria-label={`Ubah ${p.name}`}>
                            Ubah
                          </Tombol>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>

          {jumlahHalaman > 1 && (
            <nav aria-label="Halaman daftar pelanggan" className="flex flex-wrap items-center justify-between gap-3">
              <p className="text-label tabular-nums text-teks-sekunder">
                Halaman {halaman} dari {jumlahHalaman}
              </p>
              <div className="flex gap-2">
                <Tombol jenis="kedua" ukuran="padat" disabled={halaman <= 1} onClick={() => setHalaman((h) => h - 1)}>
                  <ChevronLeft className="h-4 w-4" aria-hidden />
                  Sebelumnya
                </Tombol>
                <Tombol
                  jenis="kedua"
                  ukuran="padat"
                  disabled={halaman >= jumlahHalaman}
                  onClick={() => setHalaman((h) => h + 1)}
                >
                  Berikutnya
                  <ChevronRight className="h-4 w-4" aria-hidden />
                </Tombol>
              </div>
            </nav>
          )}
        </>
      )}

      {formUntuk && (
        <DialogPelanggan
          awal={formUntuk === 'baru' ? null : formUntuk}
          onTutup={() => setFormUntuk(null)}
        />
      )}
    </div>
  )
}

/** Avatar inisial berwarna — warnanya tetap sama untuk pelanggan yang sama. */
function Avatar({ pelanggan }: { pelanggan: Pelanggan }) {
  return (
    <span
      aria-hidden
      className={cn(
        'flex h-10 w-10 shrink-0 items-center justify-center rounded-full text-label font-bold',
        kelasAvatar(pelanggan.id),
      )}
    >
      {inisialNama(pelanggan.name)}
    </span>
  )
}

/** "12× datang · Rp 450.000 · 9 hari lalu" — ringkasan satu baris untuk HP. */
function ringkasBelanja(p: Pelanggan, zona?: string): string {
  const st = p.stats
  if (!st || st.visit_count === 0) return `${p.phone || 'Tanpa nomor HP'} · belum pernah belanja`
  return [
    `${st.visit_count}× datang`,
    formatRupiah(st.total_spent),
    st.last_visit_at ? formatLaluHari(st.last_visit_at, undefined, zona) : null,
  ]
    .filter(Boolean)
    .join(' · ')
}

/**
 * Sisa kasbon sebagai tautan ke halaman pelanggan — tempat menagihnya. Nol
 * ditulis redup: "tidak berutang" bukan berita. Merah bila ada yang lewat
 * jatuh tempo.
 */
function KasbonSel({ pelanggan }: { pelanggan: Pelanggan }) {
  const sisa = pelanggan.stats?.receivable_outstanding ?? 0
  if (sisa <= 0) return <span className="text-teks-redup">—</span>
  const lewat = (pelanggan.stats?.receivable_overdue ?? 0) > 0
  return (
    <Link
      to={`/pelanggan/${pelanggan.id}`}
      className={cn('font-semibold underline-offset-4 hover:underline', lewat ? 'text-bahaya-teks' : 'text-jingga-700')}
      title={lewat ? `${formatRupiah(pelanggan.stats!.receivable_overdue!)} lewat jatuh tempo` : undefined}
    >
      {formatRupiah(sisa)}
    </Link>
  )
}

/** Baris sisa kasbon di kartu HP — hanya bila memang ada. */
function SisaKasbon({ pelanggan }: { pelanggan: Pelanggan }) {
  const sisa = pelanggan.stats?.receivable_outstanding ?? 0
  if (sisa <= 0) return null
  const lewat = pelanggan.stats?.receivable_overdue ?? 0
  return (
    <p className={cn('text-keterangan font-semibold tabular-nums', lewat > 0 ? 'text-bahaya-teks' : 'text-jingga-700')}>
      Kasbon {formatRupiah(sisa)}
      {lewat > 0 && ' · lewat jatuh tempo'}
    </p>
  )
}

/** Tombol WhatsApp bulat — hanya bila nomornya memang nomor WhatsApp. */
function TombolWA({ pelanggan: p }: { pelanggan: Pelanggan }) {
  const wa = p.phone ? nomorWA(p.phone) : null
  if (!wa) return null
  return (
    <a
      href={`https://wa.me/${wa}`}
      target="_blank"
      rel="noreferrer"
      aria-label={`WhatsApp ${p.name}`}
      className="relative z-10 flex h-10 w-10 shrink-0 items-center justify-center rounded-full bg-sorot text-hijau-800 hover:brightness-95"
    >
      <MessageCircle className="h-5 w-5" aria-hidden />
    </a>
  )
}
