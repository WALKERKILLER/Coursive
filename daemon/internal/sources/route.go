package sources

import "context"

// Router finds the provider that owns a library course.
type Router struct {
	Registry *Registry
	// SourceOf returns a course's recorded source id; "" when the course
	// predates sources.
	SourceOf func(ctx context.Context, course string) (string, error)
}

// For returns the provider for a course.
func (r Router) For(ctx context.Context, course string) (*Provider, error) {
	source, err := r.SourceOf(ctx, course)
	if err != nil {
		return nil, err
	}
	return r.Registry.ForCourse(source)
}
