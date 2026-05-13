//go:build http && !memory

package httpserver

// registerMemoryRoutes is a no-op when the binary is built without the memory tag.
func (s *Server) registerMemoryRoutes() {}

// mergeOpenAPIMemoryDoc is a no-op without the memory tag (no extra paths in served OpenAPI).
func mergeOpenAPIMemoryDoc(_ *map[string]interface{}) {}
