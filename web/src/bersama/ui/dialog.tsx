import * as D from '@radix-ui/react-dialog'
import { X } from 'lucide-react'
import { cn } from '@/bersama/util/cn'

/**
 * Dialog. Di HP masuk dari bawah (kebiasaan aplikasi ponsel), di desktop memudar.
 *
 * Dipakai HEMAT: dialog "Anda yakin?" akan diklik "Ya" tanpa dibaca setelah kali
 * kelima. Yang layak berdialog hanya aksi berat & tidak bisa dibalik — dan
 * isinya harus MENJELASKAN AKIBAT dengan angka, bukan sekadar bertanya
 * (ui/01-PRINSIP-DESAIN.md §6).
 */
export const Dialog = D.Root
export const PemicuDialog = D.Trigger
export const TutupDialog = D.Close

export function IsiDialog({
  className,
  children,
  judul,
  keterangan,
  ...sisa
}: D.DialogContentProps & { judul: string; keterangan?: string }) {
  return (
    <D.Portal>
      <D.Overlay className="gerak-lapis fixed inset-0 z-40 bg-black/40" />
      <D.Content
        className={cn(
          'gerak-dialog fixed z-50 flex flex-col gap-3 bg-permukaan shadow-dialog',
          // HP: lembar dari bawah.
          'inset-x-0 bottom-0 max-h-[90dvh] overflow-y-auto rounded-t-dialog p-5',
          // Layar besar: kotak di tengah.
          'sm:inset-auto sm:left-1/2 sm:top-1/2 sm:w-full sm:max-w-md',
          'sm:-translate-x-1/2 sm:-translate-y-1/2 sm:rounded-dialog',
          className,
        )}
        {...sisa}
      >
        <div className="flex items-start justify-between gap-4">
          <div className="flex flex-col gap-1">
            <D.Title className="text-judul-kartu font-semibold text-teks-utama">
              {judul}
            </D.Title>
            {keterangan && (
              <D.Description className="text-isi text-teks-sekunder">
                {keterangan}
              </D.Description>
            )}
          </div>
          <D.Close
            aria-label="Tutup"
            className="-m-2 shrink-0 rounded-kontrol p-2 text-teks-redup hover:bg-permukaan-2"
          >
            <X className="h-5 w-5" aria-hidden />
          </D.Close>
        </div>
        {children}
      </D.Content>
    </D.Portal>
  )
}

/** Baris tombol dialog: aksi berbahaya diberi jarak dari tombol batal. */
export function AksiDialog({ className, ...sisa }: React.HTMLAttributes<HTMLDivElement>) {
  return <div className={cn('mt-2 flex flex-col gap-2 sm:flex-row-reverse', className)} {...sisa} />
}
