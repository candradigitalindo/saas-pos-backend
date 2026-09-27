import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { ArrowLeft, AtSign, ChevronDown, Phone, Store, UserRound } from 'lucide-react'
import { Kolom, Pilihan } from '@/bersama/ui/kolom'
import { KolomSandi } from '@/bersama/ui/kolom-sandi'
import { Tombol } from '@/bersama/ui/tombol'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { zonaIndonesiaPerangkat } from '@/bersama/util/tanggal'
import { cn } from '@/bersama/util/cn'
import { GalatAPI } from '@/lib/api-client'
import { galatKolom } from '@/lib/galat-kolom'
import { authApi } from '../api'
import { JENIS_USAHA } from '../tipe'

/**
 * Pendaftaran usaha baru — DUA LANGKAH PENDEK.
 *
 * Versi satu layar menumpuk tujuh isian beserta teks bantuannya setinggi
 * ±1.000px: di laptop maupun HP calon pengguna harus menggulir sebelum melihat
 * tombol daftar, dan formulir yang terasa panjang adalah alasan orang berhenti
 * di tengah jalan. Kini:
 *
 *   1. Usaha Anda   — nama usaha, jenis, nomor HP (+ kode agen, tersembunyi)
 *   2. Akun pemilik — nama, nama pengguna, email, kata sandi
 *
 * Tiap langkah muat satu layar. Isian tetap tersimpan saat maju-mundur, dan
 * bila server menolak isian langkah 1, formulir kembali ke langkah itu dengan
 * pesannya di bawah kolom yang salah.
 *
 * Sengaja TIDAK menanyakan alamat, NPWP, atau logo: semua itu bisa menyusul,
 * dan setiap kolom tambahan di sini adalah alasan untuk berhenti mendaftar
 * (ui/05-ALUR-UTAMA.md §1).
 */

/** Kunci galat server milik langkah 1; sisanya (owner.*) milik langkah 2. */
const KUNCI_LANGKAH_1 = ['business_name', 'business_type', 'phone', 'outlet_name', 'referral_code', 'timezone']

