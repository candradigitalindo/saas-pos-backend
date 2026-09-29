import { useEffect, useRef, useState } from 'react'
import { Link, useLocation, useNavigate, useSearchParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, History, Package, SlidersHorizontal, TriangleAlert } from 'lucide-react'
import { ulid } from 'ulid'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom } from '@/bersama/ui/kolom'
import { StepperJumlah } from '@/bersama/ui/stepper-jumlah'
import { Tombol } from '@/bersama/ui/tombol'
import { FotoBarang } from '@/bersama/komponen/foto-barang'
import { Kerangka } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { useKategori } from '@/bersama/hooks/use-katalog'
import { api, GalatAPI } from '@/lib/api-client'
import { formatQty, formatQtySatuan } from '@/bersama/util/desimal'
import { formatTanggalJam } from '@/bersama/util/tanggal'
import { kelasPetak, petaWarnaKategori } from '@/bersama/util/warna-kategori'
import { cn } from '@/bersama/util/cn'
import type { Produk } from '@/bersama/tipe/katalog'
import { PemilihBarang } from '../komponen/pemilih-barang'
import { LencanaKeadaan } from '../komponen/bagian-saldo'
import { formatSelisihRupiah } from '../komponen/dialog-riwayat-hitung'
import { stokApi } from '../api'
import { keadaanSaldo, produkDariSaldo } from '../keadaan-stok'

/** Alasan yang paling sering — sekali ketuk, tetap bisa diketik bebas. */
const ALASAN_CEPAT = ['Barang rusak', 'Kedaluwarsa', 'Hilang', 'Salah hitung', 'Dipakai sendiri']

/**
 * Koreksi stok — juga dipakai mengisi saldo awal saat onboarding.
 *
 * Yang dikirim adalah `new_qty` (angka pasti hasil hitungan), bukan selisih:
 * orang menghitung barang di rak, bukan menghitung selisih. Jumlahnya DIMULAI
 * dari catatan sekarang (versi sebelumnya selalu dari 0), karena koreksi
 * biasanya kecil: 13 jadi 11, bukan mengetik ulang semuanya.
 *
 * Sebelum disimpan, akibatnya ditulis: "13 → 11 pcs · kurang 2 · ≈ −Rp 10.000".
 * Nilainya perubahan NILAI STOK (catatan minus dihitung nol, sama dengan
 * halaman Stok).
 *
 * Halaman ini tidak lagi membuka pemilih barang di atas layar kosong. Di
 * sampingnya ada jalan pintas ke barang yang tercatat MINUS — alasan paling
 * umum orang mengoreksi — dan daftar koreksi terakhir. Dibuka tanpa barang
 * tertentu, halaman tetap di sini setelah menyimpan supaya beberapa koreksi
 * bisa dicatat berturut-turut.
 */
