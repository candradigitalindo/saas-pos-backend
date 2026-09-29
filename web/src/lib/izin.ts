/**
 * Kode izin, disalin dari database/permissions_catalog.go.
 *
 * Ditulis sebagai konstanta supaya salah ketik ("sale.viod") ketahuan saat
 * menulis kode, bukan saat kasir sedang melayani pembeli.
 */
export const IZIN = {
  // Penjualan
  saleCreate: 'sale.create',
  saleVoid: 'sale.void',
  saleRefund: 'sale.refund',
  saleDiscount: 'sale.discount',
  salePriceOverride: 'sale.price_override',

  // Produk
  productView: 'product.view',
  productEdit: 'product.edit',
  productDelete: 'product.delete',
  productImport: 'product.import',

  // Stok
  stockView: 'stock.view',
  stockAdjust: 'stock.adjust',
  stockOpname: 'stock.opname',
  stockTransfer: 'stock.transfer',

  // Laporan
  reportView: 'report.view',
  reportProfit: 'report.profit',
  reportExport: 'report.export',

  // Shift & kas
  shiftOpen: 'shift.open',
  shiftClose: 'shift.close',
  shiftReconcile: 'shift.reconcile',
  cashMovement: 'cash.movement',

  // Pelanggan & piutang
  customerView: 'customer.view',
  customerEdit: 'customer.edit',
  receivableManage: 'receivable.manage',

  // Administrasi
  userManage: 'user.manage',
  roleManage: 'role.manage',
  outletManage: 'outlet.manage',
  settingManage: 'setting.manage',
  billingManage: 'billing.manage',

  // CRM
  crmLeadViewOwn: 'crm.lead.view.own',
  crmLeadViewAll: 'crm.lead.view.all',
  crmDealEdit: 'crm.deal.edit',
  crmVisitCheckin: 'crm.visit.checkin',
  crmCommissionView: 'crm.commission.view',

  // Dokumen penjualan
  quotationApprove: 'quotation.approve',
  invoiceIssue: 'invoice.issue',
  invoiceVoid: 'invoice.void',

  // Kanal online
  channelManage: 'channel.manage',
  channelOrderAccept: 'channel.order.accept',
  channelSettlementView: 'channel.settlement.view',

  // SDM & penggajian
  hrEmployeeView: 'hr.employee.view',
  hrEmployeeEdit: 'hr.employee.edit',
  hrAttendanceView: 'hr.attendance.view',
  hrAttendanceCorrect: 'hr.attendance.correct',
  hrLeaveRequest: 'hr.leave.request',
  hrLeaveApprove: 'hr.leave.approve',
  hrPayrollRun: 'hr.payroll.run',
  hrPayrollLock: 'hr.payroll.lock',
  hrPayrollPay: 'hr.payroll.pay',
  hrSalaryView: 'hr.salary.view',
  hrAdvanceApprove: 'hr.advance.approve',
} as const

export type KodeIzin = (typeof IZIN)[keyof typeof IZIN]
