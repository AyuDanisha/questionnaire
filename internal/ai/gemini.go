package ai

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Struct untuk request body
type geminiRequest struct {
	Model    string          `json:"model"`
	Contents []geminiContent `json:"contents"`
}

// Struct untuk content
type geminiContent struct {
	Role  string       `json:"role"`
	Parts []geminiPart `json:"parts"`
}

// Struct untuk part
type geminiPart struct {
	Text       string            `json:"text,omitempty"`
	InlineData *geminiInlineData `json:"inlineData,omitempty"`
}

// Struct untuk inline data (file)
type geminiInlineData struct {
	MimeType string `json:"mimeType"`
	Data     string `json:"data"`
}

// Struct untuk parsing response API
type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
}

// ProxyGemini mengirim request ke Gemini Proxy
// Menggunakan parameter baru: model, fileBuffer, dan systemInstruction
func ProxyGemini(input string, model string, fileBuffer []byte, systemInstruction string) (string, error) {
	if input == "" && fileBuffer == nil {
		return "", fmt.Errorf("input is required")
	}

	// Set default model jika kosong
	if model == "" {
		model = "gemini-2.0-flash"
	}

	// 1. Siapkan Parts
	parts := []geminiPart{}

	// Jika ada file buffer, proses
	if fileBuffer != nil {
		// Deteksi MIME type
		mimeType := http.DetectContentType(fileBuffer)

		// Encode ke Base64
		b64Data := base64.StdEncoding.EncodeToString(fileBuffer)

		parts = append(parts, geminiPart{
			InlineData: &geminiInlineData{
				MimeType: mimeType,
				Data:     b64Data,
			},
		})
	}

	// Tambahkan text input
	if input != "" {
		parts = append(parts, geminiPart{Text: input})
	}

	// 2. Susun Contents
	contents := []geminiContent{}

	// Tambahkan System Instruction jika ada
	// Catatan: Beberapa proxy menerima system instruction sebagai bagian dari contents role='system'
	if systemInstruction != "" {
		contents = append(contents, geminiContent{
			Role:  "system",
			Parts: []geminiPart{{Text: systemInstruction}},
		})
	}

	// Tambahkan User Content
	contents = append(contents, geminiContent{
		Role:  "user",
		Parts: parts,
	})

	reqBody := geminiRequest{
		Model:    model,
		Contents: contents,
	}

	jsonBody, _ := json.Marshal(reqBody)

	// 3. Kirim Request ke Proxy Endpoint
	client := &http.Client{Timeout: 120 * time.Second}

	// URL Proxy yang diberikan
	url := "https://us-central1-infinite-chain-295909.cloudfunctions.net/gemini-proxy-staging-v1"

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", err
	}

	// Set Headers persis seperti di script yang diminta
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Priority", "u=1, i")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="131", "Not_A Brand";v="24", "Microsoft Edge Simulate";v="131", "Lemur";v="131"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?1")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Android"`)
	req.Header.Set("Sec-Fetch-Dest", "empty")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Site", "cross-site")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36")

	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	if res.StatusCode != 200 {
		return "", fmt.Errorf("api error: %s - %s", res.Status, string(body))
	}

	// 4. Parse Response
	var gemRes geminiResponse
	if err := json.Unmarshal(body, &gemRes); err != nil {
		return "", err
	}

	if len(gemRes.Candidates) == 0 || len(gemRes.Candidates[0].Content.Parts) == 0 {
		return "", fmt.Errorf("no response candidates found")
	}

	return gemRes.Candidates[0].Content.Parts[0].Text, nil
}

func GenerateQuestions(projectDesc string) ([]string, error) {
	systemInstruction := "Anda adalah seorang Business Analyst dan System Analyst."

	prompt := fmt.Sprintf(`Berdasarkan deskripsi proyek berikut, buatkan 100 pertanyaan.

Deskripsi Proyek:
%s

Instruksi:
1. Buat tepat 100 pertanyaan.
2. Kategori: [BM] (Business Modeling) atau [SRM] (System Requirement Modeling).
3. Tentukan Tipe Jawaban (Type):
   - TEXT_SHORT: Untuk jawaban singkat (nama, angka spesifik).
   - TEXT_LONG: Untuk penjelasan deskriptif.
   - RADIO: Untuk pilihan ganda (contoh: Ya/Tidak, Prioritas Tinggi/Sedah/Rendah). Sertakan opsi di dalam kurung () dipisah koma.
   - CHECKBOX: Untuk pilihan ganda yang bisa lebih dari satu. Sertakan opsi di dalam kurung () dipisah koma.
   - FILE: Untuk meminta upload dokumen/gambar.
4. Format Output: [KATEGORI] [TYPE] Pertanyaan? (Opsi1, Opsi2)

Contoh:
[BM] [TEXT_LONG] Jelaskan visi bisnis Anda.
[SRM] [RADIO] Apakah sistem harus mendukung multi-currency? (Ya, Tidak)
[SRM] [CHECKBOX] Fitur mana yang prioritas? (Login, Dashboard, Laporan)
[SRM] [FILE] Lampirkan logo perusahaan.

Mulai generate:`, projectDesc)

	response, err := ProxyGemini(prompt, "gemini-2.0-flash", nil, systemInstruction)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(response, "\n")
	var questions []string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Cek apakah ada format kategori
		if strings.Contains(line, "[BM]") || strings.Contains(line, "[SRM]") {
			questions = append(questions, line)
		}
	}

	return questions, nil
}

