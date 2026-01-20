package media

import (
	"context"
	"sort"
	"strings"

	"msp/internal/config"
	"msp/internal/types"
)

// BuildMediaResponse 扫描共享目录并构建媒体列表响应。
func BuildMediaResponse(ctx context.Context, shares []config.Share, blacklist config.BlacklistConfig, maxItems int) types.MediaResponse {
	resp := types.MediaResponse{
		Shares: make([]config.Share, len(shares)),
		Videos: []types.MediaItem{},
		Audios: []types.MediaItem{},
		Images: []types.MediaItem{},
		Others: []types.MediaItem{},
	}
	copy(resp.Shares, shares)

	cb := func(item types.MediaItem, _, _ string) error {
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

	_ = WalkShares(ctx, shares, blacklist, maxItems, cb)

	sortItems := func(items []types.MediaItem) {
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
	return resp
}
