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
	"strconv"
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

// Protected: GET /api/me/files?kind=&area=&user_id=&page=&page_size=
// Lists the files the caller can see. area is personal, org, shared or
// granted (files granted to the caller, or for admins to user_id). user_id
// (admins only) otherwise keeps one user's files.
func (h *FileHandler) List(w http.ResponseWriter, r *http.Request) {
	scope := scopeOf(r)
	query := r.URL.Query()
	filter := repository.FileFilter{Kind: query.Get("kind")}
	if filter.Kind != "" && filter.Kind != "image" && filter.Kind != "pdf" {
		writeError(w, http.StatusBadRequest, "invalid kind")
		return
	}
	var userID int64
	if raw := query.Get("user_id"); raw != "" {
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 {
			writeError(w, http.StatusBadRequest, "invalid user_id")
			return
		}
		if !scope.Admin {
			writeError(w, http.StatusForbidden, "only your organisation's admins can filter by user")
			return
		}
		userID = id
	}
	switch area := query.Get("area"); area {
	case "", models.AreaPersonal, models.AreaOrg, models.AreaShared:
		filter.Area, filter.UserID = area, userID
	case "granted":
		filter.GrantedTo = scope.UserID
		if userID != 0 {
			filter.GrantedTo = userID
		}
	default:
		writeError(w, http.StatusBadRequest, "invalid area")
		return
	}
	page, size, ok := pageParams(w, r, defaultFilePageLen, maxFilePageLen)
	if !ok {
		return
	}
	filter.Limit, filter.Offset = size, (page-1)*size
	files, total, err := h.repo.ListFiles(r.Context(), scope, filter)
	if err != nil {
		log.Printf("list files: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, FilePage{Files: files, Total: total, Page: page, PageSize: size})
}

// Protected: POST /api/me/files (multipart: "file", optional "title" and "area").
// Members upload to their personal files, which count against their quota;
// admins upload to the organisation's files (the default) or the shared area.
func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	principal := middleware.PrincipalFrom(r.Context())
	userID := principal.UserID

	store, err := h.svc.StoreFor(r.Context(), principal.OrgID, "")
	if err != nil {
		h.writeStoreError(w, err)
		return
	}
	settings, err := h.repo.GetOrgStorageSettings(r.Context(), principal.OrgID)
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
	var fileName, title, area string
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
		case "area":
			var raw []byte
			raw, err = storage.ReadAllLimited(part, 16)
			area = strings.TrimSpace(string(raw))
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
	area, msg := uploadArea(principal, area)
	if msg != "" {
		writeError(w, http.StatusForbidden, msg)
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
		OrgID:        principal.OrgID,
		UserID:       userID,
		Area:         area,
		Bucket:       settings.Bucket,
		ObjectKey:    fmt.Sprintf("fronko/%d/%d/%s.%s", principal.OrgID, userID, publicID, u.ext),
		Kind:         u.kind,
		ContentType:  u.contentType,
		SizeBytes:    int64(len(data)),
		OriginalName: cleanFileName(fileName, "upload."+u.ext),
		Title:        title,
	}

	// Fail fast before uploading; CreateFile checks again under a lock.
	if area == models.AreaPersonal && !h.fitsQuota(w, r, userID, file.SizeBytes) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), storageOpTimeout)
	defer cancel()
	if err := store.Put(ctx, file.ObjectKey, file.ContentType, data); err != nil {
		log.Printf("upload for user %d: %v", userID, err)
		writeError(w, http.StatusBadGateway, "couldn't save to storage: "+storage.Describe(err))
		return
	}
	if err := h.repo.CreateFile(r.Context(), file); err != nil {
		// Don't leave an object nobody can see in the bucket.
		if delErr := store.Delete(context.WithoutCancel(r.Context()), file.ObjectKey); delErr != nil {
			log.Printf("clean up orphaned object %s: %v", file.ObjectKey, delErr)
		}
		if errors.Is(err, repository.ErrQuotaExceeded) {
			writeError(w, http.StatusRequestEntityTooLarge, errQuotaMessage)
			return
		}
		log.Printf("create file: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	saved, err := h.repo.GetFile(r.Context(), scopeOf(r), file.PublicID)
	if err != nil {
		log.Printf("reload file: %v", err)
		saved = file
	}
	writeJSON(w, http.StatusCreated, saved)
}

const errQuotaMessage = "you've reached your storage limit; delete some files or ask your organisation for more space"

// uploadArea picks where an upload goes, returning a message if the user may not put it there.
func uploadArea(p middleware.Principal, requested string) (string, string) {
	if !p.IsAdmin() {
		if requested != "" && requested != models.AreaPersonal {
			return "", "only your organisation's admins can add organisation or shared files"
		}
		return models.AreaPersonal, ""
	}
	switch requested {
	case "", models.AreaOrg:
		return models.AreaOrg, ""
	case models.AreaShared:
		return models.AreaShared, ""
	default:
		return "", "admins upload to the organisation's files or the shared area"
	}
}

// fitsQuota writes a 413 and returns false if size more bytes would take the
// user past their storage limit.
func (h *FileHandler) fitsQuota(w http.ResponseWriter, r *http.Request, userID, size int64) bool {
	user, err := h.repo.GetUserByID(r.Context(), userID)
	if err == nil && user.StorageQuotaBytes != nil {
		var used int64
		if used, err = h.repo.UsedBytes(r.Context(), userID); err == nil && used+size > *user.StorageQuotaBytes {
			writeError(w, http.StatusRequestEntityTooLarge, errQuotaMessage)
			return false
		}
	}
	if err != nil {
		log.Printf("check quota: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return false
	}
	return true
}

// Protected: PATCH /api/me/files/{id}. Members rename their own files; admins any in the organisation.
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
	file, err := h.repo.UpdateFileTitle(r.Context(), scopeOf(r), r.PathValue("id"), req.Title)
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

// Protected: DELETE /api/me/files/{id}. Removes the object from the bucket,
// then the record. Members delete their own files; admins any in the organisation.
func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	scope := scopeOf(r)
	file, err := h.repo.GetEditableFile(r.Context(), scope, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "file not found")
			return
		}
		log.Printf("get file: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}

	store, err := h.svc.StoreFor(r.Context(), scope.OrgID, file.Bucket)
	switch {
	case err == nil:
		ctx, cancel := context.WithTimeout(r.Context(), storageOpTimeout)
		defer cancel()
		// Keep the record if the object couldn't be removed, so the user can retry.
		if err := store.Delete(ctx, file.ObjectKey); err != nil {
			log.Printf("delete object %s: %v", file.ObjectKey, err)
			writeError(w, http.StatusBadGateway, "couldn't delete from storage: "+storage.Describe(err))
			return
		}
	case errors.Is(err, errStorageNotConfigured), errors.Is(err, errStorageDisabled):
		// No way to reach the bucket any more; just forget the record.
	default:
		h.writeStoreError(w, err)
		return
	}

	if err := h.repo.DeleteFile(r.Context(), file.ID, scope.OrgID); err != nil && !errors.Is(err, repository.ErrNotFound) {
		log.Printf("delete file: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Public: GET /api/files/{id}. Redirects to a short-lived signed URL in the
// organisation's private bucket.
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
	// A suspended organisation's files go offline with its cards.
	if suspended, err := h.repo.IsOrgSuspended(r.Context(), file.OrgID); err != nil || suspended {
		if err != nil {
			log.Printf("serve file %s: %v", id, err)
		}
		writeError(w, http.StatusNotFound, "file not found")
		return
	}
	store, err := h.svc.StoreFor(r.Context(), file.OrgID, file.Bucket)
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

// Protected (admins): GET /api/org/files/{id}/grants. The users a file has
// been granted to, on top of everyone who sees it anyway.
func (h *FileHandler) Grants(w http.ResponseWriter, r *http.Request) {
	file, ok := h.grantableFile(w, r)
	if !ok {
		return
	}
	users, err := h.repo.ListFileGrants(r.Context(), file.ID)
	if err != nil {
		log.Printf("list grants: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"users": users})
}

// Protected (admins): PUT /api/org/files/{id}/grants {"user_ids": [...]}.
// Replaces the list of users the file is granted to.
func (h *FileHandler) SetGrants(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserIDs []int64 `json:"user_ids"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if len(req.UserIDs) > 1000 {
		writeError(w, http.StatusBadRequest, "too many users")
		return
	}
	file, ok := h.grantableFile(w, r)
	if !ok {
		return
	}
	p := middleware.PrincipalFrom(r.Context())
	if err := h.repo.ReplaceFileGrants(r.Context(), file.ID, p.OrgID, p.UserID, req.UserIDs); err != nil {
		log.Printf("set grants: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return
	}
	h.Grants(w, r)
}

// grantableFile loads a file whose access can be managed: any non-shared file in the organisation.
func (h *FileHandler) grantableFile(w http.ResponseWriter, r *http.Request) (*models.File, bool) {
	file, err := h.repo.GetFile(r.Context(), scopeOf(r), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			writeError(w, http.StatusNotFound, "file not found")
			return nil, false
		}
		log.Printf("get file: %v", err)
		writeError(w, http.StatusInternalServerError, "internal error")
		return nil, false
	}
	if file.Area == models.AreaShared {
		writeError(w, http.StatusBadRequest, "everyone in your organisation can already see shared files")
		return nil, false
	}
	return file, true
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
