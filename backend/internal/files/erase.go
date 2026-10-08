package files

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/faiz-gh/fronko/backend/internal/platform/storage"
)

// RekeyFor returns a function that re-encrypts the organisation's bucket keys
// from one owner to another, for an ownership transfer: the ciphertexts are
// bound to the user who holds them (see aad).
func (s *StorageService) RekeyFor(fromID, toID int64) func(idEnc, secretEnc []byte) ([]byte, []byte, error) {
	return func(idEnc, secretEnc []byte) ([]byte, []byte, error) {
		if !s.Enabled() {
			return nil, nil, errStorageDisabled
		}
		id, err := s.box.Open(idEnc, aad(fromID, "access_key_id"))
		if err != nil {
			return nil, nil, fmt.Errorf("decrypting storage credentials: %w", err)
		}
		secret, err := s.box.Open(secretEnc, aad(fromID, "secret_access_key"))
		if err != nil {
			return nil, nil, fmt.Errorf("decrypting storage credentials: %w", err)
		}
		newID, err := s.box.Seal(id, aad(toID, "access_key_id"))
		if err != nil {
			return nil, nil, err
		}
		newSecret, err := s.box.Seal(secret, aad(toID, "secret_access_key"))
		if err != nil {
			return nil, nil, err
		}
		return newID, newSecret, nil
	}
}

// objectRef is where one file's objects live.
type objectRef struct {
	bucket string
	keys   []string
}

// DeleteOrgObjects removes every object the organisation's files have in its
// bucket, before the organisation itself is deleted (the database rows go
// with it, but the bucket is outside the database). Objects that can't be
// removed are logged and skipped, so a broken bucket never blocks deleting
// the account. It returns how many objects couldn't be removed.
func (s *StorageService) DeleteOrgObjects(ctx context.Context, orgID int64) (failed int, err error) {
	rows, err := s.store.db.Query(ctx, `SELECT bucket, object_key, thumb_key FROM files WHERE org_id = $1`, orgID)
	if err != nil {
		return 0, err
	}
	var refs []objectRef
	for rows.Next() {
		var bucket, key string
		var thumb *string
		if err := rows.Scan(&bucket, &key, &thumb); err != nil {
			rows.Close()
			return 0, err
		}
		ref := objectRef{bucket: bucket, keys: []string{key}}
		if thumb != nil {
			ref.keys = append(ref.keys, *thumb)
		}
		refs = append(refs, ref)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(refs) == 0 {
		return 0, nil
	}

	clients := map[string]storage.Store{}
	for _, ref := range refs {
		client, ok := clients[ref.bucket]
		if !ok {
			c, err := s.StoreFor(ctx, orgID, ref.bucket)
			if errors.Is(err, errStorageNotConfigured) || errors.Is(err, errStorageDisabled) {
				// Nothing can reach the bucket; the organisation's owner was
				// told their bucket is theirs to empty.
				return 0, nil
			}
			if err != nil {
				return 0, err
			}
			clients[ref.bucket], client = c, c
		}
		for _, key := range ref.keys {
			if err := client.Delete(ctx, key); err != nil {
				log.Printf("delete org %d: object %s: %v", orgID, key, err)
				failed++
			}
		}
	}
	return failed, nil
}
