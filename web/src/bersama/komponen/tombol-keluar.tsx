import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { LogOut } from 'lucide-react'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { Tombol } from '@/bersama/ui/tombol'
import { GalatAdaAntrean, useSesi } from '@/bersama/hooks/use-sesi'
import { cn } from '@/bersama/util/cn'

/**
 * Tombol keluar.
 *
 * Keluar membuang seluruh data lokal, termasuk transaksi yang belum terkirim.
 * Karena itu satu-satunya dialog konfirmasi yang layak di sini adalah yang
 * MENYEBUT BERAPA transaksi akan hilang — bukan "Anda yakin?".
 */
export function TombolKeluar({
  className,
  ringkas = false,
}: {
  className?: string
  /** Ikon saja (48×48), untuk baris profil di kaki navigasi samping. */
  ringkas?: boolean
}) {
  const { keluar } = useSesi()
  const navigate = useNavigate()
  const [belumTerkirim, setBelumTerkirim] = useState<number | null>(null)
  const [sedang, setSedang] = useState(false)

  async function coba(paksa = false) {
    setSedang(true)
    try {
      await keluar(paksa)
      navigate('/masuk', { replace: true })
    } catch (e) {
      if (e instanceof GalatAdaAntrean) setBelumTerkirim(e.jumlah)
      else navigate('/masuk', { replace: true })
    } finally {
      setSedang(false)
    }
  }

  return (
    <>
      {ringkas ? (
        <button
          type="button"
          onClick={() => void coba()}
          aria-label="Keluar"
          title="Keluar"
          className={cn(
            'flex h-12 w-12 shrink-0 items-center justify-center rounded-kontrol',
            'text-teks-sekunder hover:bg-permukaan-2 hover:text-teks-utama',
            className,
          )}
        >
          <LogOut className="h-5 w-5" aria-hidden />
        </button>
      ) : (
        <button
          type="button"
          onClick={() => void coba()}
          className={cn(
            'flex h-12 w-full items-center gap-3 rounded-kontrol px-3',
            'text-label font-medium text-teks-sekunder hover:bg-permukaan-2',
            className,
          )}
        >
          <LogOut className="h-5 w-5" aria-hidden />
          Keluar
        </button>
      )}

      <Dialog
        open={belumTerkirim !== null}
        onOpenChange={(o) => !o && setBelumTerkirim(null)}
      >
        <IsiDialog judul="Masih ada transaksi yang belum terkirim">
          <div className="flex flex-col gap-2 text-isi text-teks-sekunder">
            <p>
              <strong className="tabular-nums text-teks-utama">{belumTerkirim}</strong>{' '}
              transaksi masih tersimpan di perangkat ini dan belum sampai ke server.
            </p>
            <p>
              Kalau Anda keluar sekarang, catatan penjualan itu akan hilang dan
              tidak bisa dikembalikan.
            </p>
            <p>
              Sambungkan ke internet sebentar supaya semuanya terkirim dulu.
            </p>
          </div>

          <AksiDialog>
            <Tombol
              jenis="kedua"
              onClick={() => {
                setBelumTerkirim(null)
                navigate('/kasir/belum-terkirim')
              }}
            >
              Lihat transaksinya
            </Tombol>
            <Tombol
              jenis="bahaya"
              memuat={sedang}
              labelMemuat="Keluar…"
              onClick={() => void coba(true)}
            >
              Keluar & buang
            </Tombol>
          </AksiDialog>
        </IsiDialog>
      </Dialog>
    </>
  )
}
