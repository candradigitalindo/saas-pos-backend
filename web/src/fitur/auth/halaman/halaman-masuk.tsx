import { useState } from 'react'
import { Link, useLocation, useNavigate } from 'react-router-dom'
import { UserRound } from 'lucide-react'
import { Kolom } from '@/bersama/ui/kolom'
import { KolomSandi } from '@/bersama/ui/kolom-sandi'
import { Tombol } from '@/bersama/ui/tombol'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { GALAT_SESI_HABIS, GalatAPI } from '@/lib/api-client'
import { tujuanSetelahMasuk } from '@/bersama/util/tujuan-masuk'

/**
 * Masuk ke toko.
 *
 * Tombol "Masuk" SELALU aktif dan berwarna penuh. Versi sebelumnya menonaktifkan
 * tombolnya sampai dua kolom terisi, sehingga layar pertama yang dilihat
 * setiap orang didominasi tombol abu-abu yang terbaca "rusak". Kolom kosong
 * kini dijawab dengan kalimat di bawah kolomnya (ui/01 §7), sama seperti form
 * lain di aplikasi.
 *
 * Kolom pertama menerima NAMA PENGGUNA ATAU EMAIL, dan menjelaskan asalnya.
 * Dulu labelnya hanya "Nama pengguna" tanpa keterangan apa pun, dan server
 * hanya menerima nama pengguna — orang yang mengetik email (biasanya lebih
 * diingat) selalu ditolak tanpa tahu sebabnya.
 */
export function HalamanMasuk() {
  const { masuk } = useSesi()
  const navigate = useNavigate()
  const lokasi = useLocation()

  const [username, setUsername] = useState('')
  const [sandi, setSandi] = useState('')
  const [galat, setGalat] = useState<string | null>(null)
  const [kolomGalat, setKolomGalat] = useState<Record<string, string>>({})
  const [mengirim, setMengirim] = useState(false)

  async function kirim(e: React.FormEvent) {
    e.preventDefault()
    setGalat(null)

    const kosong: Record<string, string> = {}
    if (!username.trim()) kosong.username = 'Nama pengguna atau email belum diisi.'
    if (!sandi) kosong.password = 'Kata sandi belum diisi.'
    setKolomGalat(kosong)
    if (Object.keys(kosong).length > 0) return

    setMengirim(true)
    try {
      await masuk(username.trim(), sandi)
      // Kembali ke halaman yang tadi hendak dibuka — mis. kasir berikutnya yang
      // masuk dari layar Ganti Shift langsung kembali ke sana.
      navigate(tujuanSetelahMasuk(lokasi.state), { replace: true })
    } catch (e) {
      if (e instanceof GalatAPI) {
        setKolomGalat(e.kolom)
        // 401 di layar masuk bukan "sesi habis" — itu salah nama/sandi.
        setGalat(
          e.status === 401 || e.pesan === GALAT_SESI_HABIS
            ? 'Nama pengguna/email atau kata sandi salah.'
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
      <div>
        <h1 className="text-judul font-bold text-teks-utama">Masuk ke toko Anda</h1>
        <p className="mt-1 text-isi text-teks-sekunder">
          Selamat datang kembali. Ayo jualan lagi.
        </p>
      </div>

      <form onSubmit={kirim} className="flex flex-col gap-4" noValidate>
        <Kolom
          label="Nama pengguna atau email"
          ikon={UserRound}
          bantuan="Yang dibuat saat mendaftar usaha. Staf toko: tanyakan ke pemilik."
          autoComplete="username"
          autoCapitalize="none"
          autoCorrect="off"
          spellCheck={false}
          autoFocus
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          galat={kolomGalat.username}
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

      <div className="flex items-center gap-3 text-keterangan text-teks-redup" aria-hidden>
        <span className="h-px flex-1 bg-garis" />
        Belum punya akun?
        <span className="h-px flex-1 bg-garis" />
      </div>

      <Tombol asChild jenis="kedua" lebarPenuh>
        <Link to="/daftar">Daftarkan usaha Anda — gratis</Link>
      </Tombol>
    </div>
  )
}
