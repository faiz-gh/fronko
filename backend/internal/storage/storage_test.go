package storage

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateEndpoint(t *testing.T) {
	ok := []struct{ in, want string }{
		{"https://s3.us-east-1.amazonaws.com", "https://s3.us-east-1.amazonaws.com"},
		{"https://abc123.r2.cloudflarestorage.com/", "https://abc123.r2.cloudflarestorage.com"},
		{"  https://s3.eu-central-003.backblazeb2.com  ", "https://s3.eu-central-003.backblazeb2.com"},
		{"https://storage.example.com:8443", "https://storage.example.com:8443"},
	}
	for _, tc := range ok {
		got, err := ValidateEndpoint(tc.in, false)
		require.NoError(t, err, tc.in)
		assert.Equal(t, tc.want, got)
	}

	bad := []struct {
		in   string
		want error
	}{
		{"", ErrInvalidEndpoint},
		{"s3.amazonaws.com", ErrInvalidEndpoint},
		{"http://s3.amazonaws.com", ErrInvalidEndpoint},
		{"ftp://s3.amazonaws.com", ErrInvalidEndpoint},
		{"https://user:pass@s3.amazonaws.com", ErrInvalidEndpoint},
		{"https://s3.amazonaws.com/bucket", ErrInvalidEndpoint},
		{"https://s3.amazonaws.com?x=1", ErrInvalidEndpoint},
		{"https://localhost:9000", ErrPrivateEndpoint},
		{"https://minio.localhost", ErrPrivateEndpoint},
		{"https://host.docker.internal:9000", ErrPrivateEndpoint},
		{"https://127.0.0.1", ErrPrivateEndpoint},
		{"https://10.0.0.5", ErrPrivateEndpoint},
		{"https://192.168.1.10", ErrPrivateEndpoint},
		{"https://169.254.169.254", ErrPrivateEndpoint}, // cloud metadata service
		{"https://100.64.0.1", ErrPrivateEndpoint},
		{"https://[::1]", ErrPrivateEndpoint},
		{"https://[fd00::1]", ErrPrivateEndpoint},
		{"https://[::ffff:127.0.0.1]", ErrPrivateEndpoint},
	}
	for _, tc := range bad {
		_, err := ValidateEndpoint(tc.in, false)
		assert.ErrorIs(t, err, tc.want, tc.in)
	}

	t.Run("private endpoints allowed for local development", func(t *testing.T) {
		got, err := ValidateEndpoint("http://host.docker.internal:9000", true)
		require.NoError(t, err)
		assert.Equal(t, "http://host.docker.internal:9000", got)
	})
}

func TestGuardedClientBlocksPrivateAddresses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer srv.Close()

	// A name that passed URL validation can still resolve to loopback; the dialer must refuse it.
	_, err := guardedClient(false).Get(srv.URL)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPrivateEndpoint), "got %v", err)

	resp, err := guardedClient(true).Get(srv.URL)
	require.NoError(t, err)
	resp.Body.Close()
}

func TestNewS3RejectsBadEndpoint(t *testing.T) {
	_, err := NewS3(false)(Config{Endpoint: "http://10.0.0.1", Bucket: "b"})
	assert.ErrorIs(t, err, ErrInvalidEndpoint)
	assert.NotEmpty(t, Describe(err))
	assert.Equal(t, "", Describe(nil))
	assert.Equal(t, "the endpoint timed out", Describe(context.DeadlineExceeded))
	assert.Equal(t, "couldn't reach the storage provider", Describe(errors.New("boom")))
}

// fakeS3 answers like a bucket whose keys may only touch objects.
func fakeS3(t *testing.T, headStatus int, putStatus int) (*httptest.Server, *[]string) {
	t.Helper()
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls = append(calls, r.Method+" "+r.URL.Path)
		switch r.Method {
		case http.MethodHead:
			w.WriteHeader(headStatus)
		case http.MethodPut:
			if putStatus != http.StatusOK {
				w.Header().Set("Content-Type", "application/xml")
				w.WriteHeader(putStatus)
				_, _ = w.Write([]byte(`<Error><Code>SignatureDoesNotMatch</Code><Message>bad</Message></Error>`))
				return
			}
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func probe(t *testing.T, endpoint string) error {
	t.Helper()
	store, err := NewS3(true)(Config{
		Endpoint: endpoint, Region: "auto", Bucket: "cards", PathStyle: true,
		AccessKeyID: "id", SecretAccessKey: "secret",
	})
	require.NoError(t, err)
	return store.Probe(context.Background())
}

func TestProbe(t *testing.T) {
	t.Run("object-only keys pass even when HeadBucket is forbidden", func(t *testing.T) {
		srv, calls := fakeS3(t, http.StatusForbidden, http.StatusOK)
		require.NoError(t, probe(t, srv.URL))
		assert.Equal(t, []string{"HEAD /cards", "PUT /cards/.fronko-probe", "DELETE /cards/.fronko-probe"}, *calls)
	})

	t.Run("missing bucket fails clearly", func(t *testing.T) {
		srv, calls := fakeS3(t, http.StatusNotFound, http.StatusOK)
		err := probe(t, srv.URL)
		require.Error(t, err)
		assert.Contains(t, Describe(err), "bucket not found")
		assert.Len(t, *calls, 1, "no write is attempted")
	})

	t.Run("wrong secret is reported from the write", func(t *testing.T) {
		srv, _ := fakeS3(t, http.StatusForbidden, http.StatusForbidden)
		err := probe(t, srv.URL)
		require.Error(t, err)
		assert.Equal(t, "the secret access key is wrong", Describe(err))
	})
}
