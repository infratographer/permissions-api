// Package testauth implements a simple JWKS file server and token signer
// for use in test packages when jwt validation is required.
package testauth

import (
	"crypto/rand"
	"crypto/rsa"
	"errors"
	"fmt"
	"net"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v4"
	"github.com/go-jose/go-jose/v4/jwt"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

const (
	keySize = 2048
)

// Server handles serving JSON Web Key Set and signing tokens.
type Server struct {
	t *testing.T

	started    sync.Once
	hasStarted bool

	cleanup []func()
	stopped sync.Once

	engine *echo.Echo

	kid     string
	privKey *rsa.PrivateKey

	signer jose.Signer

	Issuer string
}

// Start starts an unstarted server.
func (s *Server) Start() {
	s.started.Do(func() {
		e := echo.New()

		e.GET("/.well-known/openid-configuration", s.handleOIDC)
		e.GET("/.well-known/jwks.json", s.handleJWKS)

		listener, err := net.Listen("tcp", ":0")

		require.NoError(s.t, err)

		srv := &http.Server{
			Handler: e,
		}

		go s.serve(srv, listener)

		s.cleanup = append(s.cleanup, func() {
			_ = srv.Close() //nolint:errcheck // error check not needed
		})

		s.engine = e
		s.Issuer = fmt.Sprintf("http://127.0.0.1:%d", listener.Addr().(*net.TCPAddr).Port)
		s.hasStarted = true
	})
}

// Stop shuts down the auth server.
func (s *Server) Stop() {
	// Don't stop unless we've started.
	if !s.hasStarted {
		return
	}

	s.stopped.Do(func() {
		for i := len(s.cleanup) - 1; i > 0; i-- {
			s.cleanup[i]()
		}
	})
}

func (s *Server) serve(srv *http.Server, listener net.Listener) {
	err := srv.Serve(listener)

	switch {
	case err == nil:
	case errors.Is(err, http.ErrServerClosed):
	default:
		s.t.Error("unexpected error from Server:", err)
		s.t.Fail()
	}
}

func (s *Server) handleOIDC(c echo.Context) error {
	return c.JSON(http.StatusOK, echo.Map{
		"jwks_uri": s.Issuer + "/.well-known/jwks.json",
	})
}

func (s *Server) handleJWKS(c echo.Context) error {
	return c.JSON(http.StatusOK, jose.JSONWebKeySet{
		Keys: []jose.JSONWebKey{
			{
				KeyID: s.kid,
				Key:   s.privKey,
			},
		},
	})
}

func (s *Server) buildClaims(options ...ClaimOption) jwt.Builder {
	claims := jwt.Claims{
		Issuer:    s.Issuer,
		NotBefore: jwt.NewNumericDate(time.Now().Add(-2 * time.Hour)),
	}

	for _, opt := range options {
		opt(&claims)
	}

	return jwt.Signed(s.signer).Claims(claims)
}

// TSignSubject returns a new token string with the provided subject.
// Additional claims may be provided as options.
// Any errors produced will result in the passed test argument failing.
func (s *Server) TSignSubject(t *testing.T, subject string, options ...ClaimOption) string {
	options = append(options, Subject(subject))

	claims := s.buildClaims(options...)

	token, err := claims.Serialize()

	require.NoError(t, err)

	return token
}

// SignSubject returns a new token string with the provided subject.
// Additional claims may be provided as options.
// Any errors produced will result in the test passed when initializing Server to fail.
func (s *Server) SignSubject(subject string, options ...ClaimOption) string {
	return s.TSignSubject(s.t, subject, options...)
}

// NewUnstartedServer creates a new Server without starting it.
func NewUnstartedServer(t *testing.T) *Server {
	t.Helper()

	kid := "test"
	key, err := rsa.GenerateKey(rand.Reader, keySize)

	require.NoError(t, err)

	signer, err := jose.NewSigner(
		jose.SigningKey{
			Algorithm: jose.RS256,
			Key:       key,
		}, (&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", kid),
	)

	require.NoError(t, err)

	return &Server{
		t:       t,
		kid:     kid,
		privKey: key,
		signer:  signer,
	}
}

// NewServer creates a new Server and starts it.
func NewServer(t *testing.T) *Server {
	s := NewUnstartedServer(t)

	s.Start()

	return s
}
