import { SegmenPilihan } from '@/bersama/ui/segmen'
import { KolomUang } from '@/bersama/ui/kolom-uang'
import { Kolom } from '@/bersama/ui/kolom'

export type JenisDiskon = 'nominal' | 'persen'

/** Isian diskon yang sedang diketik: jenis + angka mentah. */
export interface NilaiDiskon {
  jenis: JenisDiskon
  /** Rupiah bulat, atau persen bulat 0–100. */
  nilai: number
}

/**
 * Kolom diskon Rp / %. Dipakai dialog atur baris dan dialog diskon transaksi.
 *
 * Persen dibatasi bilangan bulat — promo toko praktis selalu 5, 10, 15 —
 * dengan tombol cepat untuk yang paling umum; angka lain tetap bisa diketik.
 */
export function KolomDiskon({
  nilai,
  onNilai,
  bantuan,
  galat,
  label = 'Diskon',
}: {
  nilai: NilaiDiskon
  onNilai: (n: NilaiDiskon) => void
  bantuan?: string
  galat?: string
  label?: string
}) {
  // Pemilih jenis menumpang di baris label — tidak menambah baris.
  const segmen = (
    <SegmenPilihan
      label="Jenis diskon"
      pilihan={[
        ['nominal', 'Rp'],
        ['persen', '%'],
      ]}
      nilai={nilai.jenis}
      // Berganti jenis mengosongkan angka: 10 rupiah dan 10 persen bukan hal
      // yang sama, dan membawanya diam-diam mudah terlewat.
      onPilih={(jenis) => onNilai({ jenis, nilai: 0 })}
    />
  )
  return (
    <div className="flex flex-col gap-2">
      {nilai.jenis === 'nominal' ? (
        <KolomUang
          label={label}
          labelKanan={segmen}
          nilai={nilai.nilai}
          onNilai={(n) => onNilai({ jenis: 'nominal', nilai: n })}
          bantuan={bantuan}
          galat={galat}
        />
      ) : (
        <>
          <Kolom
            label={label}
            labelKanan={segmen}
            inputMode="numeric"
            akhiran="%"
            value={nilai.nilai === 0 ? '' : String(nilai.nilai)}
            onChange={(e) => {
              const angka = Number.parseInt(e.target.value.replace(/\D/g, '').slice(0, 3), 10)
              onNilai({ jenis: 'persen', nilai: Number.isFinite(angka) ? angka : 0 })
            }}
            placeholder="0"
            bantuan={bantuan}
            galat={galat}
          />
          <div className="flex gap-2">
            {[5, 10, 15, 20].map((p) => (
              <button
                key={p}
                type="button"
                onClick={() => onNilai({ jenis: 'persen', nilai: p })}
                aria-pressed={nilai.nilai === p}
                className="h-11 min-w-14 rounded-kontrol border border-garis px-3 text-label font-semibold tabular-nums text-teks-utama hover:bg-permukaan-2 aria-pressed:border-utama aria-pressed:bg-sorot"
              >
                {p}%
              </button>
            ))}
          </div>
        </>
      )}
    </div>
  )
}

/** Galat isian diskon, atau undefined bila sah. `batas` = rupiah maksimum. */
export function galatDiskon(n: NilaiDiskon, batas: number): string | undefined {
  if (n.nilai < 0) return 'Diskon tidak boleh minus.'
  if (n.jenis === 'persen' && n.nilai > 100) return 'Diskon paling banyak 100%.'
  if (n.jenis === 'nominal' && n.nilai > batas) return 'Diskon melebihi nilai belanja.'
  return undefined
}
