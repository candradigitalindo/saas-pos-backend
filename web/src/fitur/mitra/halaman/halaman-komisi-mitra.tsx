import { useQuery } from '@tanstack/react-query'
import { Coins } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { formatRupiah } from '@/bersama/util/uang'
import { formatTanggal } from '@/bersama/util/tanggal'
import { mitraApi } from '../api'

/** Rincian komisi dan riwayat pencairan. */
export function HalamanKomisiMitra() {
  const komisi = useQuery({ queryKey: ['mitra-komisi'], queryFn: () => mitraApi.komisi() })
  const cair = useQuery({ queryKey: ['mitra-cair'], queryFn: () => mitraApi.pencairan() })

  return (
    <div className="flex flex-col gap-6">
      <section className="flex flex-col gap-2">
        <h1 className="text-judul font-bold text-teks-utama">Komisi</h1>

        {komisi.isLoading ? (
          <KerangkaBaris jumlah={4} />
        ) : (komisi.data?.length ?? 0) === 0 ? (
          <KeadaanKosong
            ikon={Coins}
            judul="Belum ada komisi"
            penjelasan="Komisi muncul setelah merchant binaan Anda membayar tagihan langganannya."
          />
        ) : (
          <Kartu className="divide-y divide-garis">
            {komisi.data?.map((k) => (
              <div key={k.id} className="flex items-center justify-between gap-3 p-4">
                <div className="min-w-0">
                  <p className="font-medium tabular-nums text-teks-utama">
                    {k.period_month}
                  </p>
                  <p className="text-keterangan tabular-nums text-teks-redup">
                    {formatRupiah(k.base_amount)} × {(Number(k.rate) * 100).toFixed(0)}%
                  </p>
                </div>
                <div className="flex shrink-0 items-center gap-3">
                  <p className="font-bold tabular-nums text-teks-utama">
                    {formatRupiah(k.amount)}
                  </p>
                  <StatusKomisi status={k.status} />
                </div>
              </div>
            ))}
          </Kartu>
        )}
      </section>

      <section className="flex flex-col gap-2">
        <h2 className="text-judul-kartu font-semibold text-teks-utama">
          Riwayat pencairan
        </h2>

        {cair.isLoading ? (
          <KerangkaBaris jumlah={2} />
        ) : (cair.data?.length ?? 0) === 0 ? (
          <p className="text-isi text-teks-redup">Belum pernah ada pencairan.</p>
        ) : (
          <Kartu className="divide-y divide-garis">
            {cair.data?.map((p) => (
              <div key={p.id} className="flex flex-col gap-1 p-4">
                <div className="flex items-center justify-between gap-3">
                  <p className="font-medium text-teks-utama">
                    {formatTanggal(p.period_start)} – {formatTanggal(p.period_end)}
                  </p>
                  <p className="font-bold tabular-nums text-teks-utama">
                    {formatRupiah(p.net_amount)}
                  </p>
                </div>
                {/* Rinciannya dibuka apa adanya — mitra berhak tahu kenapa yang
                    diterimanya lebih kecil dari komisi kotornya. */}
                <p className="text-keterangan tabular-nums text-teks-redup">
                  Kotor {formatRupiah(p.gross_amount)}
                  {p.clawback_amount > 0 &&
                    ` − tarikan kembali ${formatRupiah(p.clawback_amount)}`}
                  {p.tax_amount > 0 && ` − pajak ${formatRupiah(p.tax_amount)}`}
                </p>
                <div className="mt-1">
                  {p.status === 'paid' ? (
                    <LencanaStatus nada="berhasil" anak="Sudah cair" />
                  ) : (
                    <LencanaStatus nada="menunggu" anak="Sedang diproses" />
                  )}
                </div>
              </div>
            ))}
          </Kartu>
        )}
      </section>
    </div>
  )
}

function StatusKomisi({ status }: { status: string }) {
  switch (status) {
    case 'approved':
      return <LencanaStatus nada="berhasil" anak="Disetujui" />
    case 'paid':
      return <LencanaStatus nada="berhasil" anak="Sudah cair" />
    case 'held':
    case 'draft':
      return <LencanaStatus nada="menunggu" anak="Diperiksa" />
    case 'clawed_back':
      return <LencanaStatus nada="bahaya" anak="Ditarik kembali" />
    default:
      return <LencanaStatus nada="netral" anak={status} />
  }
}
