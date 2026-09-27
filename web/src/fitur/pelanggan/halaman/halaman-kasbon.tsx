import { useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { ulid } from 'ulid'
import { NotebookPen } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { Pilihan } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import { formatRupiah } from '@/bersama/util/uang'
import { formatTanggal } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { pelangganApi, type Kasbon } from '../api'
import { SegmenPilihan } from '@/bersama/ui/segmen'
import { KartuAngka } from '@/bersama/komponen/kartu-angka'
import { HandCoins } from 'lucide-react'
import { FITUR } from '@/lib/fitur'
import { useSesi } from '@/bersama/hooks/use-sesi'

/**
 * Kasbon — utang pelanggan.
 *
 * Yang dicari pemilik cuma dua: siapa yang masih berutang, dan berapa. Karena
 * itu total tunggakan ada di paling atas, dan setiap baris langsung punya
 * tombol untuk menerima setoran.
 */
export function HalamanKasbon() {
  const [hanyaBelumLunas, setHanyaBelumLunas] = useState(true)
  const [bayarUntuk, setBayarUntuk] = useState<Kasbon | null>(null)

  const kasbon = useQuery({
    queryKey: ['kasbon', hanyaBelumLunas],
    queryFn: () => pelangganApi.daftarKasbon(undefined, hanyaBelumLunas ? 'open' : undefined),
    staleTime: 30_000,
  })

  const pelanggan = useQuery({
    queryKey: ['pelanggan', ''],
    queryFn: () => pelangganApi.daftar(),
    staleTime: 60_000,
  })

  const namaPelanggan = useMemo(() => {
    const m = new Map<string, string>()
    for (const p of pelanggan.data?.data ?? []) m.set(p.id, p.name)
    return m
  }, [pelanggan.data])

  const daftar = kasbon.data?.data ?? []
  const totalTunggakan = daftar.reduce((j, k) => j + k.outstanding, 0)

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-judul font-bold text-teks-utama">Kasbon</h1>

      {daftar.length > 0 && (
        <KartuAngka
          ikon={HandCoins}
          label="Total belum dibayar"
          nilai={totalTunggakan}
          keterangan={`dari ${daftar.length} kasbon`}
        />
      )}

      <SegmenPilihan
        label="Saring kasbon"
        nilai={hanyaBelumLunas ? 'belum' : 'semua'}
        onPilih={(v) => setHanyaBelumLunas(v === 'belum')}
        pilihan={[
          ['belum', 'Belum lunas'],
          ['semua', 'Semua'],
        ]}
      />

      {kasbon.isLoading ? (
        <KerangkaBaris jumlah={4} />
      ) : kasbon.isError ? (
        <KeadaanGagal
          pesan={kasbon.error instanceof GalatAPI ? kasbon.error.pesan : 'Kasbon belum bisa dimuat.'}
          onCobaLagi={() => kasbon.refetch()}
        />
      ) : daftar.length === 0 ? (
        <KeadaanKosong
          ikon={NotebookPen}
          judul={hanyaBelumLunas ? 'Tidak ada kasbon yang belum lunas' : 'Belum ada kasbon'}
          penjelasan={
            hanyaBelumLunas
              ? 'Semua pelanggan sudah melunasi utangnya. Bagus.'
              : 'Kasbon muncul di sini saat Anda menjual dengan cara bayar "Kasbon" di kasir.'
          }
          aksi={
            hanyaBelumLunas
              ? { label: 'Lihat semua kasbon', onKlik: () => setHanyaBelumLunas(false) }
              : undefined
          }
        />
      ) : (
        <ul className="flex flex-col gap-2">
          {daftar.map((k) => (
            <li key={k.id}>
              <Kartu className="flex items-center justify-between gap-3 p-4">
                <div className="min-w-0">
                  <p className="truncate font-semibold text-teks-utama">
                    {namaPelanggan.get(k.customer_id) ?? 'Pelanggan'}
                  </p>
                  <p className="text-keterangan text-teks-redup">
                    {formatTanggal(k.created_at)}
                    {k.paid_amount > 0 &&
                      ` · sudah bayar ${formatRupiah(k.paid_amount)} dari ${formatRupiah(k.amount)}`}
                  </p>
                </div>

                <div className="flex shrink-0 items-center gap-3">
                  <div className="text-right">
                    <p
                      className={cn(
                        'font-bold tabular-nums',
                        k.outstanding > 0 ? 'text-jingga-700' : 'text-hijau-700',
                      )}
                    >
                      {formatRupiah(k.outstanding)}
                    </p>
                    {k.outstanding === 0 ? (
                      <LencanaStatus nada="berhasil" anak="Lunas" />
                    ) : (
                      <LencanaStatus nada="menunggu" anak="Belum lunas" />
                    )}
                  </div>
                  {k.outstanding > 0 && (
                    <Tombol jenis="kedua" ukuran="padat" onClick={() => setBayarUntuk(k)}>
                      Terima Setoran
                    </Tombol>
                  )}
                </div>
              </Kartu>
            </li>
          ))}
        </ul>
      )}

      {bayarUntuk && (
        <DialogSetoran
          kasbon={bayarUntuk}
          nama={namaPelanggan.get(bayarUntuk.customer_id) ?? 'Pelanggan'}
          onTutup={() => setBayarUntuk(null)}
        />
      )}
    </div>
  )
}

