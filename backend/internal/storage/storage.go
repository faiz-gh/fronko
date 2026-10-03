// Package storage talks to a user's own S3-compatible bucket (AWS S3,
// Cloudflare R2, Backblaze B2, MinIO, ...).
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"syscall"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// Store is the subset of S3 the app needs; handlers depend on this so tests can fake it.
type Store interface {
	Put(ctx context.Context, key, contentType string, body []byte) error
	Delete(ctx context.Context, key string) error
	// PresignGet returns a time-limited URL the browser can fetch directly.
	PresignGet(ctx context.Context, key string, ttl time.Duration, contentType, disposition string) (string, error)
	// Probe checks the credentials can read and write the bucket.
	Probe(ctx context.Context) error
}

// Config is everything needed to reach one bucket.
type Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PathStyle       bool
}

// Factory builds a Store for a bucket. The server injects NewS3; tests inject a fake.
type Factory func(cfg Config) (Store, error)

var (
	ErrInvalidEndpoint = errors.New("storage: invalid endpoint")
	ErrPrivateEndpoint = errors.New("storage: endpoint resolves to a private or local address")
)

// probeKey is written and deleted by Probe to prove write access.
const probeKey = ".fronko-probe"

// ValidateEndpoint normalises a user-supplied endpoint to scheme://host[:port].
// Unless allowPrivate is set (local development against MinIO), only https is
// accepted and literal private/loopback addresses are rejected up front; names
// that resolve to such addresses are caught again when dialing.
func ValidateEndpoint(raw string, allowPrivate bool) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || u.Hostname() == "" {
		return "", fmt.Errorf("%w: enter a full URL such as https://s3.example.com", ErrInvalidEndpoint)
	}
	if u.User != nil {
		return "", fmt.Errorf("%w: credentials don't belong in the URL", ErrInvalidEndpoint)
	}
	if u.Path != "" && u.Path != "/" || u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("%w: use the endpoint only, without a path", ErrInvalidEndpoint)
	}
	switch {
	case u.Scheme == "https":
	case u.Scheme == "http" && allowPrivate:
	default:
		return "", fmt.Errorf("%w: the endpoint must use https", ErrInvalidEndpoint)
	}
	if !allowPrivate {
		host := strings.ToLower(u.Hostname())
		if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".internal") {
			return "", ErrPrivateEndpoint
		}
		if addr, err := netip.ParseAddr(host); err == nil && !isPublic(addr) {
			return "", ErrPrivateEndpoint
		}
	}
	return u.Scheme + "://" + u.Host, nil
}

var nonPublicPrefixes = []netip.Prefix{
	netip.MustParsePrefix("0.0.0.0/8"),
	netip.MustParsePrefix("100.64.0.0/10"), // carrier-grade NAT
	netip.MustParsePrefix("192.0.0.0/24"),
	netip.MustParsePrefix("198.18.0.0/15"), // benchmarking
	netip.MustParsePrefix("240.0.0.0/4"),
	netip.MustParsePrefix("64:ff9b::/96"), // NAT64 can reach private IPv4
}

func isPublic(addr netip.Addr) bool {
	addr = addr.Unmap()
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsLinkLocalUnicast() || addr.IsLinkLocalMulticast() ||
		addr.IsInterfaceLocalMulticast() || addr.IsMulticast() || addr.IsUnspecified() {
		return false
	}
	for _, p := range nonPublicPrefixes {
		if p.Contains(addr) {
			return false
		}
	}
	return true
}

// guardedClient is the HTTP client used for every bucket request. Checking the
// address in the dialer's Control hook (after DNS) defeats DNS-rebinding tricks
// that a URL check alone would miss.
func guardedClient(allowPrivate bool) *http.Client {
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	if !allowPrivate {
		dialer.Control = func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return ErrPrivateEndpoint
			}
			addr, err := netip.ParseAddr(host)
			if err != nil || !isPublic(addr) {
				return ErrPrivateEndpoint
			}
			return nil
		}
	}
	return &http.Client{
		Transport: &http.Transport{
			Proxy:                 nil, // never route user traffic through an environment proxy
			DialContext:           dialer.DialContext,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 30 * time.Second,
			MaxIdleConnsPerHost:   4,
			IdleConnTimeout:       60 * time.Second,
		},
		// The SDK doesn't need redirects, and following them could leave the checked host.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
}

type s3Store struct {
	client *s3.Client
	bucket string
}

