import { api, type Halaman } from '@/lib/api-client'

export interface Karyawan {
  id: string
  user_id?: string
  outlet_id: string
  employee_no?: string
  full_name: string
  phone?: string
  position?: string
  employment_status: string
  wage_type: 'monthly' | 'daily' | 'hourly'
  base_wage: number
  payroll_period_type: string
  joined_at: string
  resigned_at?: string
  is_active: boolean
}

export interface PeriodeGaji {
  id: string
  outlet_id?: string
  period_type: string
  start_date: string
  end_date: string
  /** draft → calculated → locked → paid */
  status: string
  total_gross: number
  total_deduction: number
  total_net: number
  calculated_at?: string
  locked_at?: string
  paid_at?: string
}

export interface BarisSlip {
  name: string
  category: string
  rule_type?: string
  basis_note?: string
  quantity?: string
  amount: number
  sort_order: number
}

export interface SlipGaji {
  id: string
  payroll_period_id: string
  employee_id: string
  gross_amount: number
  deduction_amount: number
  net_amount: number
  /** Sisa utang yang tidak tertutup gaji bulan ini, dibawa ke periode depan. */
  carried_debt: number
  present_days: string
  late_count: number
  absent_days: string
  leave_days: string
  overtime_minutes: number
  status: string
  lines?: BarisSlip[]
}

export interface Absensi {
  id: string
  employee_id: string
  work_date: string
  check_in_at?: string
  check_out_at?: string
  status: string
  late_minutes: number
  overtime_minutes: number
}

export const sdmApi = {
  // ── Karyawan ─────────────────────────────────────────────────────────────
  daftarKaryawan: (page = 1, limit = 100) =>
    api.get<Halaman<Karyawan>>('/employees', { query: { page, limit } }),

  karyawan: (id: string) => api.get<Karyawan>(`/employees/${id}`),

  buatKaryawan: (input: {
    outlet_id: string
    full_name: string
    wage_type: string
    base_wage?: number
    position?: string
    phone?: string
    joined_at: string
  }) => api.post<Karyawan>('/employees', input),

  ubahKaryawan: (id: string, input: Record<string, unknown>) =>
    api.put<Karyawan>(`/employees/${id}`, input),

  // ── Absensi ──────────────────────────────────────────────────────────────
  daftarAbsensi: (employee_id?: string, from?: string, to?: string) =>
    api.get<Halaman<Absensi>>('/attendances', {
      query: { employee_id, from, to, limit: 100 },
    }),

  catatAbsensi: (input: {
    employee_id: string
    work_date: string
    check_in_at?: string
    check_out_at?: string
    status?: string
  }) => api.post<Absensi>('/attendances', input),

  // ── Periode gaji: hitung → periksa → kunci → bayar ───────────────────────
  daftarPeriode: (page = 1, limit = 20) =>
    api.get<Halaman<PeriodeGaji>>('/payroll-periods', { query: { page, limit } }),

  buatPeriode: (input: {
    outlet_id?: string
    period_type: string
    start_date: string
    end_date: string
  }) => api.post<PeriodeGaji>('/payroll-periods', input),

  hitung: (id: string) => api.post<PeriodeGaji>(`/payroll-periods/${id}/calculate`, {}),
  kunci: (id: string) => api.post<PeriodeGaji>(`/payroll-periods/${id}/lock`, {}),
  bayar: (id: string) => api.post<PeriodeGaji>(`/payroll-periods/${id}/pay`, {}),

  slipPeriode: (id: string) =>
    api.get<Halaman<SlipGaji>>(`/payroll-periods/${id}/payslips`, { query: { limit: 100 } }),

  slip: (id: string) => api.get<SlipGaji>(`/payslips/${id}`),
}
