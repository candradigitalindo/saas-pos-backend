import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Check, Lock, Plus, TriangleAlert, Wallet } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { formatTanggal } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { sdmApi, type PeriodeGaji, type SlipGaji } from '../api'
import { KartuAngka } from '@/bersama/komponen/kartu-angka'
import { Users } from 'lucide-react'

/** Empat langkah tetap, sesuai urutan yang dipaksakan backend. */
const LANGKAH = [
  { kunci: 'draft', nomor: 1, label: 'Hitung' },
  { kunci: 'calculated', nomor: 2, label: 'Periksa' },
  { kunci: 'locked', nomor: 3, label: 'Kunci' },
  { kunci: 'paid', nomor: 4, label: 'Bayar' },
] as const

/**
 * Gaji — modul paling rumit, harus terasa paling sederhana
 * (ui/05-ALUR-UTAMA.md §8).
 *
 * Mesin gaji backend punya urutan tetap dan penguncian. Kerumitan itu
 * disembunyikan di balik empat langkah yang jelas, dan setiap dialog
 * menjelaskan AKIBATNYA — bukan sekadar bertanya.
 */
export function HalamanGaji() {
  const [periodeDipilih, setPeriodeDipilih] = useState<string | null>(null)
  const [buatBaru, setBuatBaru] = useState(false)

  const periode = useQuery({
    queryKey: ['periode-gaji'],
    queryFn: () => sdmApi.daftarPeriode(),
  })

  const daftar = periode.data?.data ?? []
  const aktif = daftar.find((p) => p.id === periodeDipilih) ?? daftar[0]

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-judul font-bold text-teks-utama">Gaji</h1>
        <Tombol onClick={() => setBuatBaru(true)}>
          <Plus className="h-5 w-5" aria-hidden />
          Periode Baru
        </Tombol>
      </header>

      {periode.isLoading ? (
        <KerangkaBaris jumlah={3} />
      ) : daftar.length === 0 ? (
        <KeadaanKosong
          ikon={Wallet}
          judul="Belum ada periode gaji"
          penjelasan="Buat periode dulu — misalnya 1 sampai 30 September — lalu hitung gajinya."
          aksi={{ label: 'Buat Periode', onKlik: () => setBuatBaru(true) }}
        />
      ) : (
        <>
          {daftar.length > 1 && (
            <Pilihan
              label="Periode"
              value={aktif?.id ?? ''}
              onChange={(e) => setPeriodeDipilih(e.target.value)}
            >
              {daftar.map((p) => (
                <option key={p.id} value={p.id}>
                  {formatTanggal(p.start_date)} – {formatTanggal(p.end_date)}
                </option>
              ))}
            </Pilihan>
          )}

          {aktif && <PanelPeriode periode={aktif} />}
        </>
      )}

      {buatBaru && <DialogPeriodeBaru onTutup={() => setBuatBaru(false)} />}
    </div>
  )
}

