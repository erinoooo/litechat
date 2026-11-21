package server

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/ledongthuc/pdf"
)

const (
	maxUpload    = 25 << 20
	maxExtracted = 400_000 // characters; keeps a single file from flooding the context
)

// extractFile turns an uploaded document into plain text for the model.
func (s *Server) extractFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUpload+1<<20)
	f, hdr, err := r.FormFile("file")
	if err != nil {
		httpErr(w, http.StatusBadRequest, "upload a file in the 'file' field (max 25 MB)")
		return
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxUpload+1))
	if err != nil || len(data) > maxUpload {
		httpErr(w, http.StatusRequestEntityTooLarge, "file is larger than 25 MB")
		return
	}

	var text string
	ext := strings.ToLower(filepath.Ext(hdr.Filename))
	switch {
	case ext == ".pdf" || bytes.HasPrefix(data, []byte("%PDF")):
		text, err = pdfText(data)
	case ext == ".docx":
		text, err = docxText(data)
	case utf8.Valid(data):
		text = string(data)
	default:
		err = errors.New("unsupported file type (try PDF, DOCX or a text file)")
	}
	if err != nil {
		httpErr(w, http.StatusUnprocessableEntity, fmt.Sprintf("couldn't read %s: %v", hdr.Filename, err))
		return
	}
	text = strings.TrimSpace(text)
	if text == "" {
		httpErr(w, http.StatusUnprocessableEntity, hdr.Filename+" has no extractable text (scanned PDFs need OCR)")
		return
	}
	truncated := false
	if len(text) > maxExtracted {
		text, truncated = text[:maxExtracted], true
	}
	writeJSON(w, http.StatusOK, map[string]any{"name": hdr.Filename, "text": text, "truncated": truncated})
}

func pdfText(data []byte) (out string, err error) {
	defer func() { // the PDF parser panics on some malformed files
		if r := recover(); r != nil {
			err = fmt.Errorf("malformed PDF")
		}
	}()
	rd, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for i := 1; i <= rd.NumPage(); i++ {
		p := rd.Page(i)
		if p.V.IsNull() {
			continue
		}
		text, err := p.GetPlainText(nil)
		if err != nil {
			continue
		}
		if i > 1 {
			fmt.Fprintf(&sb, "\n\n--- page %d ---\n\n", i)
		}
		sb.WriteString(strings.TrimSpace(text))
		if sb.Len() > maxExtracted {
			break
		}
	}
	return sb.String(), nil
}

// docxText reads word/document.xml and keeps paragraph breaks.
func docxText(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", err
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer rc.Close()
		dec := xml.NewDecoder(io.LimitReader(rc, 64<<20))
		var sb strings.Builder
		inText := false
		for {
			tok, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				return "", err
			}
			switch t := tok.(type) {
			case xml.StartElement:
				switch t.Name.Local {
				case "t":
					inText = true
				case "tab":
					sb.WriteByte('\t')
				case "br":
					sb.WriteByte('\n')
				}
			case xml.EndElement:
				switch t.Name.Local {
				case "t":
					inText = false
				case "p":
					sb.WriteByte('\n')
				}
			case xml.CharData:
				if inText {
					sb.Write(t)
				}
			}
		}
		return sb.String(), nil
	}
	return "", errors.New("not a Word document")
}
