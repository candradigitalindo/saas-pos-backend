import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Search } from 'lucide-react'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { cn } from '@/bersama/util/cn'
import { kanalApi, type ItemMenuKanal } from '../api'

/**
 * Memilih barang yang dijual di aplikasi antar. Menyimpan BELUM mengirim:
 * menu baru sampai ke GoFood/Grab setelah "Kirim Menu" (yang mengganti menu
 * di sana dan wajib dikonfirmasi). Barang dikelompokkan per kategori —
 * pemilik biasanya berpikir "semua minuman ya, makanan berat sebagian".
 */
export function DialogMenuKanal({
  kanalId,
  penyedia,
  onTutup,
}: {
  kanalId: string
  penyedia: string
  onTutup: () => void
}) {
  const toast = useToast()
  const qc = useQueryClient()
  const menu = useQuery({ queryKey: ['menu-kanal', kanalId], queryFn: () => kanalApi.menu(kanalId) })
  const [dipilih, setDipilih] = useState<Set<string> | null>(null)
  const [cari, setCari] = useState('')
  const [galat, setGalat] = useState<string | null>(null)

  const pilihan = dipilih ?? new Set((menu.data?.items ?? []).filter((i) => i.in_menu).map((i) => i.product_id))
  const kelompok = useMemo(() => {
    const q = cari.trim().toLowerCase()
    const peta = new Map<string, ItemMenuKanal[]>()
    for (const i of menu.data?.items ?? []) {
      if (q && !i.name.toLowerCase().includes(q) && !(i.sku ?? '').toLowerCase().includes(q)) continue
      peta.set(i.category, [...(peta.get(i.category) ?? []), i])
    }
    return [...peta.entries()]
  }, [menu.data, cari])

  const ubah = (ids: string[], pilih: boolean) => {
    const baru = new Set(pilihan)
    for (const id of ids) {
      if (pilih) baru.add(id)
      else baru.delete(id)
    }
    setDipilih(baru)
  }

  const simpan = useMutation({
    mutationFn: () => kanalApi.simpanMenu(kanalId, [...pilihan]),
    onSuccess: (r) => {
      qc.setQueryData(['menu-kanal', kanalId], r)
      toast.berhasil(`Menu disimpan — tekan Kirim Menu untuk mengirimkannya ke ${penyedia}.`)
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Gagal menyimpan menu.'),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !simpan.isPending && onTutup()}>
      <IsiDialog
        judul={`Menu ${penyedia}`}
        keterangan="Centang barang yang dijual di aplikasi antar. Harga memakai harga kanal bila diisi, selain itu harga jual."
        className="sm:max-w-2xl"
      >
        {menu.isLoading ? (
          <KerangkaBaris jumlah={5} />
        ) : (
          <div className="flex flex-col gap-3">
            <label className="flex h-11 items-center gap-2 rounded-kontrol border border-garis bg-permukaan px-3 focus-within:border-utama">
              <Search className="h-4 w-4 shrink-0 text-teks-redup" aria-hidden />
              <input
                value={cari}
                onChange={(e) => setCari(e.target.value)}
                placeholder="Cari barang atau SKU"
                aria-label="Cari barang"
                className="min-w-0 flex-1 bg-transparent text-isi outline-none placeholder:text-teks-redup"
              />
            </label>
            <div className="flex max-h-[50vh] flex-col gap-4 overflow-y-auto">
              {kelompok.length === 0 && (
                <p className="py-6 text-center text-label text-teks-redup">Tidak ada barang yang cocok.</p>
              )}
              {kelompok.map(([kategori, items]) => {
                const ids = items.map((i) => i.product_id)
                const n = ids.filter((id) => pilihan.has(id)).length
                return (
                  <fieldset key={kategori} className="flex flex-col gap-1">
                    <legend className="mb-1 flex w-full items-center justify-between gap-2">
                      <span className="text-label font-semibold text-teks-utama">
                        {kategori} <span className="font-normal text-teks-redup">· {n}/{items.length}</span>
                      </span>
                      <button
                        type="button"
                        onClick={() => ubah(ids, n < items.length)}
                        className="-my-2 min-h-11 rounded-kontrol px-2 text-keterangan font-semibold text-utama hover:bg-sorot"
                      >
                        {n < items.length ? 'Pilih semua' : 'Lepas semua'}
                      </button>
                    </legend>
                    {items.map((i) => (
                      <label
                        key={i.product_id}
                        className="flex min-h-12 cursor-pointer items-center gap-3 rounded-kontrol px-2 py-1.5 hover:bg-permukaan-2"
                      >
                        <input
                          type="checkbox"
                          checked={pilihan.has(i.product_id)}
                          onChange={(e) => ubah([i.product_id], e.target.checked)}
                          className="h-5 w-5 shrink-0 accent-[var(--warna-utama)]"
                        />
                        <span className="min-w-0 flex-1">
                          <span className="block truncate text-label text-teks-utama">{i.name}</span>
                          {(i.sku || !i.track_stock) && (
                            <span className="block truncate text-keterangan text-teks-redup">
                              {[i.sku, i.track_stock ? null : 'selalu tersedia (stok tidak dilacak)']
                                .filter(Boolean)
                                .join(' · ')}
                            </span>
                          )}
                        </span>
                        <span
                          className={cn(
                            'shrink-0 text-label tabular-nums',
                            pilihan.has(i.product_id) ? 'text-teks-utama' : 'text-teks-redup',
                          )}
                        >
                          {formatRupiah(i.price)}
                        </span>
                      </label>
                    ))}
                  </fieldset>
                )
              })}
            </div>
          </div>
        )}
        {galat && (
          <p role="alert" className="text-label text-bahaya-teks">
            {galat}
          </p>
        )}
        <AksiDialog>
          <Tombol
            onClick={() => {
              setGalat(null)
              simpan.mutate()
            }}
            memuat={simpan.isPending}
            disabled={menu.isLoading}
          >
            Simpan Menu ({pilihan.size})
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={simpan.isPending}>
            Batal
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}
