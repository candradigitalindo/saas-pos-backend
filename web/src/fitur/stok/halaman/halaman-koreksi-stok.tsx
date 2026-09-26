import { useEffect, useRef, useState } from 'react'
import { useNavigate, useSearchParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Package } from 'lucide-react'
import { ulid } from 'ulid'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom } from '@/bersama/ui/kolom'
import { StepperJumlah } from '@/bersama/ui/stepper-jumlah'
import { Tombol } from '@/bersama/ui/tombol'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { api, GalatAPI } from '@/lib/api-client'
import { formatQtySatuan } from '@/bersama/util/desimal'
import type { Produk } from '@/bersama/tipe/katalog'
import { PemilihBarang } from '../komponen/pemilih-barang'
import { stokApi } from '../api'

/**
 * Koreksi stok — juga dipakai mengisi saldo awal saat onboarding.
 *
 * Yang dikirim adalah `new_qty` (angka pasti hasil hitungan), bukan selisih:
 * orang menghitung barang di rak, bukan menghitung selisih.
 */
export function HalamanKoreksiStok() {
  const [params] = useSearchParams()
  const onboarding = params.get('onboarding') === '1'
  const produkAwal = params.get('product_id')

  const { tokoAktif } = useSesi()
  const navigate = useNavigate()
  const toast = useToast()
  const qc = useQueryClient()

  const [produk, setProduk] = useState<Produk | null>(null)
  const [bukaPemilih, setBukaPemilih] = useState(!produkAwal)
  const [jumlah, setJumlah] = useState('0')
  const [alasan, setAlasan] = useState(onboarding ? 'Stok awal' : '')
  const [galat, setGalat] = useState<string | null>(null)

  // Datang dari onboarding: barangnya sudah ditentukan lewat URL.
  const detailAwal = useQuery({
    queryKey: ['produk', produkAwal],
    queryFn: () => api.get<Produk>(`/products/${produkAwal}`),
    enabled: !!produkAwal,
  })
  useEffect(() => {
    if (detailAwal.data) setProduk(detailAwal.data)
  }, [detailAwal.data])

  // Saldo saat ini ditampilkan apa adanya — jujur lebih baik daripada
  // "menguji" petugas dengan menyembunyikannya.
  const saldo = useQuery({
    queryKey: ['stok', tokoAktif, 'semua'],
    queryFn: () => stokApi.saldo(tokoAktif!),
    enabled: !!tokoAktif,
  })
  const saldoSekarang = saldo.data?.data.find((s) => s.product_id === produk?.id)

  // Satu kunci untuk seluruh percobaan di layar ini: menekan "Simpan" lagi
  // setelah sinyal putus tidak mencatat koreksinya dua kali.
  const kunci = useRef(ulid())
  const simpan = useMutation({
    mutationFn: () =>
      stokApi.koreksi(
        {
          outlet_id: tokoAktif!,
          product_id: produk!.id,
          new_qty: jumlah,
          reason: alasan.trim(),
        },
        kunci.current,
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['stok'] })
      qc.invalidateQueries({ queryKey: ['stok-kasir'] })
      qc.invalidateQueries({ queryKey: ['kartu-stok'] })
      toast.berhasil(
        `Stok ${produk?.name} sekarang ${formatQtySatuan(jumlah, produk?.unit_name)}.`,
      )
      navigate(onboarding ? '/kasir' : '/stok')
    },
    onError: (e) =>
      setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  return (
    <div className="mx-auto flex w-full max-w-lg flex-col gap-4">
      <header>
        <h1 className="text-judul font-bold text-teks-utama">
          {onboarding ? 'Isi Stok Awal' : 'Koreksi Stok'}
        </h1>
        <p className="text-isi text-teks-sekunder">
          {onboarding
            ? 'Berapa banyak barang ini yang Anda punya sekarang?'
            : 'Samakan catatan sistem dengan barang yang benar-benar ada di rak.'}
        </p>
      </header>

      <Kartu className="flex flex-col gap-4 p-4">
        {produk ? (
          <div className="flex items-center justify-between gap-3">
            <div className="min-w-0">
              <p className="truncate font-semibold text-teks-utama">{produk.name}</p>
              <p className="text-keterangan text-teks-redup">
                Catatan sistem:{' '}
                {saldoSekarang
                  ? formatQtySatuan(saldoSekarang.qty, saldoSekarang.unit_name)
                  : `0 ${produk.unit_name ?? ''}`}
              </p>
            </div>
            <Tombol jenis="teks" ukuran="padat" onClick={() => setBukaPemilih(true)}>
              Ganti
            </Tombol>
          </div>
        ) : (
          <Tombol jenis="kedua" lebarPenuh onClick={() => setBukaPemilih(true)}>
            <Package className="h-5 w-5" aria-hidden />
            Pilih barang
          </Tombol>
        )}

        {produk && (
          <>
            <div>
              <p className="mb-2 text-label font-medium text-teks-sekunder">
                Hitungan Anda
              </p>
              <StepperJumlah
                nilai={jumlah}
                onNilai={setJumlah}
                satuan={produk.unit_name}
                besar
                label="Hitungan Anda"
              />
            </div>

            <Kolom
              label="Alasan"
              placeholder="Contoh: stok awal, barang rusak, salah hitung"
              value={alasan}
              onChange={(e) => setAlasan(e.target.value)}
              bantuan="Dicatat di riwayat stok supaya bisa ditelusuri nanti."
              required
            />

            {galat && (
              <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
                {galat}
              </p>
            )}

            <Tombol
              lebarPenuh
              memuat={simpan.isPending}
              labelMemuat="Menyimpan…"
              disabled={!alasan.trim()}
              onClick={() => {
                setGalat(null)
                simpan.mutate()
              }}
            >
              Simpan Stok
            </Tombol>
          </>
        )}
      </Kartu>

      <PemilihBarang
        terbuka={bukaPemilih}
        onTutup={() => setBukaPemilih(false)}
        onPilih={(p) => {
          setProduk(p)
          setBukaPemilih(false)
        }}
      />
    </div>
  )
}
