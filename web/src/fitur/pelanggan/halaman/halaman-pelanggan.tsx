import { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, Search, Users } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom } from '@/bersama/ui/kolom'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { KeadaanGagal, KeadaanKosong } from '@/bersama/komponen/keadaan-kosong'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { galatKolom } from '@/lib/galat-kolom'
import { IZIN } from '@/lib/izin'
import { formatRupiah } from '@/bersama/util/uang'
import type { Pelanggan } from '@/bersama/tipe/pos'
import { pelangganApi } from '../api'

/** Daftar pelanggan. Dipakai kasir untuk kasbon dan pemilik untuk menagih. */
export function HalamanPelanggan() {
  const { boleh } = useSesi()
  const [cari, setCari] = useState('')
  const [formUntuk, setFormUntuk] = useState<Pelanggan | 'baru' | null>(null)

  const q = useQuery({
    queryKey: ['pelanggan', cari],
    queryFn: () => pelangganApi.daftar(cari || undefined),
    staleTime: 30_000,
  })

  const daftar = q.data?.data ?? []

  return (
    <div className="flex flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-judul font-bold text-teks-utama">Pelanggan</h1>
        {boleh(IZIN.customerEdit) && (
          <Tombol onClick={() => setFormUntuk('baru')}>
            <Plus className="h-5 w-5" aria-hidden />
            Tambah Pelanggan
          </Tombol>
        )}
      </header>

      <div className="relative">
        <Search
          className="pointer-events-none absolute left-3 top-1/2 h-5 w-5 -translate-y-1/2 text-teks-redup"
          aria-hidden
        />
        <input
          type="search"
          value={cari}
          onChange={(e) => setCari(e.target.value)}
          placeholder="Cari nama atau nomor HP…"
          aria-label="Cari pelanggan"
          className="h-12 w-full rounded-kontrol border border-garis bg-permukaan pl-10 pr-3 text-isi text-teks-utama placeholder:text-teks-redup"
        />
      </div>

      {q.isLoading ? (
        <KerangkaBaris jumlah={5} />
      ) : q.isError ? (
        <KeadaanGagal
          pesan={q.error instanceof GalatAPI ? q.error.pesan : 'Daftar pelanggan belum bisa dimuat.'}
          onCobaLagi={() => q.refetch()}
        />
      ) : daftar.length === 0 ? (
        <KeadaanKosong
          ikon={Users}
          judul={cari ? 'Pelanggan tidak ditemukan' : 'Belum ada pelanggan'}
          penjelasan={
            cari
              ? `Tidak ada pelanggan bernama "${cari}".`
              : 'Simpan data pelanggan langganan supaya bisa mencatat kasbon dan menagihnya nanti.'
          }
          aksi={
            boleh(IZIN.customerEdit) && !cari
              ? { label: 'Tambah Pelanggan', onKlik: () => setFormUntuk('baru') }
              : undefined
          }
        />
      ) : (
        <ul className="flex flex-col gap-2">
          {daftar.map((p) => (
            <li key={p.id}>
              <Kartu className="flex items-center justify-between gap-3 p-4">
                <div className="min-w-0">
                  <p className="truncate font-semibold text-teks-utama">{p.name}</p>
                  <p className="text-keterangan text-teks-redup">
                    {p.phone || 'Tanpa nomor HP'}
                    {p.credit_limit > 0 &&
                      ` · batas kasbon ${formatRupiah(p.credit_limit)}`}
                  </p>
                </div>
                {boleh(IZIN.customerEdit) && (
                  <Tombol jenis="kedua" ukuran="padat" onClick={() => setFormUntuk(p)}>
                    Ubah
                  </Tombol>
                )}
              </Kartu>
            </li>
          ))}
        </ul>
      )}

      {formUntuk && (
        <DialogPelanggan
          awal={formUntuk === 'baru' ? null : formUntuk}
          onTutup={() => setFormUntuk(null)}
        />
      )}
    </div>
  )
}

function DialogPelanggan({
  awal,
  onTutup,
}: {
  awal: Pelanggan | null
  onTutup: () => void
}) {
  const toast = useToast()
  const qc = useQueryClient()
  const [nama, setNama] = useState(awal?.name ?? '')
  const [hp, setHp] = useState(awal?.phone ?? '')
  const [batas, setBatas] = useState(awal?.credit_limit ?? 0)
  const [kolomGalat, setKolomGalat] = useState<Record<string, string>>({})
  const [galat, setGalat] = useState<string | null>(null)

  const simpan = useMutation({
    mutationFn: () => {
      const isi = {
        name: nama.trim(),
        phone: hp.trim() || undefined,
        credit_limit: batas,
      }
      return awal ? pelangganApi.ubah(awal.id, isi) : pelangganApi.buat(isi)
    },
    onSuccess: (p) => {
      qc.invalidateQueries({ queryKey: ['pelanggan'] })
      toast.berhasil(awal ? `${p.name} diperbarui.` : `${p.name} ditambahkan.`)
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
      <IsiDialog judul={awal ? 'Ubah Pelanggan' : 'Tambah Pelanggan'}>
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
            label="Nama"
            value={nama}
            onChange={(e) => setNama(e.target.value)}
            galat={galatKolom(kolomGalat, 'name')}
            autoFocus
            required
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
          <KolomUang
            label="Batas kasbon"
            nilai={batas}
            onNilai={setBatas}
            bantuan="Utang pelanggan tidak boleh melebihi angka ini. Isi Rp 0 bila tidak dibatasi."
            galat={galatKolom(kolomGalat, 'credit_limit')}
          />

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-red-50 px-3 py-2 text-label text-bahaya-teks">
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