// NewS3 returns a Factory for real buckets.
func NewS3(allowPrivate bool) Factory {
	httpClient := guardedClient(allowPrivate)
	return func(cfg Config) (Store, error) {
		endpoint, err := ValidateEndpoint(cfg.Endpoint, allowPrivate)
		if err != nil {
			return nil, err
		}
		region := cfg.Region
		if region == "" {
			region = "us-east-1"
		}
		client := s3.New(s3.Options{
			BaseEndpoint: aws.String(endpoint),
			Region:       region,
			UsePathStyle: cfg.PathStyle,
			Credentials:  credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, ""),
			HTTPClient:   httpClient,
			// Newer SDKs send checksum trailers by default, which R2, B2 and older
			// MinIO reject. Only add them when an operation requires it.
			RequestChecksumCalculation: aws.RequestChecksumCalculationWhenRequired,
			ResponseChecksumValidation: aws.ResponseChecksumValidationWhenRequired,
		})
		return &s3Store{client: client, bucket: cfg.Bucket}, nil
	}
}

func (s *s3Store) Put(ctx context.Context, key, contentType string, body []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(s.bucket),
		Key:           aws.String(key),
		Body:          bytes.NewReader(body),
		ContentLength: aws.Int64(int64(len(body))),
		ContentType:   aws.String(contentType),
	})
	return err
}

func (s *s3Store) Delete(ctx context.Context, key string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(s.bucket), Key: aws.String(key)})
	return err
}

func (s *s3Store) PresignGet(ctx context.Context, key string, ttl time.Duration, contentType, disposition string) (string, error) {
	req, err := s3.NewPresignClient(s.client).PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket:                     aws.String(s.bucket),
		Key:                        aws.String(key),
		ResponseContentType:        aws.String(contentType),
		ResponseContentDisposition: aws.String(disposition),
	}, s3.WithPresignExpires(ttl))
	if err != nil {
		return "", err
	}
	return req.URL, nil
}

// Probe proves the keys can write and delete objects, which is all Fronko needs.
// HeadBucket runs first because it gives the clearest "bucket not found", but
// keys scoped to objects only (R2 "Object Read & Write", AWS policies without
// s3:ListBucket) may be refused it; that alone isn't a failure.
func (s *s3Store) Probe(ctx context.Context) error {
	if _, err := s.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: aws.String(s.bucket)}); err != nil && !isForbidden(err) {
		return err
	}
	if err := s.Put(ctx, probeKey, "text/plain", []byte("ok")); err != nil {
		return err
	}
	return s.Delete(ctx, probeKey)
}

// isForbidden reports a 403, which HEAD responses carry without an error body.
func isForbidden(err error) bool {
	var respErr *smithyhttp.ResponseError
	if errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusForbidden {
		return true
	}
	var apiErr smithy.APIError
	return errors.As(err, &apiErr) && (apiErr.ErrorCode() == "AccessDenied" || apiErr.ErrorCode() == "Forbidden")
}

// Describe turns a provider or network error into a short message that is
// safe to show users; raw SDK errors can include request IDs and internals.
func Describe(err error) string {
	if err == nil {
		return ""
	}
	if errors.Is(err, ErrPrivateEndpoint) {
		return "the endpoint points at a private or local address"
	}
	if errors.Is(err, ErrInvalidEndpoint) {
		return strings.TrimPrefix(err.Error(), "storage: ")
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchBucket", "NotFound":
			return "bucket not found; check the bucket name and region"
		case "AccessDenied", "Forbidden", "AllAccessDisabled":
			return "access denied; the keys need read and write access to this bucket"
		case "InvalidAccessKeyId", "InvalidToken":
			return "the access key ID isn't recognised"
		case "SignatureDoesNotMatch":
			return "the secret access key is wrong"
		case "AuthorizationHeaderMalformed", "PermanentRedirect", "IllegalLocationConstraintException":
			return "wrong region for this bucket"
		}
		return "the storage provider rejected the request (" + apiErr.ErrorCode() + ")"
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return "the endpoint timed out"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return "the endpoint's host name couldn't be resolved"
	}
	var opErr *net.OpError
	if errors.As(err, &opErr) {
		return "couldn't connect to the endpoint"
	}
	return "couldn't reach the storage provider"
}

// ReadAllLimited reads r, failing if it is longer than limit bytes.
func ReadAllLimited(r io.Reader, limit int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, ErrTooLarge
	}
	return data, nil
}

var ErrTooLarge = errors.New("storage: file too large")
