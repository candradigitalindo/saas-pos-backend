package tests

import (
	"encoding/json"
	"strconv"
	"testing"

	"candra/backend-api/database"
)

// Uji integrasi Fase 13 — absensi & penggajian (§5.11, §13.6).

// TestPayrollDeterministicCycle — DoD Fase 13: satu periode dihitung → dikunci →
// dibayar; hitung ulang dari data yang sama = angka identik; koreksi terlambat
// jadi penyesuaian periode berikutnya.
func TestPayrollDeterministicCycle(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "payroll") // outlet Asia/Jakarta

	// Karyawan bulanan Rp6.000.000, gabung 1 Agu 2026.
	emp := call(t, "POST", "/api/v1/employees", f.token, map[string]any{
		"outlet_id": f.outletID, "full_name": "Budi", "wage_type": "monthly",
		"base_wage": 6000000, "joined_at": "2026-08-01",
	}).mustCode(t, "buat karyawan", 201).data(t)["id"].(string)

	// Jadwal: Senin kerja 08:00–17:00, toleransi telat 10 menit, istirahat 60.
	call(t, "POST", "/api/v1/employees/"+emp+"/schedule", f.token, map[string]any{
		"weekday": 1, "start_time": "08:00", "end_time": "17:00",
		"late_tolerance_minutes": 10, "break_minutes": 60, "effective_from": "2026-08-01",
	}).mustCode(t, "set jadwal", 201)

	att := func(day, hhmm, kind string) {
		call(t, "POST", "/api/v1/attendances", f.token, map[string]any{
			"employee_id": emp, "kind": kind,
			"occurred_at": "2026-09-" + day + "T" + hhmm + ":00+07:00",
		}).mustCode(t, "absen "+day+" "+kind, 200)
	}
	// Senin 7: hadir. Senin 14: telat 30 mnt. Senin 21: hadir. Senin 28: alpa (tanpa absen).
	att("07", "08:05", "in")
	att("07", "17:00", "out")
	att("14", "08:30", "in")
	att("14", "17:00", "out")
	att("21", "08:00", "in")
	att("21", "17:00", "out")

	// Aturan gaji.
	mkRule := func(code, name, typ, cat, params string) {
		call(t, "POST", "/api/v1/payroll-rules", f.token, map[string]any{
			"code": code, "name": name, "type": typ, "category": cat,
			"params": json.RawMessage(params), "effective_from": "2026-01-01",
		}).mustCode(t, "aturan "+code, 201)
	}
	mkRule("TRANSPORT", "Transport", "tunjangan_tetap", "earning", `{"amount":20000,"per":"day","require_present":true}`)
	mkRule("TELAT", "Potongan telat", "potongan_telat", "deduction", `{"threshold_minutes":10,"mode":"per_event","amount":25000}`)
	mkRule("ALPA", "Potongan alpa", "potongan_alpa", "deduction", `{"mode":"amount","amount":200000}`)
	mkRule("KASBON", "Potongan kasbon", "potongan_kasbon", "deduction", `{}`)

	// Kasbon 500rb, cicilan 100rb, dicairkan.
	adv := call(t, "POST", "/api/v1/employee-advances", f.token, map[string]any{
		"employee_id": emp, "amount": 500000, "installment_amount": 100000,
	}).mustCode(t, "ajukan kasbon", 201).data(t)["id"].(string)
	call(t, "POST", "/api/v1/employee-advances/"+adv+"/disburse", f.token, nil).mustCode(t, "cairkan kasbon", 200)

	// Periode September 2026.
	period := call(t, "POST", "/api/v1/payroll-periods", f.token, map[string]any{
		"period_type": "monthly", "start_date": "2026-09-01", "end_date": "2026-09-30",
	}).mustCode(t, "buat periode", 201).data(t)["id"].(string)

	// Hitung.
	//   gross      = 6.000.000 (upah) + 60.000 (transport 3 hari) = 6.060.000
	//   deduction  = 25.000 (telat) + 200.000 (alpa) + 100.000 (kasbon) = 325.000
	//   net        = 5.735.000
	calc1 := call(t, "POST", "/api/v1/payroll-periods/"+period+"/calculate", f.token, nil).
		mustOK(t, "hitung 1").data(t)
	assertI64(t, calc1, "total_gross", 6060000)
	assertI64(t, calc1, "total_deduction", 325000)
	assertI64(t, calc1, "total_net", 5735000)

	ps1 := call(t, "GET", "/api/v1/payroll-periods/"+period+"/payslips", f.token, nil).
		mustOK(t, "slip").Body["data"].([]any)[0].(map[string]any)
	assertI64(t, ps1, "net_amount", 5735000)
	lineSig1 := lineSignature(ps1)

	// Hitung ULANG → angka & baris identik.
	call(t, "POST", "/api/v1/payroll-periods/"+period+"/calculate", f.token, nil).mustOK(t, "hitung 2")
	ps2 := call(t, "GET", "/api/v1/payroll-periods/"+period+"/payslips", f.token, nil).mustOK(t, "slip 2").Body["data"].([]any)[0].(map[string]any)
	assertI64(t, ps2, "net_amount", 5735000)
	if lineSignature(ps2) != lineSig1 {
		t.Fatalf("hitung ulang menghasilkan baris berbeda:\n%s\nvs\n%s", lineSig1, lineSignature(ps2))
	}

	// Sisa kasbon tetap 400.000 setelah hitung ulang (cicilan tidak dobel).
	var remaining int64
	database.DB.Raw(`SELECT remaining FROM employee_advances WHERE id = ?`, adv).Row().Scan(&remaining)
	if remaining != 400000 {
		t.Fatalf("sisa kasbon = %d, mau 400000", remaining)
	}

	// Kunci.
	lk := call(t, "POST", "/api/v1/payroll-periods/"+period+"/lock", f.token, nil).mustOK(t, "kunci").data(t)
	if lk["status"] != "locked" {
		t.Fatalf("status = %v, mau locked", lk["status"])
	}
	// Hitung ulang periode terkunci → 409.
	call(t, "POST", "/api/v1/payroll-periods/"+period+"/calculate", f.token, nil).mustCode(t, "hitung setelah kunci", 409)

	// Koreksi absensi Senin 14 (08:30 → 08:05): karena September terkunci,
	// penyetuju melampirkan penyesuaian +25.000 (refund potongan telat) untuk
	// periode berikutnya.
	att14In := call(t, "GET", "/api/v1/attendances?employee_id="+emp+"&business_date=2026-09-14", f.token, nil).
		mustOK(t, "absen 14").data(t)
	var att14ID string
	for _, a := range att14In["data"].([]any) {
		m := a.(map[string]any)
		if m["kind"] == "in" {
			att14ID = m["id"].(string)
		}
	}
	corr := call(t, "POST", "/api/v1/attendance-corrections", f.token, map[string]any{
		"attendance_id": att14ID, "new_occurred_at": "2026-09-14T08:05:00+07:00",
		"reason": "mesin absen error",
	}).mustCode(t, "ajukan koreksi", 201).data(t)["id"].(string)
	call(t, "POST", "/api/v1/attendance-corrections/"+corr+"/approve", f.token, map[string]any{
		"adjustment": map[string]any{"category": "earning", "amount": 25000, "name": "Koreksi potongan telat 14 Sep"},
	}).mustOK(t, "setujui koreksi")

	// Bayar September.
	pay := call(t, "POST", "/api/v1/payroll-periods/"+period+"/pay", f.token, nil).mustOK(t, "bayar").data(t)
	if pay["status"] != "paid" {
		t.Fatalf("status = %v, mau paid", pay["status"])
	}
	var cmCount int
	database.DB.Raw(`SELECT count(*) FROM cash_movements WHERE ref_table = 'payroll_periods' AND ref_id = ?`, period).Row().Scan(&cmCount)
	if cmCount != 1 {
		t.Fatalf("cash_movements gaji = %d, mau 1", cmCount)
	}

	// Periode Oktober: penyesuaian +25.000 muncul di slip.
	oct := call(t, "POST", "/api/v1/payroll-periods", f.token, map[string]any{
		"period_type": "monthly", "start_date": "2026-10-01", "end_date": "2026-10-31",
	}).mustCode(t, "buat periode okt", 201).data(t)["id"].(string)
	octCalc := call(t, "POST", "/api/v1/payroll-periods/"+oct+"/calculate", f.token, nil).mustOK(t, "hitung okt").data(t)
	// gross Oktober = 6.000.000 (upah, tanpa transport karena tak ada hadir) + 25.000 (penyesuaian)
	assertI64(t, octCalc, "total_gross", 6025000)

	octSlip := call(t, "GET", "/api/v1/payroll-periods/"+oct+"/payslips", f.token, nil).mustOK(t, "slip okt").Body["data"].([]any)[0].(map[string]any)
	foundAdj := false
	for _, l := range octSlip["lines"].([]any) {
		m := l.(map[string]any)
		if m["rule_type"] == "adjustment" && int64(m["amount"].(float64)) == 25000 {
			foundAdj = true
		}
	}
	if !foundAdj {
		t.Fatalf("slip Oktober tidak memuat baris penyesuaian 25.000: %v", octSlip["lines"])
	}
}

