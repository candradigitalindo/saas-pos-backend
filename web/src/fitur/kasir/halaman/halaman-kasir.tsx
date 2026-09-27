import { useRef, useState } from 'react'
import { Link } from 'react-router-dom'
import { useQueryClient } from '@tanstack/react-query'
import { PackageX, ScanLine, Search, ShoppingCart, X } from 'lucide-react'
import { Tombol } from '@/bersama/ui/tombol'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { Kerangka } from '@/bersama/komponen/kerangka'
import { StatusKoneksi } from '@/bersama/komponen/status-koneksi'
import { PemindaiBarcode } from '@/bersama/komponen/pemindai-barcode'
import { bisaMemindai } from '@/bersama/hooks/use-pemindai'
import { produkDariBarcode } from '@/lib/offline/katalog-lokal'
import { useSinkron } from '@/lib/offline/mesin'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatJam } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import type { MetodeBayar, Transaksi } from '@/bersama/tipe/pos'
import { KartuProduk } from '../komponen/kartu-produk'
import { PanelKeranjang } from '../komponen/panel-keranjang'
import { SegmenPilihan } from '@/bersama/ui/segmen'
import { LayarBayar } from '../komponen/layar-bayar'
import { Struk } from '../komponen/struk'
import { itemUntukCheckout, useKeranjang } from '../keranjang'
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
  const { tokoAktif, rincianToko, punyaFitur, paketUntuk } = useSesi()
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
  const [bukaBayar, setBukaBayar] = useState(false)
  const [bukaKeranjangHP, setBukaKeranjangHP] = useState(false)
  const [struk, setStruk] = useState<{ transaksi: Transaksi; diantre: boolean } | null>(null)
  const [galatBayar, setGalatBayar] = useState<string | null>(null)

  const checkout = useCheckout()

  /**
   * Kunci idempotensi dibuat SEKALI saat layar bayar dibuka, lalu dipakai untuk
   * setiap percobaan sampai transaksi benar-benar tersimpan. Inilah yang
   * membuat menekan "Selesai" dua kali tidak menghasilkan dua transaksi.
   */
  const kunci = useRef<string | null>(null)

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

  async function selesaikan(metode: MetodeBayar, dibayar: number) {
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
          // Kasbon dicatat penuh sebagai utang; tunai memakai uang yang
          // benar-benar diterima supaya server yang menghitung kembaliannya.
          payments: [{ method: metode, amount: dibayar }],
        },
      })
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

  return (
    <div className="flex h-dvh flex-col bg-latar">
      {/* Di bawah 360px, tiga kendali plus kotak pencarian memang tidak muat
          dalam satu baris — kotak pencariannya turun ke 69px dan placeholder-nya
          terpotong. Di lebar itu saja kotaknya mengambil barisnya sendiri. */}
      <header className="flex shrink-0 flex-wrap items-center gap-3 border-b border-garis bg-permukaan px-4 py-3">
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
              'h-12 w-full rounded-kontrol border border-garis bg-permukaan pl-10 pr-3',
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
          >
            <ScanLine className="h-5 w-5" aria-hidden />
            <span className="hidden sm:inline">Pindai</span>
          </Tombol>
        )}
        {/* Label memendek di HP. Diukur pada 390px: "Tutup Shift" utuh menyisakan
            hanya ~93px untuk teks kotak pencarian, dan placeholder-nya terpotong
            jadi "Cari baran". Kotak pencarian adalah cara utama menemukan barang
            saat katalognya panjang, jadi ia yang diberi ruang. */}
        <Tombol jenis="kedua" ukuran="normal" asChild>
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
                      keranjang.tambah(x)
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
          <PanelKeranjang keranjang={keranjang} onBayar={bukaLayarBayar} />
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
          <div className="flex items-center justify-between border-b border-garis px-4 py-3">
            <h2 className="text-judul-kartu font-semibold text-teks-utama">Keranjang</h2>
            <button
              type="button"
              onClick={() => setBukaKeranjangHP(false)}
              aria-label="Tutup keranjang"
              className="-m-2 rounded-kontrol p-2 text-teks-redup hover:bg-permukaan-2"
            >
              <X className="h-6 w-6" aria-hidden />
            </button>
          </div>
          <div className="min-h-0 flex-1">
            <PanelKeranjang keranjang={keranjang} onBayar={bukaLayarBayar} />
          </div>
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
          const p = await produkDariBarcode(kode)
          if (!p) return false
          keranjang.tambah(p)
          // Dipertahankan: dialog kamera sedang menutupi keranjang & lencana,
          // jadi tanpa ini pemindaian tidak terjawab sama sekali (ui/01 §5).
          toast.tampilkan(`${p.name} ditambahkan`, 'berhasil')
          return true
        }}
      />

      <LayarBayar
        terbuka={bukaBayar}
        onTutup={() => setBukaBayar(false)}
        total={keranjang.pratinjauTotal}
        mengirim={checkout.isPending}
        galat={galatBayar}
        onSelesai={selesaikan}
        kunciQris={
          punyaFitur(FITUR.qris) ? undefined : `Paket ${paketUntuk(FITUR.qris) ?? 'berbayar'}`
        }
      />

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