function DialogSetoran({
  kasbon,
  nama,
  onTutup,
}: {
  kasbon: Kasbon
  nama: string
  onTutup: () => void
}) {
  const toast = useToast()
  const qc = useQueryClient()
  const [nominal, setNominal] = useState(kasbon.outstanding)
  const { punyaFitur, paketUntuk } = useSesi()
  const [cara, setCara] = useState<'cash' | 'qris' | 'transfer'>('cash')
  const [galat, setGalat] = useState<string | null>(null)

  // Satu kunci per dialog: tombol yang ditekan lagi setelah sinyal putus
  // tidak mencatat setoran yang sama dua kali.
  const kunci = useRef(ulid())
  const bayar = useMutation({
    mutationFn: () =>
      pelangganApi.terimaSetoran(
        {
          receivable_id: kasbon.id,
          amount: nominal,
          method: cara,
        },
        kunci.current,
      ),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['kasbon'] })
      qc.invalidateQueries({ queryKey: ['shift-aktif'] })
      toast.berhasil(`Setoran ${formatRupiah(nominal)} dari ${nama} tercatat.`)
      onTutup()
    },
    // Kalimat server dipakai apa adanya: 409 di sini berarti kasbonnya sudah
    // lunas/dihapusbukukan, sedangkan kelebihan bayar dibalas 400 dengan
    // pesannya sendiri — menebak dari kode status dulu menyebut "melebihi
    // sisa" untuk kasbon yang sudah lunas.
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  const lebih = nominal > kasbon.outstanding

  return (
    <Dialog open onOpenChange={(o) => !o && !bayar.isPending && onTutup()}>
      <IsiDialog judul={`Terima setoran dari ${nama}`}>
        <div className="flex items-baseline justify-between border-b border-garis pb-3">
          <span className="text-isi text-teks-sekunder">Sisa utang</span>
          <span className="text-judul-kartu font-bold tabular-nums text-teks-utama">
            {formatRupiah(kasbon.outstanding)}
          </span>
        </div>

        <KolomUang
          label="Jumlah setoran"
          nilai={nominal}
          onNilai={setNominal}
          bantuan="Boleh dicicil — tidak harus lunas sekaligus."
          galat={lebih ? 'Tidak boleh lebih besar dari sisa utang.' : undefined}
          autoFocus
        />

        <Pilihan
          label="Cara bayar"
          value={cara}
          onChange={(e) => setCara(e.target.value as typeof cara)}
        >
          <option value="cash">Tunai</option>
          {/* Terkunci paket: tetap tertulis (dengan alasannya) tapi tidak
              bisa dipilih — server menolak setoran QRIS di paket tanpa QRIS. */}
          <option value="qris" disabled={!punyaFitur(FITUR.qris)}>
            {punyaFitur(FITUR.qris) ? 'QRIS' : `QRIS — paket ${paketUntuk(FITUR.qris) ?? 'berbayar'}`}
          </option>
          <option value="transfer">Transfer</option>
        </Pilihan>

        {nominal > 0 && !lebih && (
          <p className="rounded-kontrol bg-sorot px-3 py-2 text-label text-hijau-800">
            Sisa utang setelah setoran ini:{' '}
            <strong className="tabular-nums">
              {formatRupiah(kasbon.outstanding - nominal)}
            </strong>
          </p>
        )}

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <AksiDialog>
          <Tombol
            memuat={bayar.isPending}
            disabled={nominal <= 0 || lebih}
            onClick={() => {
              setGalat(null)
              bayar.mutate()
            }}
          >
            Catat Setoran
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={bayar.isPending}>
            Batal
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}

