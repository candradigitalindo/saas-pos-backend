import { useEffect, useRef, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ulid } from 'ulid'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { Kolom } from '@/bersama/ui/kolom'
import { StepperJumlah } from '@/bersama/ui/stepper-jumlah'
import { Tombol } from '@/bersama/ui/tombol'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah, pratinjauBaris } from '@/bersama/util/uang'
import { formatQty } from '@/bersama/util/desimal'
import { cn } from '@/bersama/util/cn'
import { stokApi, type Pembelian, type SumberBayar } from '../api'
import { PilihSumberBayar } from './pilih-sumber-bayar'

const ALASAN = ['Rusak', 'Kedaluwarsa', 'Salah kirim', 'Kelebihan kirim']

/**
 * Retur barang ke pemasok atas satu nota (000049).
 *
 * Tiap baris dibatasi yang MASIH bisa diretur (dibeli − sudah diretur). Akibat
 * ditulis sebelum disimpan: stok berkurang, dan utang nota turun — atau, bila
 * nota sudah dibayar melebihi total barunya, pemasok MENGEMBALIKAN selisihnya
 * (dipilih: ke laci kasir atau uang lain).
 */
export function DialogReturPemasok({ pembelian: p, onTutup }: { pembelian: Pembelian | null; onTutup: () => void }) {
  const qc = useQueryClient()
  const toast = useToast()
  const [jumlah, setJumlah] = useState<Record<string, string>>({})
  const [alasan, setAlasan] = useState('')
  const [sumber, setSumber] = useState<SumberBayar>('other')
  const [galat, setGalat] = useState<string | null>(null)
  const kunci = useRef(ulid())

  useEffect(() => {
    if (!p) return
    setJumlah({})
    setAlasan('')
    setSumber('other')
    setGalat(null)
    kunci.current = ulid()
  }, [p])

  const baris = (p?.items ?? []).map((it) => {
    const maks = Number.parseFloat(it.qty) - Number.parseFloat(it.returned_qty ?? '0')
    const q = jumlah[it.id] ?? '0'
    return { it, maks, q, nilai: pratinjauBaris(it.unit_cost, q) }
  })
  const nilai = baris.reduce((j, b) => j + b.nilai, 0)
  const totalBaru = (p?.total ?? 0) - nilai
  const kembali = Math.max(0, (p?.paid_amount ?? 0) - totalBaru)
  const utangBaru = totalBaru - ((p?.paid_amount ?? 0) - kembali)

  const simpan = useMutation({
    mutationFn: () =>
      stokApi.returPembelian(
        p!.id,
        {
          items: baris.filter((b) => b.nilai > 0).map((b) => ({ purchase_item_id: b.it.id, qty: b.q })),
          reason: alasan.trim(),
          refund_source: kembali > 0 ? sumber : undefined,
        },
        kunci.current,
      ),
    onSuccess: (h) => {
      for (const k of ['pembelian', 'utang', 'stok', 'stok-ringkasan', 'kartu-stok', 'pemasok']) {
        qc.invalidateQueries({ queryKey: [k] })
      }
      if (h.refund_amount > 0 && sumber === 'drawer') {
        qc.invalidateQueries({ queryKey: ['shift-aktif'] })
        qc.invalidateQueries({ queryKey: ['gerakan-kas'] })
      }
      toast.berhasil(
        `Retur ${formatRupiah(h.total)} dicatat.` +
          (h.refund_amount > 0
            ? ` Pemasok mengembalikan ${formatRupiah(h.refund_amount)}.`
            : ` Utang nota jadi ${formatRupiah(h.purchase_outstanding)}.`),
      )
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  return (
    <Dialog open={!!p} onOpenChange={(o) => !o && onTutup()}>
      {p && (
        <IsiDialog
          judul="Retur ke pemasok"
          keterangan={`${p.supplier_name || 'Tanpa pemasok'}${p.invoice_no ? ` · nota ${p.invoice_no}` : ''}`}
          className="sm:max-w-lg"
        >
          <div className="flex flex-col gap-4">
            <ul className="flex flex-col gap-3">
              {baris.map(({ it, maks, q }) => {
                const satuan = it.unit_name || it.base_unit_name
                return (
                  <li key={it.id} className="flex flex-col gap-2 border-b border-garis pb-3 last:border-0">
                    <div>
                      <p className="font-medium text-teks-utama">
                        {it.product_name}
                        {it.variant_name && ` (${it.variant_name})`}
                      </p>
                      <p className="text-keterangan text-teks-redup">
                        dibeli {formatQty(it.qty)} {satuan} @ {formatRupiah(it.unit_cost)}
                        {Number.parseFloat(it.returned_qty ?? '0') > 0 && ` · sudah diretur ${formatQty(it.returned_qty!)}`}
                      </p>
                    </div>
                    {maks > 0 ? (
                      <StepperJumlah
                        nilai={q}
                        onNilai={(v) =>
                          setJumlah((m) => ({ ...m, [it.id]: String(Math.min(Number.parseFloat(v) || 0, maks)) }))
                        }
                        satuan={satuan}
                        label={`Jumlah retur ${it.product_name}`}
                      />
                    ) : (
                      <p className="text-keterangan text-teks-redup">Sudah diretur semua.</p>
                    )}
                  </li>
                )
              })}
            </ul>

            <div className="flex flex-col gap-2">
              <Kolom
                label="Alasan"
                value={alasan}
                onChange={(e) => setAlasan(e.target.value)}
                maxLength={255}
                placeholder="mis. kemasan bocor"
                required
              />
              <div className="flex flex-wrap gap-1.5" role="group" aria-label="Alasan cepat">
                {ALASAN.map((a) => (
                  <button
                    key={a}
                    type="button"
                    onClick={() => setAlasan(a)}
                    aria-pressed={alasan === a}
                    className={cn(
                      'min-h-9 rounded-full border px-3 text-keterangan font-medium',
                      alasan === a
                        ? 'border-utama bg-sorot text-hijau-800'
                        : 'border-garis bg-permukaan text-teks-sekunder hover:bg-permukaan-2',
                    )}
                  >
                    {a}
                  </button>
                ))}
              </div>
            </div>

            {nilai > 0 && (
              <div className="flex flex-col gap-2 rounded-kontrol bg-permukaan-2/60 p-3 text-label" aria-live="polite">
                <p className="text-teks-utama">
                  Nilai retur <strong className="tabular-nums">{formatRupiah(nilai)}</strong> · stok berkurang sebanyak yang
                  diretur.
                </p>
                {kembali > 0 ? (
                  <p className="text-teks-sekunder">
                    Nota ini sudah dibayar {formatRupiah(p.paid_amount)} — pemasok mengembalikan{' '}
                    <strong className="tabular-nums text-teks-utama">{formatRupiah(kembali)}</strong>.
                  </p>
                ) : (
                  <p className="text-teks-sekunder">
                    Utang nota turun dari {formatRupiah(p.outstanding)} jadi{' '}
                    <strong className="tabular-nums text-teks-utama">{formatRupiah(utangBaru)}</strong>.
                  </p>
                )}
              </div>
            )}
            {kembali > 0 && <PilihSumberBayar nilai={sumber} onPilih={setSumber} arah="masuk" />}

            {galat && (
              <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
                {galat}
              </p>
            )}
          </div>

          <AksiDialog>
            <Tombol
              disabled={nilai <= 0 || !alasan.trim()}
              memuat={simpan.isPending}
              labelMemuat="Mencatat…"
              onClick={() => {
                setGalat(null)
                simpan.mutate()
              }}
            >
              Catat Retur {nilai > 0 ? formatRupiah(nilai) : ''}
            </Tombol>
            <Tombol jenis="kedua" onClick={onTutup} disabled={simpan.isPending}>
              Batal
            </Tombol>
          </AksiDialog>
        </IsiDialog>
      )}
    </Dialog>
  )
}
