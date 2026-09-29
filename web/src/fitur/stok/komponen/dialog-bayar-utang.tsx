import { useEffect, useRef, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ulid } from 'ulid'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { Kolom } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { Tombol } from '@/bersama/ui/tombol'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { cn } from '@/bersama/util/cn'
import { stokApi, type Pembelian, type SumberBayar } from '../api'
import { PilihSumberBayar } from './pilih-sumber-bayar'

/**
 * Bayar utang satu pembelian — sebagian atau sekaligus. Nominal dimulai dari
 * SISA (paling sering dilunasi sekaligus); akibatnya ditulis sebelum tombol
 * ditekan: "Lunas" atau "Sisa setelah ini Rp X".
 */
export function DialogBayarUtang({
  pembelian: p,
  onTutup,
}: {
  pembelian: Pembelian | null
  onTutup: () => void
}) {
  const qc = useQueryClient()
  const toast = useToast()
  const [nominal, setNominal] = useState(0)
  const [sumber, setSumber] = useState<SumberBayar>('other')
  const [catatan, setCatatan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)
  // Satu kunci per pembayaran: menekan "Bayar" lagi setelah sinyal putus
  // tidak mencatatnya dua kali.
  const kunci = useRef(ulid())

  useEffect(() => {
    if (!p) return
    setNominal(p.outstanding)
    setSumber('other')
    setCatatan('')
    setGalat(null)
    kunci.current = ulid()
  }, [p])

  const bayar = useMutation({
    mutationFn: () =>
      stokApi.bayarPembelian(p!.id, { amount: nominal, source: sumber, note: catatan.trim() || undefined }, kunci.current),
    onSuccess: (hasil) => {
      qc.invalidateQueries({ queryKey: ['utang'] })
      qc.invalidateQueries({ queryKey: ['pembelian'] })
      if (sumber === 'drawer') {
        qc.invalidateQueries({ queryKey: ['shift-aktif'] })
        qc.invalidateQueries({ queryKey: ['gerakan-kas'] })
      }
      toast.berhasil(
        hasil.outstanding > 0
          ? `Dibayar ${formatRupiah(nominal)}. Sisa utang ${formatRupiah(hasil.outstanding)}.`
          : `Utang ${p?.supplier_name ?? 'nota ini'} lunas.`,
      )
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  const sisaSetelah = (p?.outstanding ?? 0) - nominal
  const sah = !!p && nominal > 0 && nominal <= p.outstanding

  return (
    <Dialog open={!!p} onOpenChange={(o) => !o && onTutup()}>
      {p && (
        <IsiDialog
          judul={`Bayar ${p.supplier_name || 'utang'}`}
          keterangan={`${p.invoice_no ? `Nota ${p.invoice_no} · ` : ''}sisa ${formatRupiah(p.outstanding)} dari ${formatRupiah(p.total)}`}
        >
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <KolomUang label="Dibayar sekarang" nilai={nominal} onNilai={setNominal} autoFocus />
              {nominal !== p.outstanding && (
                <button
                  type="button"
                  onClick={() => setNominal(p.outstanding)}
                  className="self-start text-label font-medium text-utama hover:underline"
                >
                  Lunasi semua ({formatRupiah(p.outstanding)})
                </button>
              )}
            </div>

            <PilihSumberBayar nilai={sumber} onPilih={setSumber} />

            <Kolom
              label="Catatan"
              value={catatan}
              onChange={(e) => setCatatan(e.target.value)}
              maxLength={255}
              placeholder="Boleh dikosongkan, mis. transfer BCA"
            />

            <p
              className={cn(
                'rounded-kontrol px-3 py-2 text-label',
                nominal > p.outstanding ? 'bg-bahaya-teks/10 text-bahaya-teks' : 'bg-permukaan-2 text-teks-sekunder',
              )}
              aria-live="polite"
            >
              {nominal > p.outstanding
                ? `Melebihi sisa utang ${formatRupiah(p.outstanding)}.`
                : sisaSetelah === 0
                  ? 'Setelah ini utang nota ini LUNAS.'
                  : nominal > 0
                    ? `Sisa setelah ini ${formatRupiah(sisaSetelah)}.`
                    : 'Isi nominal yang dibayar.'}
            </p>

            {galat && (
              <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
                {galat}
              </p>
            )}
          </div>

          <AksiDialog>
            <Tombol
              disabled={!sah}
              memuat={bayar.isPending}
              labelMemuat="Mencatat…"
              onClick={() => {
                setGalat(null)
                bayar.mutate()
              }}
            >
              Bayar {formatRupiah(nominal)}
            </Tombol>
            <Tombol jenis="kedua" onClick={onTutup} disabled={bayar.isPending}>
              Batal
            </Tombol>
          </AksiDialog>
        </IsiDialog>
      )}
    </Dialog>
  )
}
