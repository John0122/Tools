//go:build ignore

// 生成本项目原创的图片与下载演示文件；在 target_web 目录执行 go run scripts/generate-fixtures.go。
package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
)

func main() {
	root := "frontend/fixtures"
	for _, dir := range []string{"images", "downloads"} {
		must(os.MkdirAll(filepath.Join(root, dir), 0755))
	}
	canvas := image.NewRGBA(image.Rect(0, 0, 480, 270))
	for y := 0; y < 270; y++ {
		for x := 0; x < 480; x++ {
			c := color.RGBA{234, 244, 251, 255}
			if x > 40 && x < 440 && y > 40 && y < 230 {
				c = color.RGBA{0, 95, 145, 255}
			}
			if x > 70 && x < 410 && y > 80 && y < 105 {
				c = color.RGBA{129, 213, 252, 255}
			}
			if x > 70 && x < 280 && y > 130 && y < 150 {
				c = color.RGBA{255, 255, 255, 255}
			}
			if x > 70 && x < 350 && y > 170 && y < 185 {
				c = color.RGBA{167, 194, 210, 255}
			}
			canvas.SetRGBA(x, y, c)
		}
	}
	for _, ext := range []string{"png", "jpg", "gif"} {
		f, err := os.Create(filepath.Join(root, "images", "sample."+ext))
		must(err)
		switch ext {
		case "png":
			must(png.Encode(f, canvas))
		case "jpg":
			must(jpeg.Encode(f, canvas, &jpeg.Options{Quality: 85}))
		case "gif":
			must(gif.Encode(f, canvas, nil))
		}
		must(f.Close())
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg" width="480" height="270" viewBox="0 0 480 270"><rect width="480" height="270" fill="#eaf4fb"/><rect x="40" y="40" width="400" height="190" rx="12" fill="#005f91"/><text x="70" y="112" font-family="monospace" font-size="27" fill="white">TARGET WEB</text><text x="70" y="158" font-family="monospace" font-size="18" fill="#81d5fc">LINK RESOURCE SAMPLE</text><path d="M70 190h300" stroke="#81d5fc" stroke-width="4"/></svg>`
	must(os.WriteFile(filepath.Join(root, "images", "sample.svg"), []byte(svg), 0644))
	// 创建带合法对象偏移的单页 PDF，避免用改后缀的文本冒充文件。
	text := "BT /F1 20 Tf 60 740 Td (Target Web - Download Sample) Tj 0 -40 Td /F1 12 Tf (This document is a public link-detection fixture.) Tj ET"
	objects := []string{"<< /Type /Catalog /Pages 2 0 R >>", "<< /Type /Pages /Kids [3 0 R] /Count 1 >>", "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>", "<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>", fmt.Sprintf("<< /Length %d >>\nstream\n%s\nendstream", len(text), text)}
	var pdf bytes.Buffer
	pdf.WriteString("%PDF-1.4\n")
	offsets := []int{0}
	for i, obj := range objects {
		offsets = append(offsets, pdf.Len())
		fmt.Fprintf(&pdf, "%d 0 obj\n%s\nendobj\n", i+1, obj)
	}
	xref := pdf.Len()
	fmt.Fprintf(&pdf, "xref\n0 %d\n0000000000 65535 f \n", len(offsets))
	for _, n := range offsets[1:] {
		fmt.Fprintf(&pdf, "%010d 00000 n \n", n)
	}
	fmt.Fprintf(&pdf, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n", len(offsets), xref)
	must(os.WriteFile(filepath.Join(root, "downloads", "sample.pdf"), pdf.Bytes(), 0644))
	writeZip(filepath.Join(root, "downloads", "sample.zip"), map[string]string{"README.txt": "Target Web public download sample.\nNo user data is stored.\n"})
	writeZip(filepath.Join(root, "downloads", "sample.docx"), map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`,
		"_rels/.rels":         `<?xml version="1.0" encoding="UTF-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   `<?xml version="1.0" encoding="UTF-8"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>Target Web 下载示例文档</w:t></w:r></w:p><w:p><w:r><w:t>此文档用于验证外链提取、文件下载与响应类型。</w:t></w:r></w:p><w:sectPr/></w:body></w:document>`,
	})
}

func writeZip(path string, files map[string]string) {
	f, err := os.Create(path)
	must(err)
	z := zip.NewWriter(f)
	for name, body := range files {
		w, err := z.Create(name)
		must(err)
		_, err = w.Write([]byte(body))
		must(err)
	}
	must(z.Close())
	must(f.Close())
}
func must(err error) {
	if err != nil {
		panic(err)
	}
}
