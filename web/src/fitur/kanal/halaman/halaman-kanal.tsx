import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Info, Plus, Store, Trash2 } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { StepperJumlah } from '@/bersama/ui/stepper-jumlah'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useDaftarProduk } from '@/bersama/hooks/use-katalog'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatTanggalJam } from '@/bersama/util/tanggal'
import type { Produk } from '@/bersama/tipe/katalog'
import { kanalApi, type Kanal } from '../api'
import { FITUR } from '@/lib/fitur'
import { BannerKunciFitur } from '@/bersama/komponen/kunci-fitur'

const JENIS_KANAL = [
  { nilai: 'delivery_app', label: 'Aplikasi antar (GoFood, GrabFood)' },
  { nilai: 'marketplace', label: 'Marketplace (Shopee, Tokopedia)' },
  { nilai: 'conversation', label: 'Pesan langsung (WhatsApp, Instagram)' },
]

/**
 * Kanal penjualan online.
 *
 * Adaptor API per-provider BELUM ADA — masih menunggu kemitraan resmi
 * (ui/04-PETA-LAYAR.md, blueprint F.9). Karena itu entri manual dan impor CSV
 * adalah jalur utamanya, bukan jalan darurat, dan layar ini mengatakannya
 * terang-terangan supaya tidak ada yang menunggu tombol "hubungkan otomatis"
 * yang tidak akan muncul.
 */
export function HalamanKanal() {
  const { boleh, punyaFitur } = useSesi()
  // Kunci paket: menambah kanal & mencatat pesanan BARU butuh fitur kanal
  // online. Pesanan yang sudah masuk tetap bisa diproses (server sama).
  const fiturKanal = punyaFitur(FITUR.kanalOnline)
  const [buatKanal, setBuatKanal] = useState(false)
  const [pesananUntuk, setPesananUntuk] = useState<Kanal | null>(null)

  const kanal = useQuery({ queryKey: ['kanal'], queryFn: () => kanalApi.daftar() })
  const pesanan = useQuery({
    queryKey: ['pesanan-kanal'],
    queryFn: () => kanalApi.daftarPesanan(),
    enabled: boleh(IZIN.channelOrderAccept, IZIN.channelManage),
  })

  const daftar = kanal.data?.data ?? []
  const namaKanal = useMemo(
    () => new Map(daftar.map((k) => [k.id, k.name])),
    [daftar],
  )

  return (
    <div className="flex w-full max-w-2xl flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-judul font-bold text-teks-utama">Kanal Online</h1>
        {boleh(IZIN.channelManage) && fiturKanal && (
          <Tombol onClick={() => setBuatKanal(true)}>
            <Plus className="h-5 w-5" aria-hidden />
            Tambah Kanal
          </Tombol>
        )}
      </header>

      <BannerKunciFitur
        fitur={FITUR.kanalOnline}
        nama="Kanal online"
        penjelasan="Kanal dan pesanan yang sudah tercatat tetap bisa dilihat dan diselesaikan — hanya menambah kanal atau pesanan baru yang terkunci."
      />

      <p className="flex items-start gap-2 rounded-kontrol bg-info-teks/10 px-3 py-2 text-keterangan text-info-teks">
        <Info className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
        Pesanan dicatat manual atau diimpor dari laporan harian marketplace.
        Sambungan otomatis ke GoFood, Shopee, dan sejenisnya menunggu kerja sama
        resmi dengan mereka — belum bisa kami janjikan tanggalnya.
      </p>

      {kanal.isLoading ? (
        <KerangkaBaris jumlah={3} />
      ) : daftar.length === 0 ? (
        <KeadaanKosong
          ikon={Store}
          judul="Belum ada kanal"
          penjelasan="Tambahkan kanal seperti GoFood atau WhatsApp supaya penjualan dari sana ikut tercatat dan untungnya bisa dibandingkan."
          aksi={
            boleh(IZIN.channelManage) && fiturKanal
              ? { label: 'Tambah Kanal', onKlik: () => setBuatKanal(true) }
              : undefined
          }
        />
      ) : (
        <ul className="flex flex-col gap-2">
          {daftar.map((k) => (
            <li key={k.id}>
              <KartuKanal kanal={k} onCatatPesanan={() => setPesananUntuk(k)} />
            </li>
          ))}
        </ul>
      )}

      {(pesanan.data?.data.length ?? 0) > 0 && (
        <section className="flex flex-col gap-2">
          <h2 className="text-judul-kartu font-semibold text-teks-utama">
            Pesanan terakhir
          </h2>
          <Kartu className="divide-y divide-garis">
            {pesanan.data?.data.map((p) => (
              <div key={p.id} className="flex items-center justify-between gap-3 p-4">
                <div className="min-w-0">
                  <p className="truncate font-medium text-teks-utama">
                    {p.buyer_name || p.external_order_id}
                  </p>
                  <p className="text-keterangan text-teks-redup">
                    {namaKanal.get(p.channel_id) ?? 'Kanal'} ·{' '}
                    {formatTanggalJam(p.created_at)}
                  </p>
                </div>
                <div className="shrink-0 text-right">
                  <p className="font-bold tabular-nums text-teks-utama">
                    {formatRupiah(p.net_amount)}
                  </p>
                  {/* Biaya kanal ditulis apa adanya — itu yang membuat untung
                      dari kanal online sering lebih tipis dari dugaan. */}
                  {p.fee_amount > 0 && (
                    <p className="text-keterangan tabular-nums text-teks-redup">
                      setelah biaya {formatRupiah(p.fee_amount)}
                    </p>
                  )}
                </div>
              </div>
            ))}
          </Kartu>
        </section>
      )}

      {buatKanal && <DialogKanal onTutup={() => setBuatKanal(false)} />}
      {pesananUntuk && (
        <DialogPesanan kanal={pesananUntuk} onTutup={() => setPesananUntuk(null)} />
      )}
    </div>
  )
}

