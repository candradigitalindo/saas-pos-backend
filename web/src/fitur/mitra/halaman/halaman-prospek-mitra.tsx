import { useQuery } from '@tanstack/react-query'
import { UserSearch } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { formatTanggal } from '@/bersama/util/tanggal'
import { mitraApi } from '../api'

/**
 * Prospek yang didaftarkan mitra.
 *
 * Tanggal berakhirnya atribusi ditampilkan terang-terangan: setelah lewat
 * tanggal itu, merchant yang mendaftar tidak lagi dihitung sebagai hasil kerja
 * mitra ini. Menyembunyikannya hanya menunda kekecewaan.
 */
export function HalamanProspekMitra() {
  const q = useQuery({ queryKey: ['mitra-prospek'], queryFn: () => mitraApi.daftarProspek() })
  const daftar = q.data ?? []

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-judul font-bold text-teks-utama">Prospek</h1>

      {q.isLoading ? (
        <KerangkaBaris jumlah={4} />
      ) : daftar.length === 0 ? (
        <KeadaanKosong
          ikon={UserSearch}
          judul="Belum ada prospek"
          penjelasan="Daftarkan calon merchant dari halaman Beranda supaya komisinya tetap jadi milik Anda saat mereka berlangganan."
        />
      ) : (
        <Kartu className="divide-y divide-garis">
          {daftar.map((p) => (
            <div key={p.id} className="flex items-center justify-between gap-3 p-4">
              <div className="min-w-0">
                <p className="truncate font-medium text-teks-utama">{p.business_name}</p>
                <p className="text-keterangan text-teks-redup">
                  {p.city || 'Tanpa kota'}
                  {p.phone && ` · ${p.phone}`}
                </p>
                <p className="text-keterangan text-teks-redup">
                  Atribusi berlaku sampai {formatTanggal(p.attribution_expires_at)}
                </p>
              </div>
              <StatusProspek status={p.status} />
            </div>
          ))}
        </Kartu>
      )}
    </div>
  )
}

function StatusProspek({ status }: { status: string }) {
  switch (status) {
    case 'converted':
      return <LencanaStatus nada="berhasil" anak="Jadi merchant" />
    case 'expired':
      return <LencanaStatus nada="bahaya" anak="Kedaluwarsa" />
    case 'new':
    case 'contacted':
      return <LencanaStatus nada="menunggu" anak="Menunggu" />
    default:
      return <LencanaStatus nada="netral" anak={status} />
  }
}
