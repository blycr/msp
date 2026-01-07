package media

import (
	"sort"
	"strings"

	"msp/internal/config"
	"msp/internal/types"
)

func BuildMediaResponse(shares []config.Share, blacklist config.BlacklistConfig, maxItems int) types.MediaResponse {
	// Initialize DB if needed (should be done at app start, but ensuring here for safety)
	// In a real app, db.Init should be called in main.go

	resp := types.MediaResponse{
		Shares: make([]config.Share, len(shares)),
		Videos: []types.MediaItem{},
		Audios: []types.MediaItem{},
		Images: []types.MediaItem{},
		Others: []types.MediaItem{},
	}
	copy(resp.Shares, shares)

	var allItems []types.MediaItem
	cb := func(item types.MediaItem, path string, root string) error {
		allItems = append(allItems, item)
		return nil
	}

	// We ignore error here as WalkShares only returns error on file system issues or explicit stop,
	// and we want to return whatever we found.
	_ = WalkShares(shares, blacklist, maxItems, cb)

	sort.Slice(allItems, func(i, j int) bool {
		if allItems[i].Kind != allItems[j].Kind {
			return allItems[i].Kind < allItems[j].Kind
		}
		if allItems[i].ShareLabel != allItems[j].ShareLabel {
			return allItems[i].ShareLabel < allItems[j].ShareLabel
		}
		return strings.ToLower(allItems[i].Name) < strings.ToLower(allItems[j].Name)
	})

	for _, item := range allItems {
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
	}
	return resp
}
