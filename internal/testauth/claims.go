package testauth

import "github.com/go-jose/go-jose/v4/jwt"

// ClaimOption is a claim option definition.
type ClaimOption func(*jwt.Claims)

// Subject lets you specify a subject claim option.
func Subject(v string) ClaimOption {
	return func(c *jwt.Claims) {
		c.Subject = v
	}
}

// Audience lets you specify an audience claim option.
func Audience(v ...string) ClaimOption {
	return func(c *jwt.Claims) {
		c.Audience = jwt.Audience(v)
	}
}

// Expiry lets you specify an expiry claim option.
func Expiry(v *jwt.NumericDate) ClaimOption {
	return func(c *jwt.Claims) {
		c.Expiry = v
	}
}

// NotBefore lets you specify a not before claim option.
func NotBefore(v *jwt.NumericDate) ClaimOption {
	return func(c *jwt.Claims) {
		c.NotBefore = v
	}
}
