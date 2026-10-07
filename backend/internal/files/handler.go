package files

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"log"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/faiz-gh/fronko/backend/internal/auth"
	"github.com/faiz-gh/fronko/backend/internal/orgs"
	"github.com/faiz-gh/fronko/backend/internal/platform/database"
	"github.com/faiz-gh/fronko/backend/internal/platform/httpx"
	"github.com/faiz-gh/fronko/backend/internal/platform/storage"
	"github.com/faiz-gh/fronko/backend/internal/teams"
	"github.com/faiz-gh/fronko/backend/internal/users"
)

const (
	maxImageBytes = 5 << 20
	maxPDFBytes   = 20 << 20
	// Previews are small images the browser makes before uploading.
	maxThumbBytes = 300 << 10
	// The multipart envelope adds a little on top of the largest file and its preview.
	maxUploadBody = maxPDFBytes + maxThumbBytes + 1<<20

	maxFileTitleLen    = 120
	maxFileNameLen     = 200
	maxFileSearchLen   = 100
	defaultFilePageLen = 24
	maxFilePageLen     = 100
	maxBulkFiles       = 100
	// Bounds on the image size and page count the browser reports.
	maxImageSide = 20000
	maxPDFPages  = 10000

	uploadDeadline   = 2 * time.Minute
	storageOpTimeout = 60 * time.Second
	// Presigned links are short-lived; the redirect itself is cached briefly.
	presignTTL      = 15 * time.Minute
	redirectMaxAge  = 5 * time.Minute
	publicIDByteLen = 16
)

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

// classifyThumb accepts a preview image: JPEG, PNG or WebP within maxThumbBytes.
func classifyThumb(data []byte) (upload, bool) {
	u, err := classifyUpload(data)
	if err != nil || u.kind != "image" || len(data) > maxThumbBytes {
		return upload{}, false
	}
	return u, true
}

// thumbContentType is the type of a stored preview, from its key's extension.
func thumbContentType(key string) string {
	switch path.Ext(key) {
	case ".webp":
		return "image/webp"
	case ".png":
		return "image/png"
	default:
		return "image/jpeg"
	}
}

// purposeFits reports whether a file of kind may have the purpose: PDFs are
// brochures (or other), images anything but a brochure.
func purposeFits(kind, purpose string) bool {
	if !ValidPurpose(purpose) {
		return false
	}
	if kind == "pdf" {
		return purpose == PurposeBrochure || purpose == PurposeOther
	}
	return purpose != PurposeBrochure
}

