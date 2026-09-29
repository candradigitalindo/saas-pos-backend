import { cn } from '@/bersama/util/cn'

/**
 * Kerangka (skeleton) untuk proses lebih dari 1 detik.
 *
 * Bukan spinner kosong di tengah layar: kerangka memberi tahu BENTUK apa yang
 * sedang datang, sehingga layar tidak terasa melompat saat data muncul.
 */
export function Kerangka({ className }: { className?: string }) {
  return (
    <div
      className={cn('animate-pulse rounded-kontrol bg-permukaan-2', className)}
      aria-hidden
    />
  )
}

export function KerangkaBaris({ jumlah = 3 }: { jumlah?: number }) {
  return (
    <div className="flex flex-col gap-2" role="status" aria-label="Memuat data">
      {Array.from({ length: jumlah }, (_, i) => (
        <Kerangka key={i} className="h-16 w-full" />
      ))}
    </div>
  )
}

export function KerangkaKartuAngka() {
  return (
    <div className="rounded-kartu border border-garis bg-permukaan p-4 shadow-kartu">
      <Kerangka className="h-4 w-32" />
      <Kerangka className="mt-3 h-10 w-44" />
      <Kerangka className="mt-2 h-3 w-28" />
    </div>
  )
}
