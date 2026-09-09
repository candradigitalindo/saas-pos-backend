import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import type { ReactNode } from 'react'
import { PenyediaSesi } from '@/bersama/hooks/use-sesi'
import { PenyediaToast } from '@/bersama/komponen/toast'
import { GalatAPI } from '@/lib/api-client'

/**
 * Pengaturan TanStack Query disetel untuk SINYAL JELEK, bukan untuk kantor
 * dengan wifi kencang:
 *   - coba ulang beberapa kali dengan jeda menaik;
 *   - JANGAN mengulang bila masalahnya izin atau data tidak ada — mengulang
 *     permintaan 403 lima kali hanya membuat layar diam lebih lama.
 */
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      refetchOnWindowFocus: false,
      retry: (gagal, e) => {
        if (e instanceof GalatAPI && [401, 403, 404, 422].includes(e.status)) return false
        return gagal < 3
      },
      retryDelay: (i) => Math.min(1000 * 2 ** i, 8000),
    },
    mutations: {
      // Mutasi TIDAK diulang otomatis: yang menciptakan uang atau stok diulang
      // secara sadar oleh pemanggil dengan Idempotency-Key yang sama.
      retry: false,
    },
  },
})

export function Penyedia({ children }: { children: ReactNode }) {
  return (
    <QueryClientProvider client={queryClient}>
      <PenyediaToast>
        <PenyediaSesi>{children}</PenyediaSesi>
      </PenyediaToast>
    </QueryClientProvider>
  )
}
