import { useRef, useState } from 'react'
import { useLiveQuery } from 'dexie-react-hooks'
import { db, type TagihanLokal } from '@/lib/offline/db'
import { Link } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { ClipboardList, PackageX, ScanLine, Search, ShoppingCart } from 'lucide-react'
import { Tombol } from '@/bersama/ui/tombol'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka } from '@/bersama/komponen/kerangka'
import { StatusKoneksi } from '@/bersama/komponen/status-koneksi'
import { PemindaiBarcode } from '@/bersama/komponen/pemindai-barcode'
import { bisaMemindai } from '@/bersama/hooks/use-pemindai'
import { kemasanDariBarcode, produkDariBarcode, varianDariBarcode } from '@/lib/offline/katalog-lokal'
import { useSinkron } from '@/lib/offline/mesin'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatJam } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import type { Transaksi } from '@/bersama/tipe/pos'
import type { Produk } from '@/bersama/tipe/katalog'
import { KartuProduk } from '../komponen/kartu-produk'
import { PanelKeranjang } from '../komponen/panel-keranjang'
import { DialogPilihVarian } from '../komponen/dialog-pilih-varian'
import { DialogTahan } from '../komponen/dialog-tahan'
import { DialogTagihan } from '../komponen/dialog-tagihan'
import {
  batalkanTagihan,
  itemTagihanKeBaris,
  pelangganKeranjang,
  simpanTagihan,
  tagihanKeDiskon,
  useTagihanTerbuka,
} from '../tagihan'
import { SegmenPilihan } from '@/bersama/ui/segmen'
import { LayarBayar, type BagianBayar } from '../komponen/layar-bayar'
import { Struk } from '../komponen/struk'
import { itemUntukCheckout, kunciBaris, namaBaris, useKeranjang } from '../keranjang'
import { kunciBaru, useCheckout, useKatalogKasir, useShiftAktif } from '../hooks'
import { HalamanBukaShift } from './halaman-buka-shift'
import { kelasPetak } from '@/bersama/util/warna-kategori'
import { FITUR } from '@/lib/fitur'

/**
 * Layar kasir — yang paling sering dilihat sepanjang hari.
 *
 * Tablet lanskap: kiri grid barang, kanan keranjang yang selalu terlihat.
 * HP: grid penuh, keranjang jadi lembar yang ditarik dari bawah lewat tombol
 * yang SELALU KELIHATAN (bukan geser atau tekan-lama).
 */
