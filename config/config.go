package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

func LoadEnv() {
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: No .env file found, using system environment variables")
	}
}

// lookup mengambil nilai environment yang sudah dibersihkan dari spasi.
// Variabel yang di-set tetapi bernilai kosong (mis. baris "ALLOWED_ORIGINS=" di
// .env) dianggap TIDAK di-set, sehingga nilai default tetap dipakai.
//
// Tanpa perlakuan ini, baris kosong di .env akan menimpa default dengan string
// kosong — mis. strings.Split("", ",") menghasilkan []string{""} yang membuat
// konfigurasi CORS berisi satu origin kosong.
func lookup(key string) (string, bool) {
	value, ok := os.LookupEnv(key)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	return value, true
}

// GetEnv mengambil variabel environment sebagai string.
func GetEnv(key, defaultValue string) string {
	if value, ok := lookup(key); ok {
		return value
	}
	return defaultValue
}

// GetIntEnv mengambil variabel environment sebagai integer.
// Nilai default dipakai jika key tidak ditemukan atau nilainya bukan integer yang valid.
func GetIntEnv(key string, defaultValue int) int {
	valueStr, ok := lookup(key)
	if !ok {
		return defaultValue
	}

	valueInt, err := strconv.Atoi(valueStr)
	if err != nil {
		log.Printf("Peringatan: Nilai untuk env key '%s' bukan integer yang valid. Menggunakan nilai default %d.", key, defaultValue)
		return defaultValue
	}
	return valueInt
}

// GetBoolEnv mengambil variabel environment sebagai boolean.
// Menerima nilai yang dikenali strconv.ParseBool: 1/t/T/true/TRUE/True dan
// 0/f/F/false/FALSE/False.
func GetBoolEnv(key string, defaultValue bool) bool {
	valueStr, ok := lookup(key)
	if !ok {
		return defaultValue
	}

	valueBool, err := strconv.ParseBool(valueStr)
	if err != nil {
		log.Printf("Peringatan: Nilai untuk env key '%s' bukan boolean yang valid. Menggunakan nilai default %t.", key, defaultValue)
		return defaultValue
	}
	return valueBool
}

// GetStringSliceEnv mengambil variabel environment berisi daftar yang dipisah koma.
// Entri kosong dibuang dan setiap entri di-trim, sehingga nilai seperti
// "a, ,b," menghasilkan []string{"a", "b"} — bukan slice berisi string kosong.
func GetStringSliceEnv(key string, defaultValue []string) []string {
	value, ok := lookup(key)
	if !ok {
		return defaultValue
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			result = append(result, trimmed)
		}
	}
	if len(result) == 0 {
		return defaultValue
	}
	return result
}
