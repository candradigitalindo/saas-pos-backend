import { cn } from '@/bersama/util/cn'

/** Kartu: sudut 12px, bayangan nyaris rata — bayangan tebal bikin UI terasa berat. */
export function Kartu({ className, ...sisa }: React.HTMLAttributes<HTMLDivElement>) {
  return (
    <div
      className={cn(
        'rounded-kartu border border-garis bg-permukaan shadow-kartu',
        className,
      )}
      {...sisa}
    />
  )
}

export function KepalaKartu({ className, ...sisa }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('flex flex-col gap-1 p-4 pb-2', className)} {...sisa} />
}

export function JudulKartu({ className, ...sisa }: React.HTMLAttributes<HTMLHeadingElement>) {
  return (
    <h2
      className={cn('text-judul-kartu font-semibold text-teks-utama', className)}
      {...sisa}
    />
  )
}

export function KeteranganKartu({
  className,
  ...sisa
}: React.HTMLAttributes<HTMLParagraphElement>) {
  return <p className={cn('text-keterangan text-teks-redup', className)} {...sisa} />
}

export function IsiKartu({ className, ...sisa }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('p-4 pt-2', className)} {...sisa} />
}