// defaultPurpose is what a file is for when the uploader doesn't say.
func defaultPurpose(kind string) string {
	if kind == "pdf" {
		return PurposeBrochure
	}
	return PurposeOther
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

var errFileNotFound = &httpx.Error{Status: http.StatusNotFound, Msg: "file not found"}

type FileHandler struct {
	svc   *StorageService
	store *Store
	users *users.Store
	teams *teams.Store
	orgs  *orgs.Store
}

func NewFileHandler(svc *StorageService, store *Store, userStore *users.Store, teamStore *teams.Store, orgStore *orgs.Store) *FileHandler {
	return &FileHandler{svc: svc, store: store, users: userStore, teams: teamStore, orgs: orgStore}
}

// FilePage is one page of the library.
type FilePage struct {
	Files    []*File `json:"files"`
	Total    int64   `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
}

func (h *FileHandler) writeStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errStorageDisabled):
		httpx.WriteError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, errStorageNotConfigured):
		httpx.WriteError(w, http.StatusConflict, "connect your storage in Settings first")
	default:
		log.Printf("storage client: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
	}
}

// fileFilter reads the listing filters shared by List and Counts.
func fileFilter(r *http.Request) (FileFilter, *httpx.Error) {
	scope := auth.ScopeOf(r)
	query := r.URL.Query()
	filter := FileFilter{Kind: query.Get("kind"), Sort: query.Get("sort")}
	if filter.Kind != "" && filter.Kind != "image" && filter.Kind != "pdf" {
		return filter, &httpx.Error{Status: http.StatusBadRequest, Msg: "invalid kind"}
	}
	if !ValidFileSort(filter.Sort) {
		return filter, &httpx.Error{Status: http.StatusBadRequest, Msg: "invalid sort"}
	}
	filter.Search = strings.TrimSpace(query.Get("q"))
	if utf8.RuneCountInString(filter.Search) > maxFileSearchLen {
		return filter, &httpx.Error{Status: http.StatusBadRequest, Msg: "search is too long"}
	}
	if raw := query.Get("purpose"); raw != "" {
		for _, p := range strings.Split(raw, ",") {
			if !ValidPurpose(p) {
				return filter, &httpx.Error{Status: http.StatusBadRequest, Msg: "invalid purpose"}
			}
			filter.Purposes = append(filter.Purposes, p)
		}
	}
	idParam := func(name string) (int64, *httpx.Error) {
		raw := query.Get(name)
		if raw == "" {
			return 0, nil
		}
		id, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || id < 1 {
			return 0, &httpx.Error{Status: http.StatusBadRequest, Msg: "invalid " + name}
		}
		return id, nil
	}
	userID, e := idParam("user_id")
	if e != nil {
		return filter, e
	}
	if userID != 0 && !scope.Admin {
		return filter, &httpx.Error{Status: http.StatusForbidden, Msg: "only your organisation's admins can filter by user"}
	}
	if filter.TeamID, e = idParam("team_id"); e != nil {
		return filter, e
	}
	switch area := query.Get("area"); area {
	case "", AreaPersonal, AreaOrg, AreaShared, AreaTeam:
		filter.Area, filter.UserID = area, userID
	case "granted":
		filter.GrantedTo = scope.UserID
		if userID != 0 {
			filter.GrantedTo = userID
		}
	default:
		return filter, &httpx.Error{Status: http.StatusBadRequest, Msg: "invalid area"}
	}
	return filter, nil
}

// Protected: GET /api/me/files?kind=&area=&team_id=&user_id=&purpose=&q=&sort=&page=&page_size=
// Lists the files the caller can see. area is personal, org, shared, team or
// granted (files granted to the caller or their teams, or for admins to
// user_id). team_id keeps one team's files; user_id (admins only) one user's.
// purpose is a comma-separated list; q searches titles and file names; sort
// is newest (the default), oldest, name or size.
func (h *FileHandler) List(w http.ResponseWriter, r *http.Request) {
	filter, e := fileFilter(r)
	if e != nil {
		e.Write(w)
		return
	}
	page, size, ok := httpx.PageParams(w, r, defaultFilePageLen, maxFilePageLen)
	if !ok {
		return
	}
	filter.Limit, filter.Offset = size, (page-1)*size
	files, total, err := h.store.ListFiles(r.Context(), auth.ScopeOf(r), filter)
	if err != nil {
		httpx.Internal("list files", err).Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, FilePage{Files: files, Total: total, Page: page, PageSize: size})
}

// Protected: GET /api/me/files/counts with List's filters (purpose is
// ignored). How many files there are for each purpose, and in total.
func (h *FileHandler) Counts(w http.ResponseWriter, r *http.Request) {
	filter, e := fileFilter(r)
	if e != nil {
		e.Write(w)
		return
	}
	counts, err := h.store.CountFilesByPurpose(r.Context(), auth.ScopeOf(r), filter)
	if err != nil {
		httpx.Internal("count files", err).Write(w)
		return
	}
	var total int64
	for _, n := range counts {
		total += n
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"counts": counts, "total": total})
}

// uploadFields are the form fields that come with an uploaded file.
type uploadFields struct {
	fileName, title, area, purpose string
	teamID                         int64
	width, height, pages           *int
}

// Protected: POST /api/me/files (multipart: "file"; optional "thumb", a
// preview image; optional "title", "area", "team_id", "purpose", "width",
// "height" and "pages"). Members upload to their personal files, which count
// against their quota; team leads also to their teams' files; admins upload
// to the organisation's files (the default), the shared area or any team's.
func (h *FileHandler) Upload(w http.ResponseWriter, r *http.Request) {
	principal := auth.PrincipalFrom(r.Context())
	userID := principal.UserID

	store, err := h.svc.StoreFor(r.Context(), principal.OrgID, "")
	if err != nil {
		h.writeStoreError(w, err)
		return
	}
	settings, err := h.store.GetOrgStorageSettings(r.Context(), principal.OrgID)
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
		httpx.WriteError(w, http.StatusBadRequest, "expected a multipart file upload")
		return
	}

	var data, thumb []byte
	var f uploadFields
	field := func(part io.Reader, max int64) (string, error) {
		raw, err := storage.ReadAllLimited(part, max)
		return strings.TrimSpace(string(raw)), err
	}
	intField := func(part io.Reader, max int) (*int, error) {
		raw, err := field(part, 16)
		if err != nil || raw == "" {
			return nil, err
		}
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > max {
			return nil, errors.New("out of range")
		}
		return &n, nil
	}
	for {
		part, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				httpx.WriteError(w, http.StatusRequestEntityTooLarge, "files can be up to 5 MB for images and 20 MB for PDFs")
				return
			}
			httpx.WriteError(w, http.StatusBadRequest, "invalid upload")
			return
		}
		switch part.FormName() {
		case "file":
			if data != nil {
				httpx.WriteError(w, http.StatusBadRequest, "upload one file at a time")
				return
			}
			f.fileName = part.FileName()
			data, err = storage.ReadAllLimited(part, maxPDFBytes)
			if errors.Is(err, storage.ErrTooLarge) {
				httpx.WriteError(w, http.StatusRequestEntityTooLarge, "files can be up to 5 MB for images and 20 MB for PDFs")
				return
			}
		case "thumb":
			thumb, err = storage.ReadAllLimited(part, maxThumbBytes)
			if errors.Is(err, storage.ErrTooLarge) {
				thumb, err = nil, nil // a preview is optional; go without
				_, _ = io.Copy(io.Discard, part)
			}
		case "title":
			f.title, err = field(part, 4*maxFileTitleLen)
		case "area":
			f.area, err = field(part, 16)
		case "purpose":
			f.purpose, err = field(part, 16)
		case "team_id":
			var raw string
			if raw, err = field(part, 24); err == nil && raw != "" {
				f.teamID, err = strconv.ParseInt(raw, 10, 64)
			}
		case "width":
			f.width, err = intField(part, maxImageSide)
		case "height":
			f.height, err = intField(part, maxImageSide)
		case "pages":
			f.pages, err = intField(part, maxPDFPages)
		default:
			_, err = io.Copy(io.Discard, part)
		}
		if err != nil {
			httpx.WriteError(w, http.StatusBadRequest, "invalid upload")
			return
		}
	}
	if len(data) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, "choose a file to upload")
		return
	}
	if utf8.RuneCountInString(f.title) > maxFileTitleLen {
		httpx.WriteError(w, http.StatusBadRequest, "title is too long")
		return
	}
	area, msg := uploadArea(principal, f.area, f.teamID)
	if msg != "" {
		httpx.WriteError(w, http.StatusForbidden, msg)
		return
	}
	var teamID *int64
	if area == AreaTeam {
		if e := h.checkTeam(r, f.teamID); e != nil {
			e.Write(w)
			return
		}
		teamID = &f.teamID
	}

	u, err := classifyUpload(data)
	if errors.Is(err, errFileTooLarge) {
		httpx.WriteError(w, http.StatusRequestEntityTooLarge, "images can be up to 5 MB and PDFs up to 20 MB")
		return
	}
	if err != nil {
		httpx.WriteError(w, http.StatusUnsupportedMediaType, err.Error())
		return
	}
	if f.purpose == "" {
		f.purpose = defaultPurpose(u.kind)
	}
	if !purposeFits(u.kind, f.purpose) {
		httpx.WriteError(w, http.StatusBadRequest, "that purpose doesn't fit this kind of file")
		return
	}
	// Only images have a size, only PDFs pages.
	if u.kind == "pdf" {
		f.width, f.height = nil, nil
	} else {
		f.pages = nil
	}

	publicID, err := newPublicID()
	if err != nil {
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return
	}
	keyBase := fmt.Sprintf("fronko/%d/%d/%s", principal.OrgID, userID, publicID)
	file := &File{
		PublicID:     publicID,
		OrgID:        principal.OrgID,
		UserID:       userID,
		Area:         area,
		TeamID:       teamID,
		Bucket:       settings.Bucket,
		ObjectKey:    keyBase + "." + u.ext,
		Kind:         u.kind,
		Purpose:      f.purpose,
		ContentType:  u.contentType,
		SizeBytes:    int64(len(data)),
		OriginalName: cleanFileName(f.fileName, "upload."+u.ext),
		Title:        f.title,
		Width:        f.width,
		Height:       f.height,
		Pages:        f.pages,
	}

	// Fail fast before uploading; CreateFile checks again under a lock.
	if area == AreaPersonal && !h.fitsQuota(w, r, userID, file.SizeBytes) {
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), storageOpTimeout)
	defer cancel()
	if err := store.Put(ctx, file.ObjectKey, file.ContentType, data); err != nil {
		log.Printf("upload for user %d: %v", userID, err)
		httpx.WriteError(w, http.StatusBadGateway, "couldn't save to storage: "+storage.Describe(err))
		return
	}
	// The preview is a nice-to-have: the file is still usable without one.
	if t, ok := classifyThumb(thumb); ok {
		key := keyBase + ".thumb." + t.ext
		if err := store.Put(ctx, key, t.contentType, thumb); err != nil {
			log.Printf("upload preview %s: %v", key, err)
		} else {
			file.ThumbKey = &key
		}
	}
	if err := h.store.CreateFile(r.Context(), file); err != nil {
		// Don't leave objects nobody can see in the bucket.
		h.deleteObjects(context.WithoutCancel(r.Context()), store, file)
		if errors.Is(err, ErrQuotaExceeded) {
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, errQuotaMessage)
			return
		}
		log.Printf("create file: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "failed to save file")
		return
	}

	saved, err := h.store.GetFile(r.Context(), auth.ScopeOf(r), file.PublicID)
	if err != nil {
		log.Printf("reload file: %v", err)
		saved = file
	}
	httpx.WriteJSON(w, http.StatusCreated, saved)
}

const errQuotaMessage = "you've reached your storage limit; delete some files or ask your organisation for more space"

// uploadArea picks where an upload goes, returning a message if the user may
// not put it there. Team uploads also need the team to be in the
// organisation, which the caller checks.
func uploadArea(p auth.Principal, requested string, teamID int64) (string, string) {
	if requested == AreaTeam {
		switch {
		case teamID == 0:
			return "", "choose a team to upload to"
		case !p.IsAdmin() && !p.LeadsTeam(teamID):
			return "", "only the team's leads and your organisation's admins can add team files"
		}
		return AreaTeam, ""
	}
	if !p.IsAdmin() {
		if requested != "" && requested != AreaPersonal {
			return "", "only your organisation's admins can add organisation or shared files"
		}
		return AreaPersonal, ""
	}
	switch requested {
	case "", AreaOrg:
		return AreaOrg, ""
	case AreaShared:
		return AreaShared, ""
	default:
		return "", "admins upload to the organisation's files, the shared area or a team's files"
	}
}

// moveAllowed checks the user may move file to area (and team, for the team
// area), returning a message if not. Nobody moves files into someone's
// personal files; admins move anything else anywhere; team leads move their
// own personal files and their teams' files into a team they lead.
func moveAllowed(p auth.Principal, file *File, area string, teamID int64) string {
	switch area {
	case AreaPersonal:
		return "files can't be moved into someone's personal files"
	case AreaOrg, AreaShared:
		if !p.IsAdmin() {
			return "only your organisation's admins can move files there"
		}
	case AreaTeam:
		if teamID == 0 {
			return "choose a team to move the file to"
		}
		if !p.IsAdmin() && !p.LeadsTeam(teamID) {
			return "you can only move files into teams you lead"
		}
	default:
		return "invalid area"
	}
	if !p.IsAdmin() && file.Area == AreaPersonal && file.UserID != p.UserID {
		return "you can only move your own files"
	}
	return ""
}

// checkTeam makes sure a team belongs to the caller's organisation.
func (h *FileHandler) checkTeam(r *http.Request, teamID int64) *httpx.Error {
	if _, err := h.teams.GetTeam(r.Context(), auth.ScopeOf(r).OrgID, teamID); err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return &httpx.Error{Status: http.StatusBadRequest, Msg: "team not found"}
		}
		return httpx.Internal("get team", err)
	}
	return nil
}

// fitsQuota writes a 413 and returns false if size more bytes would take the
// user past their storage limit.
func (h *FileHandler) fitsQuota(w http.ResponseWriter, r *http.Request, userID, size int64) bool {
	user, err := h.users.GetUserByID(r.Context(), userID)
	if err == nil && user.StorageQuotaBytes != nil {
		var used int64
		if used, err = h.store.UsedBytes(r.Context(), userID); err == nil && used+size > *user.StorageQuotaBytes {
			httpx.WriteError(w, http.StatusRequestEntityTooLarge, errQuotaMessage)
			return false
		}
	}
	if err != nil {
		log.Printf("check quota: %v", err)
		httpx.WriteError(w, http.StatusInternalServerError, "internal error")
		return false
	}
	return true
}

// filePatchRequest is a change to a file's details; every field is optional.
// area and team_id move the file.
type filePatchRequest struct {
	Title   *string `json:"title"`
	Purpose *string `json:"purpose"`
	Area    *string `json:"area"`
	TeamID  *int64  `json:"team_id"`
}

// updateFile applies a patch to one file the caller may edit.
func (h *FileHandler) updateFile(r *http.Request, publicID string, req filePatchRequest) (*File, *httpx.Error) {
	p := auth.PrincipalFrom(r.Context())
	scope := auth.ScopeOf(r)
	file, err := h.store.GetEditableFile(r.Context(), scope, publicID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, errFileNotFound
		}
		return nil, httpx.Internal("get file", err)
	}
	patch := FilePatch{}
	if req.Title != nil {
		title := strings.TrimSpace(*req.Title)
		if utf8.RuneCountInString(title) > maxFileTitleLen {
			return nil, &httpx.Error{Status: http.StatusBadRequest, Msg: "title is too long"}
		}
		patch.Title = &title
	}
	if req.Purpose != nil {
		if !purposeFits(file.Kind, *req.Purpose) {
			return nil, &httpx.Error{Status: http.StatusBadRequest, Msg: "that purpose doesn't fit this kind of file"}
		}
		patch.Purpose = req.Purpose
	}
	if req.Area != nil {
		var teamID int64
		if req.TeamID != nil && *req.Area == AreaTeam {
			teamID = *req.TeamID
		}
		unchanged := *req.Area == file.Area && (file.TeamID == nil && teamID == 0 || file.TeamID != nil && *file.TeamID == teamID)
		if !unchanged {
			if msg := moveAllowed(p, file, *req.Area, teamID); msg != "" {
				return nil, &httpx.Error{Status: http.StatusForbidden, Msg: msg}
			}
			if teamID != 0 {
				if e := h.checkTeam(r, teamID); e != nil {
					return nil, e
				}
				patch.TeamID = &teamID
			}
			patch.Area = req.Area
		}
	}
	saved, err := h.store.UpdateFile(r.Context(), scope, publicID, patch)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return nil, errFileNotFound
		}
		return nil, httpx.Internal("update file", err)
	}
	return saved, nil
}

// Protected: PATCH /api/me/files/{id} {"title", "purpose", "area", "team_id"}.
// Every field is optional. Members change their own files; team leads their
// teams' files; admins any in the organisation. See moveAllowed for moves.
func (h *FileHandler) Update(w http.ResponseWriter, r *http.Request) {
	var req filePatchRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	file, e := h.updateFile(r, r.PathValue("id"), req)
	if e != nil {
		e.Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, file)
}

// deleteObjects removes a file's objects from the bucket, logging failures.
func (h *FileHandler) deleteObjects(ctx context.Context, store storage.Store, file *File) {
	keys := []string{file.ObjectKey}
	if file.ThumbKey != nil {
		keys = append(keys, *file.ThumbKey)
	}
	for _, key := range keys {
		if err := store.Delete(ctx, key); err != nil {
			log.Printf("clean up object %s: %v", key, err)
		}
	}
}

// deleteFile removes one file the caller may edit: its object from the
// bucket, then its record.
func (h *FileHandler) deleteFile(r *http.Request, publicID string) *httpx.Error {
	scope := auth.ScopeOf(r)
	file, err := h.store.GetEditableFile(r.Context(), scope, publicID)
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			return errFileNotFound
		}
		return httpx.Internal("get file", err)
	}

	store, err := h.svc.StoreFor(r.Context(), scope.OrgID, file.Bucket)
	switch {
	case err == nil:
		ctx, cancel := context.WithTimeout(r.Context(), storageOpTimeout)
		defer cancel()
		// Keep the record if the object couldn't be removed, so the user can retry.
		if err := store.Delete(ctx, file.ObjectKey); err != nil {
			log.Printf("delete object %s: %v", file.ObjectKey, err)
			return &httpx.Error{Status: http.StatusBadGateway, Msg: "couldn't delete from storage: " + storage.Describe(err)}
		}
		if file.ThumbKey != nil {
			if err := store.Delete(ctx, *file.ThumbKey); err != nil {
				log.Printf("delete preview %s: %v", *file.ThumbKey, err)
			}
		}
	case errors.Is(err, errStorageNotConfigured), errors.Is(err, errStorageDisabled):
		// No way to reach the bucket any more; just forget the record.
	default:
		return httpx.Internal("storage client", err)
	}

	if err := h.store.DeleteFile(r.Context(), file.ID, scope.OrgID); err != nil && !errors.Is(err, database.ErrNotFound) {
		return httpx.Internal("delete file", err)
	}
	return nil
}

// Protected: DELETE /api/me/files/{id}. Removes the object from the bucket,
// then the record. Members delete their own files; team leads their teams'
// files; admins any in the organisation.
func (h *FileHandler) Delete(w http.ResponseWriter, r *http.Request) {
	if e := h.deleteFile(r, r.PathValue("id")); e != nil {
		e.Write(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// BulkResult reports what happened to each file in a bulk action.
type BulkResult struct {
	Done   []string     `json:"done"`
	Failed []BulkFailed `json:"failed"`
}

type BulkFailed struct {
	ID    string `json:"id"`
	Error string `json:"error"`
}

// Protected: POST /api/me/files/bulk {"ids", "action": "delete"|"update",
// "patch"}. Applies the action to each file the caller may edit, reporting
// which succeeded and why the others didn't.
func (h *FileHandler) Bulk(w http.ResponseWriter, r *http.Request) {
	var req struct {
		IDs    []string         `json:"ids"`
		Action string           `json:"action"`
		Patch  filePatchRequest `json:"patch"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if len(req.IDs) == 0 || len(req.IDs) > maxBulkFiles {
		httpx.WriteError(w, http.StatusBadRequest, fmt.Sprintf("choose between 1 and %d files", maxBulkFiles))
		return
	}
	if req.Action != "delete" && req.Action != "update" {
		httpx.WriteError(w, http.StatusBadRequest, "action must be delete or update")
		return
	}
	res := BulkResult{Done: []string{}, Failed: []BulkFailed{}}
	seen := map[string]bool{}
	for _, id := range req.IDs {
		if seen[id] {
			continue
		}
		seen[id] = true
		var e *httpx.Error
		if !publicIDPattern.MatchString(id) {
			e = errFileNotFound
		} else if req.Action == "delete" {
			e = h.deleteFile(r, id)
		} else {
			_, e = h.updateFile(r, id, req.Patch)
		}
		if e != nil {
			res.Failed = append(res.Failed, BulkFailed{ID: id, Error: e.Msg})
		} else {
			res.Done = append(res.Done, id)
		}
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// Protected: GET /api/me/files/{id}/usage. The cards using a file that the
// caller can see, how many others do, and whether it's the organisation's
// logo or signature banner.
func (h *FileHandler) Usage(w http.ResponseWriter, r *http.Request) {
	scope := auth.ScopeOf(r)
	file, err := h.store.GetFile(r.Context(), scope, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			errFileNotFound.Write(w)
			return
		}
		httpx.Internal("get file", err).Write(w)
		return
	}
	usage, err := h.store.GetFileUsage(r.Context(), scope, file)
	if err != nil {
		httpx.Internal("file usage", err).Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, usage)
}

// Public: GET /api/files/{id}[?size=thumb]. Redirects to a short-lived signed
// URL in the organisation's private bucket. size=thumb asks for the small
// preview, falling back to the file itself when there isn't one.
func (h *FileHandler) Serve(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !publicIDPattern.MatchString(id) || !h.svc.Enabled() {
		httpx.WriteError(w, http.StatusNotFound, "file not found")
		return
	}
	file, err := h.store.GetFileByPublicID(r.Context(), id)
	if err != nil {
		if !errors.Is(err, database.ErrNotFound) {
			log.Printf("serve file: %v", err)
		}
		httpx.WriteError(w, http.StatusNotFound, "file not found")
		return
	}
	// A suspended organisation's files go offline with its cards.
	if suspended, err := h.orgs.IsOrgSuspended(r.Context(), file.OrgID); err != nil || suspended {
		if err != nil {
			log.Printf("serve file %s: %v", id, err)
		}
		httpx.WriteError(w, http.StatusNotFound, "file not found")
		return
	}
	store, err := h.svc.StoreFor(r.Context(), file.OrgID, file.Bucket)
	if err != nil {
		if !errors.Is(err, errStorageNotConfigured) {
			log.Printf("serve file %s: %v", id, err)
		}
		httpx.WriteError(w, http.StatusNotFound, "file not found")
		return
	}

	key, contentType := file.ObjectKey, file.ContentType
	name := file.OriginalName
	if file.Title != "" && file.Kind == "pdf" {
		name = file.Title + ".pdf"
	}
	if r.URL.Query().Get("size") == "thumb" && file.ThumbKey != nil {
		key, contentType = *file.ThumbKey, thumbContentType(*file.ThumbKey)
		name = "preview" + path.Ext(key)
	}
	disposition := mime.FormatMediaType("inline", map[string]string{"filename": name})
	if disposition == "" {
		disposition = "inline"
	}
	url, err := store.PresignGet(r.Context(), key, presignTTL, contentType, disposition)
	if err != nil {
		log.Printf("presign %s: %v", id, err)
		httpx.WriteError(w, http.StatusBadGateway, "file unavailable")
		return
	}
	w.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d", int(redirectMaxAge.Seconds())))
	w.Header().Set("Referrer-Policy", "no-referrer")
	http.Redirect(w, r, url, http.StatusFound)
}

// Protected: GET /api/me/files/{id}/content. The file's bytes, served from
// this origin so the browser can edit an image (crop it into a new copy)
// without the bucket allowing cross-origin reads.
func (h *FileHandler) Content(w http.ResponseWriter, r *http.Request) {
	scope := auth.ScopeOf(r)
	file, err := h.store.GetFile(r.Context(), scope, r.PathValue("id"))
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			errFileNotFound.Write(w)
			return
		}
		httpx.Internal("get file", err).Write(w)
		return
	}
	h.writeBytes(w, r, file, maxPDFBytes, "attachment")
}

