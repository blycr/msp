package media

import (
	"bytes"
	"context"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"msp/internal/config"
	"msp/internal/types"
	"msp/internal/util"
)

// WalkCallback is called for each valid media item found.
// root is the share root path.
type WalkCallback func(item types.MediaItem, path string, root string) error

// WalkShares walks through all shares and invokes callback for each valid media item.
// It respects blacklist and limit.
func WalkShares(ctx context.Context, shares []config.Share, blacklist config.BlacklistConfig, maxItems int, cb WalkCallback) error {
	limit := maxItems
	if limit <= 0 {
		limit = 100000
	}
	seen := 0
	dirCache := make(map[string][]fs.DirEntry)

	for _, sh := range shares {
		root := util.NormalizePath(sh.Path)
		if root == "" || !util.IsExistingDir(root) {
			continue
		}

		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			select {
			case <-ctx.Done():
				return ctx.Err()
			default:
			}

			if err != nil {
				return nil
			}
			if seen >= limit {
				return fs.SkipAll
			}

			if d.IsDir() {
				name := d.Name()
				if name == "" {
					return nil
				}
				if strings.HasPrefix(name, ".") {
					return fs.SkipDir
				}
				if IsBlockedString(blacklist.Folders, name) {
					return fs.SkipDir
				}
				return nil
			}

			ext := strings.ToLower(filepath.Ext(d.Name()))
			if ext == "" {
				return nil
			}
			if IsBlockedString(blacklist.Extensions, ext) {
				return nil
			}
			if IsBlockedString(blacklist.Filenames, d.Name()) {
				return nil
			}
			if IsSubtitleExt(ext) || IsLyricsExt(ext) {
				return nil
			}

			fi, statErr := d.Info()
			if statErr != nil {
				return nil
			}

			if IsBlockedSize(fi.Size(), blacklist.SizeRule) {
				return nil
			}

			kind := ClassifyExt(ext)
			item := types.MediaItem{
				ID:         util.EncodeID(p),
				Name:       d.Name(),
				Ext:        ext,
				Kind:       kind,
				ShareLabel: sh.Label,
				Size:       fi.Size(),
				ModTime:    fi.ModTime().Unix(),
			}

			if kind == "video" {
				item.Subtitles = FindSidecarSubtitlesCached(p, dirCache)
			}
			if kind == "audio" {
				cover, lyrics := FindAudioSidecarsCached(p, dirCache)
				if cover != "" {
					item.CoverID = util.EncodeID(cover)
				}
				if lyrics != "" {
					item.LyricsID = util.EncodeID(lyrics)
				}
			}

			seen++
			return cb(item, p, root)
		})

		if err == fs.SkipAll {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func IsBlockedString(list []string, target string) bool {
	targetLower := strings.ToLower(target)
	for _, rule := range list {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}

		if strings.HasPrefix(rule, "/") && strings.HasSuffix(rule, "/") && len(rule) > 2 {
			pattern := rule[1 : len(rule)-1]
			if matched, _ := regexp.MatchString(pattern, target); matched {
				return true
			}
			continue
		}

		if strings.EqualFold(rule, target) || strings.EqualFold(rule, targetLower) {
			return true
		}
	}
	return false
}

func IsBlockedSize(size int64, rule string) bool {
	rule = strings.TrimSpace(strings.ToUpper(rule))
	if rule == "" {
		return false
	}
	if parts := strings.Split(rule, "-"); len(parts) == 2 {
		min := util.ParseSize(parts[0])
		max := util.ParseSize(parts[1])
		if min >= 0 && max > 0 {
			return size >= min && size <= max
		}
	}
	if strings.HasPrefix(rule, ">=") {
		return size >= util.ParseSize(strings.TrimPrefix(rule, ">="))
	}
	if strings.HasPrefix(rule, "<=") {
		return size <= util.ParseSize(strings.TrimPrefix(rule, "<="))
	}
	if strings.HasPrefix(rule, ">") {
		return size > util.ParseSize(strings.TrimPrefix(rule, ">"))
	}
	if strings.HasPrefix(rule, "<") {
		return size < util.ParseSize(strings.TrimPrefix(rule, "<"))
	}
	return false
}

