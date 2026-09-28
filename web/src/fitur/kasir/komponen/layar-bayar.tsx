import { useEffect, useMemo, useState } from 'react'
import { Banknote, Lock, NotebookPen, QrCode, X } from 'lucide-react'
import { Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { Tombol } from '@/bersama/ui/tombol'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { Pilihan } from '@/bersama/ui/pilihan'
import { formatAngka, formatRupiah } from '@/bersama/util/uang'
import { cn } from '@/bersama/util/cn'
import type { MetodeBayar } from '@/bersama/tipe/pos'
import { ikonMetode, namaMetode } from '../label-transaksi'

/**
 * Layar bayar (ui/05-ALUR-UTAMA.md §2).
 *
 * KEMBALIAN adalah angka terbesar di layar — itu yang dibutuhkan kasir dalam
 * tekanan antrean, bukan nomor struk.
 *
 * Bayar gabungan ("sebagian tunai, sisanya QRIS") tidak punya mode terpisah:
 * alur biasa tetap satu ketukan, dan pembagian muncul tepat saat dibutuhkan —
 * tombol "Sisanya pakai cara lain" ketika uang tunai kurang, atau tautan
 * "Sebagian saja" di QRIS. Bagian yang sudah dibayar berbaris di bawah total
 * beserta SISA-nya; cara bayar berikutnya selalu bekerja terhadap sisa itu.
 *
 * Aturan server yang dijaga di sini (services.resolvePayments):
 *   - kasbon harus pas → kasbon hanya bisa jadi PELUNAS sisa, tidak pernah
 *     bagian di tengah; pembayaran lain sebelumnya boleh;
 *   - kembalian hanya masuk akal dari uang tunai → hanya bagian terakhir yang
 *     tunai boleh melebihi sisa; bagian non-tunai tidak boleh melebihi sisa.
 */

/** Satu bagian pembayaran, bentuk yang dikirim ke server. */
export interface BagianBayar {
  method: MetodeBayar
  amount: number
}

/** Pelanggan yang boleh dicatati kasbon — dari basis data offline kasir. */
export interface PelangganBayar {
  id: string
  name: string
  phone: string | null
  credit_limit: number
}

const CARA_BAYAR: { nilai: MetodeBayar; label: string; ikon: typeof Banknote }[] = [
  { nilai: 'cash', label: 'Tunai', ikon: Banknote },
  { nilai: 'qris', label: 'QRIS', ikon: QrCode },
  { nilai: 'credit', label: 'Kasbon', ikon: NotebookPen },
]

export function LayarBayar({
  terbuka,
  onTutup,
  total,
  mengirim,
  galat,
  onSelesai,
  kunciQris,
  pelanggan = [],
}: {
  terbuka: boolean
  onTutup: () => void
  /** Pratinjau total dari keranjang. Angka final tetap dari server. */
  total: number
  mengirim: boolean
  galat?: string | null
  /**
   * Semua bagian pembayaran, berurutan. `pelangganId` diisi untuk kasbon —
   * utangnya dicatat atas nama pelanggan itu.
   */
  onSelesai: (pembayaran: BagianBayar[], pelangganId?: string) => void
  /**
   * Diisi bila QRIS tidak termasuk paket langganan: teks paket yang
   * membukanya, mis. "Paket Basic". Petaknya tetap ada tapi tidak bisa
   * dipilih — kalau dihilangkan, kasir yang biasa melihatnya mengira
   * aplikasinya rusak; kalau bisa ditekan, server menolaknya di tengah antrean.
   */
  kunciQris?: string
  /**
   * Daftar pelanggan untuk kasbon. Dulu kasbon tidak menanyakan siapa
   * pembelinya sama sekali — server lalu menolaknya ("pembayaran kasbon
   * membutuhkan pelanggan"), dan karena kasir bekerja offline, penolakan itu
   * baru ketahuan saat sinkron, jauh setelah pembelinya pergi.
   */
  pelanggan?: PelangganBayar[]
}) {
  const [metode, setMetode] = useState<MetodeBayar>('cash')
  const [diterima, setDiterima] = useState(0)
  const [pelangganId, setPelangganId] = useState('')
  const [bagian, setBagian] = useState<BagianBayar[]>([])
  // QRIS sebagian: null = pas sebesar sisa (alur biasa).
  const [jumlahSebagian, setJumlahSebagian] = useState<number | null>(null)
  const pembeli = pelanggan.find((p) => p.id === pelangganId)

  // Tiap kali dibuka mulai bersih. Transaksi yang berhasil menutup dialog dari
  // LUAR (bukan lewat tutupDanReset), dan dulu uang diterima transaksi
  // sebelumnya ikut terbawa ke pembeli berikutnya.
  useEffect(() => {
    if (!terbuka) return
    setDiterima(0)
    setBagian([])
    setJumlahSebagian(null)
    setMetode('cash')
    setPelangganId('')
  }, [terbuka])

  const sudahDibayar = bagian.reduce((j, b) => j + b.amount, 0)
  const sisa = total - sudahDibayar
  // Kasbon dan QRIS pas sebesar sisa (kecuali QRIS sebagian): tidak ada uang
  // fisik yang dikembalikan.
  const dibayar = metode === 'cash' ? diterima : metode === 'qris' && jumlahSebagian !== null ? jumlahSebagian : sisa
  const kembalian = Math.max(0, dibayar - sisa)
  const kurang = Math.max(0, sisa - dibayar)
  const qrisTerkunci = !!kunciQris

  // Pintasan nominal yang benar-benar dipakai di laci, dibulatkan ke atas dari
  // sisa — bukan daftar tetap yang sering tidak relevan.
  const pintasan = useMemo(() => {
    const kandidat = new Set<number>([sisa])
    for (const kelipatan of [5000, 10000, 50000, 100000]) {
      const naik = Math.ceil(sisa / kelipatan) * kelipatan
      if (naik > sisa) kandidat.add(naik)
    }
    return [...kandidat].sort((a, b) => a - b).slice(0, 4)
  }, [sisa])

  function pilihMetode(m: MetodeBayar) {
    setMetode(m)
    setJumlahSebagian(null)
  }

  /** Simpan yang sudah dibayar sebagai satu bagian, lalu pindah ke cara lain untuk sisanya. */
  function tambahBagian(jumlah: number) {
    if (jumlah <= 0 || jumlah >= sisa) return
    setBagian((b) => [...b, { method: metode, amount: jumlah }])
    setDiterima(0)
    // Tunai sebagian → sisanya biasanya QRIS; QRIS sebagian → sisanya tunai.
    pilihMetode(metode === 'cash' && !qrisTerkunci ? 'qris' : 'cash')
  }

  function tutupDanReset() {
    setDiterima(0)
    setBagian([])
    setJumlahSebagian(null)
    setMetode('cash')
    onTutup()
  }

  const bolehSelesai =
    sisa > 0 &&
    (metode === 'cash'
      ? kurang === 0
      : metode === 'credit'
        ? !!pembeli
        : jumlahSebagian === null || jumlahSebagian === sisa)

  return (
    <Dialog open={terbuka} onOpenChange={(o) => !o && !mengirim && tutupDanReset()}>
      <IsiDialog judul="Bayar" className="sm:max-w-lg">
        {/* Setelah ada bagian yang dibayar, SISA yang jadi angka utama;
            total mengecil supaya layar tidak memanjang. */}
        <div
          className={cn(
            'flex items-baseline justify-between',
            bagian.length > 0 ? '-mb-2' : 'border-b border-garis pb-3',
          )}
        >
          <span className="text-isi text-teks-sekunder">Total belanja</span>
          <span
            className={cn(
              'tabular-nums',
              bagian.length > 0 ? 'text-isi font-semibold text-teks-sekunder' : 'text-judul font-extrabold text-teks-utama',
            )}
          >
            {formatRupiah(total)}
          </span>
        </div>

        {bagian.length > 0 && (
          <div className="-mt-1 flex flex-col border-b border-garis pb-2">
            <ul aria-label="Sudah dibayar">
              {bagian.map((b, i) => {
                const Ikon = ikonMetode(b.method)
                return (
                  <li key={i} className="flex items-center gap-2 text-label text-teks-sekunder">
                    <Ikon className="h-4 w-4 shrink-0" aria-hidden />
                    <span className="flex-1">{namaMetode(b.method)}</span>
                    <span className="tabular-nums">{formatRupiah(b.amount)}</span>
                    <button
                      type="button"
                      onClick={() => setBagian((l) => l.filter((_, j) => j !== i))}
                      aria-label={`Batalkan bagian ${namaMetode(b.method)} ${formatRupiah(b.amount)}`}
                      className="-mr-2 flex h-11 w-11 items-center justify-center rounded-kontrol text-teks-redup hover:bg-permukaan-2"
                    >
                      <X className="h-4 w-4" aria-hidden />
                    </button>
                  </li>
                )
              })}
            </ul>
            <div className="flex items-baseline justify-between">
              <span className="text-isi font-semibold text-teks-utama">Sisa</span>
              <span className="text-judul font-extrabold tabular-nums text-teks-utama">{formatRupiah(sisa)}</span>
            </div>
          </div>
        )}

        <fieldset className="flex flex-col gap-2">
          <legend className="mb-1 text-label font-medium text-teks-sekunder">
            Cara bayar
          </legend>
          <div className="grid grid-cols-3 gap-2">
            {CARA_BAYAR.map((c) => {
              const terkunci = c.nilai === 'qris' && !!kunciQris
              return (
                <button
                  key={c.nilai}
                  type="button"
                  disabled={terkunci}
                  onClick={() => pilihMetode(c.nilai)}
                  aria-pressed={metode === c.nilai}
                  aria-label={terkunci ? `${c.label}, terkunci — tersedia di ${kunciQris}` : undefined}
                  className={cn(
                    'flex h-16 flex-col items-center justify-center gap-1 rounded-kartu border sm:h-20',
                    terkunci
                      ? 'cursor-not-allowed border-dashed border-garis bg-permukaan-2 text-teks-redup'
                      : metode === c.nilai
                        ? 'border-utama bg-sorot text-utama'
                        : 'border-garis bg-permukaan text-teks-sekunder hover:bg-permukaan-2',
                  )}
                >
                  {terkunci ? (
                    <Lock className="h-5 w-5" aria-hidden />
                  ) : (
                    <c.ikon className="h-6 w-6" aria-hidden />
                  )}
                  <span className="text-label font-semibold">{c.label}</span>
                  {terkunci && <span className="text-keterangan leading-none">{kunciQris}</span>}
                </button>
              )
            })}
          </div>
        </fieldset>

        {metode === 'cash' && (
          <div className="flex flex-col gap-2">
            {/* Penjelasnya ada di label, bukan baris bantuan: satu baris yang
                dihemat di sini yang membuat SELESAI tetap terlihat di HP. */}
            <KolomUang label="Uang diterima dari pembeli" nilai={diterima} onNilai={setDiterima} autoFocus />
            {/* Satu baris, tanpa "Rp": empat pintasan yang melipat jadi dua
                baris mendorong SELESAI ke luar layar HP. Konteksnya sudah uang. */}
            <div className="grid grid-cols-4 gap-2">
              {pintasan.map((n) => (
                <button
                  key={n}
                  type="button"
                  onClick={() => setDiterima(n)}
                  aria-label={n === sisa ? undefined : formatRupiah(n)}
                  className="h-12 rounded-kontrol border border-garis bg-permukaan px-1 text-label font-semibold tabular-nums text-teks-utama hover:bg-permukaan-2"
                >
                  {n === sisa ? 'Pas' : formatAngka(n)}
                </button>
              ))}
            </div>
          </div>
        )}

        {metode === 'qris' &&
          (jumlahSebagian === null ? (
            <div className="flex flex-wrap items-center justify-between gap-x-3 text-label text-teks-sekunder">
              <span>
                Pembeli membayar <strong className="tabular-nums text-teks-utama">{formatRupiah(sisa)}</strong> lewat
                QRIS.
              </span>
              <button
                type="button"
                onClick={() => setJumlahSebagian(sisa)}
                className="-mx-2 min-h-11 rounded-kontrol px-2 font-medium text-utama hover:bg-sorot"
              >
                Sebagian saja
              </button>
            </div>
          ) : (
            <div className="flex flex-col gap-2">
              <KolomUang
                label="Jumlah lewat QRIS"
                nilai={jumlahSebagian}
                onNilai={setJumlahSebagian}
                galat={jumlahSebagian > sisa ? `Paling banyak sisa belanja, ${formatRupiah(sisa)}.` : undefined}
                bantuan={
                  jumlahSebagian > 0 && jumlahSebagian < sisa
                    ? `Sisa ${formatRupiah(sisa - jumlahSebagian)} dibayar dengan cara lain.`
                    : 'Isi yang dibayar lewat QRIS; sisanya dengan cara lain.'
                }
                autoFocus
              />
              <Tombol
                jenis="kedua"
                disabled={jumlahSebagian <= 0 || jumlahSebagian >= sisa}
                onClick={() => tambahBagian(jumlahSebagian)}
              >
                Tambahkan — sisanya cara lain
              </Tombol>
            </div>
          ))}

        {metode === 'credit' && (
          <div className="flex flex-col gap-3">
            {pelanggan.length === 0 ? (
              <p className="rounded-kontrol border border-jingga-600 bg-permukaan-2 px-3 py-2 text-label text-jingga-700">
                Belum ada pelanggan. Kasbon harus dicatat atas nama seseorang — tambahkan
                pelanggannya dulu di menu Pelanggan.
              </p>
            ) : (
              <Pilihan
                label="Pelanggan"
                value={pelangganId}
                onChange={(e) => setPelangganId(e.target.value)}
                placeholder="Pilih siapa yang berutang…"
                bantuan={
                  pembeli && pembeli.credit_limit > 0
                    ? `Batas kasbon ${pembeli.name}: ${formatRupiah(pembeli.credit_limit)}.`
                    : undefined
                }
                required
              >
                {pelanggan.map((p) => (
                  <option key={p.id} value={p.id}>
                    {p.phone ? `${p.name} · ${p.phone}` : p.name}
                  </option>
                ))}
              </Pilihan>
            )}
            <p className="rounded-kontrol border border-jingga-600 bg-permukaan-2 px-3 py-2 text-label text-jingga-700">
              {bagian.length > 0 ? 'Sisa belanja' : 'Belanja ini'} dicatat sebagai utang
              {pembeli ? ` ${pembeli.name}` : ' pelanggan'} sebesar {formatRupiah(sisa)}.
            </p>
          </div>
        )}

        {/* Kembalian: angka paling besar di layar.
            Baru muncul setelah kasir mengisi sesuatu — menyambut layar dengan
            "Uang belum cukup" padahal belum sempat mengetik itu menuduh tanpa
            sebab, dan nada seperti itu yang membuat orang berhenti percaya. */}
        {metode === 'cash' && diterima > 0 && (
          <div
            className={cn(
              'rounded-kartu border-2 px-4 py-2',
              kurang > 0 ? 'border-jingga-600 bg-permukaan-2' : 'border-utama bg-sorot',
            )}
          >
            {/* Label dan angka sebaris: angkanya tetap yang terbesar di layar,
                tanpa menambah tinggi yang mendorong SELESAI keluar layar HP. */}
            <div className="flex flex-wrap items-baseline justify-between gap-x-3">
              <p className="text-label font-medium text-teks-sekunder">
                {kurang > 0 ? 'Uang belum cukup' : 'KEMBALIAN'}
              </p>
              <p
                className={cn(
                  'text-angka font-extrabold tabular-nums',
                  kurang > 0 ? 'text-jingga-700' : 'text-teks-utama',
                )}
              >
                {formatRupiah(kurang > 0 ? kurang : kembalian)}
              </p>
            </div>
            {kurang > 0 && (
              <button
                type="button"
                onClick={() => tambahBagian(diterima)}
                className="-mx-2 -mb-1 min-h-11 rounded-kontrol px-2 text-label font-semibold text-utama hover:bg-sorot"
              >
                Sisanya pakai cara lain
              </button>
            )}
          </div>
        )}

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <Tombol
          ukuran="kasir"
          lebarPenuh
          memuat={mengirim}
          labelMemuat="Menyimpan transaksi…"
          disabled={!bolehSelesai}
          onClick={() =>
            onSelesai([...bagian, { method: metode, amount: dibayar }], metode === 'credit' ? pelangganId : undefined)
          }
        >
          SELESAI
        </Tombol>
      </IsiDialog>
    </Dialog>
  )
}
