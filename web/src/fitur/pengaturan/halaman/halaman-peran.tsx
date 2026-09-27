import { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Lock, Plus, ShieldCheck } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'
import { IZIN } from '@/lib/izin'
import { cn } from '@/bersama/util/cn'
import type { Peran } from '@/bersama/tipe/organisasi'
import { pengaturanApi } from '../api'

/**
 * Peran & hak akses.
 *
 * Layar ini menentukan siapa boleh apa, jadi harus jujur DAN aman:
 *
 *  - Izin dikelompokkan per `group_name` dari server, bukan 50 kotak centang polos.
 *  - Setiap izin diberi penjelasan bahasa awam, bukan kodenya.
 *  - `role.manage` pada peran bawaan Pemilik DIKUNCI. Backend menolaknya (422),
 *    dan lebih baik dicegah sebelum ditekan daripada menampilkan galat setelah
 *    pemilik mengira ia berhasil melepasnya.
 */
export function HalamanPeran() {
  const [dipilih, setDipilih] = useState<Peran | null>(null)
  const [buatBaru, setBuatBaru] = useState(false)

  const peran = useQuery({
    queryKey: ['peran'],
    queryFn: () => pengaturanApi.daftarPeran(),
  })

  const daftar = peran.data?.data ?? []

  return (
    <div className="flex w-full max-w-3xl flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h1 className="text-judul font-bold text-teks-utama">Peran & Hak Akses</h1>
          <p className="text-label text-teks-sekunder">
            Menentukan menu dan tombol apa saja yang dilihat setiap orang.
          </p>
        </div>
        <Tombol onClick={() => setBuatBaru(true)}>
          <Plus className="h-5 w-5" aria-hidden />
          Buat Peran
        </Tombol>
      </header>

      {peran.isLoading ? (
        <KerangkaBaris jumlah={4} />
      ) : (
        <ul className="flex flex-col gap-2">
          {daftar.map((p) => (
            <li key={p.id}>
              <Kartu className="flex items-center justify-between gap-3 p-4">
                <div className="min-w-0">
                  <p className="flex items-center gap-2 font-semibold text-teks-utama">
                    {p.name}
                    {p.is_system && (
                      <span className="inline-flex items-center gap-1 rounded-full bg-permukaan-2 px-2 py-0.5 text-keterangan font-medium text-teks-sekunder">
                        <Lock className="h-3 w-3" aria-hidden />
                        Bawaan
                      </span>
                    )}
                  </p>
                  <p className="text-keterangan text-teks-redup">
                    {p.description || `${p.permission_codes?.length ?? 0} izin`}
                  </p>
                </div>
                <Tombol jenis="kedua" ukuran="padat" onClick={() => setDipilih(p)}>
                  Atur Izin
                </Tombol>
              </Kartu>
            </li>
          ))}
        </ul>
      )}

      {dipilih && <DialogIzin peran={dipilih} onTutup={() => setDipilih(null)} />}
      {buatBaru && <DialogPeranBaru onTutup={() => setBuatBaru(false)} />}
    </div>
  )
}

