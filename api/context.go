package api

import "github.com/sirupsen/logrus"

// Context provides the API with all necessary data and interfaces for responding to requests.
//
// It is cloned before each request, allowing per-request changes such as logger annotations.
type Context struct {
	RequestID string
	Logger    logrus.FieldLogger
}

// Clone creates a shallow copy of context, allowing clones to apply per-request changes.
func (c *Context) Clone() *Context {
	return &Context{
		Logger: c.Logger,
	}
}