// lineSignature merangkai (name|category|amount) tiap baris slip untuk
// perbandingan determinisme.
func lineSignature(payslip map[string]any) string {
	out := ""
	for _, l := range payslip["lines"].([]any) {
		m := l.(map[string]any)
		out += m["name"].(string) + "|" + m["category"].(string) + "|"
		out += strconv.FormatInt(int64(m["amount"].(float64)), 10) + "\n"
	}
	return out
}

// TestAttendanceBusinessDateServerComputed — business_date dihitung server dari
// zona outlet, bukan diambil dari perangkat.
func TestAttendanceBusinessDateServerComputed(t *testing.T) {
	requireDB(t)
	f := setupPOS(t, "attbd")
	emp := call(t, "POST", "/api/v1/employees", f.token, map[string]any{
		"outlet_id": f.outletID, "full_name": "Sari", "wage_type": "daily",
		"base_wage": 150000, "joined_at": "2026-01-01",
	}).mustCode(t, "karyawan", 201).data(t)["id"].(string)

	// 23:30 UTC 8 Sep = 06:30 WIB 9 Sep → business_date 2026-09-09.
	a := call(t, "POST", "/api/v1/attendances", f.token, map[string]any{
		"employee_id": emp, "kind": "in", "occurred_at": "2026-09-08T23:30:00Z",
	}).mustOK(t, "absen").data(t)
	if a["business_date"] != "2026-09-09" {
		t.Fatalf("business_date = %v, mau 2026-09-09 (zona outlet WIB)", a["business_date"])
	}
}