// GenerateBM generates Business Modeling document using the new ProxyGemini function
func GenerateBM(projectDesc string, qaPairs []string) (string, error) {
	qaText := strings.Join(qaPairs, "\n")

	systemInstruction := "Anda adalah seorang Business Analyst senior."

	prompt := fmt.Sprintf(`Berdasarkan informasi berikut, susunlah dokumen Business Modeling yang lengkap dan profesional.

Deskripsi Proyek:
%s

Informasi dari Client (Q&A):
%s

Buat dokumen Business Modeling dengan struktur berikut:

# BUSINESS MODELING DOCUMENT

## 1. Executive Summary
(Ringkasan eksekutif tentang bisnis yang akan dibangun)

## 2. Visi dan Misi Bisnis
(Jelaskan visi dan misi bisnis berdasarkan informasi yang diberikan)

## 3. Tujuan Bisnis
(Tuliskan tujuan bisnis jangka pendek, menengah, dan panjang)

## 4. Stakeholder Analysis
(Identifikasi dan analisis semua stakeholder yang terlibat)

## 5. Business Process Model
(Gambarkan proses bisnis utama yang akan didukung oleh sistem)

## 6. Business Rules
(Tuliskan aturan bisnis yang harus diikuti sistem)

## 7. Value Proposition
(Jelaskan nilai yang diberikan kepada pelanggan)

## 8. Revenue Model
(Jelaskan model pendapatan bisnis)

## 9. Market Analysis
(Analisis pasar dan kompetitor)

## 10. SWOT Analysis
(Analisis Strength, Weakness, Opportunity, Threat)

## 11. Risk Assessment
(Identifikasi risiko dan strategi mitigasi)

## 12. Success Metrics
(Indikator keberhasilan bisnis)

Gunakan bahasa Indonesia yang mudah dipahami oleh client non-teknis. Format dengan markdown yang rapi.`, projectDesc, qaText)

	return ProxyGemini(prompt, "gemini-2.0-flash", nil, systemInstruction)
}

// GenerateSRM generates System Requirement Modeling document using the new ProxyGemini function
func GenerateSRM(projectDesc string, qaPairs []string) (string, error) {
	qaText := strings.Join(qaPairs, "\n")

	systemInstruction := "Anda adalah seorang System Analyst senior."

	prompt := fmt.Sprintf(`Berdasarkan informasi berikut, susunlah dokumen System Requirement Modeling yang lengkap dan profesional.

Deskripsi Proyek:
%s

Informasi dari Client (Q&A):
%s

Buat dokumen System Requirement Modeling dengan struktur berikut:

# SYSTEM REQUIREMENT MODELING DOCUMENT

## 1. Pendahuluan
### 1.1 Tujuan Dokumen
### 1.2 Ruang Lingkup
### 1.3 Definisi, Akronim, dan Singkatan

## 2. Gambaran Umum Sistem
### 2.1 Perspektif Sistem
### 2.2 Fungsi Sistem
### 2.3 Karakteristik Pengguna

## 3. Functional Requirements
### 3.1 User Requirements
(Daftar kebutuhan pengguna dengan format UR-001, UR-002, dst)

### 3.2 System Features
(Daftar fitur sistem dengan format SF-001, SF-002, dst)

### 3.3 Use Case Diagram
(Deskripsi use case utama)

### 3.4 Use Case Specifications
(Spesifikasi detail setiap use case)

## 4. Non-Functional Requirements
### 4.1 Performance Requirements
### 4.2 Security Requirements
### 4.3 Reliability Requirements
### 4.4 Availability Requirements
### 4.5 Usability Requirements

## 5. System Interface Requirements
### 5.1 User Interfaces
### 5.2 Hardware Interfaces
### 5.3 Software Interfaces
### 5.4 Communication Interfaces

## 6. Data Requirements
### 6.1 Data Entities
### 6.2 Data Flow
### 6.3 Data Storage Requirements

## 7. Constraints dan Assumptions
### 7.1 Constraints
### 7.2 Assumptions

## 8. Acceptance Criteria
(Kriteria penerimaan sistem)

Gunakan bahasa Indonesia yang mudah dipahami oleh client non-teknis. Format dengan markdown yang rapi.`, projectDesc, qaText)

	return ProxyGemini(prompt, "gemini-2.0-flash", nil, systemInstruction)
}
