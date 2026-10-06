package handlers

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	pngHeader  = []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR")
	jpegHeader = []byte("\xff\xd8\xff\xe0\x00\x10JFIF\x00")
	webpHeader = []byte("RIFF\x24\x00\x00\x00WEBPVP8 ")
	pdfHeader  = []byte("%PDF-1.7\n%\xe2\xe3\xcf\xd3\n")
)

func padded(header []byte, size int) []byte {
	return append(bytes.Clone(header), make([]byte, size-len(header))...)
}

func TestClassifyUpload(t *testing.T) {
	cases := []struct {
		name, kind, ext string
		data            []byte
	}{
		{"png", "image", "png", padded(pngHeader, 1024)},
		{"jpeg", "image", "jpg", padded(jpegHeader, 1024)},
		{"webp", "image", "webp", padded(webpHeader, 1024)},
		{"pdf", "pdf", "pdf", padded(pdfHeader, 1024)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := classifyUpload(tc.data)
			require.NoError(t, err)
			assert.Equal(t, tc.kind, u.kind)
			assert.Equal(t, tc.ext, u.ext)
		})
	}

	t.Run("type comes from content, not the name", func(t *testing.T) {
		// A PNG uploaded as "brochure.pdf" is still a PNG.
		u, err := classifyUpload(padded(pngHeader, 64))
		require.NoError(t, err)
		assert.Equal(t, "image", u.kind)
	})

	t.Run("rejects other types", func(t *testing.T) {
		for _, data := range [][]byte{
			[]byte("<html><script>alert(1)</script></html>"),
			[]byte("<svg xmlns='http://www.w3.org/2000/svg'></svg>"),
			[]byte("GIF89a\x01\x00\x01\x00"),
			[]byte("PK\x03\x04 zip"),
			[]byte("plain text"),
		} {
			_, err := classifyUpload(data)
			assert.ErrorIs(t, err, errUnsupportedType, string(data))
		}
	})

	t.Run("size limits per kind", func(t *testing.T) {
		_, err := classifyUpload(padded(pngHeader, maxImageBytes))
		assert.NoError(t, err)
		_, err = classifyUpload(padded(pngHeader, maxImageBytes+1))
		assert.ErrorIs(t, err, errFileTooLarge)

		// A PDF may be bigger than an image.
		_, err = classifyUpload(padded(pdfHeader, maxImageBytes+1))
		assert.NoError(t, err)
		_, err = classifyUpload(padded(pdfHeader, maxPDFBytes+1))
		assert.ErrorIs(t, err, errFileTooLarge)
	})
}

func TestCleanFileName(t *testing.T) {
	assert.Equal(t, "brochure.pdf", cleanFileName("brochure.pdf", "x"))
	assert.Equal(t, "passwd", cleanFileName("../../etc/passwd", "x"))
	assert.Equal(t, "report.pdf", cleanFileName(`C:\Users\me\report.pdf`, "x"))
	assert.Equal(t, "evil.pdf", cleanFileName("ev\"il\r\n.pdf", "x"))
	assert.Equal(t, "upload.pdf", cleanFileName("", "upload.pdf"))
	assert.Equal(t, "upload.pdf", cleanFileName("   ", "upload.pdf"))
	assert.Len(t, []rune(cleanFileName(strings.Repeat("é", 500), "x")), maxFileNameLen)
}

func TestReferencedFileIDs(t *testing.T) {
	a := strings.Repeat("a", 22)
	b := strings.Repeat("b", 22)
	c := strings.Repeat("d", 22)
	data := []byte(`{"name":"x","avatar_file":"` + a + `","cover_file":"` + c + `","documents":[{"file":"` + b + `"},{"file":"` + a + `"},{"file":"not-an-id"}],"other":"` + strings.Repeat("c", 22) + `"}`)
	assert.Equal(t, []string{a, c, b}, referencedFileIDs(data), "deduplicated, invalid ids and unrelated keys ignored")

	gallery := []byte(`{"avatar_file":"` + a + `","blocks":[{"type":"bio"},{"type":"gallery","images":[{"file":"` + b + `"},{"file":"` + a + `"}]}]}`)
	assert.Equal(t, []string{a, b}, referencedFileIDs(gallery), "gallery images are referenced")

	assert.Empty(t, referencedFileIDs([]byte(`{}`)))
	assert.Empty(t, referencedFileIDs([]byte(`not json`)))
	assert.Empty(t, referencedFileIDs([]byte(`{"documents":"wrong shape"}`)))
}

func TestNewPublicID(t *testing.T) {
	id, err := newPublicID()
	require.NoError(t, err)
	assert.Regexp(t, publicIDPattern, id)
	other, err := newPublicID()
	require.NoError(t, err)
	assert.NotEqual(t, id, other)
}
