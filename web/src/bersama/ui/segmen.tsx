import { cn } from '@/bersama/util/cn'

/**
 * Kontrol segmen: sekelompok pilihan saling-tolak dalam satu wadah.
 *
 * Bentuknya sengaja "tertanam": wadah berlatar `permukaan-2`, dan yang terpilih
 * naik sebagai kepingan `permukaan` berbayang tipis. Pilihan mengambang
 * satu-satu dengan garis tepi masing-masing terbaca sebagai lima tombol yang
 * kebetulan berdekatan; wadah membuatnya terbaca sebagai SATU kendali dengan
 * satu nilai — dan itu memang yang sebenarnya terjadi.
 *
 * Menggeser mendatar, tidak membungkus ke baris kedua. Pilihan yang membungkus
 * membuat tinggi halaman melompat saat labelnya berubah, dan di HP baris kedua
 * mendorong isi halaman turun tanpa alasan yang terlihat.
 *
 * Pasangan warnanya bukan pilihan bebas: `teks-sekunder` dan `utama` di atas
 * `permukaan-2` adalah pasangan yang sudah diukur di audit kontras.
 */
export function SegmenPilihan<T extends string>({
  pilihan,
  nilai,
  onPilih,
  label,
  className,
}: {
  pilihan: readonly (readonly [T, string])[]
  nilai: T
  onPilih: (v: T) => void
  /** Dibacakan pembaca layar sebagai nama kelompoknya. */
  label: string
  className?: string
}) {
  return (
    <div
      role="group"
      aria-label={label}
      className={cn(
        // `w-fit` supaya wadahnya memeluk pilihannya. Sebagai anak blok biasa
        // ia akan melar selebar induk dan jadi pil abu-abu raksasa yang
        // sebagian besar kosong; `max-w-full` menjaga ia tetap bisa digeser
        // saat pilihannya memang lebih lebar dari layar.
        'flex w-fit max-w-full gap-1 overflow-x-auto rounded-full bg-permukaan-2 p-1',
        // Bilah geser disembunyikan: di dalam kendali sekecil ini ia lebih
        // mirip cacat daripada petunjuk.
        '[-ms-overflow-style:none] [scrollbar-width:none] [&::-webkit-scrollbar]:hidden',
        className,
      )}
    >
      {pilihan.map(([v, teks]) => (
        <button
          key={v}
          type="button"
          onClick={() => onPilih(v)}
          aria-pressed={nilai === v}
          className={cn(
            // 48px untuk jari, menyusut ke 40px hanya pada penunjuk halus —
            // aturan yang sama dengan Tombol varian "padat" (ui/01 §3).
            'h-12 shrink-0 whitespace-nowrap rounded-full px-4 text-label font-semibold',
            'pointer-fine:h-10',
            'transition-colors focus-visible:outline-none focus-visible:ring-2',
            'focus-visible:ring-utama focus-visible:ring-offset-1',
            nilai === v
              ? 'bg-permukaan text-utama shadow-kartu'
              : 'text-teks-sekunder hover:text-teks-utama',
          )}
        >
          {teks}
        </button>
      ))}
    </div>
  )
}