// TestPayrollPermission — /employees butuh hr.employee.*; /payslips butuh hr.salary.view.
func TestPayrollPermission(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "hrperm")
	kasir := staffToken(t, f, roleID(t, f, "Kasir"), "kasir_hrperm")

	call(t, "GET", "/api/v1/employees", kasir, nil).mustCode(t, "kasir lihat karyawan", 403)
	call(t, "POST", "/api/v1/payroll-periods", kasir, map[string]any{
		"period_type": "monthly", "start_date": "2026-09-01", "end_date": "2026-09-30",
	}).mustCode(t, "kasir buat periode", 403)
	call(t, "GET", "/api/v1/payslips/01ARZ3NDEKTSV4RRFFQ69G5FAV", kasir, nil).mustCode(t, "kasir lihat slip", 403)
}

// TestPayrollIsolation — tenant B tak melihat karyawan / periode tenant A.
func TestPayrollIsolation(t *testing.T) {
	requireDB(t)
	a := setupPOS(t, "hrisoA")
	b := registerTenant(t, "hrisoB")

	empA := call(t, "POST", "/api/v1/employees", a.token, map[string]any{
		"outlet_id": a.outletID, "full_name": "A", "wage_type": "monthly", "base_wage": 1, "joined_at": "2026-01-01",
	}).mustCode(t, "karyawan A", 201).data(t)["id"].(string)

	call(t, "GET", "/api/v1/employees/"+empA, b.token, nil).mustCode(t, "B baca karyawan A", 404)
	if n := len(call(t, "GET", "/api/v1/employees", b.token, nil).mustOK(t, "karyawan B").data(t)["data"].([]any)); n != 0 {
		t.Fatalf("tenant B melihat %d karyawan tenant A", n)
	}
}

// TestMultipleEmployeesWithoutNumber — dua karyawan boleh didaftarkan tanpa
// nomor karyawan.
//
// `employee_no` terkena partial unique index `WHERE employee_no IS NOT NULL`.
// Ketika kolomnya bertipe string biasa, nomor yang dikosongkan tersimpan
// sebagai "" — bukan NULL — sehingga ikut terjaring indeks itu dan karyawan
// KEDUA ditolak: "nomor karyawan atau akun sudah terpakai". Pesannya
// membingungkan karena pemilik memang tidak pernah mengisi nomor apa pun, dan
// layar Karyawan di aplikasi tidak mengirimkannya sama sekali.
//
// Artinya modul SDM praktis mentok di karyawan pertama untuk setiap usaha baru.
func TestMultipleEmployeesWithoutNumber(t *testing.T) {
	requireDB(t)
	f := registerTenant(t, "empnonum")

	buat := func(nama string) string {
		return call(t, "POST", "/api/v1/employees", f.token, map[string]any{
			"outlet_id": f.outletID, "full_name": nama, "wage_type": "monthly",
			"base_wage": 3000000, "joined_at": "2026-08-01",
		}).mustCode(t, "buat karyawan "+nama, 201).data(t)["id"].(string)
	}

	a := buat("Budi Santoso")
	b := buat("Ahmad Fauzi") // dulu gagal 409 di sini
	c := buat("Siti Aminah")

	if a == b || b == c || a == c {
		t.Fatal("id karyawan tidak unik")
	}

	list := call(t, "GET", "/api/v1/employees?limit=100", f.token, nil).
		mustOK(t, "daftar karyawan").data(t)["data"].([]any)
	if len(list) != 3 {
		t.Fatalf("karyawan tersimpan = %d, mau 3", len(list))
	}
	// Nomor yang dikosongkan tetap tampil sebagai kosong ke klien.
	for _, r := range list {
		if no, ada := r.(map[string]any)["employee_no"]; ada && no != nil && no != "" {
			t.Fatalf("employee_no = %v, mau kosong", no)
		}
	}

	// Nomor yang BENAR-BENAR diisi tetap wajib unik.
	call(t, "POST", "/api/v1/employees", f.token, map[string]any{
		"outlet_id": f.outletID, "full_name": "Dewi", "wage_type": "monthly",
		"base_wage": 3000000, "joined_at": "2026-08-01", "employee_no": "K-001",
	}).mustCode(t, "karyawan bernomor", 201)
	call(t, "POST", "/api/v1/employees", f.token, map[string]any{
		"outlet_id": f.outletID, "full_name": "Eka", "wage_type": "monthly",
		"base_wage": 3000000, "joined_at": "2026-08-01", "employee_no": "K-001",
	}).mustCode(t, "nomor ganda harus ditolak", 409)
}
