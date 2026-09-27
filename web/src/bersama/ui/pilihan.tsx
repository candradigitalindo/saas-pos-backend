import {
  Children,
  Fragment,
  forwardRef,
  isValidElement,
  useId,
  useState,
  type ReactNode,
} from 'react'
import * as Popover from '@radix-ui/react-popover'
import { Command } from 'cmdk'
import { AlertCircle, Check, ChevronsUpDown, Search } from 'lucide-react'
import { cn } from '@/bersama/util/cn'

/**
 * Kolom pilihan DENGAN PENCARIAN — pengganti `<select>` bawaan peramban.
 *
 * `<select>` bawaan tidak bisa dicari: memilih satu satuan dari puluhan, satu
 * pelanggan dari ratusan, atau satu pemasok dari daftar panjang berarti
 * menggulir sambil membaca satu per satu. Kini setiap pilihan punya kotak
 * "Cari" di atas daftarnya.
 *
 * API-nya SAMA dengan `<select>` supaya seluruh pemakainya tidak perlu diubah:
 * `value`, `onChange={(e) => …e.target.value}`, dan anak berupa `<option>`.
 *
 * Kenapa Radix Popover + cmdk (pola "Combobox" shadcn/ui, tumpukan yang
 * ditetapkan ui/03): daftarnya dirender di portal, jadi tidak terpotong oleh
 * dialog bergulir tempat banyak formulir berada; cmdk mengurus navigasi papan
 * ketik (panah, Enter, Esc) dan atribut ARIA listbox.
 */

/** Satu opsi, disarikan dari anak `<option>`. */
interface Opsi {
  nilai: string
  label: string
  nonaktif: boolean
}

/** Bentuk event minimal yang dibaca pemakai: `e.target.value`. */
export interface EventPilihan {
  target: { value: string }
  currentTarget: { value: string }
}

export interface PropPilihan {
  label: string
  /** Kalimat bantuan singkat di bawah kolom. */
  bantuan?: string
  galat?: string
  value?: string | number
  onChange?: (e: EventPilihan) => void
  /** Anak `<option value="…">Label</option>`, seperti `<select>`. */
  children?: ReactNode
  required?: boolean
  disabled?: boolean
  id?: string
  className?: string
  /** Teks tombol bila `value` tidak cocok dengan opsi mana pun. */
  placeholder?: string
}

/** Teks polos dari isi `<option>` — boleh berupa gabungan string & angka. */
function teksDari(node: ReactNode): string {
  if (node == null || typeof node === 'boolean') return ''
  if (typeof node === 'string' || typeof node === 'number') return String(node)
  if (Array.isArray(node)) return node.map(teksDari).join('')
  if (isValidElement(node)) return teksDari((node.props as { children?: ReactNode }).children)
  return ''
}

/** Mengumpulkan `<option>` (juga di dalam fragmen / `<optgroup>`) menjadi Opsi. */
function kumpulkanOpsi(children: ReactNode): Opsi[] {
  const keluar: Opsi[] = []
  Children.forEach(children, (anak) => {
    if (!isValidElement(anak)) return
    const props = anak.props as { value?: string | number; children?: ReactNode; disabled?: boolean }
    if (anak.type === 'option') {
      const label = teksDari(props.children)
      keluar.push({
        nilai: props.value === undefined ? label : String(props.value),
        label,
        nonaktif: !!props.disabled,
      })
    } else if (anak.type === Fragment || anak.type === 'optgroup') {
      keluar.push(...kumpulkanOpsi(props.children))
    }
  })
  return keluar
}

/** Huruf kecil tanpa tanda aksen: "Café" dicari dengan "cafe". */
function normal(s: string): string {
  return s.normalize('NFD').replace(/\p{Diacritic}/gu, '').toLowerCase()
}

/** Opsi yang labelnya memuat `teks` (tanpa peduli huruf besar & aksen). */
function saring(opsi: Opsi[], teks: string): Opsi[] {
  const kunci = normal(teks.trim())
  return kunci ? opsi.filter((o) => normal(o.label).includes(kunci)) : opsi
}

/** Nilai item cmdk. Diberi awalan supaya opsi bernilai "" tetap punya kunci unik. */
const kunciItem = (nilai: string) => `opsi:${nilai}`

