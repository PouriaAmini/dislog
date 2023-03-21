// Package auth provides authorization functionality using Casbin.
package auth

import (
	"fmt"

	"github.com/casbin/casbin"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// New returns a new Authorizer with the given Casbin model and policy.
func New(model, policy string) *Authorizer {
	enforcer := casbin.NewEnforcer(model, policy)
	return &Authorizer{
		enforcer: enforcer,
	}
}

// Authorizer is an authorization module that uses Casbin to enforce access
// control.
type Authorizer struct {
	enforcer *casbin.Enforcer
}

// Authorize enforces the access control policy for the given subject,
// object and action.
// It returns an error if the access is denied, otherwise it returns nil.
func (a *Authorizer) Authorize(subject, object, action string) error {
	if !a.enforcer.Enforce(subject, object, action) {
		msg := fmt.Sprintf(
			"%s not permitted to %s to %s",
			subject,
			action,
			object,
		)
		st := status.New(codes.PermissionDenied, msg)
		return st.Err()
	}
	return nil
}
