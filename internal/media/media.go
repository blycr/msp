package media

import (
	"bytes"
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

func BuildMediaResponse(shares []config.Share, blacklist config.BlacklistConfig) types.MediaResponse {
	resp := types.MediaResponse{
		Shares: make([]interface{}, len(shares)),
		Videos: []types.MediaItem{},
		Audios: []types.MediaItem{},
		Images: []types.MediaItem{},
		Others: []types.MediaItem{},
	}
	for i, s := range shares {
		resp.Shares[i] = s
	}

	type gathered struct {
		item types.MediaItem
	}
	var items []gathered

	const maxItems = 8000
	for _, sh := range shares {
		root := util.NormalizeWinPath(sh.Path)
		if root == "" || !util.IsExistingDir(root) {
			continue
		}
		filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if len(items) >= maxItems {
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
				item.Subtitles = FindSidecarSubtitles(p)
			}
			if kind == "audio" {
				cover, lyrics := FindAudioSidecars(p)
				if cover != "" {
					item.CoverID = util.EncodeID(cover)
				}
				if lyrics != "" {
					item.LyricsID = util.EncodeID(lyrics)
				}
			}

			items = append(items, gathered{item: item})
			return nil
		})
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].item.Kind != items[j].item.Kind {
			return items[i].item.Kind < items[j].item.Kind
		}
		if items[i].item.ShareLabel != items[j].item.ShareLabel {
			return items[i].item.ShareLabel < items[j].item.ShareLabel
		}
		return strings.ToLower(items[i].item.Name) < strings.ToLower(items[j].item.Name)
	})

	for _, g := range items {
		switch g.item.Kind {
		case "video":
			resp.Videos = append(resp.Videos, g.item)
		case "audio":
			resp.Audios = append(resp.Audios, g.item)
		case "image":
			resp.Images = append(resp.Images, g.item)
		default:
			resp.Others = append(resp.Others, g.item)
		}
	}
	return resp
}

func IsBlockedString(list []string, target string) bool {
	targetLower := strings.ToLower(target)
	for _, rule := range list {
		rule = strings.TrimSpace(rule)
		if rule == "" {
			continue
		}

		// Regex support: /pattern/
		if strings.HasPrefix(rule, "/") && strings.HasSuffix(rule, "/") && len(rule) > 2 {
			pattern := rule[1 : len(rule)-1]
			if matched, _ := regexp.MatchString(pattern, target); matched {
				return true
			}
			continue
		}

		if strings.EqualFold(rule, target) {
			return true
		}
		if strings.EqualFold(rule, targetLower) {
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
		val := util.ParseSize(strings.TrimPrefix(rule, ">="))
		return size >= val
	}
	if strings.HasPrefix(rule, "<=") {
		val := util.ParseSize(strings.TrimPrefix(rule, "<="))
		return size <= val
	}
	if strings.HasPrefix(rule, ">") {
		val := util.ParseSize(strings.TrimPrefix(rule, ">"))
		return size > val
	}
	if strings.HasPrefix(rule, "<") {
		val := util.ParseSize(strings.TrimPrefix(rule, "<"))
		return size < val
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
	switch ext {
	case ".vtt", ".srt":
		return true
	default:
		return false
	}
}

func IsLyricsExt(ext string) bool {
	switch ext {
	case ".lrc":
		return true
	default:
		return false
	}
}

func FindSidecarSubtitles(mediaAbs string) []types.Subtitle {
	dir := filepath.Dir(mediaAbs)
	base := strings.TrimSuffix(filepath.Base(mediaAbs), filepath.Ext(mediaAbs))
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
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

		out = append(out, types.Subtitle{
			ID:    id,
			Label: label,
			Lang:  lang,
			Src:   src,
		})
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

func FindAudioSidecars(mediaAbs string) (coverAbs string, lyricsAbs string) {
	dir := filepath.Dir(mediaAbs)
	base := strings.TrimSuffix(filepath.Base(mediaAbs), filepath.Ext(mediaAbs))

	lyrics := filepath.Join(dir, base+".lrc")
	if st, err := os.Stat(lyrics); err == nil && !st.IsDir() {
		lyricsAbs = lyrics
	}

	candidates := []string{
		filepath.Join(dir, base+".jpg"),
		filepath.Join(dir, base+".jpeg"),
		filepath.Join(dir, base+".png"),
		filepath.Join(dir, base+".webp"),
		filepath.Join(dir, "cover.jpg"),
		filepath.Join(dir, "folder.jpg"),
		filepath.Join(dir, "front.jpg"),
		filepath.Join(dir, "album.jpg"),
		filepath.Join(dir, "albumart.jpg"),
	}
	for _, p := range candidates {
		st, err := os.Stat(p)
		if err == nil && !st.IsDir() {
			coverAbs = p
			break
		}
	}
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
	if st, statErr := f.Stat(); statErr == nil {
		size := st.Size()
		if size > max {
			tailSize := int64(max)
			if size < tailSize {
				tailSize = size
			}
			tail := make([]byte, tailSize)
			_, _ = f.ReadAt(tail, size-tailSize)
			b = append(head, tail...)
		}
	}

	video := ""
	audioParts := make([]string, 0, 2)

	has := func(s string) bool {
		return bytes.Contains(b, []byte(s))
	}

	if ext == ".mkv" {
		switch {
		case has("V_MPEGH/ISO/HEVC"):
			video = "H.265/HEVC"
		case has("V_MPEG4/ISO/AVC"):
			video = "H.264/AVC"
		case has("V_AV1"):
			video = "AV1"
		case has("V_VP9"):
			video = "VP9"
		}
		switch {
		case has("A_EAC3"):
			audioParts = append(audioParts, "E-AC-3")
		case has("A_AC3"):
			audioParts = append(audioParts, "AC-3")
		case has("A_OPUS"):
			audioParts = append(audioParts, "Opus")
		case has("A_AAC"):
			audioParts = append(audioParts, "AAC")
		case has("A_VORBIS"):
			audioParts = append(audioParts, "Vorbis")
		case has("A_FLAC"):
			audioParts = append(audioParts, "FLAC")
		case has("A_DTS"):
			audioParts = append(audioParts, "DTS")
		case has("A_TRUEHD"):
			audioParts = append(audioParts, "TrueHD")
		}
		return video, strings.Join(audioParts, " + ")
	}

	if ext == ".mp4" || ext == ".m4v" || ext == ".mov" {
		switch {
		case has("hvc1") || has("hev1"):
			video = "H.265/HEVC"
		case has("avc1"):
			video = "H.264/AVC"
		case has("av01"):
			video = "AV1"
		case has("vp09"):
			video = "VP9"
		}
		switch {
		case has("ec-3"):
			audioParts = append(audioParts, "E-AC-3")
		case has("ac-3"):
			audioParts = append(audioParts, "AC-3")
		case has("mp4a"):
			audioParts = append(audioParts, "AAC/MP4A")
		case has("opus"):
			audioParts = append(audioParts, "Opus")
		}
		return video, strings.Join(audioParts, " + ")
	}

	return "", ""
}

func SrtToVtt(in []byte) []byte {
	in = bytes.TrimPrefix(in, []byte{0xEF, 0xBB, 0xBF})
	s := string(in)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
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