export function HalamanKasir() {
  const { tokoAktif, rincianToko, punyaFitur, paketUntuk, profil } = useSesi()
  const qc = useQueryClient()
  const toast = useToast()
  const sinkron = useSinkron()
  const { shift, memuat: memuatShift } = useShiftAktif()

  const [cari, setCari] = useState('')
  const [kategori, setKategori] = useState<string | undefined>(undefined)
  const { produk, petaStok, kategori: daftarKategori, warnaKategori, kosong, memuat } =
    useKatalogKasir(cari, kategori)
  const keranjang = useKeranjang(rincianToko)

  const [bukaPindai, setBukaPindai] = useState(false)
  const [pilihVarian, setPilihVarian] = useState<Produk | null>(null)
  const [bukaBayar, setBukaBayar] = useState(false)
  const [bukaKeranjangHP, setBukaKeranjangHP] = useState(false)
  const [struk, setStruk] = useState<{ transaksi: Transaksi; diantre: boolean } | null>(null)
  const [galatBayar, setGalatBayar] = useState<string | null>(null)
  const [bukaTagihan, setBukaTagihan] = useState(false)
  const [bukaTahan, setBukaTahan] = useState(false)
  const [menahan, setMenahan] = useState(false)
  // Tagihan terbuka cabang ini (open bill) — hook di atas return bersyarat.
  const tagihan = useTagihanTerbuka(tokoAktif)

  const checkout = useCheckout()

  /**
   * Kunci idempotensi dibuat SEKALI saat layar bayar dibuka, lalu dipakai untuk
   * setiap percobaan sampai transaksi benar-benar tersimpan. Inilah yang
   * membuat menekan "Selesai" dua kali tidak menghasilkan dua transaksi.
   */
  const kunci = useRef<string | null>(null)
  // Pelanggan untuk kasbon, dari basis data offline — kasir tetap bisa
  // mencatat kasbon tanpa sinyal. (Hook: harus di atas return bersyarat.)
  const daftarPelanggan = useLiveQuery(() => db.pelanggan.orderBy('name').toArray(), []) ?? []

  if (memuatShift) return <KerangkaKasir />

  // Tidak ada shift terbuka: kasir tidak boleh langsung berjualan, karena
  // uang laci tidak akan punya modal awal untuk dicocokkan nanti.
  if (!shift) return <HalamanBukaShift />

  function bukaLayarBayar() {
    // Tarif pajak & biaya layanan bisa diubah pemilik kapan saja dari HP-nya,
    // sementara tablet kasir terbuka seharian tanpa pernah memuat ulang
    // profil. Total di layar bayar — yang dibayar PAS untuk QRIS/kasbon —
    // harus memakai tarif terbaru, atau server menolaknya. Saat offline
    // permintaan ini ditunda React Query dan tarif terakhir tetap dipakai.
    void qc.invalidateQueries({ queryKey: ['me'] })
    kunci.current = kunciBaru()
    setGalatBayar(null)
    setBukaBayar(true)
  }

  async function selesaikan(pembayaran: BagianBayar[], pelangganId?: string) {
    if (!tokoAktif || !shift) return
    setGalatBayar(null)
    try {
      const hasil = await checkout.mutateAsync({
        kunci: kunci.current ?? kunciBaru(),
        aturan: rincianToko,
        input: {
          outlet_id: tokoAktif,
          shift_id: shift.id,
          items: itemUntukCheckout(keranjang.baris),
          ...(keranjang.diskonTransaksiNominal > 0 ? { order_discount: keranjang.diskonTransaksiNominal } : {}),
          // Kasbon dicatat sebesar sisa sebagai utang; tunai terakhir memakai
          // uang yang benar-benar diterima supaya server yang menghitung
          // kembaliannya.
          payments: pembayaran,
          // Pelanggan keranjang ikut tercatat (harga khusus dihitung server
          // dari daftarnya); kasbon memakai pelanggan yang sama.
          customer_id: pelangganId ?? keranjang.pelanggan?.id,
          // Tagihan terbuka ditutup server di transaksi yang sama.
          ...(keranjang.tagihan ? { open_bill_id: keranjang.tagihan.id } : {}),
        },
      })
      // Tagihan yang dilunasi hilang dari daftar — juga saat penjualannya
      // baru diantre (segarkanTagihan tidak memunculkannya lagi).
      if (keranjang.tagihan) await db.tagihan.delete(keranjang.tagihan.id)
      setBukaBayar(false)
      setBukaKeranjangHP(false)
      keranjang.kosongkan()
      kunci.current = null
      setStruk(hasil)
      sinkron.segarkan()
    } catch (e) {
      if (e instanceof GalatAPI) {
        setGalatBayar(e.pesan)
        // Kunci TIDAK diganti: percobaan berikutnya harus memakai kunci yang
        // sama supaya tidak lahir transaksi kedua.
      } else {
        setGalatBayar('Terjadi kesalahan. Coba lagi.')
      }
    }
  }

  /** Simpan keranjang sebagai tagihan terbuka, lalu kosongkan untuk pembeli berikutnya. */
  async function tahan(label: string) {
    if (!tokoAktif) return
    const { tagihan: t, diantre } = await simpanTagihan({
      outletId: tokoAktif,
      aktif: keranjang.tagihan,
      label,
      baris: keranjang.baris,
      diskonTransaksi: keranjang.diskonTransaksi,
      nominal: keranjang.pratinjauTotal,
      namaPengguna: profil?.user.name,
      pelangganId: keranjang.pelanggan?.id,
    })
    keranjang.kosongkan()
    setBukaTahan(false)
    setBukaKeranjangHP(false)
    sinkron.segarkan()
    toast.berhasil(
      diantre ? `Tagihan ${t.label} disimpan di perangkat — dikirim saat online.` : `Tagihan ${t.label} disimpan.`,
    )
  }

  function tekanTahan() {
    // Tagihan yang sudah bernama langsung disimpan; yang baru ditanya namanya.
    if (!keranjang.tagihan) {
      setBukaTahan(true)
      return
    }
    setMenahan(true)
    tahan(keranjang.tagihan.label)
      .catch((e) => toast.tampilkan(e instanceof Error ? e.message : 'Gagal menyimpan tagihan.', 'perhatian'))
      .finally(() => setMenahan(false))
  }

  async function bukaTagihanDiKeranjang(t: TagihanLokal) {
    const { baris, hilang } = await itemTagihanKeBaris(t.items)
    keranjang.muatTagihan(
      { id: t.id, version: t.version, label: t.label },
      baris,
      tagihanKeDiskon(t.order_discount),
      await pelangganKeranjang(t.customer_id),
    )
    setBukaTagihan(false)
    if (hilang > 0) {
      toast.tampilkan(`${hilang} barang di tagihan ini sudah tidak ada di katalog dan dilewati.`, 'perhatian')
    }
  }

  return (
    <div className="flex h-dvh flex-col bg-latar">
      {/* Di bawah 360px, tiga kendali plus kotak pencarian memang tidak muat
          dalam satu baris — kotak pencariannya turun ke 69px dan placeholder-nya
          terpotong. Di lebar itu saja kotaknya mengambil barisnya sendiri. */}
      <header className="flex shrink-0 flex-wrap items-center gap-2 border-b border-garis bg-permukaan px-4 py-3 sm:gap-3">
        <div className="relative flex-1 max-[359px]:basis-full">
          <Search
            className="pointer-events-none absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-teks-redup"
            aria-hidden
          />
          <input
            type="search"
            value={cari}
            onChange={(e) => setCari(e.target.value)}
            placeholder="Cari barang"
            aria-label="Cari barang"
            className={cn(
              'h-12 w-full rounded-kontrol border border-garis bg-permukaan pl-9 pr-2',
              'text-isi text-teks-utama placeholder:text-teks-redup',
              'focus:border-utama focus:outline-none focus:ring-2 focus:ring-utama/30',
            )}
          />
        </div>
        {/* Tombol pindai hanya muncul bila perangkatnya memang punya kamera —
            izin & kemampuan menyembunyikan, bukan menonaktifkan (ui/01 §3). */}
        {bisaMemindai() && (
          <Tombol
            jenis="kedua"
            ukuran="normal"
            onClick={() => setBukaPindai(true)}
            aria-label="Pindai barcode"
            // Di HP cukup selebar ikonnya: tiap piksel yang dihemat di sini
            // kembali ke kotak pencarian (placeholder "Cari barang" utuh di 390px).
            className="max-sm:w-12 max-sm:px-0"
          >
            <ScanLine className="h-5 w-5" aria-hidden />
            <span className="hidden sm:inline">Pindai</span>
          </Tombol>
        )}
        {/* Tagihan terbuka: di HP cukup ikon + lencana jumlah supaya kotak
            pencarian tetap lega; di layar lebar ditulis lengkap. */}
        <Tombol
          jenis="kedua"
          ukuran="normal"
          onClick={() => {
            setBukaTagihan(true)
            // Langsung segarkan: pelayan di HP lain mungkin baru menambah tagihan.
            if (navigator.onLine) void tagihan.segarkan()
          }}
          aria-label={`Tagihan terbuka, ${tagihan.daftar.length}`}
          className="relative max-sm:w-12 max-sm:px-0"
        >
          <ClipboardList className="h-5 w-5" aria-hidden />
          <span className="hidden sm:inline">Tagihan</span>
          {tagihan.daftar.length > 0 && (
            <span
              aria-hidden
              className="flex h-6 min-w-6 items-center justify-center rounded-full bg-utama px-1.5 text-keterangan font-bold text-utama-teks max-sm:absolute max-sm:-right-2 max-sm:-top-2"
            >
              {tagihan.daftar.length}
            </span>
          )}
        </Tombol>
        {/* Label memendek di HP. Diukur pada 390px: "Tutup Shift" utuh menyisakan
            hanya ~93px untuk teks kotak pencarian, dan placeholder-nya terpotong
            jadi "Cari baran". Kotak pencarian adalah cara utama menemukan barang
            saat katalognya panjang, jadi ia yang diberi ruang. */}
        <Tombol jenis="kedua" ukuran="normal" asChild className="max-sm:px-4">
          <Link to="/kasir/tutup-shift" aria-label="Tutup shift">
            <span className="sm:hidden">Shift</span>
            <span className="hidden sm:inline">Tutup Shift</span>
          </Link>
        </Tombol>
      </header>

      {/* Baris kategori (ui/05-ALUR-UTAMA.md §2). Muncul hanya bila memang ada
          lebih dari satu kategori — satu tab "Semua" sendirian tidak menyaring
          apa pun dan cuma memakan tinggi layar. */}
      {daftarKategori.length > 1 && (
        <div className="shrink-0 border-b border-garis bg-permukaan px-4 pb-3">
          <SegmenPilihan
            label="Kategori barang"
            nilai={kategori ?? 'semua'}
            onPilih={(v) => setKategori(v === 'semua' ? undefined : v)}
            pilihan={[
              ['semua', 'Semua'] as const,
              ...daftarKategori.map((k) => [k.id, k.nama] as const),
            ]}
          />
        </div>
      )}

      <div className="flex min-h-0 flex-1">
        <main className="flex min-w-0 flex-1 flex-col">
          <div className="flex-1 overflow-y-auto p-4">
            {memuat ? (
              <GridKerangka />
            ) : produk.length === 0 ? (
              <KeadaanKosong
                ikon={PackageX}
                judul={
                  cari
                    ? 'Barang tidak ditemukan'
                    : kosong
                      ? 'Daftar barang belum tersalin ke perangkat ini'
                      : 'Belum ada barang'
                }
                penjelasan={
                  cari
                    ? `Tidak ada barang bernama "${cari}". Coba kata lain, atau tambahkan barangnya dulu.`
                    : kosong
                      ? 'Sambungkan ke internet sebentar supaya daftar barang bisa disalin. Setelah itu kasir bisa dipakai walau sinyal hilang.'
                      : 'Tambahkan barang dulu supaya bisa mulai berjualan.'
                }
                aksi={
                  cari
                    ? { label: 'Hapus pencarian', onKlik: () => setCari('') }
                    : kategori
                      ? { label: 'Lihat semua kategori', onKlik: () => setKategori(undefined) }
                      : undefined
                }
              />
            ) : (
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
                {produk.map((p) => (
                  <KartuProduk
                    key={p.id}
                    produk={p}
                    stok={petaStok.get(p.id)}
                    diKeranjang={keranjang.qtyDari(p.id)}
                    kelasWarna={kelasPetak(warnaKategori, p.category_id)}
                    onPilih={(x) => {
                      // TIDAK ada toast di sini, sengaja. Ketukannya sudah
                      // dijawab tiga kali dan seketika: lencana jumlah muncul
                      // di kartunya, barisnya masuk daftar keranjang, dan
                      // totalnya berubah. Toast tambahan hanya mengulang yang
                      // sudah terlihat — dan karena kasir menekan barang
                      // bertubi-tubi, toastnya menumpuk sampai menutupi layar
                      // beserta tombol bayar di baliknya. Bandingkan dengan
                      // jalur pindai di bawah: di sana dialog kamera menutupi
                      // layar, jadi toast memang satu-satunya umpan balik.
                      if (x.varian?.length) setPilihVarian(x)
                      else keranjang.tambah(x)
                    }}
                  />
                ))}
              </div>
            )}
          </div>

          <footer className="flex shrink-0 flex-wrap items-center justify-between gap-2 border-t border-garis bg-permukaan px-4 py-2">
            <StatusKoneksi menunggu={sinkron.menunggu} />
            <p className="text-keterangan text-teks-redup">
              Modal awal {formatRupiah(shift.opening_cash)} · Shift dibuka{' '}
              {formatJam(shift.opened_at)}
            </p>
          </footer>
        </main>

        {/* Layar lebar: keranjang menetap di kanan. */}
        <div className="hidden w-96 shrink-0 lg:block">
          <PanelKeranjang keranjang={keranjang} onBayar={bukaLayarBayar} onTahan={tekanTahan} menahan={menahan} />
        </div>
      </div>

      {/* HP: tombol keranjang selalu terlihat, tidak disembunyikan di balik ikon. */}
      {keranjang.jumlahBaris > 0 && (
        <div className="shrink-0 border-t border-garis bg-permukaan p-3 lg:hidden">
          <Tombol
            ukuran="kasir"
            lebarPenuh
            onClick={() => setBukaKeranjangHP(true)}
            className="justify-between"
          >
            <span className="flex items-center gap-2">
              <ShoppingCart className="h-5 w-5" aria-hidden />
              {keranjang.jumlahBaris} barang
            </span>
            <span className="tabular-nums">{formatRupiah(keranjang.pratinjauTotal)}</span>
          </Tombol>
        </div>
      )}

      {bukaKeranjangHP && (
        <div className="fixed inset-0 z-40 flex flex-col bg-permukaan lg:hidden">
          <PanelKeranjang
            keranjang={keranjang}
            onBayar={bukaLayarBayar}
            onTahan={tekanTahan}
            menahan={menahan}
            onTutup={() => setBukaKeranjangHP(false)}
          />
        </div>
      )}

      <PemindaiBarcode
        terbuka={bukaPindai}
        onTutup={() => setBukaPindai(false)}
        keterangan="Arahkan kamera ke barcode. Barang langsung masuk keranjang."
        onKode={async (kode) => {
          // Sengaja TIDAK menutup dialog setelah berhasil: kasir biasanya
          // memindai beberapa barang berturut-turut, dan menutup-buka kamera
          // tiap barang membuat alurnya jauh lebih lambat.
          // Barcode dus langsung jadi baris kemasannya ("1 dus").
          const kenaKemasan = await kemasanDariBarcode(kode)
          if (kenaKemasan) {
            keranjang.tambah(kenaKemasan.produk, '1', undefined, kenaKemasan.kemasan)
            toast.tampilkan(`${kenaKemasan.produk.name} — 1 ${kenaKemasan.kemasan.unit_name} ditambahkan`, 'berhasil')
            return true
          }
          // Barcode/SKU varian langsung jadi barisnya ("Kopi (Besar)").
          const kenaVarian = await varianDariBarcode(kode)
          if (kenaVarian) {
            keranjang.tambah(kenaVarian.produk, '1', kenaVarian.varian)
            toast.tampilkan(`${namaBaris(kenaVarian)} ditambahkan`, 'berhasil')
            return true
          }
          const p = await produkDariBarcode(kode)
          if (!p) return false
          if (p.varian?.length) {
            // Barcode barang induk tidak menyebut variannya: kamera ditutup
            // dan kasir memilih dulu.
            setBukaPindai(false)
            setPilihVarian(p)
            return true
          }
          keranjang.tambah(p)
          // Dipertahankan: dialog kamera sedang menutupi keranjang & lencana,
          // jadi tanpa ini pemindaian tidak terjawab sama sekali (ui/01 §5).
          toast.tampilkan(`${p.name} ditambahkan`, 'berhasil')
          return true
        }}
      />

      {pilihVarian && (
        <DialogPilihVarian
          produk={pilihVarian}
          qtyPerVarian={(vid) =>
            keranjang.baris.find((b) => b.kunci === kunciBaris(pilihVarian.id, vid))?.qty ?? '0'
          }
          onPilih={(v) => {
            keranjang.tambah(pilihVarian, '1', v)
            setPilihVarian(null)
          }}
          onTutup={() => setPilihVarian(null)}
        />
      )}

      <LayarBayar
        terbuka={bukaBayar}
        onTutup={() => setBukaBayar(false)}
        total={keranjang.pratinjauTotal}
        mengirim={checkout.isPending}
        galat={galatBayar}
        onSelesai={selesaikan}
        pelanggan={daftarPelanggan}
        pelangganTetap={keranjang.pelanggan?.id}
        kunciQris={
          punyaFitur(FITUR.qris) ? undefined : `Paket ${paketUntuk(FITUR.qris) ?? 'berbayar'}`
        }
      />

      {bukaTahan && <DialogTahan onSimpan={tahan} onTutup={() => setBukaTahan(false)} />}
      {bukaTagihan && (
        <DialogTagihan
          daftar={tagihan.daftar}
          aturan={rincianToko}
          keranjangBerisi={keranjang.jumlahBaris > 0}
          onBuka={(t) => void bukaTagihanDiKeranjang(t)}
          onBatalkan={async (t) => {
            const { diantre } = await batalkanTagihan(t)
            sinkron.segarkan()
            toast.berhasil(diantre ? `Tagihan ${t.label} dibatalkan — dikirim saat online.` : `Tagihan ${t.label} dibatalkan.`)
          }}
          onTutup={() => setBukaTagihan(false)}
        />
      )}

      {struk && (
        <Struk
          transaksi={struk.transaksi}
          menungguDikirim={struk.diantre}
          terbuka
          onTutup={() => setStruk(null)}
          onTransaksiBaru={() => setStruk(null)}
        />
      )}
    </div>
  )
}

function GridKerangka() {
  return (
    <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5">
      {Array.from({ length: 10 }, (_, i) => (
        <Kerangka key={i} className="h-30" />
      ))}
    </div>
  )
}

function KerangkaKasir() {
  return (
    <div className={cn('flex h-dvh flex-col gap-4 p-4')}>
      <Kerangka className="h-12 w-full" />
      <GridKerangka />
    </div>
  )
}