function KartuKanal({
  kanal,
  onCatatPesanan,
}: {
  kanal: Kanal
  onCatatPesanan: () => void
}) {
  const { boleh, punyaFitur } = useSesi()
  // Terkunci paket: mencatat pesanan baru & mengubah kanal ditolak server,
  // jadi tombolnya tidak ditawarkan (lihat BannerKunciFitur di atas daftar).
  const fiturKanal = punyaFitur(FITUR.kanalOnline)
  const toast = useToast()
  const qc = useQueryClient()

  const hapus = useMutation({
    mutationFn: () => kanalApi.hapus(kanal.id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['kanal'] })
      toast.berhasil(`Kanal ${kanal.name} dihapus.`)
    },
    onError: (e) => toast.gagal(e instanceof GalatAPI ? e.pesan : 'Gagal menghapus.'),
  })

  const komisi = Number(kanal.commission_rate) * 100

  return (
    <Kartu className="flex items-center justify-between gap-3 p-4">
      <div className="min-w-0">
        <p className="truncate font-semibold text-teks-utama">{kanal.name}</p>
        <p className="text-keterangan text-teks-redup">
          {kanal.provider}
          {komisi > 0 && ` · komisi ${komisi.toFixed(0)}%`}
        </p>
      </div>
      <div className="flex shrink-0 items-center gap-2">
        {kanal.is_active ? (
          <LencanaStatus nada="berhasil" anak="Aktif" />
        ) : (
          <LencanaStatus nada="netral" anak="Nonaktif" />
        )}
        {fiturKanal && boleh(IZIN.channelOrderAccept, IZIN.channelManage) && (
          <Tombol jenis="kedua" ukuran="padat" onClick={onCatatPesanan}>
            Catat Pesanan
          </Tombol>
        )}
        {fiturKanal && boleh(IZIN.channelManage) && (
          <button
            type="button"
            onClick={() => hapus.mutate()}
            aria-label={`Hapus kanal ${kanal.name}`}
            className="-mr-1 rounded-kontrol p-2 text-bahaya-teks hover:bg-bahaya-teks/10"
          >
            <Trash2 className="h-5 w-5" aria-hidden />
          </button>
        )}
      </div>
    </Kartu>
  )
}

