import { useState } from 'react'
import { Handshake } from 'lucide-react'
import { Kolom } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { GalatAPI } from '@/lib/api-client'
import { useSesiMitra } from '../sesi-mitra'

/**
 * Masuk portal mitra.
 *
 * Memakai EMAIL, bukan nama pengguna — realm mitra memang berbeda, dan
 * perbedaan itu ditampilkan sejak layar pertama supaya tidak ada mitra yang
 * mencoba memakai kredensial toko di sini.
 */
export function HalamanMasukMitra() {
  const { masuk } = useSesiMitra()
  const [email, setEmail] = useState('')
  const [sandi, setSandi] = useState('')
  const [galat, setGalat] = useState<string | null>(null)
  const [mengirim, setMengirim] = useState(false)

  async function kirim(e: React.FormEvent) {
    e.preventDefault()
    setGalat(null)
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
    <div className="flex flex-col gap-6">
      <div className="text-center">
        <Handshake className="mx-auto h-10 w-10 text-info" aria-hidden />
        <h1 className="mt-3 text-judul font-bold text-teks-utama">Portal Mitra</h1>
        <p className="mt-1 text-isi text-teks-sekunder">
          Masuk untuk melihat prospek, merchant binaan, dan komisi Anda.
        </p>
      </div>

      <form onSubmit={kirim} className="flex flex-col gap-4" noValidate>
        <Kolom
          label="Email"
          type="email"
          autoComplete="email"
          autoCapitalize="none"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          bantuan="Portal mitra memakai email, bukan nama pengguna."
          required
        />
        <Kolom
          label="Kata sandi"
          type="password"
          autoComplete="current-password"
          value={sandi}
          onChange={(e) => setSandi(e.target.value)}
          required
        />

        {galat && (
          <p className="rounded-kontrol border border-bahaya bg-bahaya-teks/10 px-3 py-2 text-label text-bahaya-teks">
            {galat}
          </p>
        )}

        <Tombol
          type="submit"
          lebarPenuh
          memuat={mengirim}
          labelMemuat="Sedang masuk…"
          disabled={!email || !sandi}
        >
          Masuk
        </Tombol>
      </form>

      <p className="text-center text-keterangan text-teks-redup">
        Belum jadi mitra? Hubungi tim kami untuk didaftarkan.
      </p>
    </div>
  )
}
