package http

import (
	"context"

	"github.com/Stoganet/api-proxy/internal/gen"
)

func (s *Server) GetHome(ctx context.Context, _ gen.GetHomeRequestObject) (gen.GetHomeResponseObject, error) {
	jfUserID := jfUserIDFromCtx(ctx)

	result, err := s.library.Home(ctx, jfUserID)
	if err != nil {
		return gen.GetHome503JSONResponse(apiError(ctx, gen.BackendUnavailable, "upstream error")), nil //nolint:nilerr // error encoded in response
	}

	sections := make([]gen.HomeSection, len(result.Sections))
	for i, sec := range result.Sections {
		items := make([]gen.LibraryItem, len(sec.Items))
		for j, it := range sec.Items {
			items[j] = toGenItem(it)
		}
		sections[i] = gen.HomeSection{
			Id:      sec.ID,
			Items:   items,
			HasMore: sec.HasMore,
		}
	}

	return gen.GetHome200JSONResponse(gen.HomeResponse{Sections: sections}), nil
}
