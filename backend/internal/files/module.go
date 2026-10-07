package files

import (
	"time"

	"github.com/faiz-gh/fronko/backend/internal/app"
)

// Uploads are authenticated, but each one costs bandwidth and a bucket write.
const (
	uploadBurst    = 10
	uploadInterval = 6 * time.Second
)

// Module is the file library and the organisation's storage settings.
type Module struct {
	Files   *FileHandler
	Storage *StorageHandler
}

func (m Module) Routes(r *app.Routes) {
	r.Public("GET /api/files/{id}", m.Files.Serve)

	r.User("GET /api/me/storage", m.Storage.Get)
	r.Owner("PUT /api/me/storage", m.Storage.Put)
	r.Owner("POST /api/me/storage/test", m.Storage.Test)
	r.Owner("DELETE /api/me/storage", m.Storage.Delete)

	r.User("GET /api/me/files", m.Files.List)
	r.User("POST /api/me/files", r.RateLimit(uploadInterval, uploadBurst)(m.Files.Upload))
	r.User("GET /api/me/files/counts", m.Files.Counts)
	r.User("POST /api/me/files/bulk", m.Files.Bulk)
	r.User("PATCH /api/me/files/{id}", m.Files.Update)
	r.User("DELETE /api/me/files/{id}", m.Files.Delete)
	r.User("GET /api/me/files/{id}/usage", m.Files.Usage)
	r.User("GET /api/me/files/{id}/content", m.Files.Content)
	r.User("GET /api/me/files/{id}/image", m.Files.Image)
	r.Admin("GET /api/org/files/{id}/grants", m.Files.Grants)
	r.Admin("PUT /api/org/files/{id}/grants", m.Files.SetGrants)
}
