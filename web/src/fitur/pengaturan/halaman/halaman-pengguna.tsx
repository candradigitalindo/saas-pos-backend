import { useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { Plus, UserPlus } from 'lucide-react'
import { Kartu } from '@/bersama/ui/kartu'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { AksiDialog, Dialog, IsiDialog } from '@/bersama/ui/dialog'
import { LencanaStatus } from '@/bersama/komponen/lencana-status'
import { KerangkaBaris } from '@/bersama/komponen/kerangka'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { galatKolom } from '@/lib/galat-kolom'
import type { Pengguna } from '@/bersama/tipe/organisasi'
import { pengaturanApi } from '../api'

/**
 * Pengguna staf.
 *
 * Backend mendukung PERAN GANDA: satu orang boleh memegang beberapa peran, dan
 * izinnya adalah gabungan. Layar ini menampilkan itu apa adanya — termasuk
 * pratinjau izin gabungan, supaya pemilik paham akibat merangkap peran sebelum
 * menyimpan.
 */
export function HalamanPengguna() {
  const { profil } = useSesi()
  const [formUntuk, setFormUntuk] = useState<Pengguna | 'baru' | null>(null)

  const pengguna = useQuery({
    queryKey: ['pengguna'],
    queryFn: () => pengaturanApi.daftarPengguna(),
  })

  const peran = useQuery({
    queryKey: ['peran'],
    queryFn: () => pengaturanApi.daftarPeran(),
  })

  const daftar = pengguna.data?.data ?? []

  return (
    <div className="mx-auto flex w-full max-w-2xl flex-col gap-4">
      <header className="flex flex-wrap items-center justify-between gap-3">
        <h1 className="text-judul font-bold text-teks-utama">Pengguna</h1>
        <Tombol onClick={() => setFormUntuk('baru')}>
          <Plus className="h-5 w-5" aria-hidden />
          Tambah Pengguna
        </Tombol>
      </header>

      {pengguna.isLoading ? (
        <KerangkaBaris jumlah={4} />
      ) : (
        <ul className="flex flex-col gap-2">
          {daftar.map((u) => (
            <li key={u.id}>
              <Kartu className="flex items-center justify-between gap-3 p-4">
                <div className="min-w-0">
                  <p className="truncate font-semibold text-teks-utama">
                    {u.name}
                    {u.id === profil?.user.id && (
                      <span className="ml-2 text-keterangan font-normal text-teks-redup">
                        (Anda)
                      </span>
                    )}
                  </p>
                  <p className="text-keterangan text-teks-redup">
                    {u.username} · {u.role_name}
                    {(u.role_ids?.length ?? 0) > 1 &&
                      ` +${(u.role_ids?.length ?? 1) - 1} peran lain`}
                  </p>
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  {u.is_active ? (
                    <LencanaStatus nada="berhasil" anak="Aktif" />
                  ) : (
                    <LencanaStatus nada="netral" anak="Nonaktif" />
                  )}
                  <Tombol jenis="kedua" ukuran="padat" onClick={() => setFormUntuk(u)}>
                    Ubah
                  </Tombol>
                </div>
              </Kartu>
            </li>
          ))}
        </ul>
      )}

      {formUntuk && (
        <DialogPengguna
          awal={formUntuk === 'baru' ? null : formUntuk}
          daftarPeran={peran.data?.data ?? []}
          onTutup={() => setFormUntuk(null)}
        />
      )}
    </div>
  )
}

function DialogPengguna({
  awal,
  daftarPeran,
  onTutup,
}: {
  awal: Pengguna | null
  daftarPeran: { id: string; name: string }[]
  onTutup: () => void
}) {
  const toast = useToast()
  const qc = useQueryClient()

  const [nama, setNama] = useState(awal?.name ?? '')
  const [username, setUsername] = useState(awal?.username ?? '')
  const [email, setEmail] = useState(awal?.email ?? '')
  const [sandi, setSandi] = useState('')
  const [peranUtama, setPeranUtama] = useState(awal?.role_id ?? daftarPeran[0]?.id ?? '')
  const [peranTambahan, setPeranTambahan] = useState<Set<string>>(
    new Set((awal?.role_ids ?? []).filter((r) => r !== awal?.role_id)),
  )
  const [kolomGalat, setKolomGalat] = useState<Record<string, string>>({})
  const [galat, setGalat] = useState<string | null>(null)

  const katalog = useQuery({
    queryKey: ['katalog-izin'],
    queryFn: () => pengaturanApi.katalogIzin(),
    staleTime: 10 * 60_000,
  })

  // GET /roles hanya mengembalikan id & nama — permission_codes cuma ada di
  // GET /roles/:id. Tanpa mengambil detailnya, pratinjau izin gabungan akan
  // selalu kosong dan justru menyesatkan.
  const rincianPeran = useQuery({
    queryKey: ['peran-rinci', daftarPeran.map((r) => r.id).join(',')],
    queryFn: async () =>
      Promise.all(daftarPeran.map((r) => pengaturanApi.peran(r.id))),
    enabled: daftarPeran.length > 0,
    staleTime: 5 * 60_000,
  })

  // Pratinjau izin gabungan — inti dari "menampilkan peran ganda dengan jujur".
  const izinGabungan = useMemo(() => {
    const kode = new Set<string>()
    for (const r of rincianPeran.data ?? []) {
      if (r.id === peranUtama || peranTambahan.has(r.id)) {
        for (const k of r.permission_codes ?? []) kode.add(k)
      }
    }
    return kode
  }, [rincianPeran.data, peranUtama, peranTambahan])

  const grupIzin = useMemo(() => {
    const m = new Map<string, number>()
    for (const i of katalog.data ?? []) {
      if (izinGabungan.has(i.code)) m.set(i.group_name, (m.get(i.group_name) ?? 0) + 1)
    }
    return [...m.entries()]
  }, [katalog.data, izinGabungan])

  const simpan = useMutation({
    mutationFn: () => {
      const tambahan = [...peranTambahan]
      if (awal) {
        return pengaturanApi.ubahPengguna(awal.id, {
          name: nama.trim(),
          username: username.trim(),
          email: email.trim(),
          ...(sandi ? { password: sandi } : {}),
          role_id: peranUtama,
          role_ids: tambahan,
        })
      }
      return pengaturanApi.buatPengguna({
        name: nama.trim(),
        username: username.trim(),
        email: email.trim(),
        password: sandi,
        role_id: peranUtama,
        ...(tambahan.length ? { role_ids: tambahan } : {}),
      })
    },
    onSuccess: (u) => {
      qc.invalidateQueries({ queryKey: ['pengguna'] })
      toast.berhasil(awal ? `${u.name} diperbarui.` : `${u.name} bisa mulai masuk sekarang.`)
      onTutup()
    },
    onError: (e) => {
      if (e instanceof GalatAPI) {
        setKolomGalat(e.kolom)
        setGalat(e.status === 422 ? null : e.pesan)
      } else setGalat('Terjadi kesalahan. Coba lagi.')
    },
  })

  const lengkap =
    nama.trim() && username.trim() && email.trim() && peranUtama && (awal || sandi.length >= 8)

  return (
    <Dialog open onOpenChange={(o) => !o && !simpan.isPending && onTutup()}>
      <IsiDialog
        judul={awal ? `Ubah ${awal.name}` : 'Tambah Pengguna'}
        className="sm:max-w-lg"
      >
        <form
          onSubmit={(e) => {
            e.preventDefault()
            setGalat(null)
            setKolomGalat({})
            simpan.mutate()
          }}
          className="flex max-h-[65vh] flex-col gap-4 overflow-y-auto"
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
            label="Nama pengguna"
            autoCapitalize="none"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            bantuan="Dipakai untuk masuk. Tanpa spasi."
            galat={galatKolom(kolomGalat, 'username')}
            required
          />
          <Kolom
            label="Email"
            type="email"
            autoCapitalize="none"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            galat={galatKolom(kolomGalat, 'email')}
            required
          />
          <Kolom
            label={awal ? 'Kata sandi baru' : 'Kata sandi'}
            type="password"
            autoComplete="new-password"
            value={sandi}
            onChange={(e) => setSandi(e.target.value)}
            bantuan={
              awal
                ? 'Kosongkan bila tidak ingin mengganti kata sandinya.'
                : 'Minimal 8 huruf.'
            }
            galat={galatKolom(kolomGalat, 'password')}
            required={!awal}
          />

          <Pilihan
            label="Peran utama"
            value={peranUtama}
            onChange={(e) => setPeranUtama(e.target.value)}
            galat={galatKolom(kolomGalat, 'role_id')}
            required
          >
            {daftarPeran.map((r) => (
              <option key={r.id} value={r.id}>
                {r.name}
              </option>
            ))}
          </Pilihan>

          <fieldset className="flex flex-col gap-2">
            <legend className="mb-1 text-label font-medium text-teks-sekunder">
              Peran tambahan
            </legend>
            <p className="mb-1 text-keterangan text-teks-redup">
              Boleh dikosongkan. Orang yang merangkap peran mendapat gabungan
              izinnya.
            </p>
            {daftarPeran
              .filter((r) => r.id !== peranUtama)
              .map((r) => (
                <label
                  key={r.id}
                  className="flex min-h-12 cursor-pointer items-center gap-3 rounded-kontrol px-2 hover:bg-permukaan-2"
                >
                  <input
                    type="checkbox"
                    checked={peranTambahan.has(r.id)}
                    onChange={(e) =>
                      setPeranTambahan((lama) => {
                        const baru = new Set(lama)
                        if (e.target.checked) baru.add(r.id)
                        else baru.delete(r.id)
                        return baru
                      })
                    }
                    className="h-5 w-5 shrink-0 accent-[var(--warna-utama)]"
                  />
                  <span className="text-label text-teks-utama">{r.name}</span>
                </label>
              ))}
          </fieldset>

          {/* Pratinjau izin gabungan: pemilik melihat akibatnya SEBELUM menyimpan. */}
          {rincianPeran.isLoading ? (
            <p className="text-keterangan text-teks-redup">Menghitung izin gabungan…</p>
          ) : grupIzin.length > 0 ? (
            <div className="rounded-kontrol bg-sorot px-3 py-2">
              <p className="text-label font-medium text-hijau-800">
                Orang ini akan bisa mengakses {izinGabungan.size} hal:
              </p>
              <ul className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5">
                {grupIzin.map(([grup, jumlah]) => (
                  <li key={grup} className="text-keterangan text-hijau-800">
                    {grup} ({jumlah})
                  </li>
                ))}
              </ul>
            </div>
          ) : null}

          {galat && (
            <p className="rounded-kontrol border border-bahaya bg-red-50 px-3 py-2 text-label text-bahaya-teks">
              {galat}
            </p>
          )}

          <AksiDialog>
            <Tombol type="submit" memuat={simpan.isPending} disabled={!lengkap}>
              <UserPlus className="h-5 w-5" aria-hidden />
              {awal ? 'Simpan Perubahan' : 'Tambah Pengguna'}
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