export const Pilihan = forwardRef<HTMLButtonElement, PropPilihan>(function Pilihan(
  {
    label,
    bantuan,
    galat,
    value,
    onChange,
    children,
    required,
    disabled,
    id,
    className,
    placeholder = 'Pilih…',
  },
  ref,
) {
  const otomatis = useId()
  const idKolom = id ?? otomatis
  const idDaftar = `${idKolom}-daftar`
  const idBantuan = `${idKolom}-bantuan`
  const idGalat = `${idKolom}-galat`

  const [buka, setBuka] = useState(false)
  const [cari, setCari] = useState('')
  const [sorot, setSorot] = useState('')

  const opsi = kumpulkanOpsi(children)
  const nilai = value === undefined ? '' : String(value)
  const terpilih = opsi.find((o) => o.nilai === nilai)

  // Penyaringan sendiri (bukan skor bawaan cmdk): urutan daftar tetap seperti
  // yang disusun pemakainya — mis. "Tanpa pemasok" tetap paling atas.
  const tersaring = saring(opsi, cari)

  function ubahBuka(o: boolean) {
    setBuka(o)
    setCari('')
    // Saat dibuka, sorotan papan ketik mulai dari opsi yang sedang terpilih.
    if (o) setSorot(kunciItem(terpilih?.nilai ?? opsi[0]?.nilai ?? ''))
  }

  function pilih(v: string) {
    ubahBuka(false)
    if (v !== nilai) onChange?.({ target: { value: v }, currentTarget: { value: v } })
  }

  return (
    <div className="flex flex-col gap-1.5">
      <label htmlFor={idKolom} className="text-label font-medium text-teks-sekunder">
        {label}
        {required && <span className="text-bahaya-teks"> *</span>}
      </label>

      <Popover.Root open={buka} onOpenChange={ubahBuka}>
        <Popover.Trigger asChild>
          <button
            ref={ref}
            id={idKolom}
            type="button"
            role="combobox"
            aria-expanded={buka}
            aria-controls={idDaftar}
            aria-haspopup="listbox"
            aria-invalid={!!galat}
            aria-describedby={cn(galat ? idGalat : undefined, bantuan ? idBantuan : undefined)}
            disabled={disabled}
            className={cn(
              'flex h-12 w-full items-center gap-2 rounded-kontrol border bg-permukaan px-3 text-left text-isi',
              'focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-utama',
              'disabled:cursor-not-allowed disabled:opacity-60',
              galat ? 'border-bahaya' : 'border-garis',
              className,
            )}
          >
            <span className={cn('min-w-0 flex-1 truncate', terpilih ? 'text-teks-utama' : 'text-teks-redup')}>
              {terpilih?.label ?? placeholder}
            </span>
            <ChevronsUpDown className="h-4 w-4 shrink-0 text-teks-redup" aria-hidden />
          </button>
        </Popover.Trigger>

        <Popover.Portal>
          <Popover.Content
            align="start"
            sideOffset={4}
            collisionPadding={12}
            // z-[60]: di atas dialog (z-50) — banyak pilihan hidup di dalam dialog.
            className="gerak-lapis z-[60] w-[var(--radix-popover-trigger-width)] min-w-56 overflow-hidden rounded-kontrol border border-garis bg-permukaan shadow-melayang"
          >
            {/* `label` milik Command menjadi nama aksesibel KOTAK CARI (cmdk
                menautkannya lewat aria-labelledby, menimpa aria-label) — jadi
                ditulis "Cari …", supaya pembaca layar tidak mendengar dua kendali
                bernama sama dengan tombol pemicunya. */}
            <Command
              shouldFilter={false}
              label={`Cari ${label.toLowerCase()}`}
              loop
              value={sorot}
              onValueChange={setSorot}
            >
              <div className="flex items-center gap-2 border-b border-garis px-3">
                <Search className="h-4 w-4 shrink-0 text-teks-redup" aria-hidden />
                <Command.Input
                  value={cari}
                  onValueChange={(v) => {
                    setCari(v)
                    // Sorotan pindah ke hasil pertama supaya Enter langsung
                    // memilihnya; kotak cari dikosongkan → kembali ke yang terpilih.
                    const pertama = v.trim() ? saring(opsi, v)[0] : (terpilih ?? opsi[0])
                    setSorot(kunciItem(pertama?.nilai ?? ''))
                  }}
                  placeholder="Cari…"
                  className="h-12 min-w-0 flex-1 bg-transparent text-isi text-teks-utama outline-none placeholder:text-teks-redup"
                />
              </div>
              <Command.List
                id={idDaftar}
                label={label}
                className="max-h-72 overflow-y-auto overscroll-contain p-1"
              >
                <Command.Empty className="px-3 py-6 text-center text-label text-teks-redup">
                  Tidak ada yang cocok dengan &ldquo;{cari.trim()}&rdquo;.
                </Command.Empty>
                {tersaring.map((o) => (
                  <Command.Item
                    key={o.nilai}
                    value={kunciItem(o.nilai)}
                    disabled={o.nonaktif}
                    onSelect={() => pilih(o.nilai)}
                    className={cn(
                      'flex min-h-12 cursor-pointer items-center gap-2 rounded-kontrol px-3 text-isi text-teks-utama',
                      'data-[selected=true]:bg-permukaan-2',
                      'data-[disabled=true]:cursor-not-allowed data-[disabled=true]:opacity-50',
                    )}
                  >
                    <Check
                      className={cn('h-4 w-4 shrink-0 text-utama', o.nilai === nilai ? 'opacity-100' : 'opacity-0')}
                      aria-hidden
                    />
                    <span className="min-w-0 flex-1">{o.label}</span>
                  </Command.Item>
                ))}
              </Command.List>
            </Command>
          </Popover.Content>
        </Popover.Portal>
      </Popover.Root>

      {galat ? (
        <p id={idGalat} className="flex items-start gap-1 text-keterangan text-bahaya-teks">
          <AlertCircle className="mt-0.5 h-4 w-4 shrink-0" aria-hidden />
          {galat}
        </p>
      ) : (
        bantuan && (
          <p id={idBantuan} className="text-keterangan text-teks-redup">
            {bantuan}
          </p>
        )
      )}
    </div>
  )
})
