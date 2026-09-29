import { useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ulid } from 'ulid'
import { History, Info, Plus, Trash2, TrendingDown, TrendingUp } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { StepperJumlah } from '@/bersama/ui/stepper-jumlah'
import { Tombol } from '@/bersama/ui/tombol'
import { SegmenPilihan } from '@/bersama/ui/segmen'
import { FotoBarang } from '@/bersama/komponen/foto-barang'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useKategori, usePemasok } from '@/bersama/hooks/use-katalog'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah, pratinjauBaris } from '@/bersama/util/uang'
import { formatQty, formatQtySatuan } from '@/bersama/util/desimal'
import { tanggalISO } from '@/bersama/util/tanggal'
import { kelasPetak, petaWarnaKategori } from '@/bersama/util/warna-kategori'
import { cn } from '@/bersama/util/cn'
import type { Kemasan, Produk } from '@/bersama/tipe/katalog'
import { PemilihBarang } from '../komponen/pemilih-barang'
import { SaranBelanja } from '../komponen/saran-belanja'
import { DialogRiwayatMasuk } from '../komponen/dialog-riwayat-masuk'
import { PilihSumberBayar } from '../komponen/pilih-sumber-bayar'
import { stokApi, type SumberBayar } from '../api'
import { produkDariSaldo } from '../keadaan-stok'
import { tambahHari } from '../utang'

interface BarisMasuk {
  produk: Produk
  /** Kemasan yang dibeli (dus); kosong = satuan dasar. */
  kemasan?: Kemasan
  qty: string
  /** Harga beli per satuan baris (per dus bila kemasan). */
  hargaBeli: number
}

/** Isi kemasan baris (1 = satuan dasar). */
const isiBaris = (b: BarisMasuk) => Number(b.kemasan?.conversion ?? 1)
/** Harga modal per satuan dasar SETELAH barang ini masuk (last-cost). */
const modalBaru = (b: BarisMasuk) => Math.round(b.hargaBeli / isiBaris(b))

/**
 * Barang masuk (ui/05-ALUR-UTAMA.md §4).
 *
 * - SARAN BELANJA: barang di bawah batas minimum atau yang habis dalam
 *   seminggu, dengan jumlah beli untuk dua minggu — sekali ketuk masuk daftar.
 *   Sebelum ke pasar ia daftar belanja; sepulangnya, jalan pintas mengisi.
 * - Tiap baris memperlihatkan sisa sekarang → sisa setelah masuk, dan
 *   PERUBAHAN HARGA MODAL-nya. Backend mengambil harga modal dari pembelian
 *   terakhir; tanpa diberi tahu per barang, pemilik bingung kenapa untungnya
 *   tiba-tiba berubah.
 * - Riwayat nota ada di dialog "Riwayat", bukan di bawah formulir: layar ini
 *   untuk mencatat, dan riwayat yang panjang mendorong tombol Simpan.
 */