function DialogIzin({ peran, onTutup }: { peran: Peran; onTutup: () => void }) {
  const toast = useToast()
  const qc = useQueryClient()

  const katalog = useQuery({
    queryKey: ['katalog-izin'],
    queryFn: () => pengaturanApi.katalogIzin(),
    staleTime: 10 * 60_000,
  })

  // Daftar peran ringkas belum tentu membawa permission_codes — ambil detailnya.
  const detail = useQuery({
    queryKey: ['peran', peran.id],
    queryFn: () => pengaturanApi.peran(peran.id),
  })

  const [terpilih, setTerpilih] = useState<Set<string>>(new Set())
  const [galat, setGalat] = useState<string | null>(null)

  useEffect(() => {
    const kode = detail.data?.permission_codes ?? peran.permission_codes
    if (kode) setTerpilih(new Set(kode))
  }, [detail.data, peran.permission_codes])

  const kelompok = useMemo(() => {
    const m = new Map<string, { code: string; description: string }[]>()
    for (const i of katalog.data ?? []) {
      const daftar = m.get(i.group_name) ?? []
      daftar.push({ code: i.code, description: i.description })
      m.set(i.group_name, daftar)
    }
    return [...m.entries()]
  }, [katalog.data])

  const simpan = useMutation({
    mutationFn: () => pengaturanApi.aturIzinPeran(peran.id, [...terpilih]),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['peran'] })
      qc.invalidateQueries({ queryKey: ['me'] })
      toast.berhasil(`Hak akses peran ${peran.name} diperbarui.`)
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  const memuat = katalog.isLoading || detail.isLoading

  return (
    <Dialog open onOpenChange={(o) => !o && !simpan.isPending && onTutup()}>
      <IsiDialog
        judul={`Hak akses: ${peran.name}`}
        keterangan="Centang yang boleh dikerjakan. Yang tidak dicentang, menunya tidak akan muncul sama sekali."
        className="sm:max-w-2xl"
      >
        {memuat ? (
          <KerangkaBaris jumlah={5} />
        ) : (
          <div className="flex max-h-[55vh] flex-col gap-4 overflow-y-auto">
            {kelompok.map(([grup, izin]) => (
              <fieldset key={grup} className="flex flex-col gap-2">
                <legend className="mb-1 text-label font-semibold text-teks-utama">
                  {grup}
                </legend>
                {izin.map((i) => {
                  // Satu-satunya izin yang tidak boleh dilepas dari peran bawaan.
                  const terkunci = peran.is_system && i.code === IZIN.roleManage
                  return (
                    <label
                      key={i.code}
                      className={cn(
                        'flex min-h-12 items-start gap-3 rounded-kontrol px-2 py-1.5',
                        terkunci ? 'bg-permukaan-2' : 'cursor-pointer hover:bg-permukaan-2',
                      )}
                    >
                      <input
                        type="checkbox"
                        checked={terpilih.has(i.code)}
                        disabled={terkunci}
                        onChange={(e) =>
                          setTerpilih((lama) => {
                            const baru = new Set(lama)
                            if (e.target.checked) baru.add(i.code)
                            else baru.delete(i.code)
                            return baru
                          })
                        }
                        className="mt-1 h-5 w-5 shrink-0 accent-[var(--warna-utama)]"
                      />
                      <span className="min-w-0">
                        {/* Penjelasan bahasa awam, bukan kodenya. */}
                        <span className="block text-label text-teks-utama">
                          {i.description}
                        </span>
                        {terkunci && (
                          <span className="mt-0.5 flex items-start gap-1 text-keterangan text-jingga-700">
                            <Lock className="mt-0.5 h-3 w-3 shrink-0" aria-hidden />
                            Izin ini tidak bisa dilepas. Tanpa ini, tidak ada lagi
                            yang bisa mengatur hak akses di usaha Anda.
                          </span>
                        )}
                      </span>
                    </label>
                  )
                })}
              </fieldset>
            ))}
          </div>
        )}

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <AksiDialog>
          <Tombol
            memuat={simpan.isPending}
            disabled={memuat}
            onClick={() => {
              setGalat(null)
              simpan.mutate()
            }}
          >
            Simpan Hak Akses ({terpilih.size} izin)
          </Tombol>
          <Tombol jenis="kedua" onClick={onTutup} disabled={simpan.isPending}>
            Batal
          </Tombol>
        </AksiDialog>
      </IsiDialog>
    </Dialog>
  )
}

function DialogPeranBaru({ onTutup }: { onTutup: () => void }) {
  const toast = useToast()
  const qc = useQueryClient()
  const [nama, setNama] = useState('')
  const [keterangan, setKeterangan] = useState('')
  const [galat, setGalat] = useState<string | null>(null)

  const buat = useMutation({
    mutationFn: () => pengaturanApi.buatPeran(nama.trim(), keterangan.trim() || undefined),
    onSuccess: (p) => {
      qc.invalidateQueries({ queryKey: ['peran'] })
      toast.berhasil(`Peran ${p.name} dibuat. Atur izinnya sekarang.`)
      onTutup()
    },
    onError: (e) => setGalat(e instanceof GalatAPI ? e.pesan : 'Terjadi kesalahan.'),
  })

  return (
    <Dialog open onOpenChange={(o) => !o && !buat.isPending && onTutup()}>
      <IsiDialog
        judul="Buat Peran Baru"
        keterangan="Peran dibuat tanpa izin apa pun. Setelah dibuat, atur izinnya lewat tombol Atur Izin."
      >
        <form
          onSubmit={(e) => {
            e.preventDefault()
            setGalat(null)
            buat.mutate()
          }}
          className="flex flex-col gap-4"
          noValidate
        >
          <Kolom
            label="Nama peran"
            placeholder="Contoh: Kepala Gudang"
            value={nama}
            onChange={(e) => setNama(e.target.value)}
            autoFocus
            required
          />
          <Kolom
            label="Keterangan"
            placeholder="Apa yang dikerjakan peran ini"
            value={keterangan}
            onChange={(e) => setKeterangan(e.target.value)}
            bantuan="Boleh dikosongkan."
          />

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <AksiDialog>
            <Tombol type="submit" memuat={buat.isPending} disabled={nama.trim().length < 2}>
              <ShieldCheck className="h-5 w-5" aria-hidden />
              Buat Peran
            </Tombol>
            <Tombol jenis="kedua" onClick={onTutup} disabled={buat.isPending}>
              Batal
            </Tombol>
          </AksiDialog>
        </form>
      </IsiDialog>
    </Dialog>
  )
}
