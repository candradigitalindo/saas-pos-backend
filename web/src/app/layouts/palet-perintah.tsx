import { useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import * as D from '@radix-ui/react-dialog'
import { Command } from 'cmdk'
import { CornerDownLeft, Lock, Search } from 'lucide-react'
import {
  AKSI_PALET,
  KELOMPOK_SAMPING,
  MENU_ATAS,
  MENU_BAWAH,
  saringMenu,
  type ItemMenu,
} from '@/app/navigasi'
import { useSesi } from '@/bersama/hooks/use-sesi'

/**
 * Palet perintah — Ctrl K (⌘ K di Mac), atau tombol "Cari…" di navigasi
 * samping.
 *
 * Pola yang dipakai aplikasi SaaS pada umumnya (Linear, Vercel, Stripe):
 * satu kotak untuk MELOMPAT ke halaman mana pun atau menjalankan aksi yang
 * sering dipakai, tanpa menelusuri menu. Isinya disaring izin yang sama
 * dengan menu, dan tiap tujuan membawa kata lain yang dipakai orang
 * (kataKunci): "piutang" menemukan Kasbon, "kulakan" menemukan barang masuk.
 *
 * Fitur yang terkunci paket tetap muncul, dengan gembok — halamannya
 * menjelaskan paket yang membukanya.
 */
export function PaletPerintah({
  buka,
  onBukaBerubah,
}: {
  buka: boolean
  onBukaBerubah: (b: boolean) => void
}) {
  const { boleh, punyaFitur } = useSesi()
  const navigate = useNavigate()

  const aksi = saringMenu(AKSI_PALET, boleh)
  const halaman = [
    ...saringMenu(MENU_ATAS, boleh),
    ...KELOMPOK_SAMPING.flatMap((k) => saringMenu(k.item, boleh)),
    ...saringMenu(MENU_BAWAH, boleh),
  ]

  function pergi(ke: string) {
    onBukaBerubah(false)
    navigate(ke)
  }

  const baris = (m: ItemMenu) => {
    const terkunci = !!m.fitur && !punyaFitur(m.fitur)
    return (
      <Command.Item
        key={m.ke}
        // Nilai unik per tujuan: dua label bisa mirip ("Stok", "Koreksi stok").
        value={`${m.label} ${m.ke}`}
        keywords={m.kataKunci}
        onSelect={() => pergi(m.ke)}
        className="group flex min-h-12 cursor-pointer items-center gap-3 rounded-kontrol px-3 text-isi text-teks-utama pointer-fine:min-h-10 data-[selected=true]:bg-permukaan-2"
      >
        <m.ikon className="h-[18px] w-[18px] shrink-0 text-teks-redup group-data-[selected=true]:text-utama" aria-hidden />
        <span className="min-w-0 flex-1 truncate">{m.label}</span>
        {terkunci && (
          <>
            <Lock className="h-4 w-4 shrink-0 text-teks-redup" aria-hidden />
            <span className="sr-only">(terkunci paket)</span>
          </>
        )}
        <CornerDownLeft
          className="h-4 w-4 shrink-0 text-teks-redup opacity-0 group-data-[selected=true]:opacity-100"
          aria-hidden
        />
      </Command.Item>
    )
  }

  return (
    <D.Root open={buka} onOpenChange={onBukaBerubah}>
      <D.Portal>
        <D.Overlay className="gerak-lapis fixed inset-0 z-50 bg-black/40" />
        <D.Content
          aria-describedby={undefined}
          className="gerak-lapis fixed inset-x-3 top-[12vh] z-50 mx-auto max-w-xl overflow-hidden rounded-dialog border border-garis bg-permukaan shadow-dialog"
        >
          <D.Title className="sr-only">Cari halaman atau aksi</D.Title>
          <Command label="Cari halaman atau aksi" loop filter={saringPalet}>
            <div className="flex items-center gap-3 border-b border-garis px-4">
              <Search className="h-5 w-5 shrink-0 text-teks-redup" aria-hidden />
              <Command.Input
                placeholder="Cari halaman atau aksi…"
                className="h-14 min-w-0 flex-1 bg-transparent text-isi text-teks-utama outline-none placeholder:text-teks-redup"
              />
              <kbd className="hidden rounded border border-garis px-1.5 py-0.5 text-keterangan text-teks-redup sm:inline">
                Esc
              </kbd>
            </div>
            <Command.List className="max-h-[min(60vh,420px)] overflow-y-auto overscroll-contain p-2">
              <Command.Empty className="px-3 py-8 text-center text-label text-teks-redup">
                Tidak ada halaman atau aksi yang cocok.
              </Command.Empty>
              {aksi.length > 0 && (
                <Command.Group
                  heading="Aksi cepat"
                  className="[&_[cmdk-group-heading]]:px-3 [&_[cmdk-group-heading]]:pb-1 [&_[cmdk-group-heading]]:pt-2 [&_[cmdk-group-heading]]:text-keterangan [&_[cmdk-group-heading]]:font-semibold [&_[cmdk-group-heading]]:text-teks-redup"
                >
                  {aksi.map(baris)}
                </Command.Group>
              )}
              <Command.Group
                heading="Halaman"
                className="[&_[cmdk-group-heading]]:px-3 [&_[cmdk-group-heading]]:pb-1 [&_[cmdk-group-heading]]:pt-2 [&_[cmdk-group-heading]]:text-keterangan [&_[cmdk-group-heading]]:font-semibold [&_[cmdk-group-heading]]:text-teks-redup"
              >
                {halaman.map(baris)}
              </Command.Group>
            </Command.List>
            <div className="hidden items-center gap-4 border-t border-garis px-4 py-2 text-keterangan text-teks-redup sm:flex">
              <span>↑↓ pilih</span>
              <span>Enter buka</span>
              <span>Esc tutup</span>
            </div>
          </Command>
        </D.Content>
      </D.Portal>
    </D.Root>
  )
}

/** Huruf kecil tanpa tanda aksen. */
function normal(s: string): string {
  return s.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase()
}

/**
 * Penyaring palet: cocok bila SETIAP kata yang diketik ada di label atau kata
 * kuncinya (sebagai potongan kata), dengan label di depan lebih diutamakan.
 *
 * Bukan pencocokan kabur bawaan cmdk, yang menerima huruf berserakan: "hutang"
 * ikut menemukan "Hitung Fisik" (h·u·t·n·g) — di palet yang dipakai orang
 * untuk mencari satu halaman, hasil tambahan seperti itu hanya derau.
 */
export function saringPalet(nilai: string, cari: string, kataKunci?: string[]): number {
  const kata = normal(cari).split(/\s+/).filter(Boolean)
  if (kata.length === 0) return 1
  const label = normal(nilai)
  const semua = `${label} ${normal((kataKunci ?? []).join(' '))}`
  if (!kata.every((k) => semua.includes(k))) return 0
  if (label.startsWith(kata[0] ?? '')) return 1
  return kata.every((k) => label.includes(k)) ? 0.8 : 0.6
}

/** Mac memakai ⌘, yang lain Ctrl — untuk label pintasan di tombol "Cari…". */
export function labelPintasan(): string {
  const mac = typeof navigator !== 'undefined' && /Mac|iPhone|iPad/.test(navigator.platform || navigator.userAgent)
  return mac ? '⌘K' : 'Ctrl K'
}

/**
 * Ctrl K / ⌘ K membuka (atau menutup) palet dari mana pun di dalam aplikasi —
 * termasuk saat kursor di kotak isian: pintasan ini tidak bentrok dengan
 * mengetik biasa, dan aplikasi SaaS lain memperlakukannya sama.
 */
export function usePintasanPalet(ubah: (f: (b: boolean) => boolean) => void) {
  useEffect(() => {
    function tekan(e: KeyboardEvent) {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        ubah((b) => !b)
      }
    }
    window.addEventListener('keydown', tekan)
    return () => window.removeEventListener('keydown', tekan)
  }, [ubah])
}