func ClassifyExt(ext string) string {
	switch ext {
	case ".mp4", ".webm", ".mkv", ".mov", ".avi", ".m4v":
		return "video"
	case ".mp3", ".aac", ".wav", ".flac", ".m4a", ".ogg", ".opus":
		return "audio"
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg":
		return "image"
	default:
		return "other"
	}
}

func IsSubtitleExt(ext string) bool {
	return ext == ".vtt" || ext == ".srt"
}

func IsLyricsExt(ext string) bool {
	return ext == ".lrc"
}

func FindSidecarSubtitles(mediaAbs string) []types.Subtitle {
	return FindSidecarSubtitlesCached(mediaAbs, make(map[string][]fs.DirEntry))
}

func FindSidecarSubtitlesCached(mediaAbs string, cache map[string][]fs.DirEntry) []types.Subtitle {
	dir := filepath.Dir(mediaAbs)
	base := strings.TrimSuffix(filepath.Base(mediaAbs), filepath.Ext(mediaAbs))
	ents, ok := cache[dir]
	if !ok {
		var err error
		ents, err = os.ReadDir(dir)
		if err != nil {
			return nil
		}
		cache[dir] = ents
	}
	baseLower := strings.ToLower(base)
	var out []types.Subtitle

	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		low := strings.ToLower(name)
		ext := strings.ToLower(filepath.Ext(low))
		if ext != ".vtt" && ext != ".srt" {
			continue
		}
		stem := strings.TrimSuffix(low, ext)
		token := ""
		if stem == baseLower {
			token = ""
		} else if strings.HasPrefix(stem, baseLower+".") {
			token = strings.TrimPrefix(stem, baseLower+".")
		} else {
			continue
		}
		abs := filepath.Join(dir, name)
		id := util.EncodeID(abs)
		src := "/api/stream?id=" + id
		if ext == ".srt" {
			src = "/api/subtitle?id=" + id
		}
		lang := "zh"
		label := "字幕"
		if token != "" {
			lang = token
			label = SubtitleLabel(token)
		}
		out = append(out, types.Subtitle{ID: id, Label: label, Lang: lang, Src: src})
	}
	if len(out) == 0 {
		return nil
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Lang == "zh" && out[j].Lang != "zh" {
			return true
		}
		if out[i].Lang != "zh" && out[j].Lang == "zh" {
			return false
		}
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})
	out[0].Default = true
	return out
}

func SubtitleLabel(token string) string {
	t := strings.ToLower(strings.TrimSpace(token))
	switch t {
	case "zh", "zh-cn", "zh-hans":
		return "中文"
	case "zh-tw", "zh-hant":
		return "繁體"
	case "en", "en-us", "en-gb":
		return "English"
	case "ja", "jp":
		return "日本語"
	case "ko":
		return "한국어"
	case "fr":
		return "Français"
	case "de":
		return "Deutsch"
	case "es":
		return "Español"
	case "ru":
		return "Русский"
	default:
		return token
	}
}

func FindAudioSidecarsCached(mediaAbs string, cache map[string][]fs.DirEntry) (coverAbs string, lyricsAbs string) {
	dir := filepath.Dir(mediaAbs)
	base := strings.TrimSuffix(filepath.Base(mediaAbs), filepath.Ext(mediaAbs))
	ents, ok := cache[dir]
	if !ok {
		var err error
		ents, err = os.ReadDir(dir)
		if err != nil {
			return "", ""
		}
		cache[dir] = ents
	}
	baseLower := strings.ToLower(base)
	bestLrc := ""
	langLrc := ""
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		low := strings.ToLower(name)
		ext := strings.ToLower(filepath.Ext(low))
		if ext == ".lrc" {
			stem := strings.TrimSuffix(low, ext)
			if stem == baseLower {
				bestLrc = name
			} else if strings.HasPrefix(stem, baseLower+".") && langLrc == "" {
				langLrc = name
			} else if bestLrc == "" && langLrc == "" {
				bestLrc = name
			}
		}
	}
	candidate := bestLrc
	if candidate == "" {
		candidate = langLrc
	}
	if candidate != "" {
		lyricsAbs = filepath.Join(dir, candidate)
	}

	candidates := []string{
		baseLower + ".jpg", baseLower + ".jpeg", baseLower + ".png", baseLower + ".webp",
		"cover.jpg", "folder.jpg", "front.jpg", "album.jpg", "albumart.jpg",
	}
	for _, c := range candidates {
		for _, e := range ents {
			if !e.IsDir() && strings.EqualFold(e.Name(), c) {
				coverAbs = filepath.Join(dir, e.Name())
				goto done
			}
		}
	}