function PanelPeriode({ periode }: { periode: PeriodeGaji }) {
  const { boleh } = useSesi()
  const toast = useToast()
  const qc = useQueryClient()
  const [konfirmasi, setKonfirmasi] = useState<'kunci' | 'bayar' | null>(null)
  const [galat, setGalat] = useState<string | null>(null)

  const slip = useQuery({
    queryKey: ['slip-gaji', periode.id],
    queryFn: () => sdmApi.slipPeriode(periode.id),
    enabled: periode.status !== 'draft' && boleh(IZIN.hrSalaryView),
  })

  const jalankan = useMutation({
    mutationFn: (aksi: 'hitung' | 'kunci' | 'bayar') =>
      aksi === 'hitung'
        ? sdmApi.hitung(periode.id)
        : aksi === 'kunci'
          ? sdmApi.kunci(periode.id)
          : sdmApi.bayar(periode.id),
    onSuccess: (_d, aksi) => {
      qc.invalidateQueries({ queryKey: ['periode-gaji'] })
      qc.invalidateQueries({ queryKey: ['slip-gaji'] })
      setKonfirmasi(null)
      toast.berhasil(
        aksi === 'hitung'
          ? 'Gaji sudah dihitung. Periksa dulu sebelum dikunci.'
          : aksi === 'kunci'
            ? 'Periode dikunci. Angkanya tidak bisa diubah lagi.'
            : 'Gaji ditandai sudah dibayar.',
      )
    },
    onError: (e) =>
      setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan. Coba lagi.'),
  })

  const daftarSlip = slip.data?.data ?? []

  // Slip yang perlu dilihat manusia: potongan melebihi gaji, sehingga ada
  // sisa utang yang terbawa ke periode berikutnya.
  const perluDiperiksa = useMemo(
    () => daftarSlip.filter((s) => s.carried_debt > 0 || s.net_amount <= 0),
    [daftarSlip],
  )

  const langkahKini = LANGKAH.findIndex((l) => l.kunci === periode.status)

  return (
    <div className="flex flex-col gap-4">
      <Kartu className="p-4">
        <p className="mb-3 text-label text-teks-sekunder">
          {formatTanggal(periode.start_date)} – {formatTanggal(periode.end_date)}
        </p>
        <ol className="flex items-center justify-between gap-1">
          {LANGKAH.map((l, i) => (
            <li key={l.kunci} className="flex flex-1 flex-col items-center gap-1">
              <span
                className={cn(
                  'flex h-8 w-8 items-center justify-center rounded-full text-label font-bold',
                  i < langkahKini
                    ? 'bg-hijau-100 text-hijau-800'
                    : i === langkahKini
                      ? 'bg-utama text-utama-teks'
                      : 'bg-permukaan-2 text-teks-redup',
                )}
              >
                {i < langkahKini ? <Check className="h-4 w-4" aria-hidden /> : l.nomor}
              </span>
              <span
                className={cn(
                  'text-keterangan font-medium',
                  i <= langkahKini ? 'text-teks-utama' : 'text-teks-redup',
                )}
              >
                {l.label}
              </span>
            </li>
          ))}
        </ol>
      </Kartu>

      {periode.status !== 'draft' && (
        <KartuAngka
          ikon={Users}
          label={`Gaji bersih · ${daftarSlip.length || '—'} karyawan`}
          nilai={periode.total_net}
          keterangan={`Gaji kotor ${formatRupiah(periode.total_gross)} − potongan ${formatRupiah(periode.total_deduction)}`}
        />
      )}

      {/* hr.salary.view adalah izin paling sensitif — tanpa itu, nominalnya
          tidak muncul sama sekali. */}
      {boleh(IZIN.hrSalaryView) && daftarSlip.length > 0 && (
        <>
          {perluDiperiksa.length > 0 && (
            <Kartu className="flex items-start gap-3 border-jingga-600 bg-jingga-100 p-4">
              <TriangleAlert className="mt-0.5 h-5 w-5 shrink-0 text-jingga-700" aria-hidden />
              <div>
                <p className="font-medium text-jingga-700">
                  {perluDiperiksa.length} slip perlu diperiksa
                </p>
                <p className="text-keterangan text-jingga-700">
                  Potongannya lebih besar dari gaji, jadi sisanya dibawa ke periode
                  berikutnya.
                </p>
              </div>
            </Kartu>
          )}

          <Kartu className="divide-y divide-garis">
            {daftarSlip.map((s) => (
              <BarisSlipGaji key={s.id} slip={s} />
            ))}
          </Kartu>
        </>
      )}

      {galat && (
        <p className="rounded-kontrol border border-bahaya bg-red-50 px-3 py-2 text-label text-bahaya-teks">
          {galat}
        </p>
      )}

      {/* Satu tombol utama per tahap — pengguna tidak perlu memilih. */}
      {periode.status === 'draft' && boleh(IZIN.hrPayrollRun) && (
        <Tombol
          lebarPenuh
          memuat={jalankan.isPending}
          labelMemuat="Menghitung…"
          onClick={() => jalankan.mutate('hitung')}
        >
          Hitung Gaji
        </Tombol>
      )}
      {periode.status === 'calculated' && boleh(IZIN.hrPayrollLock) && (
        <Tombol lebarPenuh onClick={() => setKonfirmasi('kunci')}>
          <Lock className="h-5 w-5" aria-hidden />
          Kunci Periode Ini
        </Tombol>
      )}
      {periode.status === 'locked' && boleh(IZIN.hrPayrollPay) && (
        <Tombol lebarPenuh onClick={() => setKonfirmasi('bayar')}>
          Tandai Sudah Dibayar
        </Tombol>
      )}
      {periode.status === 'paid' && (
        <p className="flex items-center gap-2 rounded-kontrol bg-sorot px-3 py-2 text-label text-hijau-800">
          <Check className="h-5 w-5" aria-hidden />
          Gaji periode ini sudah dibayar
          {periode.paid_at && ` pada ${formatTanggal(periode.paid_at)}`}.
        </p>
      )}

      <Dialog open={konfirmasi === 'kunci'} onOpenChange={(o) => !o && setKonfirmasi(null)}>
        <IsiDialog judul={`Kunci periode ${formatTanggal(periode.start_date)} – ${formatTanggal(periode.end_date)}?`}>
          <div className="flex flex-col gap-2 text-isi text-teks-sekunder">
            <p>Setelah dikunci, angka gaji tidak bisa diubah lagi.</p>
            {/* Kalimat ini menjelaskan perilaku backend yang sebenarnya rumit
                (penyesuaian periode berikutnya) dalam satu kalimat. */}
            <p>
              Koreksi absensi yang datang setelah ini akan otomatis masuk sebagai
              penyesuaian di gaji periode berikutnya.
            </p>
            <p className="font-medium text-teks-utama">
              Total yang dikunci: {formatRupiah(periode.total_net)}
            </p>
          </div>
          <AksiDialog>
            <Tombol
              memuat={jalankan.isPending}
              labelMemuat="Mengunci…"
              onClick={() => jalankan.mutate('kunci')}
            >
              Ya, Kunci Periode
            </Tombol>
            <Tombol jenis="kedua" onClick={() => setKonfirmasi(null)}>
              Periksa lagi
            </Tombol>
          </AksiDialog>
        </IsiDialog>
      </Dialog>

      <Dialog open={konfirmasi === 'bayar'} onOpenChange={(o) => !o && setKonfirmasi(null)}>
        <IsiDialog judul="Tandai gaji sudah dibayar?">
          <div className="flex flex-col gap-2 text-isi text-teks-sekunder">
            <p>
              Ini mencatat bahwa {formatRupiah(periode.total_net)} sudah Anda
              serahkan ke karyawan.
            </p>
            <p>
              Aplikasi tidak mengirim uangnya — pembayaran tetap Anda lakukan
              sendiri lewat transfer atau tunai.
            </p>
          </div>
          <AksiDialog>
            <Tombol
              memuat={jalankan.isPending}
              labelMemuat="Menyimpan…"
              onClick={() => jalankan.mutate('bayar')}
            >
              Ya, Sudah Dibayar
            </Tombol>
            <Tombol jenis="kedua" onClick={() => setKonfirmasi(null)}>
              Belum
            </Tombol>
          </AksiDialog>
        </IsiDialog>
      </Dialog>
    </div>
  )
}

