import { useState } from 'react'
import { useLiveQuery } from 'dexie-react-hooks'
import { ChevronRight, Trash2 } from 'lucide-react'
import { Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { Tombol } from '@/bersama/ui/tombol'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { formatRupiah } from '@/bersama/util/uang'
import { formatJam } from '@/bersama/util/tanggal'
import { hitungTotal, type AturanHarga } from '@/bersama/util/total'
import type { TagihanLokal } from '@/lib/offline/db'
import { barisHitung, nominalDiskonTransaksi } from '../keranjang'
import { itemTagihanKeBaris, tagihanKeDiskon } from '../tagihan'

/**
 * Daftar tagihan terbuka cabang ini. Ketuk untuk membukanya di keranjang;
 * batal butuh dua ketukan (pesanan yang lenyap sebelum dibayar harus
 * disengaja). Total dihitung dari katalog lokal — harga final tetap dari
 * server saat dibayar.
 */
export function DialogTagihan({
  daftar,
  aturan,
  keranjangBerisi,
  onBuka,
  onBatalkan,
  onTutup,
}: {
  daftar: TagihanLokal[]
  aturan?: AturanHarga
  /** Keranjang sedang berisi pesanan lain — membuka tagihan akan menimpanya. */
  keranjangBerisi: boolean
  onBuka: (t: TagihanLokal) => void
  /** Melempar galat bila gagal. */
  onBatalkan: (t: TagihanLokal) => Promise<void>
  onTutup: () => void
}) {
  const [yakinBatal, setYakinBatal] = useState<string | null>(null)
  const [membatalkan, setMembatalkan] = useState(false)
  const [galat, setGalat] = useState<string | null>(null)

  // Ringkasan tiap tagihan (jumlah barang & pratinjau total) dari katalog lokal.
  const ringkasan = useLiveQuery(async () => {
    const peta = new Map<string, { jumlah: number; total: number }>()
    for (const t of daftar) {
      const { baris } = await itemTagihanKeBaris(t.items)
      const hitung = barisHitung(baris)
      const nominal = nominalDiskonTransaksi(hitungTotal(hitung, aturan), tagihanKeDiskon(t.order_discount))
      peta.set(t.id, { jumlah: baris.length, total: hitungTotal(hitung, aturan, nominal).total })
    }
    return peta
  }, [daftar, aturan])

  return (
    <Dialog open onOpenChange={(o) => !o && !membatalkan && onTutup()}>
      <IsiDialog judul="Tagihan Terbuka" keterangan="Pesanan yang ditahan, dari semua perangkat di cabang ini.">
        {keranjangBerisi && (
          <p className="rounded-kontrol border border-jingga-600 bg-permukaan-2 px-3 py-2 text-label text-jingga-700">
            Keranjang sedang berisi pesanan. Tahan atau kosongkan dulu sebelum membuka tagihan lain.
          </p>
        )}
        {daftar.length === 0 ? (
          <p className="py-6 text-center text-label text-teks-redup">
            Belum ada tagihan. Tekan “Tahan” di keranjang untuk menyimpan pesanan yang belum dibayar.
          </p>
        ) : (
          <ul className="-mx-2 flex max-h-[55dvh] flex-col overflow-y-auto">
            {daftar.map((t) => {
              const r = ringkasan?.get(t.id)
              if (yakinBatal === t.id) {
                return (
                  <li key={t.id} className="flex flex-col gap-2 rounded-kontrol bg-permukaan-2 px-3 py-2">
                    <p className="text-label text-teks-utama">
                      Batalkan tagihan <strong>{t.label}</strong>? Pesanannya tidak bisa dibuka lagi.
                    </p>
                    <div className="flex gap-2">
                      <Tombol
                        jenis="bahaya"
                        ukuran="padat"
                        memuat={membatalkan}
                        onClick={async () => {
                          setMembatalkan(true)
                          setGalat(null)
                          try {
                            await onBatalkan(t)
                            setYakinBatal(null)
                          } catch (e) {
                            setGalat(e instanceof Error ? e.message : 'Gagal membatalkan tagihan.')
                          } finally {
                            setMembatalkan(false)
                          }
                        }}
                      >
                        Ya, Batalkan
                      </Tombol>
                      <Tombol jenis="kedua" ukuran="padat" onClick={() => setYakinBatal(null)} disabled={membatalkan}>
                        Tidak
                      </Tombol>
                    </div>
                  </li>
                )
              }
              return (
                <li key={t.id} className="flex items-center">
                  <button
                    type="button"
                    disabled={keranjangBerisi}
                    onClick={() => onBuka(t)}
                    className="flex min-h-14 min-w-0 flex-1 items-center gap-3 rounded-kontrol px-2 py-2 text-left hover:bg-permukaan-2 disabled:cursor-not-allowed disabled:opacity-60"
                  >
                    <span className="min-w-0 flex-1">
                      <span className="flex items-center gap-2">
                        <span className="truncate text-isi font-semibold text-teks-utama">{t.label}</span>
                        {t.tertunda && <LencanaStatus nada="menunggu" anak="Belum terkirim" />}
                      </span>
                      <span className="block truncate text-keterangan text-teks-redup">
                        {[
                          r ? `${r.jumlah} barang` : null,
                          t.updated_by_name ?? t.created_by_name,
                          formatJam(t.updated_at),
                        ]
                          .filter(Boolean)
                          .join(' · ')}
                      </span>
                    </span>
                    {r && (
                      <span className="shrink-0 text-label font-semibold tabular-nums text-teks-utama">
                        {formatRupiah(r.total)}
                      </span>
                    )}
                    <ChevronRight className="h-4 w-4 shrink-0 text-teks-redup" aria-hidden />
                  </button>
                  <button
                    type="button"
                    onClick={() => setYakinBatal(t.id)}
                    aria-label={`Batalkan tagihan ${t.label}`}
                    className="flex h-12 w-12 shrink-0 items-center justify-center rounded-kontrol text-bahaya-teks hover:bg-bahaya-teks/10"
                  >
                    <Trash2 className="h-5 w-5" aria-hidden />
                  </button>
                </li>
              )
            })}
          </ul>
        )}
        {galat && (
          <p role="alert" className="text-label text-bahaya-teks">
            {galat}
          </p>
        )}
      </IsiDialog>
    </Dialog>
  )
}
