import { useMemo, useState } from 'react'
import { useLiveQuery } from 'dexie-react-hooks'
import { Check, Search } from 'lucide-react'
import { Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { db } from '@/lib/offline/db'
import { cn } from '@/bersama/util/cn'
import type { PelangganKeranjang } from '../keranjang'

/**
 * Memilih pembeli transaksi dari data lokal (jalan saat offline). Pembeli yang
 * punya daftar harga khusus langsung mengubah harga di keranjang — namanya
 * disebut di sini supaya kasir tahu sebelum memilih.
 */
export function DialogPilihPelanggan({
  sekarang,
  onPilih,
  onTutup,
}: {
  sekarang: PelangganKeranjang | null
  onPilih: (p: PelangganKeranjang | null) => void
  onTutup: () => void
}) {
  const [cari, setCari] = useState('')
  const semua = useLiveQuery(() => db.pelanggan.orderBy('name').toArray(), []) ?? []
  const namaDaftar = useLiveQuery(
    async () => new Map((await db.daftarHarga.toArray()).filter((l) => !l.is_default).map((l) => [l.id, l.name])),
    [],
  )
  const cocok = useMemo(() => {
    const q = cari.trim().toLowerCase()
    const hasil = q ? semua.filter((p) => p.name.toLowerCase().includes(q) || (p.phone ?? '').includes(q)) : semua
    return hasil.slice(0, 50)
  }, [semua, cari])

  const baris = (aktif: boolean, isi: React.ReactNode, onKlik: () => void, kunci: string) => (
    <li key={kunci}>
      <button
        type="button"
        onClick={onKlik}
        aria-pressed={aktif}
        className={cn(
          'flex min-h-12 w-full items-center gap-3 rounded-kontrol px-2 py-1.5 text-left hover:bg-permukaan-2',
          aktif && 'bg-sorot',
        )}
      >
        <span className="min-w-0 flex-1">{isi}</span>
        {aktif && <Check className="h-5 w-5 shrink-0 text-utama" aria-hidden />}
      </button>
    </li>
  )

  return (
    <Dialog open onOpenChange={(o) => !o && onTutup()}>
      <IsiDialog judul="Pelanggan" keterangan="Pelanggan ber-daftar harga (member, reseller) mendapat harga khususnya.">
        <label className="flex h-12 items-center gap-2 rounded-kontrol border border-garis bg-permukaan px-3 focus-within:border-utama">
          <Search className="h-5 w-5 shrink-0 text-teks-redup" aria-hidden />
          <input
            value={cari}
            onChange={(e) => setCari(e.target.value)}
            placeholder="Cari nama atau nomor HP"
            aria-label="Cari pelanggan"
            className="min-w-0 flex-1 bg-transparent text-isi outline-none placeholder:text-teks-redup"
          />
        </label>
        <ul className="-mx-2 flex max-h-[50dvh] flex-col overflow-y-auto">
          {!cari.trim() &&
            baris(
              sekarang === null,
              <span className="block text-isi text-teks-utama">Umum (tanpa pelanggan)</span>,
              () => onPilih(null),
              'umum',
            )}
          {cocok.map((p) => {
            const daftar = p.price_list_id ? namaDaftar?.get(p.price_list_id) : undefined
            return baris(
              sekarang?.id === p.id,
              <>
                <span className="block truncate text-isi text-teks-utama">{p.name}</span>
                <span className="block truncate text-keterangan text-teks-redup">
                  {[p.phone, daftar ? `harga ${daftar}` : null].filter(Boolean).join(' · ') || 'Harga umum'}
                </span>
              </>,
              () =>
                onPilih({
                  id: p.id,
                  name: p.name,
                  phone: p.phone,
                  price_list_id: daftar ? p.price_list_id : null,
                  nama_daftar: daftar,
                }),
              p.id,
            )
          })}
          {cocok.length === 0 && (
            <li className="px-2 py-6 text-center text-label text-teks-redup">Tidak ada pelanggan yang cocok.</li>
          )}
        </ul>
      </IsiDialog>
    </Dialog>
  )
}
