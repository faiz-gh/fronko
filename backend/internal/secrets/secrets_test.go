package secrets

import (
	"bytes"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newBox(t *testing.T, fill byte) *Box {
	t.Helper()
	b, err := New(bytes.Repeat([]byte{fill}, KeySize))
	require.NoError(t, err)
	return b
}

func TestSealOpen(t *testing.T) {
	box := newBox(t, 1)
	aad := []byte("storage:1:secret")

	sealed, err := box.Seal([]byte("s3cr3t"), aad)
	require.NoError(t, err)
	assert.NotContains(t, string(sealed), "s3cr3t")

	t.Run("round trip", func(t *testing.T) {
		got, err := box.Open(sealed, aad)
		require.NoError(t, err)
		assert.Equal(t, "s3cr3t", string(got))
	})

	t.Run("nonce is random", func(t *testing.T) {
		again, err := box.Seal([]byte("s3cr3t"), aad)
		require.NoError(t, err)
		assert.NotEqual(t, sealed, again)
	})

	t.Run("tampered ciphertext fails", func(t *testing.T) {
		bad := bytes.Clone(sealed)
		bad[len(bad)-1] ^= 0xff
		_, err := box.Open(bad, aad)
		assert.ErrorIs(t, err, ErrDecrypt)
	})

	t.Run("wrong aad fails", func(t *testing.T) {
		_, err := box.Open(sealed, []byte("storage:2:secret"))
		assert.ErrorIs(t, err, ErrDecrypt)
	})

	t.Run("wrong key fails", func(t *testing.T) {
		_, err := newBox(t, 2).Open(sealed, aad)
		assert.ErrorIs(t, err, ErrDecrypt)
	})

	t.Run("truncated or unknown version fails", func(t *testing.T) {
		_, err := box.Open(sealed[:5], aad)
		assert.ErrorIs(t, err, ErrDecrypt)
		other := bytes.Clone(sealed)
		other[0] = 9
		_, err = box.Open(other, aad)
		assert.ErrorIs(t, err, ErrDecrypt)
	})
}

func TestParseKey(t *testing.T) {
	good := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{7}, KeySize))
	key, err := ParseKey(good)
	require.NoError(t, err)
	assert.Len(t, key, KeySize)

	_, err = ParseKey("not base64!")
	assert.Error(t, err)
	_, err = ParseKey(base64.StdEncoding.EncodeToString([]byte("too short")))
	assert.Error(t, err)
}
