import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Users } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { galatKolom } from '@/lib/galat-kolom'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import { tanggalISO } from '@/bersama/util/tanggal'
import { sdmApi, type Karyawan } from '../api'

const JENIS_UPAH = [
  { nilai: 'monthly', label: 'Bulanan' },
  { nilai: 'daily', label: 'Harian' },
  { nilai: 'hourly', label: 'Per jam' },
]

/** Daftar karyawan. Nominal upah hanya terlihat oleh pemegang hr.salary.view. */
export function HalamanKaryawan() {
  const { boleh } = useSesi()
  const [formUntuk, setFormUntuk] = useState<Karyawan | 'baru' | null>(null)

  const q = useQuery({
    queryKey: ['karyawan'],
    queryFn: () => sdmApi.daftarKaryawan(),
  })

  const daftar = q.data?.data ?? []
  const bolehLihatUpah = boleh(IZIN.hrSalaryView)

  return (
    <div className="flex w-full max-w-2xl flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-judul font-bold text-teks-utama">Karyawan</h1>
        {boleh(IZIN.hrEmployeeEdit) && (
          <Tombol onClick={() => setFormUntuk('baru')}>
            <Plus className="h-5 w-5" aria-hidden />
            Tambah Karyawan
          </Tombol>
        )}
      </header>

      {q.isLoading ? (
        <KerangkaBaris jumlah={4} />
      ) : daftar.length === 0 ? (
        <KeadaanKosong
          ikon={Users}
          judul="Belum ada karyawan"
          penjelasan="Daftarkan karyawan dulu supaya absensi dan gajinya bisa dihitung."
          aksi={
            boleh(IZIN.hrEmployeeEdit)
              ? { label: 'Tambah Karyawan', onKlik: () => setFormUntuk('baru') }
              : undefined
          }
        />
      ) : (
        <ul className="flex flex-col gap-2">
          {daftar.map((k) => (
            <li key={k.id}>
              <Kartu className="flex items-center justify-between gap-3 p-4">
                <div className="min-w-0">
                  <p className="truncate font-semibold text-teks-utama">{k.full_name}</p>
                  <p className="text-keterangan text-teks-redup">
                    {k.position || 'Tanpa jabatan'}
                    {/* Upah adalah data paling sensitif — disembunyikan, bukan
                        diburamkan, bila tidak berhak. */}
                    {bolehLihatUpah &&
                      ` · ${formatRupiah(k.base_wage)} ${
                        JENIS_UPAH.find((j) => j.nilai === k.wage_type)?.label.toLowerCase() ?? ''
                      }`}
                  </p>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  {k.is_active ? (
                    <LencanaStatus nada="berhasil" anak="Aktif" />
                  ) : (
                    <LencanaStatus nada="netral" anak="Keluar" />
                  )}
                  {boleh(IZIN.hrEmployeeEdit) && (
                    <Tombol jenis="kedua" ukuran="padat" onClick={() => setFormUntuk(k)}>
                      Ubah
                    </Tombol>
                  )}
                </div>
              </Kartu>
            </li>
          ))}
        </ul>
      )}

      {formUntuk && (
        <DialogKaryawan
          awal={formUntuk === 'baru' ? null : formUntuk}
          onTutup={() => setFormUntuk(null)}
        />
      )}
    </div>
  )
}

function DialogKaryawan({ awal, onTutup }: { awal: Karyawan | null; onTutup: () => void }) {
  const { tokoAktif } = useSesi()
  const toast = useToast()
  const qc = useQueryClient()

  const [nama, setNama] = useState(awal?.full_name ?? '')
  const [jabatan, setJabatan] = useState(awal?.position ?? '')
  const [hp, setHp] = useState(awal?.phone ?? '')
  const [jenisUpah, setJenisUpah] = useState(awal?.wage_type ?? 'monthly')
  const [upah, setUpah] = useState(awal?.base_wage ?? 0)
  const [masuk, setMasuk] = useState(awal?.joined_at?.slice(0, 10) ?? tanggalISO())
  const [kolomGalat, setKolomGalat] = useState<Record<string, string>>({})
  const [galat, setGalat] = useState<string | null>(null)

  const simpan = useMutation({
    mutationFn: () => {
      const isi = {
        full_name: nama.trim(),
        position: jabatan.trim() || undefined,
        phone: hp.trim() || undefined,
        wage_type: jenisUpah,
        base_wage: upah,
      }
      return awal
        ? sdmApi.ubahKaryawan(awal.id, isi)
        : sdmApi.buatKaryawan({ ...isi, outlet_id: tokoAktif!, joined_at: masuk })
    },
    onSuccess: (k) => {
      qc.invalidateQueries({ queryKey: ['karyawan'] })
      toast.berhasil(awal ? `${k.full_name} diperbarui.` : `${k.full_name} didaftarkan.`)
      onTutup()
    },
    onError: (e) => {
      if (e instanceof GalatAPI) {
        setKolomGalat(e.kolom)
        setGalat(e.status === 422 ? null : e.pesan)
      } else setGalat('Terjadi kesalahan. Coba lagi.')
    },
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !simpan.isPending && onTutup()}>
      <IsiDialog judul={awal ? `Ubah ${awal.full_name}` : 'Tambah Karyawan'}>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            setGalat(null)
            setKolomGalat({})
            simpan.mutate()
          }}
          className="flex flex-col gap-4"
          noValidate
        >
          <Kolom
            label="Nama lengkap"
            value={nama}
            onChange={(e) => setNama(e.target.value)}
            galat={galatKolom(kolomGalat, 'full_name')}
            autoFocus
            required
          />
          <Kolom
            label="Jabatan"
            placeholder="Contoh: Kasir"
            value={jabatan}
            onChange={(e) => setJabatan(e.target.value)}
            bantuan="Boleh dikosongkan."
            galat={galatKolom(kolomGalat, 'position')}
          />
          <Kolom
            label="Nomor HP"
            type="tel"
            inputMode="tel"
            value={hp}
            onChange={(e) => setHp(e.target.value)}
            bantuan="Boleh dikosongkan."
            galat={galatKolom(kolomGalat, 'phone')}
          />
          <Pilihan
            label="Jenis upah"
            value={jenisUpah}
            onChange={(e) => setJenisUpah(e.target.value as Karyawan['wage_type'])}
            galat={galatKolom(kolomGalat, 'wage_type')}
          >
            {JENIS_UPAH.map((j) => (
              <option key={j.nilai} value={j.nilai}>
                {j.label}
              </option>
            ))}
          </Pilihan>
          <KolomUang
            label="Upah pokok"
            nilai={upah}
            onNilai={setUpah}
            bantuan={
              jenisUpah === 'monthly'
                ? 'Gaji pokok per bulan.'
                : jenisUpah === 'daily'
                  ? 'Upah per hari kerja.'
                  : 'Upah per jam kerja.'
            }
            galat={galatKolom(kolomGalat, 'base_wage')}
          />
          {!awal && (
            <Kolom
              label="Mulai bekerja"
              type="date"
              value={masuk}
              onChange={(e) => setMasuk(e.target.value)}
              galat={galatKolom(kolomGalat, 'joined_at')}
              required
            />
          )}

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <AksiDialog>
            <Tombol type="submit" memuat={simpan.isPending} disabled={!nama.trim()}>
              Simpan
            </Tombol>
            <Tombol jenis="kedua" onClick={onTutup} disabled={simpan.isPending}>
              Batal
            </Tombol>
          </AksiDialog>
        </form>
      </IsiDialog>
    </Dialog>
  )
}
