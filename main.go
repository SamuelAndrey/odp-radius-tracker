package main

import (
	"encoding/csv"
	"fmt"
	"html/template"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

type Point struct {
	Name      string
	Latitude  float64
	Longitude float64
}

type Result struct {
	Data1Name string
	Lat1      float64
	Lon1      float64
	Data2Name string
	Lat2      float64
	Lon2      float64
	Distance  float64
}

func main() {

	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))
	http.Handle("/uploads/", http.StripPrefix("/uploads/", http.FileServer(http.Dir("uploads"))))

	http.HandleFunc("/", serveIndex)

	http.HandleFunc("/upload", handleUpload)

	http.HandleFunc("/download", handleDownload)

	fmt.Println("Server berjalan di http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		fmt.Println("Error menjalankan server:", err)
	}
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, "Tidak dapat memuat template", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, nil)
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Hanya metode POST yang didukung", http.StatusMethodNotAllowed)
		return
	}

	// Ambil file yang diunggah
	data1File, data1Header, err1 := r.FormFile("data1")
	data2File, data2Header, err2 := r.FormFile("data2")
	radiusStr := r.FormValue("radius")

	if err1 != nil || err2 != nil {
		http.Error(w, "Gagal mengunggah file", http.StatusBadRequest)
		return
	}
	defer data1File.Close()
	defer data2File.Close()

	// Parsing radius
	radius, err := strconv.ParseFloat(radiusStr, 64)
	if err != nil {
		http.Error(w, "Radius tidak valid", http.StatusBadRequest)
		return
	}

	// Simpan file sementara
	data1Path := filepath.Join("uploads", data1Header.Filename)
	data2Path := filepath.Join("uploads", data2Header.Filename)
	saveFile(data1File, data1Path)
	saveFile(data2File, data2Path)

	// Proses file
	results, err := processFiles(data1Path, data2Path, radius)
	if err != nil {
		http.Error(w, "Gagal memproses file", http.StatusInternalServerError)
		fmt.Println("Error:", err)
		return
	}

	// Tulis hasil ke file
	outputFile := filepath.Join("uploads", "results.csv")
	if err := writeResults(outputFile, results); err != nil {
		http.Error(w, "Gagal menyimpan hasil", http.StatusInternalServerError)
		fmt.Println("Error:", err)
		return
	}

	// Tampilkan pesan sukses melalui template
	tmpl, err := template.ParseFiles("templates/index.html")
	if err != nil {
		http.Error(w, "Tidak dapat memuat template", http.StatusInternalServerError)
		return
	}
	tmpl.Execute(w, map[string]interface{}{
		"Processed":   true,
		"DownloadURL": "/download",
	})
}

func handleDownload(w http.ResponseWriter, r *http.Request) {

	outputFile := filepath.Join("uploads", "results.csv")

	if _, err := os.Stat(outputFile); os.IsNotExist(err) {
		http.Error(w, "File tidak ditemukan", http.StatusNotFound)
		return
	}

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	fileName := fmt.Sprintf("Result_%s.csv", timestamp)

	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileName))
	w.Header().Set("Content-Type", "application/octet-stream")

	http.ServeFile(w, r, outputFile)
}

func saveFile(file io.Reader, path string) {
	out, err := os.Create(path)
	if err != nil {
		fmt.Println("Error menyimpan file:", err)
		return
	}
	defer out.Close()
	io.Copy(out, file)
}

func processFiles(data1Path, data2Path string, maxRadius float64) ([]Result, error) {
	data1, err := loadCSV(data1Path)
	if err != nil {
		return nil, err
	}

	data2, err := loadCSV(data2Path)
	if err != nil {
		return nil, err
	}

	var results []Result
	for _, p1 := range data1 {
		for _, p2 := range data2 {
			distance := haversine(p1.Latitude, p1.Longitude, p2.Latitude, p2.Longitude)
			if distance <= maxRadius {
				results = append(results, Result{
					Data1Name: p1.Name,
					Lat1:      p1.Latitude,
					Lon1:      p1.Longitude,
					Data2Name: p2.Name,
					Lat2:      p2.Latitude,
					Lon2:      p2.Longitude,
					Distance:  distance,
				})
			}
		}
	}
	return results, nil
}

func loadCSV(filePath string) ([]Point, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	reader := csv.NewReader(file)
	rows, err := reader.ReadAll()
	if err != nil {
		return nil, err
	}

	points := make([]Point, len(rows)-1)
	for i, row := range rows[1:] {
		lat, _ := strconv.ParseFloat(row[1], 64)
		lon, _ := strconv.ParseFloat(row[2], 64)
		points[i] = Point{Name: row[0], Latitude: lat, Longitude: lon}
	}

	return points, nil
}

func writeResults(filePath string, results []Result) error {
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	writer.Write([]string{
		"data1_name", "lat1", "lon1", "data2_name", "lat2", "lon2", "distance_meters",
	})

	for _, result := range results {
		writer.Write([]string{
			result.Data1Name,
			fmt.Sprintf("%.6f", result.Lat1),
			fmt.Sprintf("%.6f", result.Lon1),
			result.Data2Name,
			fmt.Sprintf("%.6f", result.Lat2),
			fmt.Sprintf("%.6f", result.Lon2),
			fmt.Sprintf("%.2f", result.Distance),
		})
	}

	return nil
}

func haversine(lat1, lon1, lat2, lon2 float64) float64 {
	const R = 6371000
	lat1Rad, lon1Rad := lat1*math.Pi/180, lon1*math.Pi/180
	lat2Rad, lon2Rad := lat2*math.Pi/180, lon2*math.Pi/180

	dlat := lat2Rad - lat1Rad
	dlon := lon2Rad - lon1Rad

	a := math.Sin(dlat/2)*math.Sin(dlat/2) +
		math.Cos(lat1Rad)*math.Cos(lat2Rad)*math.Sin(dlon/2)*math.Sin(dlon/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))

	return R * c
}
