import { Link, useNavigate } from 'react-router-dom'
import { Check, Lock, PartyPopper } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { cn } from '@/bersama/util/cn'

/**
 * Layar setelah pendaftaran berhasil.
 *
 * Tiga langkah, boleh dilewati, dan tetap muncul di Beranda sampai selesai.
 * Targetnya: dari daftar sampai transaksi pertama di bawah 5 menit. Kalau
 * pemilik warung tidak berhasil menjual barang pertamanya di hari pertama,
 * dia tidak akan kembali (ui/05-ALUR-UTAMA.md §1).
 */
export function HalamanSelamatDatang() {
  const { profil } = useSesi()
  const navigate = useNavigate()
  const nama = profil?.user.name?.split(' ')[0] ?? 'Bos'

  return (
    <div className="mx-auto flex w-full max-w-md flex-col gap-6 px-4 py-8">
      <div className="text-center">
        <PartyPopper className="mx-auto h-12 w-12 text-jingga-600" aria-hidden />
        <h1 className="mt-3 text-judul font-bold text-teks-utama">
          Selamat datang, {nama}!
        </h1>
        <p className="mt-1 text-isi text-teks-sekunder">
          Usaha Anda sudah siap. Tinggal 3 langkah lagi.
        </p>
      </div>

      <Kartu className="divide-y divide-garis">
        <LangkahOnboarding
          nomor={1}
          judul="Tambah barang pertama"
          keterangan="Cukup nama, harga jual, dan harga beli."
          keadaan="aktif"
          aksi={{ label: 'Mulai', ke: '/barang/baru?onboarding=1' }}
        />
        <LangkahOnboarding
          nomor={2}
          judul="Isi stok awal"
          keterangan="Berapa banyak barang yang Anda punya sekarang."
          keadaan="terkunci"
        />
        <LangkahOnboarding
          nomor={3}
          judul="Coba transaksi pertama"
          keterangan="Jual satu barang untuk memastikan semuanya jalan."
          keadaan="terkunci"
        />
      </Kartu>

      <Tombol jenis="teks" onClick={() => navigate('/', { replace: true })}>
        Lewati dulu →
      </Tombol>
    </div>
  )
}

function LangkahOnboarding({
  nomor,
  judul,
  keterangan,
  keadaan,
  aksi,
}: {
  nomor: number
  judul: string
  keterangan: string
  keadaan: 'selesai' | 'aktif' | 'terkunci'
  aksi?: { label: string; ke: string }
}) {
  return (
    <div className="flex items-center gap-3 p-4">
      <span
        className={cn(
          'flex h-9 w-9 shrink-0 items-center justify-center rounded-full text-label font-bold',
          keadaan === 'selesai' && 'bg-hijau-100 text-hijau-800',
          keadaan === 'aktif' && 'bg-utama text-utama-teks',
          keadaan === 'terkunci' && 'bg-permukaan-2 text-teks-redup',
        )}
        aria-hidden
      >
        {keadaan === 'selesai' ? <Check className="h-5 w-5" /> : nomor}
      </span>

      <div className="min-w-0 flex-1">
        <p
          className={cn(
            'font-semibold',
            keadaan === 'terkunci' ? 'text-teks-redup' : 'text-teks-utama',
          )}
        >
          {judul}
        </p>
        <p className="text-keterangan text-teks-redup">{keterangan}</p>
      </div>

      {aksi ? (
        <Tombol ukuran="padat" asChild>
          <Link to={aksi.ke}>{aksi.label}</Link>
        </Tombol>
      ) : (
        <span className="flex items-center gap-1 text-keterangan text-teks-redup">
          <Lock className="h-4 w-4" aria-hidden />
          Terkunci
        </span>
      )}
    </div>
  )
}
