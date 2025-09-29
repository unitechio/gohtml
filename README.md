# 📝 GoHTML with UniDoc – HTML to PDF in Go

## 📦 Giới thiệu

[GoHTML](https://github.com/unitechio/gohtml) là một phần của **GoDoc**, cho phép render HTML/CSS sang PDF bằng cách sử dụng **Chromium + headless**.
Bạn có thể dùng nó trong Go để tạo báo cáo, hóa đơn, hoặc in ấn tài liệu trực tiếp từ HTML.

---

## 🚀 Cài đặt

### Yêu cầu

* Go `>= 1.21` (nên dùng Go `1.24+`)
* Google Chrome / Chromium cài sẵn trên máy
* Git

### Clone repo

```bash
git clone https://github.com/unitechio/gohtml.git
cd gohtml
```

### Cài dependencies

```bash
go mod tidy
```

---

## 🖥️ Chạy ví dụ

Ví dụ convert HTML cơ bản:

```go
package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/unitechio/gohtml"
	"github.com/unitechio/gohtml/client"
	"github.com/unitechio/gohtml/content"
	"github.com/unitechio/gohtml/sizes"
	"github.com/unitechio/gopdf/common"
)

func main() {
	// Enable detailed logging
	common.Log = common.NewConsoleLogger(common.LogLevelTrace)

	// 1. First try a simple HTTP request to check server
	resp, err := http.Get("http://localhost:8080/health")
	if err != nil {
		fmt.Printf("Basic HTTP check error: %v\n", err)
	} else {
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Printf("Health endpoint response: %s\n", string(body))
	}

	// 2. Connect to the server with explicit options
	err = gohtml.ConnectOptions(gohtml.Options{
		Hostname: "localhost",
		Port:     8080,
		Secure:   false,
	})
	if err != nil {
		fmt.Printf("Connection error: %v\n", err)
		return
	}

	// 3. Create a very simple test document
	htmlContent := `<!DOCTYPE html><html><body><p>Test</p></body></html>`
	doc, err := gohtml.NewDocumentFromString(htmlContent)
	if err != nil {
		fmt.Printf("Document creation error: %v\n", err)
		return
	}

	// 4. Set page size and margins explicitly
	doc.SetPageSize(sizes.A4)
	doc.SetMargins(10, 10, 10, 10)
	doc.SetTimeoutDuration(30 * time.Second)

	// 5. Try to write to file
	err = doc.WriteToFile("test_output.pdf")
	if err != nil {
		fmt.Printf("PDF generation error: %v\n", err)

		// 6. Try to diagnose by manually creating the request
		htmlObj, _ := content.NewStringContent(htmlContent)
		query, _ := client.BuildHTMLQuery().
			PageSize(sizes.A4).
			SetContent(htmlObj).
			Query()

		// Create a client manually
		cli := client.New(client.Options{
			Hostname: "localhost",
			Port:     8080,
		})

		// Send a manual request and examine the full response
		resp, err := cli.ConvertHTML(context.Background(), query)
		if err != nil {
			fmt.Printf("Manual request error: %v\n", err)
		} else {
			fmt.Printf("Response ID: %s, Data length: %d\n", resp.ID, len(resp.Data))
		}

		return
	}

	fmt.Println("Successfully generated PDF: test_output.pdf")
}

```

---

## 📂 Cách hoạt động

* GoHTML sử dụng **chromedp** để điều khiển Chrome/Chromium chạy headless.
* HTML/CSS được render bởi engine của Chrome, đảm bảo hiển thị gần như giống hệt trình duyệt.
* Kết quả được xuất thành file PDF.

---

## ⚠️ Lưu ý khi deploy

* Cần có **Chrome / Chromium** trong môi trường runtime (Docker, server).
* Khi deploy bằng Docker, thêm Chromium vào image:

```dockerfile
FROM golang:1.24.5 AS builder
WORKDIR /app
COPY . .
RUN go build -o server .

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y chromium ca-certificates && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=builder /app/server .
CMD ["./server"]
```
