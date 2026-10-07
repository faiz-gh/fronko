package sso

import (
	"context"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/crewjam/saml"

	"github.com/faiz-gh/fronko/backend/internal/integrations"
)

// Metadata fetched from a URL is kept this long, and served stale for up
// to staleFor while the identity provider can't be reached.
const (
	metadataTTL = time.Hour
	staleFor    = 24 * time.Hour
)

type spURLs struct{ acs, metadata string }

// urlsFor are a connection's service provider URLs. They name the
// connection, not the organisation's handle, so changing the handle doesn't
// break sign-in.
func urlsFor(publicURL string, connectionID int64) spURLs {
	base := fmt.Sprintf("%s/auth/saml/%d", publicURL, connectionID)
	return spURLs{acs: base + "/acs", metadata: base + "/metadata"}
}

// loadMetadata reads the identity provider's metadata from the pasted XML,
// or else fetches it from the URL.
func loadMetadata(ctx context.Context, client *http.Client, cfg integrations.Settings) (*saml.EntityDescriptor, error) {
	if xml := strings.TrimSpace(cfg.String("metadata_xml")); xml != "" {
		return parseIDPMetadata([]byte(xml))
	}
	raw := cfg.String("metadata_url")
	if raw == "" {
		return nil, errors.New("no identity provider metadata is set")
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, raw, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		var ue *url.Error
		if errors.As(err, &ue) {
			err = ue.Err
		}
		return nil, fmt.Errorf("couldn't fetch the metadata: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, integrations.WithDetail(fmt.Errorf("fetching the metadata answered %d", resp.StatusCode),
			map[string]any{"status": resp.StatusCode})
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 2*maxMetadataLen))
	if err != nil {
		return nil, fmt.Errorf("couldn't read the metadata: %w", err)
	}
	return parseIDPMetadata(data)
}

type cachedMetadata struct {
	source  string
	md      *saml.EntityDescriptor
	fetched time.Time
}

// metadataCache keeps identity providers' metadata between sign-ins.
type metadataCache struct {
	mu      sync.Mutex
	entries map[int64]cachedMetadata
	client  *http.Client
}

func newMetadataCache(client *http.Client) *metadataCache {
	return &metadataCache{entries: map[int64]cachedMetadata{}, client: client}
}

func (m *metadataCache) get(ctx context.Context, link *integrations.Linked) (*saml.EntityDescriptor, error) {
	cfg := link.Settings
	sum := sha256.Sum256([]byte(cfg.String("metadata_url") + "\x00" + cfg.String("metadata_xml")))
	source := string(sum[:])
	id := link.Connection.ID

	m.mu.Lock()
	cached, ok := m.entries[id]
	m.mu.Unlock()
	if ok && cached.source == source && time.Since(cached.fetched) < metadataTTL {
		return cached.md, nil
	}
	md, err := loadMetadata(ctx, m.client, cfg)
	if err != nil {
		if ok && cached.source == source && time.Since(cached.fetched) < staleFor {
			return cached.md, nil
		}
		return nil, err
	}
	m.mu.Lock()
	m.entries[id] = cachedMetadata{source: source, md: md, fetched: time.Now()}
	m.mu.Unlock()
	return md, nil
}

// serviceProvider is Fronko as the service provider for one connection.
func serviceProvider(link *integrations.Linked, publicURL string, md *saml.EntityDescriptor, client *http.Client) (*saml.ServiceProvider, error) {
	key, cert, err := keyPair(link.Settings)
	if err != nil {
		return nil, err
	}
	urls := urlsFor(publicURL, link.Connection.ID)
	acs, _ := url.Parse(urls.acs)
	metadata, _ := url.Parse(urls.metadata)
	return &saml.ServiceProvider{
		EntityID:    urls.metadata,
		Key:         key,
		Certificate: cert,
		HTTPClient:  client,
		MetadataURL: *metadata,
		AcsURL:      *acs,
		IDPMetadata: md,
		// Let the identity provider send the name ID it's configured with;
		// the email is read from it or from the attributes.
		AuthnNameIDFormat:     saml.UnspecifiedNameIDFormat,
		AllowIDPInitiated:     link.Settings.Bool("idp_initiated"),
		MetadataValidDuration: 7 * 24 * time.Hour,
	}, nil
}

func keyPair(cfg integrations.Settings) (*rsa.PrivateKey, *x509.Certificate, error) {
	kb, _ := pem.Decode([]byte(cfg.Secrets[secretKey]))
	cb, _ := pem.Decode([]byte(cfg.Secrets[secretCert]))
	if kb == nil || cb == nil {
		return nil, nil, errors.New("the connection has no key pair; delete it and set it up again")
	}
	key, err := x509.ParsePKCS1PrivateKey(kb.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("reading the key: %w", err)
	}
	cert, err := x509.ParseCertificate(cb.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("reading the certificate: %w", err)
	}
	return key, cert, nil
}

// parseCertData reads a base64 certificate from metadata.
func parseCertData(data string) (*x509.Certificate, error) {
	clean := strings.Map(func(r rune) rune {
		if r == ' ' || r == '\n' || r == '\r' || r == '\t' {
			return -1
		}
		return r
	}, data)
	der, err := base64.StdEncoding.DecodeString(clean)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(der)
}
