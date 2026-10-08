package files

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/faiz-gh/fronko/backend/internal/platform/secrets"
)

func TestRekeyForMovesKeysToTheNewOwner(t *testing.T) {
	box, err := secrets.New(bytes.Repeat([]byte{7}, 32))
	require.NoError(t, err)
	s := &StorageService{box: box}

	id, err := box.Seal([]byte("AKIA123"), aad(1, "access_key_id"))
	require.NoError(t, err)
	secret, err := box.Seal([]byte("s3cret"), aad(1, "secret_access_key"))
	require.NoError(t, err)

	newID, newSecret, err := s.RekeyFor(1, 2)(id, secret)
	require.NoError(t, err)
	gotID, gotSecret, err := s.credentials(&StorageSettings{UserID: 2, AccessKeyIDEnc: newID, SecretAccessKeyEnc: newSecret})
	require.NoError(t, err)
	assert.Equal(t, "AKIA123", gotID)
	assert.Equal(t, "s3cret", gotSecret)

	_, _, err = s.credentials(&StorageSettings{UserID: 1, AccessKeyIDEnc: newID, SecretAccessKeyEnc: newSecret})
	assert.Error(t, err, "bound to the new owner only")

	_, _, err = s.RekeyFor(3, 2)(id, secret)
	assert.Error(t, err, "keys that aren't the old owner's are refused")

	_, _, err = (&StorageService{}).RekeyFor(1, 2)(id, secret)
	assert.Error(t, err, "no SECRETS_KEY")
}
