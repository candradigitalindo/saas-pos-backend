import { useEffect, useRef, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { ulid } from 'ulid'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { Kolom } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { SegmenPilihan } from '@/bersama/ui/segmen'
import { Tombol } from '@/bersama/ui/tombol'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { FITUR } from '@/lib/fitur'
import { formatRupiah } from '@/bersama/util/uang'
import { cn } from '@/bersama/util/cn'
import { useShiftAktif } from '@/fitur/kasir/hooks'
import type { SumberBayar } from '@/fitur/stok/api'
import { PilihSumberBayar } from '@/fitur/stok/komponen/pilih-sumber-bayar'
import { pelangganApi, type CaraSetor } from '../api'

/**
 * Terima setoran kasbon dari SEORANG PELANGGAN — bukan per nota. Pelanggan
 * membayar "sebagian utang saya"; server membaginya ke kasbon terlama dulu.
 *
 * Tunai yang diterima di kasir masuk LACI (pilihan awal bila kasir buka),
 * supaya hitungan laci saat tutup shift cocok — dulu setoran tidak tercatat di
 * laci dan tutup shift selalu tampak "lebih".
 */
export function DialogSetoran({
  pelanggan,
  onTutup,
}: {
  /** null = tertutup. */
  pelanggan: { id: string; name: string; outstanding: number } | null
  onTutup: () => void
}) {
  const qc = useQueryClient()
  const toast = useToast()
  const { tokoAktif, punyaFitur } = useSesi()
  const { shift } = useShiftAktif()
  const [nominal, setNominal] = useState(0)
  const [cara, setCara] = useState<CaraSetor>('cash')
  const [sumber, setSumber] = useState<SumberBayar>('other')
  const [catatan, setCatatan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)
  // Satu kunci per setoran: menekan lagi setelah sinyal putus tidak
  // mencatatnya dua kali.
  const kunci = useRef(ulid())

  useEffect(() => {
    if (!pelanggan) return
    setNominal(pelanggan.outstanding)
    setCara('cash')
    setSumber(shift ? 'drawer' : 'other')
    setCatatan('')
    setGalat(null)
    kunci.current = ulid()
    // Pilihan awal laci hanya saat dialog dibuka — bukan setiap shift dimuat ulang.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [pelanggan])

  const keLaci = cara === 'cash' && sumber === 'drawer'
  const setor = useMutation({
    mutationFn: () =>
      pelangganApi.setor(
        pelanggan!.id,
        {
          amount: nominal,
          method: cara,
          source: cara === 'cash' ? sumber : undefined,
          outlet_id: tokoAktif ?? undefined,
          note: catatan.trim() || undefined,
        },
        kunci.current,
      ),
    onSuccess: (h) => {
      qc.invalidateQueries({ queryKey: ['kasbon'] })
      qc.invalidateQueries({ queryKey: ['pelanggan'] })
      if (h.to_drawer) {
        qc.invalidateQueries({ queryKey: ['shift-aktif'] })
        qc.invalidateQueries({ queryKey: ['gerakan-kas'] })
      }
      const lunas = h.allocations.filter((a) => a.settled).length
      toast.berhasil(
        h.outstanding > 0
          ? `Setoran ${formatRupiah(h.amount)} dari ${pelanggan?.name} tercatat. Sisa kasbon ${formatRupiah(h.outstanding)}.`
          : `Kasbon ${pelanggan?.name} lunas${lunas > 1 ? ` (${lunas} nota)` : ''}.`,
      )
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  const sisa = pelanggan?.outstanding ?? 0
  const lebih = nominal > sisa
  const sah = !!pelanggan && nominal > 0 && !lebih
  const pilihanCara: [CaraSetor, string][] = [
    ['cash', 'Tunai'],
    ...(punyaFitur(FITUR.qris) ? ([['qris', 'QRIS']] as [CaraSetor, string][]) : []),
    ['transfer', 'Transfer'],
  ]

  return (
    <Dialog open={!!pelanggan} onOpenChange={(o) => !o && !setor.isPending && onTutup()}>
      {pelanggan && (
        <IsiDialog judul={`Terima setoran ${pelanggan.name}`} keterangan={`Sisa kasbon ${formatRupiah(sisa)}`}>
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <KolomUang
                label="Jumlah setoran"
                nilai={nominal}
                onNilai={setNominal}
                bantuan="Boleh dicicil — dipakai melunasi kasbon terlama dulu."
                autoFocus
              />
              {nominal !== sisa && (
                <button
                  type="button"
                  onClick={() => setNominal(sisa)}
                  className="self-start text-label font-medium text-utama hover:underline"
                >
                  Lunasi semua ({formatRupiah(sisa)})
                </button>
              )}
            </div>

            <div className="flex flex-col gap-1.5">
              <span className="text-label font-medium text-teks-sekunder">Cara bayar</span>
              <SegmenPilihan label="Cara bayar" nilai={cara} onPilih={setCara} pilihan={pilihanCara} />
            </div>

            {cara === 'cash' && (
              <PilihSumberBayar
                nilai={sumber}
                onPilih={setSumber}
                arah="masuk"
                legenda="Uang tunai masuk ke"
                butuhIzinKas={false}
              />
            )}

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
                lebih ? 'bg-bahaya-teks/10 text-bahaya-teks' : nominal === sisa ? 'bg-sorot text-hijau-800' : 'bg-permukaan-2 text-teks-sekunder',
              )}
              aria-live="polite"
            >
              {lebih
                ? `Melebihi sisa kasbon ${formatRupiah(sisa)}.`
                : nominal === sisa
                  ? 'Setelah ini seluruh kasbon LUNAS.'
                  : nominal > 0
                    ? `Sisa kasbon setelah ini ${formatRupiah(sisa - nominal)}.`
                    : 'Isi jumlah yang disetor.'}
              {keLaci && !lebih && nominal > 0 && ' Uangnya dicatat masuk laci kasir.'}
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
              memuat={setor.isPending}
              labelMemuat="Mencatat…"
              onClick={() => {
                setGalat(null)
                setor.mutate()
              }}
            >
              Terima {formatRupiah(nominal)}
            </Tombol>
            <Tombol jenis="kedua" onClick={onTutup} disabled={setor.isPending}>
              Batal
            </Tombol>
          </AksiDialog>
        </IsiDialog>
      )}
    </Dialog>
  )
}