/**
 * Satu slip. Rinciannya ditampilkan per baris dengan nama komponen yang
 * di-snapshot backend — supaya karyawan bisa memeriksa sendiri dan tidak perlu
 * bertanya.
 */
function BarisSlipGaji({ slip }: { slip: SlipGaji }) {
  const [buka, setBuka] = useState(false)

  const karyawan = useQuery({
    queryKey: ['karyawan', slip.employee_id],
    queryFn: () => sdmApi.karyawan(slip.employee_id),
    staleTime: 5 * 60_000,
  })

  const rinci = useQuery({
    queryKey: ['slip', slip.id],
    queryFn: () => sdmApi.slip(slip.id),
    enabled: buka,
  })

  return (
    <div>
      <button
        type="button"
        onClick={() => setBuka((v) => !v)}
        aria-expanded={buka}
        className="flex min-h-14 w-full items-center justify-between gap-3 px-4 text-left hover:bg-permukaan-2"
      >
        <span className="min-w-0">
          <span className="block truncate font-medium text-teks-utama">
            {karyawan.data?.full_name ?? 'Karyawan'}
          </span>
          <span className="block text-keterangan text-teks-redup">
            hadir {slip.present_days} hari
            {slip.late_count > 0 && ` · telat ${slip.late_count}×`}
            {slip.carried_debt > 0 && ' · ada sisa utang'}
          </span>
        </span>
        <span className="shrink-0 font-bold tabular-nums text-teks-utama">
          {formatRupiah(slip.net_amount)}
        </span>
      </button>

      {buka && (
        <div className="border-t border-garis bg-permukaan-2 px-4 py-3">
          {rinci.isLoading ? (
            <KerangkaBaris jumlah={2} />
          ) : (
            <dl className="flex flex-col gap-1 text-label">
              {rinci.data?.lines?.map((l, i) => (
                <div key={`${l.name}-${i}`} className="flex items-baseline justify-between gap-3">
                  <dt className="min-w-0 text-teks-sekunder">
                    {l.name}
                    {l.basis_note && (
                      <span className="block text-keterangan text-teks-redup">
                        {l.basis_note}
                      </span>
                    )}
                  </dt>
                  <dd
                    className={cn(
                      'shrink-0 tabular-nums',
                      l.category === 'deduction' ? 'text-bahaya-teks' : 'text-teks-utama',
                    )}
                  >
                    {l.category === 'deduction' ? '−' : ''}
                    {formatRupiah(Math.abs(l.amount))}
                  </dd>
                </div>
              ))}
              <div className="mt-1 flex items-baseline justify-between border-t border-garis pt-2">
                <dt className="font-semibold text-teks-utama">Diterima</dt>
                <dd className="font-bold tabular-nums text-teks-utama">
                  {formatRupiah(slip.net_amount)}
                </dd>
              </div>
              {slip.carried_debt > 0 && (
                <p className="mt-1 rounded-kontrol bg-jingga-100 px-2 py-1.5 text-keterangan text-jingga-700">
                  Sisa utang {formatRupiah(slip.carried_debt)} dibawa ke periode
                  berikutnya karena gaji periode ini tidak cukup menutupinya.
                </p>
              )}
            </dl>
          )}
        </div>
      )}
    </div>
  )
}

