import { useState } from 'react'
import { ArrowDownLeft, ArrowUpRight, Wallet } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { Kolom } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatJam } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { useGerakanKas, useShiftAktif } from '../hooks'

/**
 * Uang masuk & keluar laci di luar penjualan — ambil kembalian, bayar parkir,
 * setor ke pemilik.
 *
 * Angka ini ikut membentuk "uang yang seharusnya ada di laci" saat tutup shift,
 * jadi keterangannya wajib diisi: tanpa alasan, selisih kas jadi tidak bisa
 * ditelusuri.
 */
export function HalamanKas() {
  const { tokoAktif } = useSesi()
  const toast = useToast()
  const { shift, memuat } = useShiftAktif()
  const { daftar, catat } = useGerakanKas(shift?.id)

  const [arah, setArah] = useState<'in' | 'out'>('out')
  const [nominal, setNominal] = useState(0)
  const [alasan, setAlasan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)

  if (memuat) return <KerangkaBaris jumlah={3} />

  if (!shift) {
    return (
      <KeadaanKosong
        ikon={Wallet}
        judul="Kasir belum dibuka"
        penjelasan="Uang masuk dan keluar laci hanya bisa dicatat saat kasir sedang dibuka."
      />
    )
  }

  async function kirim(e: React.FormEvent) {
    e.preventDefault()
    if (!tokoAktif || !shift) return
    setGalat(null)
    try {
      await catat.mutateAsync({
        outlet_id: tokoAktif,
        shift_id: shift.id,
        direction: arah,
        amount: nominal,
        reason: alasan.trim(),
      })
      toast.berhasil(
        arah === 'in'
          ? `Uang masuk ${formatRupiah(nominal)} tercatat.`
          : `Uang keluar ${formatRupiah(nominal)} tercatat.`,
      )
      setNominal(0)
      setAlasan('')
    } catch (e) {
      setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.')
    }
  }

  return (
    <div className="mx-auto flex w-full max-w-lg flex-col gap-4">
      <h1 className="text-judul font-bold text-teks-utama">Uang Masuk & Keluar</h1>

      <Kartu className="p-4">
        <form onSubmit={kirim} className="flex flex-col gap-4">
          <fieldset>
            <legend className="mb-2 text-label font-medium text-teks-sekunder">
              Jenis
            </legend>
            <div className="grid grid-cols-2 gap-2">
              <PilihArah
                aktif={arah === 'in'}
                onKlik={() => setArah('in')}
                ikon={ArrowDownLeft}
                label="Uang masuk"
                keterangan="Tambahan modal, setoran"
              />
              <PilihArah
                aktif={arah === 'out'}
                onKlik={() => setArah('out')}
                ikon={ArrowUpRight}
                label="Uang keluar"
                keterangan="Belanja, setor ke pemilik"
              />
            </div>
          </fieldset>

          <KolomUang
            label="Nominal"
            nilai={nominal}
            onNilai={setNominal}
            bantuan="Jumlah uang yang berpindah dari atau ke laci."
          />

          <Kolom
            label="Keterangan"
            placeholder="Contoh: beli plastik kresek"
            value={alasan}
            onChange={(e) => setAlasan(e.target.value)}
            bantuan="Wajib diisi supaya selisih kas nanti bisa ditelusuri."
            maxLength={200}
            required
          />

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <Tombol
            type="submit"
            lebarPenuh
            memuat={catat.isPending}
            labelMemuat="Menyimpan…"
            disabled={nominal <= 0 || alasan.trim().length === 0}
          >
            Catat {arah === 'in' ? 'Uang Masuk' : 'Uang Keluar'}
          </Tombol>
        </form>
      </Kartu>

      <section>
        <h2 className="mb-2 text-judul-kartu font-semibold text-teks-utama">
          Sudah dicatat hari ini
        </h2>
        {daftar.isLoading ? (
          <KerangkaBaris jumlah={2} />
        ) : (daftar.data?.data.length ?? 0) === 0 ? (
          <p className="text-isi text-teks-redup">
            Belum ada uang masuk atau keluar di shift ini.
          </p>
        ) : (
          <ul className="flex flex-col gap-2">
            {daftar.data?.data.map((g) => (
              <li key={g.id}>
                <Kartu className="flex items-center justify-between gap-3 p-3">
                  <div className="min-w-0">
                    <p className="truncate text-label text-teks-utama">{g.reason}</p>
                    <p className="text-keterangan text-teks-redup">
                      {formatJam(g.occurred_at)}
                    </p>
                  </div>
                  <p
                    className={cn(
                      'shrink-0 font-bold tabular-nums',
                      g.direction === 'in' ? 'text-hijau-700' : 'text-bahaya-teks',
                    )}
                  >
                    {g.direction === 'in' ? '+' : '−'}
                    {formatRupiah(g.amount).replace('Rp ', 'Rp ')}
                  </p>
                </Kartu>
              </li>
            ))}
          </ul>
        )}
      </section>
    </div>
  )
}

function PilihArah({
  aktif,
  onKlik,
  ikon: Ikon,
  label,
  keterangan,
}: {
  aktif: boolean
  onKlik: () => void
  ikon: typeof ArrowDownLeft
  label: string
  keterangan: string
}) {
  return (
    <button
      type="button"
      onClick={onKlik}
      aria-pressed={aktif}
      className={cn(
        'flex min-h-20 flex-col items-start gap-1 rounded-kartu border p-3 text-left',
        aktif
          ? 'border-utama bg-sorot'
          : 'border-garis bg-permukaan hover:bg-permukaan-2',
      )}
    >
      <Ikon
        className={cn('h-5 w-5', aktif ? 'text-utama' : 'text-teks-redup')}
        aria-hidden
      />
      <span
        className={cn(
          'text-label font-semibold',
          aktif ? 'text-utama' : 'text-teks-utama',
        )}
      >
        {label}
      </span>
      <span className="text-keterangan text-teks-redup">{keterangan}</span>
    </button>
  )
}
