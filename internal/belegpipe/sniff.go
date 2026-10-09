package belegpipe

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"io"
	"strings"
	"unicode"
)

// MaxPDFPages is the upload limit for a PDF.
const MaxPDFPages = 50

// MaxXMLBytes is the upload limit for an e-invoice.
const MaxXMLBytes = 5 << 20

// Sniff classifies a file by magic bytes.
func Sniff(b []byte) string {
	if len(b) >= 3 && b[0] == 0xFF && b[1] == 0xD8 && b[2] == 0xFF {
		return "jpeg"
	}
	if len(b) >= 8 && bytes.Equal(b[:8], []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A}) {
		return "png"
	}
	if len(b) >= 12 && string(b[0:4]) == "RIFF" && string(b[8:12]) == "WEBP" {
		return "webp"
	}
	if bytes.HasPrefix(b, []byte("%PDF-")) {
		return "pdf"
	}
	if looksXML(b) {
		return "xml"
	}
	return ""
}

func looksXML(b []byte) bool {
	s := bytes.TrimSpace(b)
	if bytes.HasPrefix(s, []byte{0xEF, 0xBB, 0xBF}) {
		s = bytes.TrimSpace(s[3:])
	}
	return bytes.HasPrefix(s, []byte("<?xml")) || bytes.HasPrefix(s, []byte("<"))
}

// SHA256Hex is the lowercase hex digest.
func SHA256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// PDFPages counts /Type /Page objects and checks the header and trailer.
func PDFPages(b []byte) (int, error) {
	if !bytes.HasPrefix(b, []byte("%PDF-")) {
		return 0, errors.New("not a pdf")
	}
	if !bytes.Contains(b, []byte("%%EOF")) {
		return 0, errors.New("pdf trailer")
	}
	n := 0
	rest := b
	for {
		i := bytes.Index(rest, []byte("/Type"))
		if i < 0 {
			break
		}
		rest = rest[i+len("/Type"):]
		rest = bytes.TrimLeftFunc(rest, unicode.IsSpace)
		if !bytes.HasPrefix(rest, []byte("/Page")) {
			continue
		}
		rest = rest[len("/Page"):]
		if len(rest) > 0 && (rest[0] == 's' || rest[0] == 'S') {
			continue
		}
		n++
	}
	if n == 0 {
		return 0, errors.New("no pages")
	}
	if n > MaxPDFPages {
		return n, errors.New("too many pages")
	}
	return n, nil
}

// PDFHybrid reports a ZUGFeRD / Factur-X payload inside a PDF.
func PDFHybrid(b []byte) bool {
	s := strings.ToLower(string(b))
	return strings.Contains(s, "crossindustryinvoice") || strings.Contains(s, "factur-x") || strings.Contains(s, "zugferd")
}

// XMLRoot returns Invoice, CreditNote or CrossIndustryInvoice.
// encoding/xml does not resolve external entities.
func XMLRoot(b []byte) (string, error) {
	if len(b) > MaxXMLBytes {
		return "", errors.New("xml too large")
	}
	dec := xml.NewDecoder(bytes.NewReader(b))
	var root string
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		start, ok := tok.(xml.StartElement)
		if !ok || root != "" {
			continue
		}
		switch start.Name.Local {
		case "Invoice", "CreditNote", "CrossIndustryInvoice":
			root = start.Name.Local
		default:
			return "", errors.New("xml root")
		}
	}
	if root == "" {
		return "", errors.New("xml root")
	}
	return root, nil
}