export function HalamanKoreksiStok() {
  const [params] = useSearchParams()
  const onboarding = params.get('onboarding') === '1'
  const produkAwal = params.get('product_id')

  const { tokoAktif } = useSesi()
  const navigate = useNavigate()
  const lokasi = useLocation()
  const toast = useToast()
  const qc = useQueryClient()
  const kat = useKategori()
  const warna = petaWarnaKategori(kat.data?.data ?? [])

  const [produk, setProduk] = useState<Produk | null>(null)
  const [bukaPemilih, setBukaPemilih] = useState(false)
  /** null = mengikuti catatan sekarang (belum diubah). */
  const [jumlah, setJumlah] = useState<string | null>(null)
  const [alasan, setAlasan] = useState(onboarding ? 'Stok awal' : '')
  const [galat, setGalat] = useState<string | null>(null)

  // Datang dari onboarding / baris Stok / Riwayat Stok: barangnya lewat URL.
  const detailAwal = useQuery({
    queryKey: ['produk', produkAwal],
    queryFn: () => api.get<Produk>(`/products/${produkAwal}`),
    enabled: !!produkAwal,
  })
  useEffect(() => {
    if (detailAwal.data) setProduk(detailAwal.data)
  }, [detailAwal.data])

  // Saldo saat ini ditampilkan apa adanya — jujur lebih baik daripada
  // "menguji" petugas dengan menyembunyikannya. Diambil per barang
  // (?product_ids=), bukan dicari di 100 baris pertama daftar stok.
  const saldo = useQuery({
    queryKey: ['stok', tokoAktif, 'barang', produk?.id],
    queryFn: () => stokApi.saldo(tokoAktif!, false, 1, 1, { produk: [produk!.id] }),
    enabled: !!tokoAktif && !!produk,
  })
  const saldoSekarang = saldo.data?.data[0]
  const catatan = saldoSekarang?.qty ?? '0'
  const nilaiJumlah = jumlah ?? String(Math.max(Number.parseFloat(catatan), 0))
  const selisih = Number.parseFloat(nilaiJumlah) - Number.parseFloat(catatan)
  const modal = saldoSekarang?.cost_price ?? produk?.cost_price ?? 0
  const nilai = Math.round(
    (Number.parseFloat(nilaiJumlah) - Math.max(Number.parseFloat(catatan), 0)) * modal,
  )

  function pilih(p: Produk) {
    setProduk(p)
    setJumlah(null)
    setGalat(null)
    setBukaPemilih(false)
  }

  // Satu kunci untuk seluruh percobaan satu koreksi: menekan "Simpan" lagi
  // setelah sinyal putus tidak mencatatnya dua kali. Kunci baru untuk koreksi
  // berikutnya.
  const kunci = useRef(ulid())
  const simpan = useMutation({
    mutationFn: () =>
      stokApi.koreksi(
        { outlet_id: tokoAktif!, product_id: produk!.id, new_qty: nilaiJumlah, reason: alasan.trim() },
        kunci.current,
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['stok'] })
      qc.invalidateQueries({ queryKey: ['stok-ringkasan'] })
      qc.invalidateQueries({ queryKey: ['stok-kasir'] })
      qc.invalidateQueries({ queryKey: ['kartu-stok'] })
      qc.invalidateQueries({ queryKey: ['koreksi'] })
      toast.berhasil(`Stok ${produk?.name} sekarang ${formatQtySatuan(nilaiJumlah, produk?.unit_name)}.`)
      if (onboarding) navigate('/kasir')
      // Datang untuk satu barang tertentu → kembali ke asalnya.
      else if (produkAwal) {
        if (lokasi.key !== 'default') navigate(-1)
        else navigate('/stok')
      }
      else {
        kunci.current = ulid()
        setProduk(null)
        setJumlah(null)
        setAlasan('')
      }
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  const bisaSimpan = !!produk && !!alasan.trim() && (selisih !== 0 || !saldoSekarang)

  return (
    <div className="flex w-full max-w-lg flex-col gap-4 lg:grid lg:max-w-5xl lg:grid-cols-[minmax(0,30rem)_minmax(0,1fr)] lg:items-start lg:gap-x-6">
      <header className="lg:col-span-2">
        <h1 className="text-judul font-bold text-teks-utama">{onboarding ? 'Isi Stok Awal' : 'Koreksi Stok'}</h1>
        <p className="text-label text-teks-sekunder">
          {onboarding
            ? 'Berapa banyak barang ini yang Anda punya sekarang?'
            : 'Samakan catatan dengan barang yang benar-benar ada di rak.'}
        </p>
      </header>

      <Kartu className="flex flex-col gap-4 p-4">
        {!produk ? (
          produkAwal && detailAwal.isLoading ? (
            <Kerangka className="h-40 w-full" />
          ) : (
            <div className="flex flex-col items-center gap-3 py-4 text-center">
              <span className="flex h-12 w-12 items-center justify-center rounded-full bg-sorot text-hijau-800">
                <SlidersHorizontal className="h-6 w-6" aria-hidden />
              </span>
              <p className="text-isi text-teks-sekunder">Pilih barang yang catatannya mau dikoreksi.</p>
              <Tombol onClick={() => setBukaPemilih(true)}>
                <Package className="h-5 w-5" aria-hidden />
                Pilih barang
              </Tombol>
            </div>
          )
        ) : (
          <>
            <div className="flex items-center gap-3">
              <FotoBarang
                nama={produk.name}
                url={produk.image_url}
                kecil
                kelasWarna={kelasPetak(warna, produk.category_id)}
                className="w-12"
              />
              <div className="min-w-0 flex-1">
                <p className="truncate font-semibold text-teks-utama">{produk.name}</p>
                <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-keterangan text-teks-redup">
                  {saldo.isLoading ? (
                    'Memuat catatan…'
                  ) : (
                    <>
                      <span className={cn(Number.parseFloat(catatan) < 0 && 'font-semibold text-bahaya-teks')}>
                        Catatan sekarang: {formatQtySatuan(catatan, produk.unit_name).replace('-', '−')}
                      </span>
                      {saldoSekarang && <LencanaKeadaan keadaan={keadaanSaldo(saldoSekarang)} />}
                    </>
                  )}
                </p>
              </div>
              {!onboarding && (
                <Tombol jenis="teks" ukuran="padat" onClick={() => setBukaPemilih(true)}>
                  Ganti
                </Tombol>
              )}
            </div>

            <div>
              <p className="mb-2 text-label font-medium text-teks-sekunder">Jumlah sebenarnya di rak</p>
              <StepperJumlah
                nilai={nilaiJumlah}
                onNilai={setJumlah}
                satuan={produk.unit_name}
                besar
                label="Jumlah sebenarnya"
              />
            </div>

            {/* Akibatnya ditulis sebelum disimpan. Selalu ada (juga saat sama)
                supaya tombol di bawahnya tidak melompat. */}
            {!saldo.isLoading && (
              <p
                className={cn(
                  'flex flex-wrap items-center justify-between gap-x-3 gap-y-1 rounded-kontrol px-3 py-2 text-label',
                  selisih === 0 ? 'bg-permukaan-2 text-teks-sekunder' : 'bg-permukaan-2 text-teks-utama',
                )}
                aria-live="polite"
              >
                {selisih === 0 ? (
                  <span className="flex items-center gap-1.5">
                    <Check className="h-4 w-4 text-utama" aria-hidden />
                    Sama dengan catatan — ubah jumlahnya dulu
                  </span>
                ) : (
                  <>
                    <span className="tabular-nums">
                      {formatQty(catatan).replace('-', '−')} → <strong>{formatQtySatuan(nilaiJumlah, produk.unit_name)}</strong>
                      <span className={cn('ml-2 font-semibold', selisih > 0 ? 'text-hijau-700' : 'text-bahaya-teks')}>
                        {selisih > 0 ? 'lebih' : 'kurang'} {formatQty(String(Math.abs(selisih)))}
                      </span>
                    </span>
                    {nilai !== 0 && (
                      <span className="tabular-nums text-teks-sekunder">≈ {formatSelisihRupiah(nilai)}</span>
                    )}
                  </>
                )}
              </p>
            )}

            <div className="flex flex-col gap-2">
              <Kolom
                label="Alasan"
                placeholder="Contoh: barang rusak, salah hitung"
                value={alasan}
                onChange={(e) => setAlasan(e.target.value)}
                bantuan="Dicatat di riwayat stok supaya bisa ditelusuri nanti."
                required
              />
              {!onboarding && (
                <div className="flex flex-wrap gap-1.5" role="group" aria-label="Alasan cepat">
                  {ALASAN_CEPAT.map((a) => (
                    <button
                      key={a}
                      type="button"
                      onClick={() => setAlasan(a)}
                      aria-pressed={alasan === a}
                      className={cn(
                        'min-h-9 rounded-full border px-3 text-keterangan font-medium',
                        alasan === a
                          ? 'border-utama bg-sorot text-hijau-800'
                          : 'border-garis bg-permukaan text-teks-sekunder hover:bg-permukaan-2',
                      )}
                    >
                      {a}
                    </button>
                  ))}
                </div>
              )}
            </div>

            {galat && (
              <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
                {galat}
              </p>
            )}

            <Tombol
              lebarPenuh
              memuat={simpan.isPending}
              labelMemuat="Menyimpan…"
              disabled={!bisaSimpan}
              onClick={() => {
                setGalat(null)
                simpan.mutate()
              }}
            >
              {onboarding ? 'Simpan Stok Awal' : 'Simpan Koreksi'}
            </Tombol>
          </>
        )}
      </Kartu>

      {!onboarding && (
        <div className="flex flex-col gap-4">
          <PerluDicocokkan terpilih={produk?.id} onPilih={(p) => pilih(p)} />
          <KoreksiTerakhir />
        </div>
      )}

      <PemilihBarang
        terbuka={bukaPemilih}
        onTutup={() => setBukaPemilih(false)}
        onPilih={pilih}
        judul="Pilih barang yang dikoreksi"
        sudahDipilih={produk ? [produk.id] : []}
      />
    </div>
  )
}

/** Jalan pintas ke barang tercatat MINUS — catatan yang pasti salah. */
function PerluDicocokkan({ terpilih, onPilih }: { terpilih?: string; onPilih: (p: Produk) => void }) {
  const { tokoAktif } = useSesi()
  const kat = useKategori()
  const warna = petaWarnaKategori(kat.data?.data ?? [])
  const q = useQuery({
    queryKey: ['stok', tokoAktif, 'koreksi-minus'],
    queryFn: () => stokApi.saldo(tokoAktif!, false, 1, 20, { keadaan: 'negative', urut: 'urgent' }),
    enabled: !!tokoAktif,
    staleTime: 30_000,
  })
  const daftar = q.data?.data ?? []
  if (!daftar.length) return null
  return (
    <Kartu className="flex flex-col gap-1 p-4">
      <h2 className="flex items-center gap-2 text-judul-kartu font-semibold text-teks-utama">
        <TriangleAlert className="h-5 w-5 shrink-0 text-bahaya-teks" aria-hidden />
        Perlu dicocokkan
        <span className="text-label font-medium tabular-nums text-teks-redup">{q.data?.total}</span>
      </h2>
      <p className="text-keterangan text-teks-redup">Tercatat minus — pasti tidak sama dengan rak. Ketuk untuk mengoreksi.</p>
      <ul className="-mx-1 mt-1 divide-y divide-garis">
        {daftar.map((s) => (
          <li key={s.product_id}>
            <button
              type="button"
              onClick={() => onPilih(produkDariSaldo(s))}
              aria-pressed={terpilih === s.product_id}
              className={cn(
                'flex min-h-12 w-full items-center gap-3 rounded-kontrol px-1 py-2 text-left hover:bg-permukaan-2/60',
                terpilih === s.product_id && 'bg-sorot hover:bg-sorot',
              )}
            >
              <FotoBarang
                nama={s.product_name}
                url={s.image_url}
                kecil
                kelasWarna={kelasPetak(warna, s.category_id)}
                className="w-9"
              />
              <span className="min-w-0 flex-1 truncate font-medium text-teks-utama">{s.product_name}</span>
              <span className="shrink-0 text-keterangan font-semibold tabular-nums text-bahaya-teks">
                {formatQtySatuan(s.qty, s.unit_name).replace('-', '−')}
              </span>
            </button>
          </li>
        ))}
      </ul>
    </Kartu>
  )
}

/** Koreksi terbaru di toko ini — apa, berapa, kenapa, siapa. */
function KoreksiTerakhir() {
  const { tokoAktif } = useSesi()
  const q = useQuery({
    queryKey: ['koreksi', tokoAktif],
    queryFn: () => stokApi.daftarKoreksi(tokoAktif!, 1, 8),
    enabled: !!tokoAktif,
  })
  const daftar = q.data?.data ?? []
  if (q.isLoading) return <Kerangka className="h-40 w-full rounded-kartu" />
  if (!daftar.length) return null
  return (
    <Kartu className="flex flex-col gap-1 p-4">
      <h2 className="flex items-center gap-2 text-judul-kartu font-semibold text-teks-utama">
        <History className="h-5 w-5 shrink-0 text-teks-sekunder" aria-hidden />
        Koreksi terakhir
      </h2>
      <ul className="-mx-1 mt-1 divide-y divide-garis">
        {daftar.map((k) => {
          const delta = Number.parseFloat(k.qty_delta)
          return (
            <li key={k.id}>
              <Link
                to={`/stok/kartu/${k.product_id}`}
                className="flex items-start gap-3 rounded-kontrol px-1 py-2.5 hover:bg-permukaan-2/60"
              >
                <span className="min-w-0 flex-1">
                  <span className="block truncate font-medium text-teks-utama">{k.product_name}</span>
                  <span className="block truncate text-keterangan text-teks-sekunder">
                    {k.kind === 'initial' ? 'Stok awal' : k.reason || 'Koreksi'}
                  </span>
                  <span className="block text-keterangan text-teks-redup">
                    {formatTanggalJam(k.occurred_at)}
                    {k.created_by_name && ` · ${k.created_by_name}`}
                  </span>
                </span>
                <span className="shrink-0 text-right tabular-nums">
                  <span
                    className={cn(
                      'block font-bold',
                      delta > 0 ? 'text-hijau-700' : delta < 0 ? 'text-bahaya-teks' : 'text-teks-redup',
                    )}
                  >
                    {delta > 0 ? '+' : delta < 0 ? '−' : ''}
                    {formatQty(String(Math.abs(delta)))}
                  </span>
                  <span className="block text-keterangan text-teks-redup">
                    jadi {formatQtySatuan(k.balance_after, k.unit_name)}
                  </span>
                </span>
              </Link>
            </li>
          )
        })}
      </ul>
    </Kartu>
  )
}
