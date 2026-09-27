import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { IZIN } from '@/lib/izin'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ArrowRight, Plus, Store, Trash2, Truck } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { StepperJumlah } from '@/bersama/ui/stepper-jumlah'
import { Tombol } from '@/bersama/ui/tombol'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { api, GalatAPI, type Halaman } from '@/lib/api-client'
import { formatQtySatuan } from '@/bersama/util/desimal'
import { formatTanggal } from '@/bersama/util/tanggal'
import type { Produk } from '@/bersama/tipe/katalog'
import type { Toko } from '@/bersama/tipe/organisasi'
import { PemilihBarang } from '../komponen/pemilih-barang'
import { stokApi, type Transfer } from '../api'

interface BarisKirim {
  produk: Produk
  qty: string
}

/**
 * Kirim barang antar toko.
 *
 * Tiga keadaan yang ditampilkan apa adanya: dibuat → dikirim → diterima. Stok
 * baru benar-benar berpindah saat toko tujuan MENERIMA, jadi barang yang masih
 * di jalan tidak boleh terlihat seolah sudah sampai.
 */
export function HalamanTransfer() {
  const { tokoAktif, boleh } = useSesi()
  const navigate = useNavigate()
  const toast = useToast()
  const qc = useQueryClient()

  const [keToko, setKeToko] = useState('')
  const [catatan, setCatatan] = useState('')
  const [baris, setBaris] = useState<BarisKirim[]>([])
  const [bukaPemilih, setBukaPemilih] = useState(false)
  const [galat, setGalat] = useState<string | null>(null)

  const toko = useQuery({
    queryKey: ['outlets'],
    queryFn: () => api.get<Halaman<Toko>>('/outlets', { query: { limit: 100 } }),
  })

  const daftar = useQuery({
    queryKey: ['transfer'],
    queryFn: () => stokApi.daftarTransfer(),
  })

  const tokoLain = (toko.data?.data ?? []).filter((t) => t.id !== tokoAktif)

  const buat = useMutation({
    mutationFn: () =>
      stokApi.buatTransfer({
        from_outlet_id: tokoAktif!,
        to_outlet_id: keToko,
        note: catatan.trim() || undefined,
        items: baris.map((b) => ({ product_id: b.produk.id, qty: b.qty })),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['transfer'] })
      toast.berhasil('Pengiriman dibuat. Tekan "Kirim" bila barangnya sudah berangkat.')
      setBaris([])
      setCatatan('')
    },
    onError: (e) =>
      setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  const ubahStatus = useMutation({
    mutationFn: ({ id, aksi }: { id: string; aksi: 'kirim' | 'terima' }) =>
      aksi === 'kirim' ? stokApi.kirimTransfer(id) : stokApi.terimaTransfer(id),
    onSuccess: (_d, v) => {
      qc.invalidateQueries({ queryKey: ['transfer'] })
      qc.invalidateQueries({ queryKey: ['stok'] })
      qc.invalidateQueries({ queryKey: ['stok-kasir'] })
      toast.berhasil(
        v.aksi === 'kirim'
          ? 'Barang ditandai sudah berangkat. Stok toko asal berkurang.'
          : 'Barang diterima. Stok toko tujuan bertambah.',
      )
    },
    onError: (e) =>
      toast.gagal(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  const namaToko = (id: string) =>
    toko.data?.data.find((t) => t.id === id)?.name ?? 'Toko lain'

  if (tokoLain.length === 0 && !toko.isLoading) {
    return (
      <KeadaanKosong
        ikon={Store}
        judul="Baru ada satu toko"
        penjelasan="Kirim barang antar toko baru berguna kalau Anda punya lebih dari satu cabang. Tambahkan cabang dulu di Pengaturan."
        aksi={
          boleh(IZIN.outletManage)
            ? { label: 'Tambah Cabang', onKlik: () => navigate('/pengaturan/toko') }
            : undefined
        }
      />
    )
  }

  return (
    // Layar lebar: formulir pengiriman di kiri, pengiriman terakhir di kanan.
    <div className="flex w-full max-w-lg flex-col gap-4 lg:grid lg:max-w-5xl lg:grid-cols-[minmax(0,26rem)_minmax(0,1fr)] lg:items-start lg:gap-x-6">
      <h1 className="text-judul font-bold text-teks-utama lg:col-span-2">Kirim Barang Antar Toko</h1>

      <Kartu className="flex flex-col gap-4 p-4">
        <Pilihan
          label="Kirim ke"
          value={keToko}
          onChange={(e) => setKeToko(e.target.value)}
          required
        >
          <option value="">Pilih toko tujuan</option>
          {tokoLain.map((t) => (
            <option key={t.id} value={t.id}>
              {t.name}
            </option>
          ))}
        </Pilihan>

        <div className="flex items-center justify-between">
          <p className="text-label font-medium text-teks-sekunder">Barang</p>
          <Tombol jenis="kedua" ukuran="padat" onClick={() => setBukaPemilih(true)}>
            <Plus className="h-5 w-5" aria-hidden />
            Tambah
          </Tombol>
        </div>

        {baris.length === 0 ? (
          <p className="py-4 text-center text-isi text-teks-redup">
            Belum ada barang dipilih.
          </p>
        ) : (
          <ul className="flex flex-col gap-3">
            {baris.map((b, i) => (
              <li key={b.produk.id} className="flex flex-col gap-2 border-t border-garis pt-3">
                <div className="flex items-start justify-between gap-3">
                  <p className="min-w-0 font-medium text-teks-utama">{b.produk.name}</p>
                  <button
                    type="button"
                    onClick={() => setBaris((l) => l.filter((_, j) => j !== i))}
                    aria-label={`Hapus ${b.produk.name}`}
                    className="-m-2 shrink-0 rounded-kontrol p-2 text-bahaya-teks hover:bg-bahaya-teks/10"
                  >
                    <Trash2 className="h-5 w-5" aria-hidden />
                  </button>
                </div>
                <StepperJumlah
                  nilai={b.qty}
                  onNilai={(q) => setBaris((l) => l.map((x, j) => (j === i ? { ...x, qty: q } : x)))}
                  satuan={b.produk.unit_name}
                  minimal="1"
                  label={`Jumlah ${b.produk.name}`}
                />
              </li>
            ))}
          </ul>
        )}

        <Kolom
          label="Catatan"
          value={catatan}
          onChange={(e) => setCatatan(e.target.value)}
          bantuan="Boleh dikosongkan."
          maxLength={255}
        />

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <Tombol
          lebarPenuh
          memuat={buat.isPending}
          disabled={!keToko || baris.length === 0}
          onClick={() => {
            setGalat(null)
            buat.mutate()
          }}
        >
          Buat Pengiriman
        </Tombol>
      </Kartu>

      <section className="flex flex-col gap-2">
        <h2 className="text-judul-kartu font-semibold text-teks-utama">
          Pengiriman terakhir
        </h2>

        {daftar.isLoading ? (
          <KerangkaBaris jumlah={3} />
        ) : (daftar.data?.data.length ?? 0) === 0 ? (
          <p className="text-isi text-teks-redup">Belum ada pengiriman antar toko.</p>
        ) : (
          <ul className="flex flex-col gap-2">
            {daftar.data?.data.map((t) => (
              <li key={t.id}>
                <KartuTransfer
                  transfer={t}
                  namaToko={namaToko}
                  tokoAktif={tokoAktif}
                  sedang={ubahStatus.isPending}
                  onAksi={(aksi) => ubahStatus.mutate({ id: t.id, aksi })}
                />
              </li>
            ))}
          </ul>
        )}
      </section>

      <PemilihBarang
        terbuka={bukaPemilih}
        onTutup={() => setBukaPemilih(false)}
        onPilih={(p) => {
          setBukaPemilih(false)
          setBaris((l) =>
            l.some((b) => b.produk.id === p.id) ? l : [...l, { produk: p, qty: '1' }],
          )
        }}
        judul="Pilih barang yang dikirim"
      />
    </div>
  )
}

function KartuTransfer({
  transfer,
  namaToko,
  tokoAktif,
  sedang,
  onAksi,
}: {
  transfer: Transfer
  namaToko: (id: string) => string
  tokoAktif?: string
  sedang: boolean
  onAksi: (aksi: 'kirim' | 'terima') => void
}) {
  const dariSini = transfer.from_outlet_id === tokoAktif
  const keSini = transfer.to_outlet_id === tokoAktif

  return (
    <Kartu className="flex flex-col gap-3 p-4">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="flex flex-wrap items-center gap-1.5 font-medium text-teks-utama">
            {namaToko(transfer.from_outlet_id)}
            <ArrowRight className="h-4 w-4 shrink-0 text-teks-redup" aria-hidden />
            {namaToko(transfer.to_outlet_id)}
          </p>
          <p className="text-keterangan text-teks-redup">
            {formatTanggal(transfer.created_at)}
            {transfer.items && ` · ${transfer.items.length} jenis barang`}
          </p>
        </div>
        <StatusTransfer status={transfer.status} />
      </div>

      {transfer.items && transfer.items.length > 0 && (
        <ul className="text-keterangan text-teks-sekunder">
          {transfer.items.map((i) => (
            <li key={i.id} className="tabular-nums">
              {formatQtySatuan(i.qty)} × {i.product_id.slice(0, 8)}…
            </li>
          ))}
        </ul>
      )}

      {/* Tombolnya hanya muncul untuk toko yang memang berwenang di tahap itu. */}
      {transfer.status === 'draft' && dariSini && (
        <Tombol ukuran="padat" memuat={sedang} onClick={() => onAksi('kirim')}>
          <Truck className="h-5 w-5" aria-hidden />
          Barang Sudah Berangkat
        </Tombol>
      )}
      {transfer.status === 'sent' && keSini && (
        <Tombol ukuran="padat" memuat={sedang} onClick={() => onAksi('terima')}>
          Terima Barang
        </Tombol>
      )}
      {transfer.status === 'sent' && dariSini && (
        <p className="text-keterangan text-teks-redup">
          Menunggu {namaToko(transfer.to_outlet_id)} menerima barangnya.
        </p>
      )}
    </Kartu>
  )
}

function StatusTransfer({ status }: { status: string }) {
  switch (status) {
    case 'draft':
      return <LencanaStatus nada="netral" anak="Disiapkan" />
    case 'sent':
      return <LencanaStatus nada="menunggu" anak="Di jalan" />
    case 'received':
      return <LencanaStatus nada="berhasil" anak="Diterima" />
    default:
      return <LencanaStatus nada="netral" anak={status} />
  }
}