done:
	return coverAbs, lyricsAbs
}

func SniffContainerCodecs(fileAbs string, ext string) (string, string) {
	f, err := os.Open(fileAbs)
	if err != nil {
		return "", ""
	}
	defer f.Close()
	const max = 2 << 20
	head, err := io.ReadAll(io.LimitReader(f, max))
	if err != nil || len(head) == 0 {
		return "", ""
	}
	b := head
	if st, err := f.Stat(); err == nil && st.Size() > max {
		tailSize := int64(max)
		if st.Size() < tailSize {
			tailSize = st.Size()
		}
		tail := make([]byte, tailSize)
		_, _ = f.ReadAt(tail, st.Size()-tailSize)
		b = append(head, tail...)
	}
	video := ""
	var audioParts []string
	has := func(s string) bool { return bytes.Contains(b, []byte(s)) }
	if ext == ".mkv" {
		if has("V_MPEGH/ISO/HEVC") {
			video = "H.265/HEVC"
		} else if has("V_MPEG4/ISO/AVC") {
			video = "H.264/AVC"
		} else if has("V_AV1") {
			video = "AV1"
		} else if has("V_VP9") {
			video = "VP9"
		}
		if has("A_EAC3") {
			audioParts = append(audioParts, "E-AC-3")
		} else if has("A_AC3") {
			audioParts = append(audioParts, "AC-3")
		} else if has("A_OPUS") {
			audioParts = append(audioParts, "Opus")
		} else if has("A_AAC") {
			audioParts = append(audioParts, "AAC")
		} else if has("A_VORBIS") {
			audioParts = append(audioParts, "Vorbis")
		} else if has("A_FLAC") {
			audioParts = append(audioParts, "FLAC")
		} else if has("A_DTS") {
			audioParts = append(audioParts, "DTS")
		} else if has("A_TRUEHD") {
			audioParts = append(audioParts, "TrueHD")
		}
		return video, strings.Join(audioParts, " + ")
	}
	if ext == ".mp4" || ext == ".m4v" || ext == ".mov" {
		if has("hvc1") || has("hev1") {
			video = "H.265/HEVC"
		} else if has("avc1") {
			video = "H.264/AVC"
		} else if has("av01") {
			video = "AV1"
		} else if has("vp09") {
			video = "VP9"
		}
		if has("ec-3") {
			audioParts = append(audioParts, "E-AC-3")
		} else if has("ac-3") {
			audioParts = append(audioParts, "AC-3")
		} else if has("mp4a") {
			audioParts = append(audioParts, "AAC/MP4A")
		} else if has("opus") {
			audioParts = append(audioParts, "Opus")
		}
		return video, strings.Join(audioParts, " + ")
	}
	return "", ""
}

func SrtToVtt(in []byte) []byte {
	in = bytes.TrimPrefix(in, []byte{0xEF, 0xBB, 0xBF})
	s := strings.ReplaceAll(strings.ReplaceAll(string(in), "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(s, "\n")
	var out strings.Builder
	out.WriteString("WEBVTT\n\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			out.WriteString("\n")
			continue
		}
		if IsAllDigits(trimmed) {
			continue
		}
		if strings.Contains(line, "-->") {
			out.WriteString(strings.ReplaceAll(line, ",", "."))
			out.WriteString("\n")
			continue
		}
		out.WriteString(line)
		out.WriteString("\n")
	}
	return []byte(out.String())
}

func IsAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
