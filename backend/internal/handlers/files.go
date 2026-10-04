package handlers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/faiz-gh/fronko/backend/internal/middleware"
	"github.com/faiz-gh/fronko/backend/internal/models"
	"github.com/faiz-gh/fronko/backend/internal/repository"
	"github.com/faiz-gh/fronko/backend/internal/storage"
)

const (
	maxImageBytes = 5 << 20
	maxPDFBytes   = 20 << 20
	// The multipart envelope adds a little on top of the largest file.
	maxUploadBody = maxPDFBytes + 1<<20

	maxFileTitleLen    = 120
	maxFileNameLen     = 200
	defaultFilePageLen = 24
	maxFilePageLen     = 100

	uploadDeadline   = 2 * time.Minute
	storageOpTimeout = 60 * time.Second
	// Presigned links are short-lived; the redirect itself is cached briefly.
	presignTTL      = 15 * time.Minute
	redirectMaxAge  = 5 * time.Minute
	publicIDByteLen = 16
)

// Public ids are 16 random bytes, base64url without padding.
var publicIDPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{22}$`)

var (
	errUnsupportedType = errors.New("only JPEG, PNG or WebP images and PDF files can be uploaded")
	errFileTooLarge    = errors.New("file too large")
)

// upload describes an accepted file after sniffing its contents.
type upload struct {
	kind        string // "image" or "pdf"
	contentType string
	ext         string
	limit       int64
}

// classifyUpload decides what a file is from its bytes, never from the client's
// claimed type or file name, and enforces the per-kind size limit.
func classifyUpload(data []byte) (upload, error) {
	var u upload
	switch ct := http.DetectContentType(data); ct {
	case "image/jpeg":
		u = upload{"image", ct, "jpg", maxImageBytes}
	case "image/png":
		u = upload{"image", ct, "png", maxImageBytes}
	case "image/webp":
		u = upload{"image", ct, "webp", maxImageBytes}
	case "application/pdf":
		u = upload{"pdf", ct, "pdf", maxPDFBytes}
	default:
		return upload{}, errUnsupportedType
	}
	if int64(len(data)) > u.limit {
		return upload{}, errFileTooLarge
	}
	return u, nil
}

// cleanFileName keeps a display-safe base name, falling back to a default.
func cleanFileName(name, fallback string) string {
	name = path.Base(strings.ReplaceAll(name, `\`, "/"))
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || r == '"' {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if name == "" || name == "." || name == "/" {
		return fallback
	}
	for utf8.RuneCountInString(name) > maxFileNameLen {
		_, size := utf8.DecodeLastRuneInString(name)
		name = name[:len(name)-size]
	}
	return name
}

func newPublicID() (string, error) {
	b := make([]byte, publicIDByteLen)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

type FileHandler struct {
	svc  *StorageService
	repo *repository.Repository
}

func NewFileHandler(svc *StorageService, repo *repository.Repository) *FileHandler {
	return &FileHandler{svc: svc, repo: repo}
}

// FilePage is one page of the library.
type FilePage struct {
	Files    []*models.File `json:"files"`
	Total    int64          `json:"total"`
	Page     int            `json:"page"`
	PageSize int            `json:"page_size"`
}

func (h *FileHandler) writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errStorageDisabled):
		writeError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, errStorageNotConfigured):
		writeError(w, http.StatusConflict, "connect your storage in Settings first")
	default:
		log.Printf("storage client: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

// Protected: GET /api/me/files?kind=&page=&page_size=
func (h *FileHandler) List(w http.ResponseWriter, r *http.Request) {
	kind := r.URL.Query().Get("kind")
	if kind != "" && kind != "image" && kind != "pdf" {
		writeError(w, http.StatusBadRequest, "invalid kind")
		return
	}
	page, size, ok := pageParams(w, r, defaultFilePageLen, maxFilePageLen)
	if !ok {
		return
	}
	files, total, err := h.repo.ListFilesForUser(r.Context(), middleware.UserID(r.Context()), kind, size, (page-1)*size)
	if err != nil {
		log.Printf("list files: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, FilePage{Files: files, Total: total, Page: page, PageSize: size})
}

// Protected: POST /api/me/files (multipart: "file", optional "title")
func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r.Context())

	store, err := h.svc.StoreFor(r.Context(), userID, "")
	if err != nil {
		h.writeStoreError(w, err)
		return
	}
	settings, err := h.repo.GetStorageSettings(r.Context(), userID)
	if err != nil {
		h.writeStoreError(w, err)
		return
	}

	// Large files on slow connections outlast the server-wide timeouts.
	rc := http.NewResponseController(w)
	_ = rc.SetReadDeadline(time.Now().Add(uploadDeadline))
	_ = rc.SetWriteDeadline(time.Now().Add(uploadDeadline))

	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBody)
	mr, err := r.MultipartReader()
	if err != nil {
		writeError(w, http.StatusBadRequest, "expected a multipart file upload")
		return
	}

	var data []byte
	var fileName, title string
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeError(w, http.StatusRequestEntityTooLarge, "files can be up to 5 MB for images and 20 MB for PDFs")
				return
			}
			writeError(w, http.StatusBadRequest, "invalid upload")
			return
		}
		switch part.FormName() {
		case "file":
			if data != nil {
				writeError(w, http.StatusBadRequest, "upload one file at a time")
				return
			}
			fileName = part.FileName()
			data, err = storage.ReadAllLimited(part, maxPDFBytes)
			if errors.Is(err, storage.ErrTooLarge) {
				writeError(w, http.StatusRequestEntityTooLarge, "files can be up to 5 MB for images and 20 MB for PDFs")
				return
			}
		case "title":
			var raw []byte
			raw, err = storage.ReadAllLimited(part, 4*maxFileTitleLen)
			title = strings.TrimSpace(string(raw))
		default:
			_, err = io.Copy(io.Discard, part)
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid upload")
			return
		}
	}
	if len(data) == 0 {
		writeError(w, http.StatusBadRequest, "choose a file to upload")
		return
	}
	if utf8.RuneCountInString(title) > maxFileTitleLen {
		writeError(w, http.StatusBadRequest, "title is too long")
		return
	}

	u, err := classifyUpload(data)
	if errors.Is(err, errFileTooLarge) {
		writeError(w, http.StatusRequestEntityTooLarge, "images can be up to 5 MB and PDFs up to 20 MB")
		return
	}
	if err != nil {
		writeError(w, http.StatusUnsupportedMediaType, err.Error())
		return
	}

	publicID, err := newPublicID()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	file := &models.File{
		PublicID:     publicID,
		UserID:       userID,
		Bucket:       settings.Bucket,
		ObjectKey:    fmt.Sprintf("fronko/%d/%s.%s", userID, publicID, u.ext),
		Kind:         u.kind,
		ContentType:  u.contentType,
		SizeBytes:    int64(len(data)),
		OriginalName: cleanFileName(fileName, "upload."+u.ext),
		Title:        title,
	}

	ctx, cancel := context.WithTimeout(r.Context(), storageOpTimeout)
	defer cancel()
	if err := store.Put(ctx, file.ObjectKey, file.ContentType, data); err != nil {
		log.Printf("upload for user %d: %v", userID, err)
		writeError(w, http.StatusBadGateway, "couldn't save to your storage: "+storage.Describe(err))
		return
	}
	if err := h.repo.CreateFile(r.Context(), file); err != nil {
		log.Printf("create file: %v", err)
		// Don't leave an object nobody can see in the user's bucket.
		if delErr := store.Delete(context.WithoutCancel(r.Context()), file.ObjectKey); delErr != nil {
			log.Printf("clean up orphaned object %s: %v", file.ObjectKey, delErr)
		}
		writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	writeJSON(w, http.StatusCreated, file)
}

// Protected: PATCH /api/me/files/{id}
func (h *FileHandler) Rename(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	req.Title = strings.TrimSpace(req.Title)
	if utf8.RuneCountInString(req.Title) > maxFileTitleLen {
		writeError(w, http.StatusBadRequest, "title is too long")
		return
	}
	file, err := h.repo.UpdateFileTitle(r.Context(), r.PathValue("id"), middleware.UserID(r.Context()), req.Title)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		log.Printf("rename file: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, file)
}

// Protected: DELETE /api/me/files/{id}. Removes the object from the bucket, then the record.
func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	userID := middleware.UserID(r.Context())
	file, err := h.repo.GetFileForUser(r.Context(), r.PathValue("id"), userID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		log.Printf("get file: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	store, err := h.svc.StoreFor(r.Context(), userID, file.Bucket)
	switch {
	case err == nil:
		ctx, cancel := context.WithTimeout(r.Context(), storageOpTimeout)
		defer cancel()
		// Keep the record if the object couldn't be removed, so the user can retry.
		if err := store.Delete(ctx, file.ObjectKey); err != nil {
			log.Printf("delete object %s: %v", file.ObjectKey, err)
			writeError(w, http.StatusBadGateway, "couldn't delete from your storage: "+storage.Describe(err))
			return
		}
	case errors.Is(err, errStorageNotConfigured), errors.Is(err, errStorageDisabled):
		// No way to reach the bucket any more; just forget the record.
	default:
		h.writeStoreError(w, err)
		return
	}

	if err := h.repo.DeleteFile(r.Context(), file.PublicID, userID); err != nil && !errors.Is(err, repository.ErrNotFound) {
		log.Printf("delete file: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Public: GET /api/files/{id}. Redirects to a short-lived signed URL in the
// owner's private bucket.
func (h *FileHandler) Serve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !publicIDPattern.MatchString(id) || !h.svc.Enabled() {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	file, err := h.repo.GetFileByPublicID(r.Context(), id)
	if err != nil {
		if !errors.Is(err, repository.ErrNotFound) {
			log.Printf("serve file: %v", err)
		}
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	store, err := h.svc.StoreFor(r.Context(), file.UserID, file.Bucket)
	if err != nil {
		if !errors.Is(err, errStorageNotConfigured) {
			log.Printf("serve file %s: %v", id, err)
		}
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	name := file.OriginalName
	if file.Title != "" && file.Kind == "pdf" {
		name = file.Title + ".pdf"
	}
	disposition := mime.FormatMediaType("inline", map[string]string{"filename": name})
	if disposition == "" {
		disposition = "inline"
	}
	url, err := store.PresignGet(r.Context(), file.ObjectKey, presignTTL, file.ContentType, disposition)
	if err != nil {
		log.Printf("presign %s: %v", id, err)
		writeError(w, http.StatusBadGateway, "file unavailable")
		return
	}
	w.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d", int(redirectMaxAge.Seconds())))
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, url, http.StatusFound)
}

// referencedFileIDs pulls the file ids a card's data points at. The backend
// otherwise treats card data as opaque; these keys are the exception, so a
// public card only ever reveals files it actually uses.
func referencedFileIDs(data []byte) []string {
	var card struct {
		AvatarFile string `json:"avatar_file"`
		CoverFile  string `json:"cover_file"`
		Documents  []struct {
			File string `json:"file"`
		} `json:"documents"`
	}
	if err := json.Unmarshal(data, &card); err != nil {
		return nil
	}
	seen := map[string]bool{}
	var ids []string
	add := func(id string) {
		if publicIDPattern.MatchString(id) && !seen[id] && len(ids) < 32 {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	add(card.AvatarFile)
	add(card.CoverFile)
	for _, d := range card.Documents {
		add(d.File)
	}
	return ids
}
