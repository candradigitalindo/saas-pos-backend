import { useState } from 'react'
import { Coins, Mail, Store, Users } from 'lucide-react'
import { Kolom } from '@/bersama/ui/kolom'
import { KolomSandi } from '@/bersama/ui/kolom-sandi'
import { Tombol } from '@/bersama/ui/tombol'
import { KerangkaMasuk, type Keunggulan } from '@/bersama/komponen/kerangka-masuk'
import { GalatAPI } from '@/lib/api-client'
import { useSesiMitra } from '../sesi-mitra'

/**
 * Masuk portal mitra.
 *
 * Memakai EMAIL, bukan nama pengguna — realm mitra memang berbeda, dan
 * perbedaan itu ditampilkan sejak layar pertama (warna biru, lencana "Portal
 * Mitra") supaya tidak ada mitra yang mencoba memakai kredensial toko di sini.
 *
 * Tombol "Masuk" selalu aktif, seperti halaman masuk toko: kolom kosong
 * dijawab kalimat di bawah kolomnya, bukan tombol abu-abu yang terbaca rusak.
 */
const KEUNGGULAN: Keunggulan[] = [
  { ikon: Users, judul: 'Catat & tindak lanjuti prospek', isi: 'Semua calon merchant Anda di satu daftar.' },
  { ikon: Store, judul: 'Pantau merchant binaan', isi: 'Status langganan dan keaktifan — tanpa data jualan mereka.' },
  { ikon: Coins, judul: 'Komisi yang transparan', isi: 'Rincian per bulan: dihitung, disetujui, dan dicairkan.' },
]

export function HalamanMasukMitra() {
  const { masuk } = useSesiMitra()
  const [email, setEmail] = useState('')
  const [sandi, setSandi] = useState('')
  const [galat, setGalat] = useState<string | null>(null)
  const [kolomGalat, setKolomGalat] = useState<Record<string, string>>({})
  const [mengirim, setMengirim] = useState(false)

  async function kirim(e: React.FormEvent) {
    e.preventDefault()
    setGalat(null)
    const kosong: Record<string, string> = {}
    if (!email.trim()) kosong.email = 'Email belum diisi.'
    if (!sandi) kosong.password = 'Kata sandi belum diisi.'
    setKolomGalat(kosong)
    if (Object.keys(kosong).length > 0) return

    setMengirim(true)
    try {
      await masuk(email.trim(), sandi)
    } catch (e) {
      setGalat(
        e instanceof GalatAPI && e.status === 401
          ? 'Email atau kata sandi salah.'
          : e instanceof GalatAPI
            ? e.pesan
            : 'Terjadi kesalahan. Coba lagi.',
      )
    } finally {
      setMengirim(false)
    }
  }

  return (
    <KerangkaMasuk
      nada="mitra"
      lencana="Portal Mitra"
      judul="Tumbuh bersama merchant binaan Anda"
      subjudul="Prospek, merchant binaan, dan komisi Anda — dalam satu tempat."
      keunggulan={KEUNGGULAN}
    >
      <div className="flex flex-col gap-6">
        <div>
          <h1 className="text-judul font-bold text-teks-utama">Masuk sebagai mitra</h1>
          <p className="mt-1 text-isi text-teks-sekunder">
            Pakai email yang didaftarkan tim kami untuk Anda.
          </p>
        </div>

        <form onSubmit={kirim} className="flex flex-col gap-4" noValidate>
          <Kolom
            label="Email"
            type="email"
            ikon={Mail}
            autoComplete="email"
            autoCapitalize="none"
            spellCheck={false}
            autoFocus
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            bantuan="Portal mitra memakai email, bukan nama pengguna toko."
            galat={kolomGalat.email}
            required
          />
          <KolomSandi
            label="Kata sandi"
            autoComplete="current-password"
            value={sandi}
            onChange={(e) => setSandi(e.target.value)}
            galat={kolomGalat.password}
            required
          />

          {galat && (
            <p
              role="alert"
              className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks"
            >
              {galat}
            </p>
          )}

          <Tombol type="submit" lebarPenuh memuat={mengirim} labelMemuat="Sedang masuk…">
            Masuk
          </Tombol>
        </form>

        <p className="text-center text-keterangan text-teks-redup">
          Belum jadi mitra? Hubungi tim kami untuk didaftarkan.
        </p>
      </div>
    </KerangkaMasuk>
  )
}
