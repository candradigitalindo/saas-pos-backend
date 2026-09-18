import { useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { Kolom } from '@/bersama/ui/kolom'
import { Tombol } from '@/bersama/ui/tombol'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GALAT_SESI_HABIS, GalatAPI } from '@/lib/api-client'

export function HalamanMasuk() {
  const { masuk } = useSesi()
  const navigate = useNavigate()

  const [username, setUsername] = useState('')
  const [sandi, setSandi] = useState('')
  const [galat, setGalat] = useState<string | null>(null)
  const [kolomGalat, setKolomGalat] = useState<Record<string, string>>({})
  const [mengirim, setMengirim] = useState(false)

  async function kirim(e: React.FormEvent) {
    e.preventDefault()
    setGalat(null)
    setKolomGalat({})
    setMengirim(true)
    try {
      await masuk(username.trim(), sandi)
      navigate('/', { replace: true })
    } catch (e) {
      if (e instanceof GalatAPI) {
        setKolomGalat(e.kolom)
        // 401 di layar masuk bukan "sesi habis" — itu salah nama/sandi.
        setGalat(
          e.status === 401 || e.pesan === GALAT_SESI_HABIS
            ? 'Nama pengguna atau kata sandi salah.'
            : e.pesan,
        )
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
        <h1 className="text-judul font-bold text-teks-utama">Masuk</h1>
        <p className="mt-1 text-isi text-teks-sekunder">
          Selamat datang kembali. Ayo jualan lagi.
        </p>
      </div>

      <form onSubmit={kirim} className="flex flex-col gap-4" noValidate>
        <Kolom
          label="Nama pengguna"
          autoComplete="username"
          autoCapitalize="none"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          galat={kolomGalat.username}
          required
        />
        <Kolom
          label="Kata sandi"
          type="password"
          autoComplete="current-password"
          value={sandi}
          onChange={(e) => setSandi(e.target.value)}
          galat={kolomGalat.password}
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
          disabled={!username || !sandi}
        >
          Masuk
        </Tombol>
      </form>

      <p className="text-center text-label text-teks-sekunder">
        Belum punya akun?{' '}
        <Link to="/daftar" className="font-semibold text-utama underline underline-offset-2">
          Daftarkan usaha Anda
        </Link>
      </p>
    </div>
  )
}
