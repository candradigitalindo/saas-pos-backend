import { useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ClipboardList, Search } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { StepperJumlah } from '@/bersama/ui/stepper-jumlah'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { bandingQty, formatQty, formatQtySatuan } from '@/bersama/util/desimal'
import { cn } from '@/bersama/util/cn'
import type { SaldoStok } from '@/bersama/tipe/katalog'
import { stokApi } from '../api'

interface Hitungan {
  productId: string
  nama: string
  satuan: string
  sistem: string
  dihitung: string
  dilewati: boolean
}

/**
 * Hitung fisik (opname) — dirancang untuk dikerjakan sambil berdiri di depan rak
 * (ui/05-ALUR-UTAMA.md §5).
 *
 * Satu barang satu layar, dengan indikator kemajuan. Catatan sistem
 * DITAMPILKAN, tidak disembunyikan: jujur lebih baik daripada "menguji" petugas,
 * dan petugas yang merasa diuji akan mulai menyalin angka sistem.
 */
export function HalamanOpname() {
  const { tokoAktif } = useSesi()
  const navigate = useNavigate()
  const toast = useToast()
  const qc = useQueryClient()

  const [langkah, setLangkah] = useState<1 | 2 | 3>(1)
  const [terpilih, setTerpilih] = useState<Set<string>>(new Set())
  const [hitungan, setHitungan] = useState<Hitungan[]>([])
  const [ke, setKe] = useState(0)
  const [cari, setCari] = useState('')
  const [konfirmasi, setKonfirmasi] = useState(false)
  const [galat, setGalat] = useState<string | null>(null)

  const saldo = useQuery({
    queryKey: ['stok', tokoAktif, 'opname'],
    queryFn: () => stokApi.saldo(tokoAktif!),
    enabled: !!tokoAktif,
  })

  const daftarStok = saldo.data?.data ?? []
  const tersaring = useMemo(
    () =>
      cari
        ? daftarStok.filter((s) =>
            s.product_name.toLowerCase().includes(cari.toLowerCase()),
          )
        : daftarStok,
    [daftarStok, cari],
  )

  const simpan = useMutation({
    mutationFn: async () => {
      const dipakai = hitungan.filter((h) => !h.dilewati)
      const opname = await stokApi.buatOpname(tokoAktif!, 'Hitung fisik dari aplikasi')
      await stokApi.simpanHitungan(
        opname.id,
        dipakai.map((h) => ({ product_id: h.productId, counted_qty: h.dihitung })),
      )
      return stokApi.postingOpname(opname.id)
    },
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['stok'] })
      qc.invalidateQueries({ queryKey: ['stok-kasir'] })
      qc.invalidateQueries({ queryKey: ['kartu-stok'] })
      toast.berhasil('Stok sudah disesuaikan mengikuti hitungan Anda.')
      navigate('/stok')
    },
    onError: (e) =>
      setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  // ── Langkah 1: pilih barang ──────────────────────────────────────────────
  if (langkah === 1) {
    return (
      <div className="flex w-full max-w-lg flex-col gap-4">
        <Kepala langkah={1} judul="Pilih barang yang mau dihitung" />

        {saldo.isLoading ? (
          <KerangkaBaris jumlah={5} />
        ) : daftarStok.length === 0 ? (
          <KeadaanKosong
            ikon={ClipboardList}
            judul="Belum ada stok untuk dihitung"
            penjelasan="Catat barang masuk atau isi stok awal dulu, baru hitung fisik bisa dijalankan."
            aksi={{ label: 'Ke halaman Stok', onKlik: () => navigate('/stok') }}
          />
        ) : (
          <>
            <div className="relative">
              <Search
                className="pointer-events-none absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-teks-redup"
                aria-hidden
              />
              <input
                type="search"
                value={cari}
                onChange={(e) => setCari(e.target.value)}
                placeholder="Cari barang…"
                aria-label="Cari barang"
                className="h-12 w-full rounded-kontrol border border-garis bg-permukaan pl-10 pr-3 text-isi text-teks-utama placeholder:text-teks-redup"
              />
            </div>

            <div className="flex gap-2">
              <Tombol
                jenis="kedua"
                ukuran="padat"
                onClick={() => setTerpilih(new Set(tersaring.map((s) => s.product_id)))}
              >
                Pilih semua
              </Tombol>
              {terpilih.size > 0 && (
                <Tombol jenis="teks" ukuran="padat" onClick={() => setTerpilih(new Set())}>
                  Kosongkan
                </Tombol>
              )}
            </div>

            <Kartu className="divide-y divide-garis">
              {tersaring.map((s) => (
                <BarisPilih
                  key={s.product_id}
                  stok={s}
                  dipilih={terpilih.has(s.product_id)}
                  onUbah={(pilih) =>
                    setTerpilih((lama) => {
                      const baru = new Set(lama)
                      if (pilih) baru.add(s.product_id)
                      else baru.delete(s.product_id)
                      return baru
                    })
                  }
                />
              ))}
            </Kartu>

            {/* Menempel di atas navigasi bawah: daftar barangnya panjang, dan
                dulu tombol ini baru terlihat setelah digulir sampai habis. */}
            <Tombol
              lebarPenuh
              className="sticky bottom-[calc(4.5rem+env(safe-area-inset-bottom))] z-10 shadow-melayang lg:bottom-4"
              disabled={terpilih.size === 0}
              onClick={() => {
                setHitungan(
                  daftarStok
                    .filter((s) => terpilih.has(s.product_id))
                    .map((s) => ({
                      productId: s.product_id,
                      nama: s.product_name,
                      satuan: s.unit_name,
                      sistem: s.qty,
                      // Dimulai dari angka sistem: petugas biasanya mengoreksi
                      // beberapa barang saja, bukan menghitung ulang dari nol.
                      dihitung: s.qty,
                      dilewati: false,
                    })),
                )
                setKe(0)
                setLangkah(2)
              }}
            >
              Mulai Hitung ({terpilih.size} barang)
            </Tombol>
          </>
        )}
      </div>
    )
  }

  // ── Langkah 2: hitung, satu barang satu layar ────────────────────────────
  if (langkah === 2) {
    const h = hitungan[ke]
    if (!h) return null
    const selisih = Number.parseFloat(h.dihitung) - Number.parseFloat(h.sistem)

    const lanjut = () => {
      if (ke + 1 < hitungan.length) setKe(ke + 1)
      else setLangkah(3)
    }

    return (
      <div className="flex w-full max-w-lg flex-col gap-4">
        <Kepala langkah={2} judul="Hitung" />

        <Kartu className="flex flex-col gap-4 p-5">
          <div>
            <h2 className="text-judul font-bold text-teks-utama">{h.nama}</h2>
            {/* Ditampilkan dengan sengaja — lihat catatan di kepala berkas. */}
            <p className="text-label text-teks-sekunder">
              Catatan sistem: {formatQtySatuan(h.sistem, h.satuan)}
            </p>
          </div>

          <div>
            <p className="mb-2 text-label font-medium text-teks-sekunder">
              Hitungan Anda
            </p>
            <StepperJumlah
              nilai={h.dihitung}
              onNilai={(q) =>
                setHitungan((l) => l.map((x, i) => (i === ke ? { ...x, dihitung: q } : x)))
              }
              satuan={h.satuan}
              besar
              label={`Hitungan ${h.nama}`}
            />
          </div>

          {selisih !== 0 && (
            <p
              className={cn(
                'rounded-kontrol px-3 py-2 text-label font-medium',
                'bg-permukaan-2 text-jingga-700',
              )}
            >
              Selisih {selisih > 0 ? '+' : '−'}
              {formatQty(String(Math.abs(selisih)))} {h.satuan}
            </p>
          )}

          <div className="flex gap-2">
            <Tombol
              jenis="kedua"
              lebarPenuh
              onClick={() => {
                setHitungan((l) =>
                  l.map((x, i) => (i === ke ? { ...x, dilewati: true } : x)),
                )
                lanjut()
              }}
            >
              Lewati
            </Tombol>
            <Tombol
              lebarPenuh
              onClick={() => {
                setHitungan((l) =>
                  l.map((x, i) => (i === ke ? { ...x, dilewati: false } : x)),
                )
                lanjut()
              }}
            >
              {ke + 1 < hitungan.length ? 'Berikutnya →' : 'Tinjau →'}
            </Tombol>
          </div>

          <Kemajuan ke={ke} dari={hitungan.length} />
        </Kartu>

        {ke > 0 && (
          <Tombol jenis="teks" onClick={() => setKe(ke - 1)} className="self-start">
            ← Kembali ke barang sebelumnya
          </Tombol>
        )}
      </div>
    )
  }

  // ── Langkah 3: tinjau & simpan ───────────────────────────────────────────
  const dipakai = hitungan.filter((h) => !h.dilewati)
  const berubah = dipakai.filter((h) => bandingQty(h.dihitung, h.sistem) !== 0)

  return (
    <div className="flex w-full max-w-lg flex-col gap-4">
      <Kepala langkah={3} judul="Tinjau & simpan" />

      {berubah.length === 0 ? (
        <Kartu className="p-6 text-center">
          <p className="text-isi text-teks-sekunder">
            Semua hitungan Anda cocok dengan catatan sistem. Tidak ada yang perlu
            diubah.
          </p>
        </Kartu>
      ) : (
        <Kartu className="divide-y divide-garis">
          {berubah.map((h) => {
            const selisih = Number.parseFloat(h.dihitung) - Number.parseFloat(h.sistem)
            return (
              <div key={h.productId} className="flex items-center justify-between gap-3 p-4">
                <div className="min-w-0">
                  <p className="truncate font-medium text-teks-utama">{h.nama}</p>
                  <p className="text-keterangan tabular-nums text-teks-redup">
                    {formatQty(h.sistem)} → {formatQty(h.dihitung)} {h.satuan}
                  </p>
                </div>
                <p
                  className={cn(
                    'shrink-0 font-bold tabular-nums',
                    selisih > 0 ? 'text-hijau-700' : 'text-bahaya-teks',
                  )}
                >
                  {selisih > 0 ? '+' : '−'}
                  {formatQty(String(Math.abs(selisih)))}
                </p>
              </div>
            )
          })}
        </Kartu>
      )}

      {hitungan.some((h) => h.dilewati) && (
        <p className="text-label text-teks-sekunder">
          {hitungan.filter((h) => h.dilewati).length} barang dilewati dan stoknya
          tidak akan diubah.
        </p>
      )}

      {galat && (
        <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
          {galat}
        </p>
      )}

      <div className="flex gap-2">
        <Tombol jenis="kedua" lebarPenuh onClick={() => setLangkah(2)}>
          ← Hitung ulang
        </Tombol>
        <Tombol
          lebarPenuh
          disabled={dipakai.length === 0}
          onClick={() => setKonfirmasi(true)}
        >
          Simpan Hasil Hitungan
        </Tombol>
      </div>

      {/* Aksi berat dan tidak bisa dibalik: dialognya menyebut angka, bukan
          sekadar bertanya "Anda yakin?". */}
      <Dialog open={konfirmasi} onOpenChange={(o) => !o && setKonfirmasi(false)}>
        <IsiDialog judul="Simpan hasil hitungan?">
          <div className="flex flex-col gap-2 text-isi text-teks-sekunder">
            <p>Stok akan disesuaikan mengikuti hitungan Anda.</p>
            <p>
              <strong className="text-teks-utama">{berubah.length} barang</strong>{' '}
              berubah
              {berubah.length > 0 && (
                <>
                  , dari{' '}
                  <strong className="text-teks-utama">{dipakai.length}</strong> yang
                  dihitung
                </>
              )}
              .
            </p>
            <p>Tindakan ini tercatat di riwayat stok dan tidak bisa dibatalkan.</p>
          </div>

          <AksiDialog>
            <Tombol
              memuat={simpan.isPending}
              labelMemuat="Menyimpan…"
              onClick={() => simpan.mutate()}
            >
              Ya, Simpan
            </Tombol>
            <Tombol
              jenis="kedua"
              onClick={() => setKonfirmasi(false)}
              disabled={simpan.isPending}
            >
              Periksa lagi
            </Tombol>
          </AksiDialog>
        </IsiDialog>
      </Dialog>
    </div>
  )
}

function Kepala({ langkah, judul }: { langkah: number; judul: string }) {
  return (
    <header>
      <p className="text-label font-medium text-utama">Langkah {langkah} dari 3</p>
      <h1 className="text-judul font-bold text-teks-utama">{judul}</h1>
    </header>
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

function BarisPilih({
  stok,
  dipilih,
  onUbah,
}: {
  stok: SaldoStok
  dipilih: boolean
  onUbah: (pilih: boolean) => void
}) {
  return (
    <label className="flex min-h-14 cursor-pointer items-center gap-3 px-4 hover:bg-permukaan-2">
      <input
        type="checkbox"
        checked={dipilih}
        onChange={(e) => onUbah(e.target.checked)}
        className="h-5 w-5 shrink-0 accent-[var(--warna-utama)]"
      />
      <span className="min-w-0 flex-1 truncate text-isi text-teks-utama">
        {stok.product_name}
      </span>
      <span className="shrink-0 text-keterangan tabular-nums text-teks-redup">
        {formatQtySatuan(stok.qty, stok.unit_name)}
      </span>
    </label>
  )
}
