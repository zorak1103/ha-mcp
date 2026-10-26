//go:build integration

package integration

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/suite"
)

// SignMediaPathToolDispatchTestSuite covers sign_media_path through the real
// registry+handler+HybridClient dispatch path (CallTool) against a live Home
// Assistant. auth/sign_path only signs a string - it never checks that the
// target exists - so no camera or media file has to be present, and nothing is
// created, modified or deleted.
type SignMediaPathToolDispatchTestSuite struct {
	IntegrationTestSuite
}

func TestSignMediaPathToolDispatch(t *testing.T) {
	suite.Run(t, new(SignMediaPathToolDispatchTestSuite))
}

// signedPathClaims are the JWT claims Home Assistant embeds in the authSig
// query parameter of a signed path.
type signedPathClaims struct {
	Path string `json:"path"`
	Iat  int64  `json:"iat"`
	Exp  int64  `json:"exp"`
}

// signMediaPathResult is the JSON body sign_media_path returns on success.
type signMediaPathResult struct {
	SignedPath string `json:"signed_path"`
	ExpiresIn  string `json:"expires_in"`
}

// signOK calls sign_media_path, requires success and returns the parsed body,
// the signed URL's query parameters and the decoded authSig claims.
func (s *SignMediaPathToolDispatchTestSuite) signOK(args map[string]any) (signMediaPathResult, url.Values, signedPathClaims) {
	result := s.CallTool("sign_media_path", args)
	s.Require().False(result.IsError, "sign_media_path %v should succeed: %s", args, resultText(result))

	var body signMediaPathResult
	s.Require().NoError(json.Unmarshal([]byte(resultText(result)), &body), "response is not the expected JSON: %s", resultText(result))

	signed, err := url.Parse(body.SignedPath)
	s.Require().NoError(err, "signed_path %q is not a valid URL", body.SignedPath)

	query := signed.Query()
	authSig := query.Get("authSig")
	s.Require().NotEmpty(authSig, "signed_path %q carries no authSig", body.SignedPath)

	// A JWT is header.payload.signature; the payload is unpadded base64url JSON.
	parts := strings.Split(authSig, ".")
	s.Require().Len(parts, 3, "authSig is not a JWT: %q", authSig)
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	s.Require().NoError(err, "authSig payload is not base64url")

	var claims signedPathClaims
	s.Require().NoError(json.Unmarshal(payload, &claims), "authSig payload is not JSON: %s", payload)

	return body, query, claims
}

// TestSignsAllowedPaths covers every allowed prefix and the expiry bounds. The
// authSig claims prove the requested lifetime reached Home Assistant, not just
// that the handler echoed it back.
func (s *SignMediaPathToolDispatchTestSuite) TestSignsAllowedPaths() {
	camera := "/api/camera_proxy/camera." + GenerateTestID("cam")
	localMedia := "/media/local/" + GenerateTestID("clip") + ".mp3"

	tests := []struct {
		name        string
		args        map[string]any
		wantPath    string
		wantExpires int64
	}{
		{"camera proxy with default expires", map[string]any{"path": camera}, camera, 30},
		{"camera proxy with custom expires", map[string]any{"path": camera, "expires": float64(60)}, camera, 60},
		{"media path at minimum expires", map[string]any{"path": localMedia, "expires": float64(1)}, localMedia, 1},
		{"media path at maximum expires", map[string]any{"path": localMedia, "expires": float64(300)}, localMedia, 300},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			body, _, claims := s.signOK(tt.args)

			s.True(strings.HasPrefix(body.SignedPath, tt.wantPath+"?"), "signed_path %q should start with %q", body.SignedPath, tt.wantPath)
			s.Equal(tt.wantPath, claims.Path, "authSig must be bound to the requested path")
			s.Equal(tt.wantExpires, claims.Exp-claims.Iat, "authSig lifetime must equal the requested expires")
		})
	}
}

// TestAcceptsQueryString verifies a query string on an allowed path is accepted
// and carried into the signed URL. A neutral parameter is used on purpose:
// Home Assistant itself decides which parameters it signs (a camera_proxy
// width=640 did not come back in the signed URL on the instance this was
// written against), which is not the tool's concern.
func (s *SignMediaPathToolDispatchTestSuite) TestAcceptsQueryString() {
	camera := "/api/camera_proxy/camera." + GenerateTestID("cam")

	body, query, claims := s.signOK(map[string]any{"path": camera + "?mcptest=1"})

	s.Equal("1", query.Get("mcptest"), "query string should be kept in signed_path %q", body.SignedPath)
	s.Equal(camera, claims.Path, "the signed path excludes the query string")
}

// TestRejectsInvalidInput verifies the hardening from issue #347 end to end:
// bad arguments come back as tool errors naming the problem, with no signed URL.
func (s *SignMediaPathToolDispatchTestSuite) TestRejectsInvalidInput() {
	tests := []struct {
		name         string
		args         map[string]any
		wantContains string
	}{
		{"zero expires", map[string]any{"path": "/media/local/a.mp3", "expires": float64(0)}, "between 1 and 300"},
		{"negative expires", map[string]any{"path": "/media/local/a.mp3", "expires": float64(-30)}, "between 1 and 300"},
		{"expires above cap", map[string]any{"path": "/media/local/a.mp3", "expires": float64(301)}, "between 1 and 300"},
		{"day-long expires", map[string]any{"path": "/media/local/a.mp3", "expires": float64(86400)}, "between 1 and 300"},
		{"fractional expires", map[string]any{"path": "/media/local/a.mp3", "expires": 1.5}, "must be an integer"},
		{"arbitrary api path", map[string]any{"path": "/api/states"}, "not allowed"},
		{"api config path", map[string]any{"path": "/api/config"}, "not allowed"},
		{"absolute url", map[string]any{"path": "http://evil.example/media/local/a.mp3"}, "not allowed"},
		{"traversal out of media", map[string]any{"path": "/media/../api/states"}, "traversal"},
		{"encoded traversal out of media", map[string]any{"path": "/media/%2e%2e/api/states"}, "traversal"},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			result := s.CallTool("sign_media_path", tt.args)

			s.True(result.IsError, "sign_media_path %v must be rejected, got: %s", tt.args, resultText(result))
			s.Contains(resultText(result), tt.wantContains)
			s.NotContains(resultText(result), "authSig", "a rejected call must not return a signed URL")
		})
	}
}
