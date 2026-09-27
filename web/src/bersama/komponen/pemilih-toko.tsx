import { useState } from 'react'
import * as Popover from '@radix-ui/react-popover'
import { Check, ChevronsUpDown, Store } from 'lucide-react'
import { LogoKasir } from '@/bersama/komponen/logo'
import { useToast } from '@/bersama/komponen/toast'
import { useSesi } from '@/bersama/hooks/use-sesi'
import { cn } from '@/bersama/util/cn'

/**
 * Kepala navigasi samping: logo, nama usaha, dan TOKO YANG SEDANG DIPAKAI —
 * sekaligus tempat berpindah toko bila cabangnya lebih dari satu.
 *
 * Dulu berpindah toko hanya bisa dari Pengaturan → Toko & Cabang, tiga
 * ketukan jauhnya, dan tidak ada satu pun tempat di layar yang menyebut toko
 * mana yang angkanya sedang ditampilkan. Pemilik dua cabang bisa membaca
 * "uang masuk hari ini" milik cabang yang salah tanpa menyadarinya. Dasbor
 * SaaS lazim menaruh pemilih ini di pojok kiri atas; di situ pula ia di sini.
 *
 * Pengguna satu toko (hampir semua kasir) melihat kepala yang sama tanpa
 * tombol — tidak ada pilihan yang pura-pura bisa dipilih.
 */
export function PemilihToko() {
  const { profil, tokoAktif, rincianToko, gantiToko } = useSesi()
  const toast = useToast()
  const [buka, setBuka] = useState(false)

  const daftar = profil?.outlets ?? []
  const namaUsaha = profil?.tenant.business_name ?? 'Usaha Saya'
  const namaToko = rincianToko?.name ?? namaUsaha
  // Toko pertama dinamai sama dengan usahanya saat mendaftar. Satu toko
  // bernama sama tidak perlu disebut dua kali — baris kedua lalu menyebut
  // nama produknya, seperti kepala navigasi SaaS pada umumnya.
  const sebutToko = daftar.length > 1 || namaToko !== namaUsaha

  const isi = (
    <>
      <LogoKasir className="h-9 w-9" />
      <span className="min-w-0 flex-1 text-left">
        <span className="block truncate text-label font-bold text-teks-utama">{namaUsaha}</span>
        {sebutToko ? (
          <span className="flex items-center gap-1 truncate text-keterangan text-teks-redup">
            <Store className="h-3.5 w-3.5 shrink-0" aria-hidden />
            <span className="truncate">{namaToko}</span>
          </span>
        ) : (
          <span className="block truncate text-keterangan text-teks-redup">Kasir UMKM</span>
        )}
      </span>
    </>
  )

  if (daftar.length <= 1) {
    return <div className="flex items-center gap-3 px-2 py-1">{isi}</div>
  }

  return (
    <Popover.Root open={buka} onOpenChange={setBuka}>
      <Popover.Trigger asChild>
        <button
          type="button"
          aria-label={`Toko yang dipakai: ${namaToko}. Ganti toko`}
          className={cn(
            'flex min-h-12 w-full items-center gap-3 rounded-kontrol px-2 py-1',
            'hover:bg-permukaan-2 focus-visible:outline focus-visible:outline-2 focus-visible:outline-utama',
            buka && 'bg-permukaan-2',
          )}
        >
          {isi}
          <ChevronsUpDown className="h-4 w-4 shrink-0 text-teks-redup" aria-hidden />
        </button>
      </Popover.Trigger>

      <Popover.Portal>
        <Popover.Content
          align="start"
          sideOffset={6}
          collisionPadding={12}
          className="gerak-lapis z-[60] w-[var(--radix-popover-trigger-width)] min-w-60 rounded-kontrol border border-garis bg-permukaan p-1 shadow-melayang"
        >
          <p className="px-3 pb-1 pt-2 text-keterangan font-semibold uppercase tracking-wide text-teks-redup">
            Pindah toko
          </p>
          <ul>
            {daftar.map((t) => {
              const aktif = t.id === tokoAktif
              return (
                <li key={t.id}>
                  <button
                    type="button"
                    aria-current={aktif ? 'true' : undefined}
                    onClick={() => {
                      setBuka(false)
                      if (aktif) return
                      gantiToko(t.id)
                      toast.berhasil(`Sekarang memakai ${t.name}`)
                    }}
                    className="flex min-h-12 w-full items-center gap-3 rounded-kontrol px-3 text-left text-label text-teks-utama hover:bg-permukaan-2"
                  >
                    <Store className="h-4 w-4 shrink-0 text-teks-redup" aria-hidden />
                    <span className="min-w-0 flex-1 truncate">{t.name}</span>
                    <Check
                      className={cn('h-4 w-4 shrink-0 text-utama', aktif ? 'opacity-100' : 'opacity-0')}
                      aria-hidden
                    />
                  </button>
                </li>
              )
            })}
          </ul>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}
