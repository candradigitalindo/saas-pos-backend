import { useState } from 'react'
import { Link } from 'react-router-dom'
import { keepPreviousData, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ChevronLeft, ChevronRight, Plus, Search, Users } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useDaftarHarga } from '@/bersama/hooks/use-katalog'
import { GalatAPI } from '@/lib/api-client'
import { galatKolom } from '@/lib/galat-kolom'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatLaluHari } from '@/bersama/util/tanggal'
import { inisialNama } from '@/bersama/util/inisial'
import { kelasAvatar } from '@/bersama/util/warna-kategori'
import { cn } from '@/bersama/util/cn'
import type { Pelanggan } from '@/bersama/tipe/pos'
import { pelangganApi } from '../api'

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
 * kolomnya muncul hanya bila datanya ada.
 */
export function HalamanPelanggan() {
  const { boleh, rincianToko } = useSesi()
  const [cari, setCari] = useState('')
  const [halaman, setHalaman] = useState(1)
  const [formUntuk, setFormUntuk] = useState<Pelanggan | 'baru' | null>(null)

  const q = useQuery({
    queryKey: ['pelanggan', cari, halaman],
    queryFn: () => pelangganApi.daftar(cari || undefined, halaman, PER_HALAMAN),
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
                <Kartu className="flex items-center gap-3 p-4">
                  <Avatar pelanggan={p} />
                  <div className="min-w-0 flex-1">
                    <p className="truncate font-semibold text-teks-utama">{p.name}</p>
                    {/* Boleh membungkus: dipotong, "9 hari lalu" jadi "9 har…". */}
                    <p className="text-keterangan text-teks-redup">{ringkasBelanja(p, zona)}</p>
                    <SisaKasbon pelanggan={p} />
                  </div>
                  {bolehUbah && (
                    <Tombol jenis="kedua" ukuran="padat" onClick={() => setFormUntuk(p)}>
                      Ubah
                    </Tombol>
                  )}
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
                          <p className="font-medium text-teks-utama">{p.name}</p>
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
                    <td className="px-4 py-2.5 text-right">
                      {bolehUbah && (
                        <Tombol jenis="teks" ukuran="padat" onClick={() => setFormUntuk(p)} aria-label={`Ubah ${p.name}`}>
                          Ubah
                        </Tombol>
                      )}
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

function DialogPelanggan({
  awal,
  onTutup,
}: {
  awal: Pelanggan | null
  onTutup: () => void
}) {
  const toast = useToast()
  const qc = useQueryClient()
  const [nama, setNama] = useState(awal?.name ?? '')
  const [hp, setHp] = useState(awal?.phone ?? '')
  const [batas, setBatas] = useState(awal?.credit_limit ?? 0)
  const [daftar, setDaftar] = useState(awal?.price_list_id ?? '')
  const daftarHarga = useDaftarHarga()
  const [kolomGalat, setKolomGalat] = useState<Record<string, string>>({})
  const [galat, setGalat] = useState<string | null>(null)

  const simpan = useMutation({
    mutationFn: () => {
      const isi = {
        name: nama.trim(),
        phone: hp.trim() || undefined,
        credit_limit: batas,
        price_list_id: daftar,
      }
      return awal ? pelangganApi.ubah(awal.id, isi) : pelangganApi.buat(isi)
    },
    onSuccess: (p) => {
      qc.invalidateQueries({ queryKey: ['pelanggan'] })
      toast.berhasil(awal ? `${p.name} diperbarui.` : `${p.name} ditambahkan.`)
      onTutup()
    },
    onError: (e) => {
      if (e instanceof GalatAPI) {
        setKolomGalat(e.kolom)
        setGalat(e.status === 422 ? null : e.pesan)
      } else setGalat('Terjadi kesalahan. Coba lagi.')
    },
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !simpan.isPending && onTutup()}>
      <IsiDialog judul={awal ? 'Ubah Pelanggan' : 'Tambah Pelanggan'}>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            setGalat(null)
            setKolomGalat({})
            simpan.mutate()
          }}
          className="flex flex-col gap-4"
          noValidate
        >
          <Kolom
            label="Nama"
            value={nama}
            onChange={(e) => setNama(e.target.value)}
            galat={galatKolom(kolomGalat, 'name')}
            autoFocus
            required
          />
          <Kolom
            label="Nomor HP"
            type="tel"
            inputMode="tel"
            value={hp}
            onChange={(e) => setHp(e.target.value)}
            bantuan="Boleh dikosongkan."
            galat={galatKolom(kolomGalat, 'phone')}
          />
          <KolomUang
            label="Batas kasbon"
            nilai={batas}
            onNilai={setBatas}
            bantuan="Utang pelanggan tidak boleh melebihi angka ini. Isi Rp 0 bila tidak dibatasi."
            galat={galatKolom(kolomGalat, 'credit_limit')}
          />
          {/* Hanya tampil bila toko sudah membuat daftar harga khusus. */}
          {(daftarHarga.data?.length ?? 0) > 0 && (
            <Pilihan
              label="Daftar harga"
              value={daftar}
              onChange={(e) => setDaftar(e.target.value)}
              bantuan="Di kasir, pelanggan ini mendapat harga khusus daftarnya."
              galat={galatKolom(kolomGalat, 'price_list_id')}
            >
              <option value="">Harga umum</option>
              {daftarHarga.data!.map((d) => (
                <option key={d.id} value={d.id}>
                  {d.name}
                </option>
              ))}
            </Pilihan>
          )}

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <AksiDialog>
            <Tombol type="submit" memuat={simpan.isPending} disabled={!nama.trim()}>
              Simpan
            </Tombol>
            <Tombol jenis="kedua" onClick={onTutup} disabled={simpan.isPending}>
              Batal
            </Tombol>
          </AksiDialog>
        </form>
      </IsiDialog>
    </Dialog>
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
 * Sisa kasbon sebagai tautan ke halaman Kasbon — tempat menagihnya. Nol ditulis
 * redup: "tidak berutang" bukan berita.
 */
function KasbonSel({ pelanggan }: { pelanggan: Pelanggan }) {
  const sisa = pelanggan.stats?.receivable_outstanding ?? 0
  if (sisa <= 0) return <span className="text-teks-redup">—</span>
  return (
    <Link to="/kasbon" className="font-semibold text-jingga-700 underline-offset-4 hover:underline">
      {formatRupiah(sisa)}
    </Link>
  )
}

/** Baris sisa kasbon di kartu HP — hanya bila memang ada. */
function SisaKasbon({ pelanggan }: { pelanggan: Pelanggan }) {
  const sisa = pelanggan.stats?.receivable_outstanding ?? 0
  if (sisa <= 0) return null
  return (
    <p className="text-keterangan font-semibold tabular-nums text-jingga-700">
      Kasbon {formatRupiah(sisa)}
    </p>
  )
}
