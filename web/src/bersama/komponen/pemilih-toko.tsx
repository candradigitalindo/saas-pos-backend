import { useState } from 'react'
import * as Popover from '@radix-ui/react-popover'
import { Command } from 'cmdk'
import { Check, ChevronsUpDown, Search, Store } from 'lucide-react'
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
 *
 * Daftarnya bisa dicari seperti setiap pilihan lain di aplikasi (lihat
 * `Pilihan`): pemilik belasan cabang tidak perlu menggulir sambil membaca.
 */
export function PemilihToko({ ringkas = false }: { ringkas?: boolean }) {
  const { profil, tokoAktif, rincianToko, gantiToko } = useSesi()
  const toast = useToast()
  const [buka, setBuka] = useState(false)
  const [cari, setCari] = useState('')

  const daftar = profil?.outlets ?? []
  const namaUsaha = profil?.tenant.business_name ?? 'Usaha Saya'
  const namaToko = rincianToko?.name ?? namaUsaha
  // Toko pertama dinamai sama dengan usahanya saat mendaftar. Satu toko
  // bernama sama tidak perlu disebut dua kali — baris kedua lalu menyebut
  // nama produknya, seperti kepala navigasi SaaS pada umumnya.
  const sebutToko = daftar.length > 1 || namaToko !== namaUsaha

  // Ringkas (navigasi samping diciutkan): logo saja — nama usaha & toko
  // tetap dibacakan pembaca layar dan muncul sebagai petunjuk arahkan-kursor.
  const isi = ringkas ? (
    <>
      <LogoKasir className="h-9 w-9" />
      <span className="sr-only">
        {namaUsaha}
        {sebutToko ? `, ${namaToko}` : ''}
      </span>
    </>
  ) : (
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
    return (
      <div
        className={cn('flex items-center gap-3 py-1', ringkas ? 'justify-center' : 'px-2')}
        title={ringkas ? namaUsaha : undefined}
      >
        {isi}
      </div>
    )
  }

  return (
    <Popover.Root
      open={buka}
      onOpenChange={(o) => {
        setBuka(o)
        setCari('')
      }}
    >
      <Popover.Trigger asChild>
        <button
          type="button"
          aria-label={`Toko yang dipakai: ${namaToko}. Ganti toko`}
          title={ringkas ? `${namaToko} — ganti toko` : undefined}
          className={cn(
            'flex min-h-12 w-full items-center gap-3 rounded-kontrol py-1',
            ringkas ? 'justify-center' : 'px-2',
            'hover:bg-permukaan-2 focus-visible:outline focus-visible:outline-2 focus-visible:outline-utama',
            buka && 'bg-permukaan-2',
          )}
        >
          {isi}
          {!ringkas && <ChevronsUpDown className="h-4 w-4 shrink-0 text-teks-redup" aria-hidden />}
        </button>
      </Popover.Trigger>

      <Popover.Portal>
        <Popover.Content
          align="start"
          sideOffset={6}
          collisionPadding={12}
          side={ringkas ? 'right' : 'bottom'}
          className="gerak-lapis z-[60] w-[var(--radix-popover-trigger-width)] min-w-60 rounded-kontrol border border-garis bg-permukaan p-1 shadow-melayang"
        >
          <Command shouldFilter={false} label="Cari toko" loop>
            <div className="flex items-center gap-2 border-b border-garis px-3">
              <Search className="h-4 w-4 shrink-0 text-teks-redup" aria-hidden />
              <Command.Input
                value={cari}
                onValueChange={setCari}
                placeholder="Cari toko…"
                className="h-12 min-w-0 flex-1 bg-transparent text-isi text-teks-utama outline-none placeholder:text-teks-redup"
              />
            </div>
            <p className="px-3 pb-1 pt-2 text-keterangan font-semibold uppercase tracking-wide text-teks-redup">
              Pindah toko
            </p>
            <Command.List className="max-h-72 overflow-y-auto overscroll-contain">
              <Command.Empty className="px-3 py-6 text-center text-label text-teks-redup">
                Tidak ada toko bernama &ldquo;{cari.trim()}&rdquo;.
              </Command.Empty>
              {daftar
                .filter((t) => normal(t.name).includes(normal(cari.trim())))
                .map((t) => {
                  const aktif = t.id === tokoAktif
                  return (
                    <Command.Item
                      key={t.id}
                      value={t.id}
                      aria-current={aktif ? 'true' : undefined}
                      onSelect={() => {
                        setBuka(false)
                        if (aktif) return
                        gantiToko(t.id)
                        toast.berhasil(`Sekarang memakai ${t.name}`)
                      }}
                      className="flex min-h-12 cursor-pointer items-center gap-3 rounded-kontrol px-3 text-left text-label text-teks-utama data-[selected=true]:bg-permukaan-2"
                    >
                      <Store className="h-4 w-4 shrink-0 text-teks-redup" aria-hidden />
                      <span className="min-w-0 flex-1 truncate">{t.name}</span>
                      <Check
                        className={cn('h-4 w-4 shrink-0 text-utama', aktif ? 'opacity-100' : 'opacity-0')}
                        aria-hidden
                      />
                    </Command.Item>
                  )
                })}
            </Command.List>
          </Command>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}

/** Huruf kecil tanpa tanda aksen: "Café" dicari dengan "cafe". */
function normal(t: string): string {
  return t.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase()
}
