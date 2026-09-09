import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { formatRupiah } from '@/bersama/util/uang'
import type { BarisLaporan } from '../api'

/**
 * Penjualan per hari.
 *
 * Satu deret, jadi tidak perlu legenda — judulnya sudah menyebut apa yang
 * digambar. Sumbu tegak hanya satu (tidak pernah dua skala), grid samar, dan
 * angka tepatnya muncul saat disentuh.
 *
 * Berkas ini sengaja dipisah supaya Recharts hanya diunduh oleh orang yang
 * benar-benar membuka laporan.
 */
export function GrafikHarian({ rows }: { rows: BarisLaporan[] }) {
  const data = rows.map((r) => ({
    hari: r.key.slice(8),  // "2026-09-10" → "10"
    tanggal: r.key,
    omzet: r.net_amount,
  }))

  return (
    <div className="h-56 w-full">
      <ResponsiveContainer width="100%" height="100%">
        <BarChart data={data} margin={{ top: 8, right: 8, bottom: 0, left: 8 }}>
          <CartesianGrid
            strokeDasharray="3 3"
            vertical={false}
            stroke="var(--warna-garis)"
          />
          <XAxis
            dataKey="hari"
            tickLine={false}
            axisLine={false}
            tick={{ fill: 'var(--warna-teks-redup)', fontSize: 12 }}
          />
          <YAxis
            tickLine={false}
            axisLine={false}
            width={64}
            tick={{ fill: 'var(--warna-teks-redup)', fontSize: 12 }}
            tickFormatter={(n: number) =>
              n >= 1_000_000
                ? `${Math.round(n / 100_000) / 10} jt`
                : `${Math.round(n / 1000)} rb`
            }
          />
          <Tooltip
            cursor={{ fill: 'var(--warna-permukaan-2)' }}
            contentStyle={{
              background: 'var(--warna-permukaan)',
              border: '1px solid var(--warna-garis)',
              borderRadius: 12,
              color: 'var(--warna-teks-utama)',
            }}
            labelFormatter={(_l, p) => p?.[0]?.payload?.tanggal ?? ''}
            formatter={(n) => [formatRupiah(Number(n) || 0), 'Uang masuk']}
          />
          <Bar
            dataKey="omzet"
            fill="var(--color-grafik-1)"
            radius={[4, 4, 0, 0]}
            maxBarSize={40}
          />
        </BarChart>
      </ResponsiveContainer>
    </div>
  )
}