// Protected: GET /api/me/files/{id}/image. Any image in the user's
// organisation, served from this origin so the browser can draw it (the
// organisation logo or a card's image in the middle of its QR code). Card
// images are public through /api/files/{id} anyway; this only avoids the
// bucket's cross-origin rules, and is limited to images in one's own organisation.
func (h *FileHandler) Image(w http.ResponseWriter, r *http.Request) {
	scope := auth.ScopeOf(r)
	id := r.PathValue("id")
	if !publicIDPattern.MatchString(id) {
		errFileNotFound.Write(w)
		return
	}
	file, err := h.store.GetFileByPublicID(r.Context(), id)
	if err != nil || file.OrgID != scope.OrgID || file.Kind != "image" {
		if err != nil && !errors.Is(err, database.ErrNotFound) {
			httpx.Internal("get file", err).Write(w)
			return
		}
		errFileNotFound.Write(w)
		return
	}
	h.writeBytes(w, r, file, maxImageBytes, "inline")
}

// writeBytes reads a file from the organisation's bucket and sends it.
func (h *FileHandler) writeBytes(w http.ResponseWriter, r *http.Request, file *File, limit int64, disposition string) {
	store, err := h.svc.StoreFor(r.Context(), file.OrgID, file.Bucket)
	if err != nil {
		h.writeStoreError(w, err)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), storageOpTimeout)
	defer cancel()
	data, err := store.Get(ctx, file.ObjectKey, limit)
	if err != nil {
		log.Printf("read object %s: %v", file.ObjectKey, err)
		httpx.WriteError(w, http.StatusBadGateway, "couldn't read from storage: "+storage.Describe(err))
		return
	}
	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Content-Length", strconv.Itoa(len(data)))
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Disposition", mime.FormatMediaType(disposition, map[string]string{"filename": file.OriginalName}))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

