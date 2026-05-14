package media

import (
	"context"
	"sort"
	"strings"

	"msp/internal/config"
	"msp/internal/domain"
	"msp/internal/scanner"
)

func newMediaResponse(shares []domain.Share) domain.MediaResponse {
	return domain.MediaResponse{
		Shares: make([]domain.Share, len(shares)),
		Videos: []domain.MediaItem{},
		Audios: []domain.MediaItem{},
		Images: []domain.MediaItem{},
		Others: []domain.MediaItem{},
	}
}

func BuildMediaResponse(ctx context.Context, shares []domain.Share, blacklist config.BlacklistConfig, maxItems int) (domain.MediaResponse, error) {
	resp := newMediaResponse(shares)
	copy(resp.Shares, shares)

	cb := func(item domain.MediaItem, _, _ string) error {
		switch item.Kind {
		case "video":
			resp.Videos = append(resp.Videos, item)
		case "audio":
			resp.Audios = append(resp.Audios, item)
		case "image":
			resp.Images = append(resp.Images, item)
		default:
			resp.Others = append(resp.Others, item)
		}
		return nil
	}

	if err := scanner.WalkShares(ctx, shares, blacklist, maxItems, cb); err != nil {
		return domain.MediaResponse{}, err
	}

	sortItems := func(items []domain.MediaItem) {
		sort.Slice(items, func(i, j int) bool {
			if items[i].ShareLabel != items[j].ShareLabel {
				return items[i].ShareLabel < items[j].ShareLabel
			}
			return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		})
	}
	sortItems(resp.Videos)
	sortItems(resp.Audios)
	sortItems(resp.Images)
	sortItems(resp.Others)
	return resp, nil
}