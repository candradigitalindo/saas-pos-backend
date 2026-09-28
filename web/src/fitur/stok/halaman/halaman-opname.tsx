import { useDeferredValue, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { keepPreviousData, useInfiniteQuery, useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, ClipboardList, History, Search, TriangleAlert } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { SegmenPilihan } from '@/bersama/ui/segmen'
import { StepperJumlah } from '@/bersama/ui/stepper-jumlah'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { FotoBarang } from '@/bersama/komponen/foto-barang'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useKategori } from '@/bersama/hooks/use-katalog'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { bandingQty, formatQty, formatQtySatuan } from '@/bersama/util/desimal'
import { kelasPetak, petaWarnaKategori } from '@/bersama/util/warna-kategori'
import { cn } from '@/bersama/util/cn'
import type { SaldoStok } from '@/bersama/tipe/katalog'
import { stokApi, type KeadaanStok } from '../api'
import { keadaanSaldo } from '../keadaan-stok'
import { DialogRiwayatHitung, formatSelisihRupiah } from '../komponen/dialog-riwayat-hitung'

/** Baris per muatan di langkah 1. */
const PER_MUAT = 50

interface Hitungan {
  saldo: SaldoStok
  dihitung: string
  dilewati: boolean
}

type Saring = 'semua' | Extract<KeadaanStok, 'negative' | 'out' | 'low'>

const selisihDari = (h: Hitungan) => Number.parseFloat(h.dihitung) - Number.parseFloat(h.saldo.qty)
/**
 * ≈ perubahan NILAI STOK menurut harga modal sekarang. Catatan minus dihitung
 * nol — sama dengan nilai stok di halaman Stok — jadi mencocokkan catatan
 * "−10" menjadi 7 bernilai +7 barang, bukan +17: itu koreksi catatan, bukan
 * untung. Selisih JUMLAH-nya tetap ditampilkan apa adanya.
 */
const nilaiSelisih = (h: Hitungan) =>
  Math.round(
    (Number.parseFloat(h.dihitung) - Math.max(Number.parseFloat(h.saldo.qty), 0)) * (h.saldo.cost_price ?? 0),
  )

/**
 * Hitung fisik (opname) — dikerjakan sambil berdiri di depan rak
 * (ui/05-ALUR-UTAMA.md §5).
 *
 * Langkah 1 memilih barang: dicari & disaring DI SERVER (versi sebelumnya
 * hanya memuat 100 barang pertama dan mencari di antaranya, jadi barang ke-101
 * dst. tidak bisa dihitung). Barang minus — catatan yang pasti salah — punya
 * jalan pintas sendiri: itulah alasan paling umum orang menghitung fisik.
 *
 * Langkah 2 satu barang satu layar. Catatan sistem DITAMPILKAN, tidak
 * disembunyikan: jujur lebih baik daripada "menguji" petugas, dan petugas yang
 * merasa diuji akan mulai menyalin angka sistem.
 *
 * Selisih disebut juga dalam RUPIAH (harga modal): "kurang 3" tidak terasa
 * apa-apa, "≈ Rp 45.000 hilang" langsung terasa.
 */