function DialogKanal({ onTutup }: { onTutup: () => void }) {
  const { tokoAktif } = useSesi()
  const toast = useToast()
  const qc = useQueryClient()
  const [jenis, setJenis] = useState('delivery_app')
  const [penyedia, setPenyedia] = useState('')
  const [nama, setNama] = useState('')
  const [komisi, setKomisi] = useState('20')
  const [galat, setGalat] = useState<string | null>(null)

  const buat = useMutation({
    mutationFn: () =>
      kanalApi.buat({
        outlet_id: tokoAktif!,
        kind: jenis,
        provider: penyedia.trim() || nama.trim(),
        name: nama.trim(),
        // Pengguna mengetik persen; backend menyimpan pecahan.
        commission_rate: String((Number(komisi) || 0) / 100),
        integration_mode: 'manual',
      }),
    onSuccess: (k) => {
      qc.invalidateQueries({ queryKey: ['kanal'] })
      toast.berhasil(`Kanal ${k.name} ditambahkan.`)
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !buat.isPending && onTutup()}>
      <IsiDialog judul="Tambah Kanal">
        <form
          onSubmit={(e) => {
            e.preventDefault()
            setGalat(null)
            buat.mutate()
          }}
          className="flex flex-col gap-4"
          noValidate
        >
          <Pilihan label="Jenis kanal" value={jenis} onChange={(e) => setJenis(e.target.value)}>
            {JENIS_KANAL.map((j) => (
              <option key={j.nilai} value={j.nilai}>
                {j.label}
              </option>
            ))}
          </Pilihan>
          <Kolom
            label="Nama kanal"
            placeholder="Contoh: GoFood"
            value={nama}
            onChange={(e) => setNama(e.target.value)}
            autoFocus
            required
          />
          <Kolom
            label="Nama penyedia"
            placeholder="Kosongkan bila sama dengan nama kanal"
            value={penyedia}
            onChange={(e) => setPenyedia(e.target.value)}
            bantuan="Dipakai untuk mencocokkan laporan dari mereka."
          />
          <Kolom
            label="Komisi kanal"
            type="number"
            inputMode="decimal"
            akhiran="%"
            value={komisi}
            onChange={(e) => setKomisi(e.target.value)}
            bantuan="Potongan yang diambil kanal dari setiap penjualan. Ini yang membuat untung dari kanal online lebih tipis."
          />

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <AksiDialog>
            <Tombol type="submit" memuat={buat.isPending} disabled={!nama.trim()}>
              Tambah Kanal
            </Tombol>
            <Tombol jenis="kedua" onClick={onTutup} disabled={buat.isPending}>
              Batal
            </Tombol>
          </AksiDialog>
        </form>
      </IsiDialog>
    </Dialog>
  )
}

function DialogPesanan({ kanal, onTutup }: { kanal: Kanal; onTutup: () => void }) {
  const toast = useToast()
  const qc = useQueryClient()
  const [nomor, setNomor] = useState('')
  const [pembeli, setPembeli] = useState('')
  const [cari, setCari] = useState('')
  const [baris, setBaris] = useState<{ produk: Produk; qty: string }[]>([])
  const [galat, setGalat] = useState<string | null>(null)

  const produk = useDaftarProduk(cari)

  const catat = useMutation({
    mutationFn: () =>
      kanalApi.catatPesanan({
        channel_id: kanal.id,
        external_order_id: nomor.trim(),
        buyer_name: pembeli.trim() || undefined,
        items: baris.map((b) => ({ product_id: b.produk.id, qty: b.qty })),
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['pesanan-kanal'] })
      qc.invalidateQueries({ queryKey: ['stok'] })
      // Nomor pesanan yang sudah pernah masuk dibalas 200 oleh backend, bukan
      // galat — jadi kalimat ini sengaja tidak menjanjikan stok ikut berkurang,
      // karena pada kasus itu memang tidak.
      toast.berhasil('Pesanan tercatat.')
      onTutup()
    },
    onError: (e) =>
      setGalat(
        e instanceof GalatAPI
          ? e.status === 409
            ? 'Nomor pesanan ini sudah pernah dicatat.'
            : e.pesan
          : 'Terjadi kesalahan.',
      ),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !catat.isPending && onTutup()}>
      <IsiDialog
        judul={`Catat pesanan ${kanal.name}`}
        keterangan="Untuk pesanan yang datang lewat chat atau aplikasi yang belum tersambung otomatis."
        className="sm:max-w-lg"
      >
        <div className="flex max-h-[60vh] flex-col gap-4 overflow-y-auto">
          <Kolom
            label="Nomor pesanan"
            placeholder="Nomor dari aplikasi kanal"
            value={nomor}
            onChange={(e) => setNomor(e.target.value)}
            bantuan="Dipakai supaya pesanan yang sama tidak tercatat dua kali."
            autoFocus
            required
          />
          <Kolom
            label="Nama pembeli"
            value={pembeli}
            onChange={(e) => setPembeli(e.target.value)}
            bantuan="Boleh dikosongkan."
          />

          <div className="flex flex-col gap-2">
            <Kolom
              label="Cari barang"
              type="search"
              value={cari}
              onChange={(e) => setCari(e.target.value)}
              placeholder="Ketik nama barang…"
            />
            {cari && (
              <ul className="max-h-40 divide-y divide-garis overflow-y-auto rounded-kontrol border border-garis">
                {produk.data?.data.map((p) => (
                  <li key={p.id}>
                    <button
                      type="button"
                      onClick={() => {
                        setBaris((l) =>
                          l.some((b) => b.produk.id === p.id)
                            ? l
                            : [...l, { produk: p, qty: '1' }],
                        )
                        setCari('')
                      }}
                      className="flex min-h-12 w-full items-center justify-between gap-3 px-3 text-left hover:bg-permukaan-2"
                    >
                      <span className="min-w-0 truncate text-label text-teks-utama">
                        {p.name}
                      </span>
                      <span className="shrink-0 tabular-nums text-keterangan text-teks-redup">
                        {formatRupiah(p.sell_price)}
                      </span>
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>

          {baris.map((b, i) => (
            <div key={b.produk.id} className="flex flex-col gap-2 border-t border-garis pt-3">
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
            </div>
          ))}

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}
        </div>

        <AksiDialog>
          <Tombol
            memuat={catat.isPending}
            disabled={!nomor.trim() || baris.length === 0}
            onClick={() => {
              setGalat(null)
              catat.mutate()
            }}
          >
            Catat Pesanan
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={catat.isPending}>
            Batal
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}