function DialogPeriodeBaru({ onTutup }: { onTutup: () => void }) {
  const { tokoAktif } = useSesi()
  const toast = useToast()
  const qc = useQueryClient()

  const kini = new Date()
  const awalBulan = `${kini.getFullYear()}-${String(kini.getMonth() + 1).padStart(2, '0')}-01`
  const akhirBulan = new Date(kini.getFullYear(), kini.getMonth() + 1, 0)
    .toISOString()
    .slice(0, 10)

  const [jenis, setJenis] = useState('monthly')
  const [mulai, setMulai] = useState(awalBulan)
  const [selesai, setSelesai] = useState(akhirBulan)
  const [galat, setGalat] = useState<string | null>(null)

  const buat = useMutation({
    mutationFn: () =>
      sdmApi.buatPeriode({
        outlet_id: tokoAktif,
        period_type: jenis,
        start_date: mulai,
        end_date: selesai,
      }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['periode-gaji'] })
      toast.berhasil('Periode dibuat. Sekarang gajinya bisa dihitung.')
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !buat.isPending && onTutup()}>
      <IsiDialog judul="Periode Gaji Baru">
        <Pilihan label="Jenis periode" value={jenis} onChange={(e) => setJenis(e.target.value)}>
          <option value="monthly">Bulanan</option>
          <option value="biweekly">Dua mingguan</option>
          <option value="weekly">Mingguan</option>
          <option value="daily">Harian</option>
        </Pilihan>
        <Kolom
          label="Mulai tanggal"
          type="date"
          value={mulai}
          onChange={(e) => setMulai(e.target.value)}
          required
        />
        <Kolom
          label="Sampai tanggal"
          type="date"
          value={selesai}
          onChange={(e) => setSelesai(e.target.value)}
          required
        />

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-red-50 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <AksiDialog>
          <Tombol
            memuat={buat.isPending}
            onClick={() => {
              setGalat(null)
              buat.mutate()
            }}
          >
            Buat Periode
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={buat.isPending}>
            Batal
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}
