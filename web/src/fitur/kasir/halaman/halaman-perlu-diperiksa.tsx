import { useLiveQuery } from 'dexie-react-hooks'
import { CheckCircle2, RefreshCw, Trash2 } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Tombol } from '@/bersama/ui/tombol'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { useToast } from '@/bersama/komponen/toast'
import { buangDariAntrean, cobaLagi } from '@/lib/offline/antrean'
import { db } from '@/lib/offline/db'
import { useSinkron } from '@/lib/offline/mesin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatTanggalJam } from '@/bersama/util/tanggal'

/**
 * Satu tempat untuk transaksi yang gagal terkirim setelah beberapa percobaan.
 *
 * Nadanya sengaja tidak menyalahkan: kegagalan jaringan bukan kesalahan
 * pengguna. Yang ditawarkan adalah tindakan, bukan teguran
 * (ui/05-ALUR-UTAMA.md §7).
 */
export function HalamanPerluDiperiksa() {
  const toast = useToast()
  const sinkron = useSinkron()

  const daftar = useLiveQuery(
    () => db.antrean.where('status').equals('perlu-diperiksa').toArray(),
    [],
  )

  const menunggu = useLiveQuery(
    () => db.antrean.where('status').anyOf('menunggu', 'mengirim').toArray(),
    [],
  )

  if (daftar === undefined) return null

  return (
    <div className="flex w-full max-w-2xl flex-col gap-4">
      <header>
        <h1 className="text-judul font-bold text-teks-utama">Transaksi Belum Terkirim</h1>
        <p className="text-isi text-teks-sekunder">
          Transaksinya tersimpan di perangkat ini dan tidak hilang.
        </p>
      </header>

      {(menunggu?.length ?? 0) > 0 && (
        <Kartu className="flex items-center justify-between gap-3 border-jingga-600 bg-permukaan-2 p-4">
          <p className="text-label text-jingga-700">
            {menunggu?.length} transaksi sedang menunggu giliran dikirim. Ini
            berjalan sendiri begitu ada internet.
          </p>
          <Tombol
            jenis="kedua"
            ukuran="padat"
            onClick={() => void sinkron.kirimSekarang()}
          >
            <RefreshCw className="h-5 w-5" aria-hidden />
            Kirim sekarang
          </Tombol>
        </Kartu>
      )}

      {daftar.length === 0 ? (
        <KeadaanKosong
          ikon={CheckCircle2}
          judul="Semua transaksi sudah terkirim"
          penjelasan="Tidak ada yang perlu diperiksa. Semua penjualan sudah tercatat di server."
        />
      ) : (
        <ul className="flex flex-col gap-2">
          {daftar.map((o) => (
            <li key={o.id}>
              <Kartu className="flex flex-col gap-3 p-4">
                <div className="flex items-start justify-between gap-3">
                  <div className="min-w-0">
                    <p className="font-semibold text-teks-utama">{o.ringkasan}</p>
                    <p className="text-keterangan text-teks-redup">
                      {formatTanggalJam(new Date(o.dibuatPada).toISOString())} ·
                      dicoba {o.percobaan}×
                    </p>
                  </div>
                  <p className="shrink-0 font-bold tabular-nums text-teks-utama">
                    {formatRupiah(o.nominal)}
                  </p>
                </div>

                {o.alasan && (
                  <p className="rounded-kontrol bg-permukaan-2 px-3 py-2 text-keterangan text-teks-sekunder">
                    {o.alasan}
                  </p>
                )}

                <div className="flex flex-wrap gap-2">
                  <Tombol
                    ukuran="padat"
                    onClick={async () => {
                      await cobaLagi(o.id)
                      sinkron.segarkan()
                      await sinkron.kirimSekarang()
                    }}
                  >
                    <RefreshCw className="h-5 w-5" aria-hidden />
                    Coba kirim lagi
                  </Tombol>
                  {/* Membuang transaksi berarti kehilangan catatan penjualan,
                      jadi dijauhkan dari tombol utama dan diberi peringatan. */}
                  <Tombol
                    jenis="teks"
                    ukuran="padat"
                    onClick={async () => {
                      await buangDariAntrean(o.id)
                      sinkron.segarkan()
                      toast.tampilkan(
                        'Transaksi dibuang dari antrean perangkat ini.',
                        'perhatian',
                      )
                    }}
                    className="text-bahaya-teks"
                  >
                    <Trash2 className="h-5 w-5" aria-hidden />
                    Buang
                  </Tombol>
                </div>
              </Kartu>
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
