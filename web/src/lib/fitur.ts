/**
 * Kode fitur paket langganan (plans.features di backend,
 * services/plan_entitlement_service.go).
 *
 * Penegakannya di SERVER. Klien memakai kode-kode ini hanya untuk tidak
 * menawarkan yang pasti ditolak — tombol QRIS yang ditekan lalu dijawab
 * "butuh paket Basic" di tengah antrean jauh lebih buruk daripada tombol yang
 * sejak awal menjelaskan dirinya terkunci.
 */
export const FITUR = {
  qris: 'qris',
  kanalOnline: 'online_channel',
  crmFreelance: 'crm_freelance',
  salesLapangan: 'crm_sales',
  banyakCabang: 'multi_outlet',
} as const

export type KodeFitur = (typeof FITUR)[keyof typeof FITUR]
