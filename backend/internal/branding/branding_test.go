package branding

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestValidateBranding(t *testing.T) {
	ok := func(b OrgBranding) OrgBranding {
		t.Helper()
		assert.Empty(t, validateBranding(&b))
		return b
	}
	bad := func(b OrgBranding) {
		t.Helper()
		assert.NotEmpty(t, validateBranding(&b))
	}

	b := ok(OrgBranding{})
	assert.Equal(t, LogoOptional, b.LogoPolicy, "defaults to optional")
	blank := " "
	b = ok(OrgBranding{LogoFile: &blank, LogoPolicy: LogoRequired})
	assert.Nil(t, b.LogoFile, "a blank logo is no logo")
	b = ok(OrgBranding{Signature: OrgSignature{
		LockedTemplate: "corporate", BrandColor: "#1A2b3C", Disclaimer: "  Confidential.  ", BannerURL: "https://acme.test/x",
	}})
	assert.Equal(t, "Confidential.", b.Signature.Disclaimer)

	bad(OrgBranding{LogoPolicy: "sometimes"})
	bad(OrgBranding{Signature: OrgSignature{LockedTemplate: "fancy"}})
	bad(OrgBranding{Signature: OrgSignature{BrandColor: "red"}})
	bad(OrgBranding{Signature: OrgSignature{BrandColor: "#fff"}})
	bad(OrgBranding{Signature: OrgSignature{Disclaimer: strings.Repeat("a", 1001)}})
	bad(OrgBranding{Signature: OrgSignature{BannerURL: "javascript:alert(1)"}})
	bad(OrgBranding{Signature: OrgSignature{BannerURL: "acme.test"}})
}
