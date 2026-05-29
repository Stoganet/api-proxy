package http

import (
	"encoding/json"
	stdhttp "net/http"

	"github.com/Stoganet/api-proxy/internal/gen"
)

// writeError is used by middleware that has access to r but cannot return
// a typed response object (e.g. jwtStrictMiddleware). Handlers use apiError
// + typed response objects instead.
func writeError(w stdhttp.ResponseWriter, r *stdhttp.Request, status int, code gen.ErrorErrorCode, message string) {
	e := apiError(r.Context(), code, message)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(e)
}