export function HalamanOpname() {
  const { tokoAktif } = useSesi()
  const navigate = useNavigate()
  const toast = useToast()
  const qc = useQueryClient()
  const kat = useKategori()
  const warna = petaWarnaKategori(kat.data?.data ?? [])

  const [langkah, setLangkah] = useState<1 | 2 | 3>(1)
  const [dipilih, setDipilih] = useState<Map<string, SaldoStok>>(new Map())
  const [hitungan, setHitungan] = useState<Hitungan[]>([])
  const [ke, setKe] = useState(0)
  const [konfirmasi, setKonfirmasi] = useState(false)
  const [bukaRiwayat, setBukaRiwayat] = useState(false)
  const [galat, setGalat] = useState<string | null>(null)

  const simpan = useMutation({
    mutationFn: async () => {
      const dipakai = hitungan.filter((h) => !h.dilewati)
      const opname = await stokApi.buatOpname(tokoAktif!, 'Hitung fisik dari aplikasi')
      await stokApi.simpanHitungan(
        opname.id,
        dipakai.map((h) => ({ product_id: h.saldo.product_id, counted_qty: h.dihitung })),
      )
      return stokApi.postingOpname(opname.id)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['stok'] })
      qc.invalidateQueries({ queryKey: ['stok-ringkasan'] })
      qc.invalidateQueries({ queryKey: ['stok-kasir'] })
      qc.invalidateQueries({ queryKey: ['kartu-stok'] })
      qc.invalidateQueries({ queryKey: ['opname'] })
      toast.berhasil('Stok sudah disesuaikan mengikuti hitungan Anda.')
      navigate('/stok')
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  const riwayat = <DialogRiwayatHitung terbuka={bukaRiwayat} onTutup={() => setBukaRiwayat(false)} />

  // ── Langkah 1: pilih barang ──────────────────────────────────────────────
  if (langkah === 1) {
    return (
      <>
        <LangkahPilih
          dipilih={dipilih}
          setDipilih={setDipilih}
          onRiwayat={() => setBukaRiwayat(true)}
          onMulai={() => {
            setHitungan(
              [...dipilih.values()].map((s) => ({
                saldo: s,
                // Dimulai dari angka sistem: petugas biasanya mengoreksi
                // beberapa barang saja, bukan menghitung ulang dari nol. Catatan
                // minus dimulai dari 0 — rak tidak bisa berisi minus.
                dihitung: Number.parseFloat(s.qty) < 0 ? '0' : s.qty,
                dilewati: false,
              })),
            )
            setKe(0)
            setLangkah(2)
          }}
        />
        {riwayat}
      </>
    )
  }

  // ── Langkah 2: hitung, satu barang satu layar ────────────────────────────
  if (langkah === 2) {
    const h = hitungan[ke]
    if (!h) return null
    return (
      <LangkahHitung
        h={h}
        ke={ke}
        dari={hitungan.length}
        onUbah={(q) => setHitungan((l) => l.map((x, i) => (i === ke ? { ...x, dihitung: q } : x)))}
        onLanjut={(lewati) => {
          setHitungan((l) => l.map((x, i) => (i === ke ? { ...x, dilewati: lewati } : x)))
          if (ke + 1 < hitungan.length) setKe(ke + 1)
          else setLangkah(3)
        }}
        onKembali={ke > 0 ? () => setKe(ke - 1) : () => setLangkah(1)}
        adaSebelumnya={ke > 0}
      />
    )
  }

  // ── Langkah 3: tinjau & simpan ───────────────────────────────────────────
  const dipakai = hitungan.filter((h) => !h.dilewati)
  const berubah = dipakai.filter((h) => bandingQty(h.dihitung, h.saldo.qty) !== 0)
  const dilewati = hitungan.length - dipakai.length
  const totalNilai = berubah.reduce((j, h) => j + nilaiSelisih(h), 0)

  return (
    <div className="flex w-full max-w-lg flex-col gap-4">
      <Kepala langkah={3} judul="Tinjau & simpan" />

      <Kartu className="flex flex-col gap-3 p-4">
        <dl className="grid grid-cols-3 gap-2 text-center">
          <Angka label="Dihitung" nilai={dipakai.length} />
          <Angka label="Berubah" nilai={berubah.length} tegas={berubah.length > 0} />
          <Angka label="Dilewati" nilai={dilewati} />
        </dl>
        {berubah.length > 0 && (
          <p className="flex items-baseline justify-between gap-3 border-t border-garis pt-3 text-label text-teks-sekunder">
            <span>
              Nilai stok berubah
              <span className="block text-keterangan text-teks-redup">menurut harga modal</span>
            </span>
            <strong
              className={cn(
                'text-judul-kartu font-extrabold tabular-nums',
                totalNilai < 0 ? 'text-bahaya-teks' : totalNilai > 0 ? 'text-hijau-700' : 'text-teks-utama',
              )}
            >
              ≈ {formatSelisihRupiah(totalNilai)}
            </strong>
          </p>
        )}
      </Kartu>

      {berubah.length === 0 ? (
        <Kartu className="p-6 text-center">
          <p className="text-isi text-teks-sekunder">
            Semua hitungan Anda cocok dengan catatan sistem. Tidak ada yang perlu diubah.
          </p>
        </Kartu>
      ) : (
        <Kartu className="divide-y divide-garis">
          {berubah.map((h) => {
            const selisih = selisihDari(h)
            return (
              // Ketuk untuk kembali menghitung barang itu saja.
              <button
                key={h.saldo.product_id}
                type="button"
                onClick={() => {
                  setKe(hitungan.indexOf(h))
                  setLangkah(2)
                }}
                className="flex w-full items-center gap-3 p-3 text-left hover:bg-permukaan-2/50"
              >
                <FotoBarang
                  nama={h.saldo.product_name}
                  url={h.saldo.image_url}
                  kecil
                  kelasWarna={kelasPetak(warna, h.saldo.category_id)}
                  className="w-10"
                />
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium text-teks-utama">{h.saldo.product_name}</span>
                  <span className="block text-keterangan tabular-nums text-teks-redup">
                    catatan {formatQty(h.saldo.qty).replace('-', '−')} → dihitung {formatQty(h.dihitung)}{' '}
                    {h.saldo.unit_name}
                  </span>
                </span>
                <span className="shrink-0 text-right tabular-nums">
                  <span
                    className={cn('block font-bold', selisih > 0 ? 'text-hijau-700' : 'text-bahaya-teks')}
                  >
                    {selisih > 0 ? '+' : '−'}
                    {formatQty(String(Math.abs(selisih)))}
                  </span>
                  {nilaiSelisih(h) !== 0 && (
                    <span className="block text-keterangan text-teks-redup">
                      ≈ {formatSelisihRupiah(nilaiSelisih(h))}
                    </span>
                  )}
                </span>
              </button>
            )
          })}
        </Kartu>
      )}

      {galat && (
        <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
          {galat}
        </p>
      )}

      <div className="flex gap-2">
        <Tombol
          jenis="kedua"
          lebarPenuh
          onClick={() => {
            setKe(0)
            setLangkah(2)
          }}
        >
          ← Hitung ulang
        </Tombol>
        <Tombol lebarPenuh disabled={dipakai.length === 0} onClick={() => setKonfirmasi(true)}>
          Simpan Hasil
        </Tombol>
      </div>

      {/* Aksi berat dan tidak bisa dibalik: dialognya menyebut angka, bukan
          sekadar bertanya "Anda yakin?". */}
      <Dialog open={konfirmasi} onOpenChange={(o) => !o && setKonfirmasi(false)}>
        <IsiDialog judul="Simpan hasil hitungan?">
          <div className="flex flex-col gap-2 text-isi text-teks-sekunder">
            <p>Stok akan disesuaikan mengikuti hitungan Anda.</p>
            <p>
              <strong className="text-teks-utama">{berubah.length} barang</strong> berubah
              {berubah.length > 0 && (
                <>
                  , dari <strong className="text-teks-utama">{dipakai.length}</strong> yang dihitung
                </>
              )}
              {totalNilai !== 0 && (
                <>
                  {' '}
                  — nilai stok berubah ≈{' '}
                  <strong className="text-teks-utama">{formatSelisihRupiah(totalNilai)}</strong>
                </>
              )}
              .
            </p>
            <p>Tindakan ini tercatat di riwayat stok dan tidak bisa dibatalkan.</p>
          </div>

          <AksiDialog>
            <Tombol memuat={simpan.isPending} labelMemuat="Menyimpan…" onClick={() => simpan.mutate()}>
              Ya, Simpan
            </Tombol>
            <Tombol jenis="kedua" onClick={() => setKonfirmasi(false)} disabled={simpan.isPending}>
              Periksa lagi
            </Tombol>
          </AksiDialog>
        </IsiDialog>
      </Dialog>
    </div>
  )
}

// ── Langkah 1 ────────────────────────────────────────────────────────────────

function LangkahPilih({
  dipilih,
  setDipilih,
  onRiwayat,
  onMulai,
}: {
  dipilih: Map<string, SaldoStok>
  setDipilih: (f: (m: Map<string, SaldoStok>) => Map<string, SaldoStok>) => void
  onRiwayat: () => void
  onMulai: () => void
}) {
  const { tokoAktif } = useSesi()
  const navigate = useNavigate()
  const [saring, setSaring] = useState<Saring>('semua')
  const [cari, setCari] = useState('')
  const cariTunda = useDeferredValue(cari.trim())
  const kat = useKategori()
  const warna = petaWarnaKategori(kat.data?.data ?? [])

  const ringkasan = useQuery({
    queryKey: ['stok-ringkasan', tokoAktif],
    queryFn: () => stokApi.ringkasan(tokoAktif!),
    enabled: !!tokoAktif,
    staleTime: 30_000,
  })
  const r = ringkasan.data

  // Minus lebih dulu: itulah yang paling perlu dihitung ulang.
  const q = useInfiniteQuery({
    queryKey: ['stok', tokoAktif, 'opname', saring, cariTunda],
    queryFn: ({ pageParam }) =>
      stokApi.saldo(tokoAktif!, false, pageParam, PER_MUAT, {
        cari: cariTunda,
        keadaan: saring === 'semua' ? undefined : saring,
        urut: 'urgent',
      }),
    initialPageParam: 1,
    getNextPageParam: (akhir, semua) => (semua.length * PER_MUAT < akhir.total ? semua.length + 1 : undefined),
    enabled: !!tokoAktif,
    placeholderData: keepPreviousData,
  })
  const daftar = q.data?.pages.flatMap((h) => h.data) ?? []
  const total = q.data?.pages[0]?.total ?? 0

  const ubah = (s: SaldoStok, pilih: boolean) =>
    setDipilih((lama) => {
      const baru = new Map(lama)
      if (pilih) baru.set(s.product_id, s)
      else baru.delete(s.product_id)
      return baru
    })
  const pilihBanyak = (rows: SaldoStok[]) =>
    setDipilih((lama) => {
      const baru = new Map(lama)
      rows.forEach((s) => baru.set(s.product_id, s))
      return baru
    })

  // Jalan pintas "hitung ulang yang minus": semua barang minus sekaligus,
  // bukan hanya yang kebetulan termuat di layar.
  const pilihMinus = useMutation({
    mutationFn: () => stokApi.saldo(tokoAktif!, false, 1, 100, { keadaan: 'negative', urut: 'urgent' }),
    onSuccess: (h) => pilihBanyak(h.data),
  })
  const minusDipilih = [...dipilih.values()].filter((s) => keadaanSaldo(s) === 'negative').length
  const minusBelum = (r?.negative ?? 0) > minusDipilih

  const tidakAdaStok = !q.isLoading && !cariTunda && saring === 'semua' && total === 0

  return (
    <div className="flex w-full max-w-lg flex-col gap-4 lg:max-w-2xl">
      <div className="flex items-end justify-between gap-3">
        <Kepala langkah={1} judul="Pilih barang yang mau dihitung" />
        <Tombol jenis="kedua" onClick={onRiwayat} className="shrink-0 px-4" aria-label="Riwayat hitung fisik">
          <History className="h-5 w-5" aria-hidden />
          <span className="hidden sm:inline">Riwayat</span>
        </Tombol>
      </div>

      {tidakAdaStok ? (
        <KeadaanKosong
          ikon={ClipboardList}
          judul="Belum ada stok untuk dihitung"
          penjelasan="Catat barang masuk atau isi stok awal dulu, baru hitung fisik bisa dijalankan."
          aksi={{ label: 'Ke halaman Stok', onKlik: () => navigate('/stok') }}
        />
      ) : (
        <>
          {minusBelum && (
            <Kartu className="flex flex-col gap-3 border-bahaya-teks/30 bg-bahaya-teks/5 p-4 sm:flex-row sm:items-center">
              <p className="flex flex-1 items-start gap-2 text-label text-teks-utama">
                <TriangleAlert className="mt-0.5 h-4 w-4 shrink-0 text-bahaya-teks" aria-hidden />
                <span>
                  <strong className="font-semibold">{r!.negative} barang tercatat minus</strong> — catatannya pasti
                  tidak cocok dengan rak. Hitung yang ini dulu.
                </span>
              </p>
              <Tombol
                jenis="kedua"
                ukuran="padat"
                memuat={pilihMinus.isPending}
                onClick={() => pilihMinus.mutate()}
                className="self-start sm:self-auto"
              >
                Pilih {r!.negative} barang ini
              </Tombol>
            </Kartu>
          )}

          <div className="flex flex-col gap-2">
            <div className="relative">
              <Search
                className="pointer-events-none absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-teks-redup"
                aria-hidden
              />
              <input
                type="search"
                value={cari}
                onChange={(e) => setCari(e.target.value)}
                placeholder="Cari nama, SKU, atau barcode…"
                aria-label="Cari barang"
                className="h-12 w-full rounded-kontrol border border-garis bg-permukaan pl-10 pr-3 text-isi text-teks-utama placeholder:text-teks-redup"
              />
            </div>
            <SegmenPilihan
              label="Saring barang"
              nilai={saring}
              onPilih={setSaring}
              pilihan={[
                ['semua', 'Semua'],
                ['negative', `Minus${r?.negative ? ` ${r.negative}` : ''}`],
                ['out', `Habis${r?.out ? ` ${r.out}` : ''}`],
                ['low', `Hampir habis${r?.low ? ` ${r.low}` : ''}`],
              ]}
            />
          </div>

          <div className="flex flex-wrap items-center justify-between gap-2 text-label text-teks-sekunder">
            <span className="tabular-nums">
              {dipilih.size > 0 ? (
                <>
                  <strong className="font-semibold text-teks-utama">{dipilih.size}</strong> dipilih
                </>
              ) : (
                `${total.toLocaleString('id-ID')} barang`
              )}
            </span>
            <span className="flex gap-1">
              {daftar.length > 0 && (
                <Tombol jenis="teks" ukuran="padat" onClick={() => pilihBanyak(daftar)}>
                  Pilih {daftar.length < total ? `${daftar.length} yang tampil` : 'semua'}
                </Tombol>
              )}
              {dipilih.size > 0 && (
                <Tombol jenis="teks" ukuran="padat" onClick={() => setDipilih(() => new Map())}>
                  Kosongkan
                </Tombol>
              )}
            </span>
          </div>

          {q.isLoading ? (
            <KerangkaBaris jumlah={6} />
          ) : daftar.length === 0 ? (
            <Kartu className="p-6 text-center text-isi text-teks-redup">
              {cari ? `Tidak ada barang bernama "${cari}".` : 'Tidak ada barang dalam saringan ini.'}
            </Kartu>
          ) : (
            <Kartu className={cn('divide-y divide-garis', q.isPlaceholderData && 'opacity-60')}>
              {daftar.map((s) => (
                <BarisPilih
                  key={s.product_id}
                  s={s}
                  kelasWarna={kelasPetak(warna, s.category_id)}
                  dipilih={dipilih.has(s.product_id)}
                  onUbah={(p) => ubah(s, p)}
                />
              ))}
            </Kartu>
          )}

          {q.hasNextPage && (
            <Tombol jenis="kedua" lebarPenuh memuat={q.isFetchingNextPage} onClick={() => q.fetchNextPage()}>
              Muat lebih banyak ({(total - daftar.length).toLocaleString('id-ID')} lagi)
            </Tombol>
          )}

          {/* Menempel di atas navigasi bawah: daftar barangnya panjang, dan
              dulu tombol ini baru terlihat setelah digulir sampai habis. */}
          <Tombol
            lebarPenuh
            className="sticky bottom-[calc(4.5rem+env(safe-area-inset-bottom))] z-10 shadow-melayang lg:bottom-4"
            disabled={dipilih.size === 0}
            onClick={onMulai}
          >
            Mulai Hitung ({dipilih.size} barang)
          </Tombol>
        </>
      )}
    </div>
  )
}

function BarisPilih({
  s,
  kelasWarna,
  dipilih,
  onUbah,
}: {
  s: SaldoStok
  kelasWarna: string
  dipilih: boolean
  onUbah: (pilih: boolean) => void
}) {
  const k = keadaanSaldo(s)
  return (
    <label
      className={cn(
        'flex min-h-14 cursor-pointer items-center gap-3 px-3 py-2 hover:bg-permukaan-2/60',
        dipilih && 'bg-sorot/60',
      )}
    >
      <input
        type="checkbox"
        checked={dipilih}
        onChange={(e) => onUbah(e.target.checked)}
        className="h-5 w-5 shrink-0 accent-utama"
      />
      <FotoBarang nama={s.product_name} url={s.image_url} kecil kelasWarna={kelasWarna} className="w-10" />
      <span className="min-w-0 flex-1">
        <span className="block truncate text-isi text-teks-utama">{s.product_name}</span>
        {s.category_name && <span className="block truncate text-keterangan text-teks-redup">{s.category_name}</span>}
      </span>
      <span className="shrink-0 text-right text-keterangan tabular-nums">
        {k === 'negative' ? (
          <span className="font-semibold text-bahaya-teks">
            tercatat {formatQtySatuan(s.qty, s.unit_name).replace('-', '−')}
          </span>
        ) : k === 'out' ? (
          <span className="font-semibold text-bahaya-teks">Habis</span>
        ) : (
          <span className={cn(k === 'low' ? 'font-semibold text-jingga-700' : 'text-teks-sekunder')}>
            {formatQtySatuan(s.qty, s.unit_name)}
          </span>
        )}
      </span>
    </label>
  )
}

// ── Langkah 2 ────────────────────────────────────────────────────────────────

function LangkahHitung({
  h,
  ke,
  dari,
  onUbah,
  onLanjut,
  onKembali,
  adaSebelumnya,
}: {
  h: Hitungan
  ke: number
  dari: number
  onUbah: (q: string) => void
  onLanjut: (lewati: boolean) => void
  onKembali: () => void
  adaSebelumnya: boolean
}) {
  const kat = useKategori()
  const warna = petaWarnaKategori(kat.data?.data ?? [])
  const s = h.saldo
  const selisih = selisihDari(h)
  const minus = Number.parseFloat(s.qty) < 0
  const nilai = nilaiSelisih(h)

  return (
    <div className="flex w-full max-w-lg flex-col gap-4">
      <Kepala langkah={2} judul="Hitung" />

      <Kartu className="flex flex-col gap-4 p-5">
        <div className="flex items-center gap-3">
          <FotoBarang
            nama={s.product_name}
            url={s.image_url}
            kecil
            kelasWarna={kelasPetak(warna, s.category_id)}
            className="w-14"
          />
          <div className="min-w-0">
            <h2 className="text-judul-kartu font-bold text-teks-utama">{s.product_name}</h2>
            {/* Ditampilkan dengan sengaja — lihat catatan di kepala berkas. */}
            <p className={cn('text-label', minus ? 'text-bahaya-teks' : 'text-teks-sekunder')}>
              Catatan sistem: {formatQtySatuan(s.qty, s.unit_name).replace('-', '−')}
              {minus && ' — minus, hitung apa adanya'}
            </p>
          </div>
        </div>

        <div>
          <p className="mb-2 text-label font-medium text-teks-sekunder">Hitungan Anda</p>
          <StepperJumlah nilai={h.dihitung} onNilai={onUbah} satuan={s.unit_name} besar label={`Hitungan ${s.product_name}`} />
        </div>

        {/* Selalu ada (juga saat cocok) supaya tombol di bawahnya tidak
            melompat setiap kali angka berubah. */}
        <p
          className={cn(
            'flex items-center justify-between gap-2 rounded-kontrol px-3 py-2 text-label font-medium',
            selisih === 0 ? 'bg-sorot text-hijau-800' : 'bg-permukaan-2 text-jingga-700',
          )}
          aria-live="polite"
        >
          {selisih === 0 ? (
            <span className="flex items-center gap-1.5">
              <Check className="h-4 w-4" aria-hidden />
              Cocok dengan catatan
            </span>
          ) : (
            <>
              <span>
                {selisih > 0 ? 'Lebih' : 'Kurang'} {formatQty(String(Math.abs(selisih)))} {s.unit_name}
              </span>
              {nilai !== 0 && <span className="tabular-nums">≈ {formatRupiah(Math.abs(nilai))}</span>}
            </>
          )}
        </p>

        <div className="flex gap-2">
          <Tombol jenis="kedua" lebarPenuh onClick={() => onLanjut(true)}>
            Lewati
          </Tombol>
          <Tombol lebarPenuh onClick={() => onLanjut(false)}>
            {ke + 1 < dari ? 'Berikutnya →' : 'Tinjau →'}
          </Tombol>
        </div>

        <Kemajuan ke={ke} dari={dari} />
      </Kartu>

      <Tombol jenis="teks" onClick={onKembali} className="self-start">
        {adaSebelumnya ? '← Kembali ke barang sebelumnya' : '← Kembali memilih barang'}
      </Tombol>
    </div>
  )
}

// ── Bersama ──────────────────────────────────────────────────────────────────

function Kepala({ langkah, judul }: { langkah: number; judul: string }) {
  return (
    <header className="min-w-0">
      <p className="text-label font-medium text-utama">Hitung Fisik · langkah {langkah} dari 3</p>
      <h1 className="text-judul font-bold text-teks-utama">{judul}</h1>
    </header>
  )
}

function Angka({ label, nilai, tegas = false }: { label: string; nilai: number; tegas?: boolean }) {
  return (
    <div className="rounded-kontrol bg-permukaan-2/60 px-2 py-2">
      <dt className="text-keterangan text-teks-sekunder">{label}</dt>
      <dd className={cn('text-judul-kartu font-extrabold tabular-nums', tegas ? 'text-teks-utama' : 'text-teks-sekunder')}>
        {nilai.toLocaleString('id-ID')}
      </dd>
    </div>
  )
}

function Kemajuan({ ke, dari }: { ke: number; dari: number }) {
  return (
    <div className="flex flex-col items-center gap-1.5">
      <div className="flex flex-wrap justify-center gap-1.5" aria-hidden>
        {Array.from({ length: Math.min(dari, 20) }, (_, i) => (
          <span
            key={i}
            className={cn(
              'h-2 w-2 rounded-full',
              i < ke ? 'bg-utama' : i === ke ? 'bg-utama ring-2 ring-hijau-100' : 'bg-garis',
            )}
          />
        ))}
      </div>
      <p className="text-keterangan text-teks-redup" aria-live="polite">
        {ke + 1} dari {dari} barang
      </p>
    </div>
  )
}