export function HalamanDaftar() {
  const { pakaiSesiBaru } = useSesi()
  const navigate = useNavigate()

  const [langkah, setLangkah] = useState<1 | 2>(1)
  const [form, setForm] = useState({
    business_name: '',
    business_type: 'retail',
    phone: '',
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

  /** Pemeriksaan ringan sebelum pindah langkah — batas yang sama dengan server. */
  function periksaLangkah1(): Record<string, string> {
    const g: Record<string, string> = {}
    if (form.business_name.trim().length < 2) g.business_name = 'Nama usaha belum diisi.'
    if (form.phone.trim().length < 5) g.phone = 'Nomor HP belum lengkap.'
    return g
  }
  function periksaLangkah2(): Record<string, string> {
    const g: Record<string, string> = {}
    if (form.nama.trim().length < 2) g.name = 'Nama Anda belum diisi.'
    if (form.username.trim().length < 3) g.username = 'Nama pengguna minimal 3 huruf.'
    else if (/\s/.test(form.username.trim())) g.username = 'Nama pengguna tidak boleh berisi spasi.'
    if (!/^\S+@\S+\.\S+$/.test(form.email.trim())) g.email = 'Alamat email belum benar.'
    if (form.password.length < 8) g.password = 'Kata sandi minimal 8 huruf.'
    return g
  }

  function lanjut(e: React.FormEvent) {
    e.preventDefault()
    setGalat(null)
    const g = periksaLangkah1()
    setKolomGalat(g)
    if (Object.keys(g).length === 0) setLangkah(2)
  }

  async function kirim(e: React.FormEvent) {
    e.preventDefault()
    setGalat(null)
    const g = periksaLangkah2()
    setKolomGalat(g)
    if (Object.keys(g).length > 0) return

    setMengirim(true)
    try {
      const hasil = await authApi.daftar({
        business_name: form.business_name.trim(),
        business_type: form.business_type,
        phone: form.phone.trim(),
        // Toko pertama dinamai sama dengan usahanya. Pemilik warung tidak
        // memikirkan "cabang" di hari pertama — itu bisa diubah nanti.
        outlet_name: form.business_name.trim(),
        timezone: zonaIndonesiaPerangkat(),
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
        // Isian yang ditolak ada di langkah 1? Kembali ke sana, supaya pesannya
        // terlihat di bawah kolom yang harus diperbaiki.
        if (Object.keys(e.kolom).some((k) => KUNCI_LANGKAH_1.includes(k))) setLangkah(1)
      } else {
        setGalat('Terjadi kesalahan. Coba lagi.')
      }
    } finally {
      setMengirim(false)
    }
  }

  const pesanGalat = galat && (
    <p
      role="alert"
      className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks"
    >
      {galat}
    </p>
  )

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h1 className="text-judul font-bold text-teks-utama">
          {langkah === 1 ? 'Daftarkan usaha Anda' : 'Buat akun pemilik'}
        </h1>
        <p className="mt-1 text-isi text-teks-sekunder">
          {langkah === 1
            ? 'Gratis, dan bisa mulai jualan hari ini juga.'
            : `Akun ini untuk masuk ke ${form.business_name.trim() || 'usaha Anda'}.`}
        </p>
        <PenandaLangkah
          langkah={langkah}
          onKembali={() => {
            setGalat(null)
            setKolomGalat({})
            setLangkah(1)
          }}
        />
      </div>

      {langkah === 1 ? (
        <form onSubmit={lanjut} className="flex flex-col gap-4" noValidate>
          <Kolom
            label="Nama usaha"
            ikon={Store}
            placeholder="Warung Bu Sari"
            autoFocus
            value={form.business_name}
            onChange={ubah('business_name')}
            galat={galatKolom(kolomGalat, 'business_name', 'outlet_name')}
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
            ikon={Phone}
            type="tel"
            inputMode="tel"
            autoComplete="tel"
            placeholder="0812…"
            value={form.phone}
            onChange={ubah('phone')}
            galat={galatKolom(kolomGalat, 'phone')}
            required
          />

          {/* Kode agen disembunyikan di balik tautan kecil supaya tidak
              membingungkan mayoritas yang mendaftar sendiri. */}
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
              // min-h-12: tombol teks tetap target sentuh 48px (ui/01 §3).
              className="-my-2 flex min-h-12 items-center gap-1 self-start text-label font-medium text-utama"
            >
              <ChevronDown className="h-4 w-4" aria-hidden />
              Punya kode dari agen?
            </button>
          )}

          {pesanGalat}

          <Tombol type="submit" lebarPenuh>
            Lanjut
          </Tombol>
        </form>
      ) : (
        <form onSubmit={kirim} className="flex flex-col gap-4" noValidate>
          <Kolom
            label="Nama Anda"
            ikon={UserRound}
            autoComplete="name"
            autoFocus
            value={form.nama}
            onChange={ubah('nama')}
            galat={galatKolom(kolomGalat, 'owner.name')}
            required
          />
          <Kolom
            label="Nama pengguna"
            ikon={AtSign}
            bantuan="Untuk masuk nanti (email juga bisa). Tanpa spasi."
            autoCapitalize="none"
            autoComplete="username"
            value={form.username}
            onChange={ubah('username')}
            galat={galatKolom(kolomGalat, 'owner.username')}
            required
          />
          <Kolom
            label="Email"
            type="email"
            autoCapitalize="none"
            autoComplete="email"
            value={form.email}
            onChange={ubah('email')}
            galat={galatKolom(kolomGalat, 'owner.email')}
            required
          />
          <KolomSandi
            label="Kata sandi"
            autoComplete="new-password"
            bantuan="Minimal 8 huruf."
            value={form.password}
            onChange={ubah('password')}
            galat={galatKolom(kolomGalat, 'owner.password')}
            required
          />

          {pesanGalat}

          <Tombol type="submit" lebarPenuh memuat={mengirim} labelMemuat="Menyiapkan usaha Anda…">
            Buat Usaha Saya
          </Tombol>
        </form>
      )}

      {/* Hanya di langkah 1: di langkah 2 orangnya sudah memutuskan mendaftar,
          dan ruangnya lebih berguna untuk isian. */}
      {langkah === 1 && (
        <>
          <div className="flex items-center gap-3 text-keterangan text-teks-redup" aria-hidden>
            <span className="h-px flex-1 bg-garis" />
            Sudah punya akun?
            <span className="h-px flex-1 bg-garis" />
          </div>

          <Tombol asChild jenis="kedua" lebarPenuh>
            <Link to="/masuk">Masuk ke toko Anda</Link>
          </Tombol>
        </>
      )}
    </div>
  )
}

/**
 * "Langkah 1 dari 2" + dua ruas kemajuan. Di langkah 2 tombol "Kembali" duduk
 * di baris yang sama — sebaris tersendiri menghabiskan 56px dan mendorong
 * tombol "Buat Usaha Saya" keluar layar di HP kecil (360×740).
 */
function PenandaLangkah({ langkah, onKembali }: { langkah: 1 | 2; onKembali: () => void }) {
  return (
    <div className="mt-3 flex min-h-12 items-center gap-3">
      {langkah === 2 && (
        <button
          type="button"
          onClick={onKembali}
          aria-label="Kembali ke data usaha"
          className="-ml-1 flex min-h-12 shrink-0 items-center gap-1 px-1 text-label font-medium text-teks-sekunder hover:text-teks-utama"
        >
          <ArrowLeft className="h-4 w-4" aria-hidden />
          Kembali
        </button>
      )}
      <div className="flex flex-1 gap-1.5" aria-hidden>
        {[1, 2].map((n) => (
          <span
            key={n}
            className={cn('h-1.5 flex-1 rounded-full', n <= langkah ? 'bg-utama' : 'bg-garis')}
          />
        ))}
      </div>
      <p className="shrink-0 text-keterangan font-medium text-teks-redup">
        Langkah {langkah} dari 2
      </p>
    </div>
  )
}
