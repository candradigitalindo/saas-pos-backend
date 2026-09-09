import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ChevronDown } from 'lucide-react'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GalatAPI } from '@/lib/api-client'
import { galatKolom } from '@/lib/galat-kolom'
import { authApi } from '../api'
import { JENIS_USAHA } from '../tipe'

/**
 * Pendaftaran usaha baru — satu layar.
 *
 * Sengaja TIDAK menanyakan alamat, NPWP, atau logo: semua itu bisa menyusul,
 * dan setiap kolom tambahan di sini adalah alasan untuk berhenti mendaftar
 * (ui/05-ALUR-UTAMA.md §1).
 *
 * Kode agen disembunyikan di balik tautan kecil supaya tidak membingungkan
 * mayoritas yang mendaftar sendiri.
 */
export function HalamanDaftar() {
  const { pakaiSesiBaru } = useSesi()
  const navigate = useNavigate()

  const [form, setForm] = useState({
    business_name: '',
    business_type: 'retail',
    phone: '',
    outlet_name: '',
    nama: '',
    username: '',
    email: '',
    password: '',
    referral_code: '',
  })
  const [bukaKodeAgen, setBukaKodeAgen] = useState(false)
  const [galat, setGalat] = useState<string | null>(null)
  const [kolomGalat, setKolomGalat] = useState<Record<string, string>>({})
  const [mengirim, setMengirim] = useState(false)

  const ubah = (k: keyof typeof form) => (e: { target: { value: string } }) =>
    setForm((f) => ({ ...f, [k]: e.target.value }))

  async function kirim(e: React.FormEvent) {
    e.preventDefault()
    setGalat(null)
    setKolomGalat({})
    setMengirim(true)
    try {
      const hasil = await authApi.daftar({
        business_name: form.business_name.trim(),
        business_type: form.business_type,
        phone: form.phone.trim(),
        // Toko pertama dinamai sama dengan usahanya. Pemilik warung tidak
        // memikirkan "cabang" di hari pertama — itu bisa diubah nanti.
        outlet_name: form.outlet_name.trim() || form.business_name.trim(),
        referral_code: form.referral_code.trim() || undefined,
        owner: {
          name: form.nama.trim(),
          username: form.username.trim(),
          email: form.email.trim(),
          password: form.password,
        },
      })
      // Server sudah mengembalikan sesi — pemilik langsung masuk, tidak
      // disuruh mengetik ulang kata sandi yang baru saja dibuatnya.
      pakaiSesiBaru(hasil.auth)
      navigate('/selamat-datang', { replace: true })
    } catch (e) {
      if (e instanceof GalatAPI) {
        setKolomGalat(e.kolom)
        setGalat(e.status === 422 ? null : e.pesan)
      } else {
        setGalat('Terjadi kesalahan. Coba lagi.')
      }
    } finally {
      setMengirim(false)
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <div className="text-center">
        <h1 className="text-judul font-bold text-teks-utama">Daftarkan usaha Anda</h1>
        <p className="mt-1 text-isi text-teks-sekunder">
          Gratis, dan bisa mulai jualan hari ini juga.
        </p>
      </div>

      <form onSubmit={kirim} className="flex flex-col gap-4" noValidate>
        <Kolom
          label="Nama usaha"
          placeholder="Warung Bu Sari"
          value={form.business_name}
          onChange={ubah('business_name')}
          galat={galatKolom(kolomGalat, 'business_name')}
          required
        />
        <Pilihan
          label="Jenis usaha"
          value={form.business_type}
          onChange={ubah('business_type')}
          galat={galatKolom(kolomGalat, 'business_type')}
        >
          {JENIS_USAHA.map((j) => (
            <option key={j.nilai} value={j.nilai}>
              {j.label}
            </option>
          ))}
        </Pilihan>
        <Kolom
          label="Nomor HP"
          type="tel"
          inputMode="tel"
          placeholder="0812…"
          value={form.phone}
          onChange={ubah('phone')}
          galat={galatKolom(kolomGalat, 'phone')}
          required
        />

        <hr className="border-garis" />

        <Kolom
          label="Nama Anda"
          value={form.nama}
          onChange={ubah('nama')}
          galat={galatKolom(kolomGalat, 'owner.name')}
          required
        />
        <Kolom
          label="Nama pengguna"
          bantuan="Dipakai untuk masuk. Tanpa spasi."
          autoCapitalize="none"
          value={form.username}
          onChange={ubah('username')}
          galat={galatKolom(kolomGalat, 'owner.username')}
          required
        />
        <Kolom
          label="Email"
          type="email"
          autoCapitalize="none"
          bantuan="Untuk memulihkan akun bila lupa kata sandi."
          value={form.email}
          onChange={ubah('email')}
          galat={galatKolom(kolomGalat, 'owner.email')}
          required
        />
        <Kolom
          label="Kata sandi"
          type="password"
          autoComplete="new-password"
          bantuan="Minimal 8 huruf."
          value={form.password}
          onChange={ubah('password')}
          galat={galatKolom(kolomGalat, 'owner.password')}
          required
        />

        {bukaKodeAgen ? (
          <Kolom
            label="Kode dari agen"
            bantuan="Kosongkan bila Anda mendaftar sendiri."
            autoCapitalize="characters"
            value={form.referral_code}
            onChange={ubah('referral_code')}
            galat={galatKolom(kolomGalat, 'referral_code')}
          />
        ) : (
          <button
            type="button"
            onClick={() => setBukaKodeAgen(true)}
            className="flex items-center gap-1 self-start text-label font-medium text-utama"
          >
            <ChevronDown className="h-4 w-4" aria-hidden />
            Punya kode dari agen?
          </button>
        )}

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-red-50 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <Tombol type="submit" lebarPenuh memuat={mengirim} labelMemuat="Menyiapkan usaha Anda…">
          Buat Usaha Saya
        </Tombol>
      </form>

      <p className="text-center text-label text-teks-sekunder">
        Sudah punya akun?{' '}
        <Link to="/masuk" className="font-semibold text-utama underline underline-offset-2">
          Masuk di sini
        </Link>
      </p>
    </div>
  )
}