// Protected (admins): GET /api/org/files/{id}/grants. The users and teams a
// file has been granted to, on top of everyone who sees it anyway.
func (h *FileHandler) Grants(w http.ResponseWriter, r *http.Request) {
	file, ok := h.grantableFile(w, r)
	if !ok {
		return
	}
	users, err := h.store.ListFileGrants(r.Context(), file.ID)
	if err != nil {
		httpx.Internal("list grants", err).Write(w)
		return
	}
	teams, err := h.store.ListFileTeamGrants(r.Context(), file.ID)
	if err != nil {
		httpx.Internal("list team grants", err).Write(w)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"users": users, "teams": teams})
}

// Protected (admins): PUT /api/org/files/{id}/grants {"user_ids": [...],
// "team_ids": [...]}. Replaces the users and teams the file is granted to;
// a list that isn't sent is left as it is.
func (h *FileHandler) SetGrants(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserIDs *[]int64 `json:"user_ids"`
		TeamIDs *[]int64 `json:"team_ids"`
	}
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.UserIDs != nil && len(*req.UserIDs) > 1000 || req.TeamIDs != nil && len(*req.TeamIDs) > 1000 {
		httpx.WriteError(w, http.StatusBadRequest, "too many users or teams")
		return
	}
	file, ok := h.grantableFile(w, r)
	if !ok {
		return
	}
	p := auth.PrincipalFrom(r.Context())
	if req.UserIDs != nil {
		if err := h.store.ReplaceFileGrants(r.Context(), file.ID, p.OrgID, p.UserID, *req.UserIDs); err != nil {
			httpx.Internal("set grants", err).Write(w)
			return
		}
	}
	if req.TeamIDs != nil {
		if err := h.store.ReplaceFileTeamGrants(r.Context(), file.ID, p.OrgID, p.UserID, *req.TeamIDs); err != nil {
			httpx.Internal("set team grants", err).Write(w)
			return
		}
	}
	h.Grants(w, r)
}

// grantableFile loads a file whose access can be managed: any non-shared file in the organisation.
func (h *FileHandler) grantableFile(w http.ResponseWriter, r *http.Request) (*File, bool) {
	file, err := h.store.GetFile(r.Context(), auth.ScopeOf(r), r.PathValue("id"))
	if err != nil {
		if errors.Is(err, database.ErrNotFound) {
			errFileNotFound.Write(w)
			return nil, false
		}
		httpx.Internal("get file", err).Write(w)
		return nil, false
	}
	if file.Area == AreaShared {
		httpx.WriteError(w, http.StatusBadRequest, "everyone in your organisation can already see shared files")
		return nil, false
	}
	return file, true
}