export function HalamanBarangMasuk() {
  const { tokoAktif } = useSesi()
  const navigate = useNavigate()
  const toast = useToast()
  const qc = useQueryClient()
  const pemasok = usePemasok()
  const kat = useKategori()
  const warna = petaWarnaKategori(kat.data?.data ?? [])

  // Dari halaman Pemasok ("Catat barang masuk"): pemasoknya sudah terisi.
  const [params] = useSearchParams()
  const [pemasokId, setPemasokId] = useState(params.get('pemasok') ?? '')
  const [noNota, setNoNota] = useState('')
  const [baris, setBaris] = useState<BarisMasuk[]>([])
  const [bukaPemilih, setBukaPemilih] = useState(false)
  const [bukaRiwayat, setBukaRiwayat] = useState(false)
  // Sudah mengambil dari saran belanja → saran tetap terbuka supaya bisa
  // menambah beberapa berturut-turut; mengisi lewat pemilih → saran dilipat.
  const [pakaiSaran, setPakaiSaran] = useState(false)
  const [galat, setGalat] = useState<string | null>(null)
  // Pembayaran (000047): lunas / sebagian / belum bayar. Sisanya jadi utang
  // pemasok; sumber uang yang dibayar dipilih tiap kali (laci atau uang lain).
  const [cara, setCara] = useState<'lunas' | 'sebagian' | 'utang'>('lunas')
  const [dibayar, setDibayar] = useState(0)
  const [sumber, setSumber] = useState<SumberBayar>('other')
  const [jatuhTempo, setJatuhTempo] = useState('')

  // Kunci idempotensi dibuat sekali per formulir: barang masuk menciptakan
  // stok DAN utang ke pemasok, jadi tidak boleh tercatat dua kali.
  const kunci = useRef(ulid())

  const ids = baris.map((b) => b.produk.id)
  const saldo = useQuery({
    queryKey: ['stok', tokoAktif, 'barang-masuk', ids.join(',')],
    queryFn: () => stokApi.saldo(tokoAktif!, false, 1, 100, { produk: ids }),
    enabled: !!tokoAktif && ids.length > 0,
    staleTime: 30_000,
  })
  const sisa = new Map((saldo.data?.data ?? []).map((s) => [s.product_id, s.qty]))

  const total = baris.reduce((j, b) => j + pratinjauBaris(b.hargaBeli, b.qty), 0)
  const terbayar = cara === 'lunas' ? total : cara === 'sebagian' ? Math.min(Math.max(dibayar, 0), total) : 0
  const sisaUtang = total - terbayar
  const namaPemasok = pemasok.data?.data.find((p) => p.id === pemasokId)?.name
  const pembayaranSah = cara !== 'sebagian' || (dibayar > 0 && dibayar < total)
  const modalBerubah = baris.filter((b) => b.produk.cost_price > 0 && modalBaru(b) !== b.produk.cost_price).length

  const simpan = useMutation({
    mutationFn: () =>
      stokApi.barangMasuk(
        {
          outlet_id: tokoAktif!,
          supplier_id: pemasokId || undefined,
          invoice_no: noNota.trim() || undefined,
          paid_amount: terbayar,
          payment_source: terbayar > 0 ? sumber : undefined,
          due_date: sisaUtang > 0 && jatuhTempo ? jatuhTempo : undefined,
          items: baris.map((b) => ({
            product_id: b.produk.id,
            ...(b.kemasan ? { product_unit_id: b.kemasan.id } : {}),
            qty: b.qty,
            unit_cost: b.hargaBeli,
          })),
        },
        kunci.current,
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['stok'] })
      qc.invalidateQueries({ queryKey: ['stok-ringkasan'] })
      qc.invalidateQueries({ queryKey: ['stok-kasir'] })
      qc.invalidateQueries({ queryKey: ['kartu-stok'] })
      qc.invalidateQueries({ queryKey: ['produk'] })
      qc.invalidateQueries({ queryKey: ['pembelian'] })
      qc.invalidateQueries({ queryKey: ['utang'] })
      if (terbayar > 0 && sumber === 'drawer') {
        qc.invalidateQueries({ queryKey: ['shift-aktif'] })
        qc.invalidateQueries({ queryKey: ['gerakan-kas'] })
      }

      const ringkas = baris
        .map((b) => `${b.qty} ${b.kemasan?.unit_name ?? b.produk.unit_name ?? ''} ${b.produk.name}`.trim())
        .join(', ')
      toast.berhasil(
        `${ringkas} masuk. Harga modal diperbarui otomatis.` +
          (sisaUtang > 0 ? ` Sisa ${formatRupiah(sisaUtang)} dicatat sebagai utang${namaPemasok ? ` ke ${namaPemasok}` : ''}.` : ''),
      )
      navigate('/stok')
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  /**
   * Daftar barang tidak memuat kemasan; barang lengkap diambil terpisah supaya
   * baris bisa dicatat per dus. Gagal diambil → tetap bisa per satuan dasar.
   */
  function lengkapi(id: string) {
    void stokApi
      .barang(id)
      .then((lengkap) =>
        setBaris((l) => l.map((b) => (b.produk.id === id ? { ...b, produk: { ...b.produk, ...lengkap } } : b))),
      )
      .catch(() => {})
  }

  function tambahBaris(daftar: { produk: Produk; qty: string }[]) {
    const baru = daftar.filter((d) => !baris.some((b) => b.produk.id === d.produk.id))
    if (!baru.length) return
    setBaris((lama) => [
      ...lama,
      ...baru.map((d) => ({ produk: d.produk, qty: d.qty, hargaBeli: d.produk.cost_price })),
    ])
    baru.forEach((d) => lengkapi(d.produk.id))
  }

  /** Ganti satuan beli satu baris; harga beli disesuaikan ke satuan barunya. */
  function gantiSatuan(i: number, kemasan: Kemasan | undefined) {
    setBaris((l) =>
      l.map((b, j) => {
        if (j !== i) return b
        const isiBaru = Number(kemasan?.conversion ?? 1)
        return { ...b, kemasan, hargaBeli: Math.round((b.hargaBeli / isiBaris(b)) * isiBaru) }
      }),
    )
  }

  const ubahBaris = (i: number, isi: Partial<BarisMasuk>) =>
    setBaris((l) => l.map((x, j) => (j === i ? { ...x, ...isi } : x)))

  return (
    // Layar lebar: nota & total (menempel saat digulir) di kiri, daftar barang
    // di kanan — dulu semuanya satu kolom 512px dengan dua pertiga layar kosong.
    // Baris terakhir 1fr: tinggi daftar barang (yang merentang tiga baris)
    // diserap baris itu, jadi kartu total menempel tepat di bawah kartu nota —
    // tanpa itu grid membagi tingginya dan menyisakan celah kosong di antaranya.
    <div className="flex w-full max-w-lg flex-col gap-4 lg:grid lg:max-w-5xl lg:grid-cols-[minmax(0,22rem)_minmax(0,1fr)] lg:grid-rows-[auto_auto_auto_1fr] lg:items-start lg:gap-x-6">
      <header className="flex items-center justify-between gap-3 lg:col-span-2">
        <div className="min-w-0">
          <h1 className="text-judul font-bold text-teks-utama">Barang Masuk</h1>
          <p className="hidden text-label text-teks-sekunder sm:block">
            Catat barang yang datang — stok bertambah dan harga modal ikut diperbarui.
          </p>
        </div>
        <Tombol jenis="kedua" onClick={() => setBukaRiwayat(true)} className="shrink-0 px-4">
          <History className="h-5 w-5" aria-hidden />
          Riwayat
        </Tombol>
      </header>

      <Kartu className="flex flex-col gap-4 p-4 lg:col-start-1">
        <Pilihan
          label="Dari pemasok"
          value={pemasokId}
          onChange={(e) => setPemasokId(e.target.value)}
          bantuan="Boleh dikosongkan bila beli sendiri di pasar."
        >
          <option value="">Tanpa pemasok</option>
          {pemasok.data?.data.map((p) => (
            <option key={p.id} value={p.id}>
              {p.name}
            </option>
          ))}
        </Pilihan>

        <Kolom label="Nomor nota" value={noNota} onChange={(e) => setNoNota(e.target.value)} bantuan="Boleh dikosongkan." />
      </Kartu>

      <section className="flex flex-col gap-2 lg:col-start-2 lg:row-span-3 lg:row-start-2">
        <div className="flex items-center justify-between">
          <h2 className="text-judul-kartu font-semibold text-teks-utama">
            Barang
            {baris.length > 0 && (
              <span className="ml-2 text-label font-medium tabular-nums text-teks-redup">{baris.length}</span>
            )}
          </h2>
          <Tombol jenis="kedua" ukuran="padat" onClick={() => setBukaPemilih(true)}>
            <Plus className="h-5 w-5" aria-hidden />
            Tambah
          </Tombol>
        </div>

        {baris.length === 0 ? (
          <Kartu className="p-5 text-center">
            <p className="text-isi text-teks-redup">
              Belum ada barang. Ketuk &ldquo;Tambah&rdquo; untuk memilih barang yang datang.
            </p>
          </Kartu>
        ) : (
          <ul className="flex flex-col gap-2">
            {baris.map((b, i) => (
              <li key={b.produk.id}>
                <KartuBaris
                  b={b}
                  sisa={sisa.get(b.produk.id)}
                  kelasWarna={kelasPetak(warna, b.produk.category_id)}
                  onHapus={() => setBaris((l) => l.filter((x) => x.produk.id !== b.produk.id))}
                  onSatuan={(k) => gantiSatuan(i, k)}
                  onQty={(qty) => ubahBaris(i, { qty })}
                  onHarga={(hargaBeli) => ubahBaris(i, { hargaBeli })}
                />
              </li>
            ))}
          </ul>
        )}

        <SaranBelanja
          sudah={new Set(ids)}
          pemasokId={pemasokId || undefined}
          terlipat={baris.length > 0 && !pakaiSaran}
          bisaDilipat={baris.length > 0}
          onTambah={(d) => {
            setPakaiSaran(true)
            tambahBaris(d.map(({ saldo: s, qty }) => ({ produk: produkDariSaldo(s), qty: String(qty) })))
          }}
        />
      </section>

      {baris.length > 0 && (
        <Kartu className="flex flex-col gap-3 p-4 lg:sticky lg:top-4 lg:col-start-1 lg:row-start-3">
          <div className="flex items-baseline justify-between gap-3">
            <span className="text-isi text-teks-sekunder">
              Total belanja
              <span className="block text-keterangan text-teks-redup">{baris.length} barang</span>
            </span>
            <span className="text-judul font-extrabold tabular-nums text-teks-utama">{formatRupiah(total)}</span>
          </div>

          <div className="flex flex-col gap-3 border-t border-garis pt-3">
            <div className="flex flex-col gap-1.5">
              <span className="text-label font-medium text-teks-sekunder">Pembayaran</span>
              <SegmenPilihan
                label="Pembayaran"
                nilai={cara}
                onPilih={setCara}
                pilihan={[
                  ['lunas', 'Lunas'],
                  ['sebagian', 'Sebagian'],
                  ['utang', 'Belum bayar'],
                ]}
              />
            </div>
            {cara === 'sebagian' && (
              <KolomUang
                label="Dibayar sekarang"
                nilai={dibayar}
                onNilai={setDibayar}
                galat={dibayar >= total && total > 0 ? 'Sama dengan total — pilih "Lunas".' : undefined}
              />
            )}
            {terbayar > 0 && <PilihSumberBayar nilai={sumber} onPilih={setSumber} />}
            {sisaUtang > 0 && (
              <div className="flex flex-col gap-2 rounded-kontrol bg-permukaan-2/60 p-3">
                <p className="text-label text-teks-utama">
                  Sisa <strong className="tabular-nums">{formatRupiah(sisaUtang)}</strong> dicatat sebagai utang
                  {namaPemasok ? ` ke ${namaPemasok}` : ''}.
                </p>
                {!pemasokId && (
                  <p className="text-keterangan text-jingga-700">Pilih pemasok di atas supaya jelas utang ke siapa.</p>
                )}
                <Kolom
                  label="Jatuh tempo"
                  type="date"
                  value={jatuhTempo}
                  min={tanggalISO()}
                  onChange={(e) => setJatuhTempo(e.target.value)}
                />
                <div className="flex flex-wrap gap-1.5" role="group" aria-label="Jatuh tempo cepat">
                  {([7, 14, 30] as const).map((n) => {
                    const tgl = tambahHari(tanggalISO(), n)
                    return (
                      <button
                        key={n}
                        type="button"
                        onClick={() => setJatuhTempo(tgl)}
                        aria-pressed={jatuhTempo === tgl}
                        className={cn(
                          'min-h-9 rounded-full border px-3 text-keterangan font-medium',
                          jatuhTempo === tgl
                            ? 'border-utama bg-sorot text-hijau-800'
                            : 'border-garis bg-permukaan text-teks-sekunder hover:bg-permukaan-2',
                        )}
                      >
                        {n} hari
                      </button>
                    )
                  })}
                  {jatuhTempo && (
                    <button
                      type="button"
                      onClick={() => setJatuhTempo('')}
                      className="min-h-9 px-2 text-keterangan font-medium text-teks-sekunder hover:underline"
                    >
                      Tanpa jatuh tempo
                    </button>
                  )}
                </div>
              </div>
            )}
          </div>

          <p className="flex items-start gap-2 rounded-kontrol bg-info-teks/10 px-3 py-2 text-keterangan text-info-teks">
            <Info className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
            {modalBerubah > 0
              ? `Harga modal ${modalBerubah} barang ikut berubah mengikuti harga beli ini — itu yang dipakai menghitung untung Anda.`
              : 'Harga beli sama dengan modal sekarang, jadi harga modal tidak berubah.'}
          </p>

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <Tombol
            lebarPenuh
            memuat={simpan.isPending}
            labelMemuat="Menyimpan…"
            disabled={!pembayaranSah}
            onClick={() => {
              setGalat(null)
              simpan.mutate()
            }}
          >
            Simpan Barang Masuk
          </Tombol>
        </Kartu>
      )}

      <PemilihBarang
        terbuka={bukaPemilih}
        onTutup={() => setBukaPemilih(false)}
        onPilih={(p) => {
          setBukaPemilih(false)
          tambahBaris([{ produk: p, qty: '1' }])
        }}
        judul="Pilih barang yang masuk"
        harga="modal"
        sudahDipilih={ids}
      />
      <DialogRiwayatMasuk terbuka={bukaRiwayat} onTutup={() => setBukaRiwayat(false)} />
    </div>
  )
}

function KartuBaris({
  b,
  sisa,
  kelasWarna,
  onHapus,
  onSatuan,
  onQty,
  onHarga,
}: {
  b: BarisMasuk
  sisa?: string
  kelasWarna: string
  onHapus: () => void
  onSatuan: (k: Kemasan | undefined) => void
  onQty: (qty: string) => void
  onHarga: (n: number) => void
}) {
  const satuan = b.produk.unit_name ?? ''
  const masuk = (Number(b.qty) || 0) * isiBaris(b)
  const lama = b.produk.cost_price
  const baru = modalBaru(b)
  const naik = baru > lama
  const persen = lama > 0 ? Math.round((Math.abs(baru - lama) / lama) * 100) : 0
  // Lonjakan sebesar ini hampir selalu salah ketik — paling sering harga per
  // dus diisi di kolom harga per satuan.
  const janggal = persen >= 50

  return (
    <Kartu className="flex flex-col gap-3 p-4">
      <div className="flex items-start gap-3">
        <FotoBarang nama={b.produk.name} url={b.produk.image_url} kecil kelasWarna={kelasWarna} className="w-11" />
        <div className="min-w-0 flex-1">
          <p className="font-semibold text-teks-utama">{b.produk.name}</p>
          {/* Sisa sekarang → sesudah masuk, dalam satuan dasar. */}
          {sisa !== undefined && (
            <p className="text-keterangan tabular-nums text-teks-redup">
              sisa {formatQtySatuan(sisa, satuan).replace('-', '−')} →{' '}
              <strong className="font-semibold text-teks-utama">
                jadi {formatQtySatuan(String(Number.parseFloat(sisa) + masuk), satuan).replace('-', '−')}
              </strong>
            </p>
          )}
        </div>
        <button
          type="button"
          onClick={onHapus}
          aria-label={`Hapus ${b.produk.name}`}
          className="-m-2 shrink-0 rounded-kontrol p-2 text-bahaya-teks hover:bg-bahaya-teks/10"
        >
          <Trash2 className="h-5 w-5" aria-hidden />
        </button>
      </div>

      {/* Barang berkemasan: dicatat per dus atau per satuan dasar. Stok
          selalu masuk dalam satuan dasar. */}
      {b.produk.packagings?.length ? (
        <SegmenPilihan
          label={`Satuan beli ${b.produk.name}`}
          nilai={b.kemasan?.id ?? 'dasar'}
          onPilih={(v) => onSatuan(b.produk.packagings!.find((k) => k.id === v))}
          pilihan={[
            ['dasar', satuan || 'satuan'] as const,
            ...b.produk.packagings.map((k) => [k.id, `${k.unit_name} isi ${formatQty(k.conversion)}`] as const),
          ]}
        />
      ) : null}

      <div className="grid gap-3 sm:grid-cols-2 sm:items-start">
        {/* Label tampak supaya sejajar dengan kolom harga di sebelahnya;
            nama aksesibelnya tetap dari `label`. */}
        <div className="flex flex-col gap-1.5">
          <span aria-hidden className="text-label font-medium text-teks-sekunder">
            Jumlah
          </span>
          <StepperJumlah
            nilai={b.qty}
            onNilai={onQty}
            satuan={b.kemasan?.unit_name ?? satuan}
            minimal="1"
            label={`Jumlah ${b.produk.name}`}
          />
        </div>

        <KolomUang
          label={b.kemasan ? `Harga beli per ${b.kemasan.unit_name}` : 'Harga beli satuan'}
          nilai={b.hargaBeli}
          onNilai={onHarga}
          bantuan={
            b.kemasan
              ? `Stok masuk ${formatQty(String(masuk))} ${satuan} · modal ${formatRupiah(baru)} per ${satuan || 'satuan'}.`
              : undefined
          }
        />
      </div>

      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 border-t border-garis pt-2">
        {/* Perubahan harga modal per barang — naik ditandai jingga (untung
            menipis), turun hijau. Ikon + teks, bukan warna saja. */}
        {lama > 0 && baru !== lama ? (
          <span
            className={cn(
              'flex items-center gap-1 text-keterangan tabular-nums',
              naik ? 'text-jingga-700' : 'text-hijau-700',
            )}
          >
            {naik ? (
              <TrendingUp className="h-3.5 w-3.5" aria-hidden />
            ) : (
              <TrendingDown className="h-3.5 w-3.5" aria-hidden />
            )}
            modal {formatRupiah(lama)} → {formatRupiah(baru)} ({naik ? 'naik' : 'turun'} {persen}%)
            {janggal && <strong className="font-semibold"> — periksa lagi harganya</strong>}
          </span>
        ) : (
          <span className="text-keterangan text-teks-redup">
            {lama > 0 ? `modal tetap ${formatRupiah(lama)}` : 'Subtotal'}
          </span>
        )}
        <span className="font-bold tabular-nums text-teks-utama">{formatRupiah(pratinjauBaris(b.hargaBeli, b.qty))}</span>
      </div>
    </Kartu>
  )
}
