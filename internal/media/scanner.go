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
	w := shareWalker{
		ctx:       ctx,
		blacklist: blacklist,
		limit:     limit,
		seen:      0,
		dirCache:  make(map[string][]fs.DirEntry),
		cb:        cb,
	}

	for _, sh := range shares {
		root := util.NormalizePath(sh.Path)
		if root == "" || !util.IsExistingDir(root) {
			continue
		}

		err := w.walkShare(root, sh.Label)

		if err == fs.SkipAll {
			return nil
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type shareWalker struct {
	ctx       context.Context
	blacklist config.BlacklistConfig
	limit     int
	seen      int
	dirCache  map[string][]fs.DirEntry
	cb        WalkCallback
}

func (w *shareWalker) walkShare(root string, shareLabel string) error {
	return filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		return w.handleEntry(p, d, err, shareLabel, root)
	})
}

func (w *shareWalker) handleEntry(p string, d fs.DirEntry, err error, shareLabel string, root string) error {
	select {
	case <-w.ctx.Done():
		return w.ctx.Err()
	default:
	}

	if err != nil {
		return nil
	}
	if w.seen >= w.limit {
		return fs.SkipAll
	}

	if d.IsDir() {
		if shouldSkipDir(d.Name(), w.blacklist) {
			return fs.SkipDir
		}
		return nil
	}

	if shouldSkipFile(d, w.blacklist) {
		return nil
	}

	item, err := buildMediaItem(p, d, shareLabel, w.dirCache)
	if err != nil {
		return nil
	}

	w.seen++
	return w.cb(item, p, root)
}

func shouldSkipDir(name string, blacklist config.BlacklistConfig) bool {
	if name == "" {
		return false
	}
	if strings.HasPrefix(name, ".") {
		return true
	}
	if IsBlockedString(blacklist.Folders, name) {
		return true
	}
	return false
}

func shouldSkipFile(d fs.DirEntry, blacklist config.BlacklistConfig) bool {
	name := d.Name()
	ext := strings.ToLower(filepath.Ext(name))
	if ext == "" {
		return true
	}
	if IsBlockedString(blacklist.Extensions, ext) {
		return true
	}
	if IsBlockedString(blacklist.Filenames, name) {
		return true
	}
	if IsSubtitleExt(ext) || IsLyricsExt(ext) {
		return true
	}

	fi, err := d.Info()
	if err != nil {
		return true
	}

	if IsBlockedSize(fi.Size(), blacklist.SizeRule) {
		return true
	}
	return false
}

func buildMediaItem(path string, d fs.DirEntry, shareLabel string, dirCache map[string][]fs.DirEntry) (types.MediaItem, error) {
	fi, err := d.Info()
	if err != nil {
		return types.MediaItem{}, err
	}

	ext := strings.ToLower(filepath.Ext(d.Name()))
	kind := ClassifyExt(ext)
	item := types.MediaItem{
		ID:         util.EncodeID(path),
		Name:       d.Name(),
		Ext:        ext,
		Kind:       kind,
		ShareLabel: shareLabel,
		Size:       fi.Size(),
		ModTime:    fi.ModTime().Unix(),
	}

	if kind == "video" {
		item.Subtitles = FindSidecarSubtitlesCached(path, dirCache)
	}
	if kind == "audio" {
		cover, lyrics := FindAudioSidecarsCached(path, dirCache)
		if cover != "" {
			item.CoverID = util.EncodeID(cover)
		}
		if lyrics != "" {
			item.LyricsID = util.EncodeID(lyrics)
		}
	}
	return item, nil
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

	out := collectSubtitles(dir, base, ents)
	if len(out) == 0 {
		return nil
	}
	sortSubtitles(out)
	out[0].Default = true
	return out
}

func collectSubtitles(dir, base string, ents []fs.DirEntry) []types.Subtitle {
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
	return out
}

func sortSubtitles(out []types.Subtitle) {
	sort.Slice(out, func(i, j int) bool {
		if out[i].Lang == "zh" && out[j].Lang != "zh" {
			return true
		}
		if out[i].Lang != "zh" && out[j].Lang == "zh" {
			return false
		}
		return strings.ToLower(out[i].Label) < strings.ToLower(out[j].Label)
	})
}

func SubtitleLabel(token string) string {
	t := strings.ToLower(strings.TrimSpace(token))
	if v, ok := subtitleLabelMap[t]; ok {
		return v
	}
	return token
}

var subtitleLabelMap = map[string]string{
	"zh":      "中文",
	"zh-cn":   "中文",
	"zh-hans": "中文",
	"zh-tw":   "繁體",
	"zh-hant": "繁體",
	"en":      "English",
	"en-us":   "English",
	"en-gb":   "English",
	"ja":      "日本語",
	"jp":      "日本語",
	"ko":      "한국어",
	"fr":      "Français",
	"de":      "Deutsch",
	"es":      "Español",
	"ru":      "Русский",
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

	lyricsAbs = findLyrics(dir, baseLower, ents)
	coverAbs = findCover(dir, baseLower, ents)

	return coverAbs, lyricsAbs
}

func findLyrics(dir, baseLower string, ents []fs.DirEntry) string {
	var pick lrcPicker
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		low := strings.ToLower(name)
		if strings.ToLower(filepath.Ext(low)) != ".lrc" {
			continue
		}
		pick.consider(name, strings.TrimSuffix(low, ".lrc"), baseLower)
	}
	candidate := pick.choose()
	if candidate == "" {
		return ""
	}
	return filepath.Join(dir, candidate)
}

type lrcPicker struct {
	exact string
	lang  string
	any   string
}

func (p *lrcPicker) consider(name string, stem string, baseLower string) {
	if stem == baseLower {
		p.exact = name
		return
	}
	if strings.HasPrefix(stem, baseLower+".") {
		if p.lang == "" {
			p.lang = name
		}
		return
	}
	if p.any == "" {
		p.any = name
	}
}

func (p *lrcPicker) choose() string {
	if p.exact != "" {
		return p.exact
	}
	if p.lang != "" {
		return p.lang
	}
	return p.any
}

func findCover(dir, baseLower string, ents []fs.DirEntry) string {
	candidates := []string{
		baseLower + ".jpg", baseLower + ".jpeg", baseLower + ".png", baseLower + ".webp",
		"cover.jpg", "folder.jpg", "front.jpg", "album.jpg", "albumart.jpg",
	}
	for _, c := range candidates {
		for _, e := range ents {
			if !e.IsDir() && strings.EqualFold(e.Name(), c) {
				return filepath.Join(dir, e.Name())
			}
		}
	}
	return ""
}

// SniffContainerCodecs reads the file header (and tail for MOV/MP4 atoms) to guess codecs.
// Returns (videoCodec, audioCodec).
func SniffContainerCodecs(fileAbs string, ext string) (string, string) {
	b, err := readSniffBytes(fileAbs)
	if err != nil {
		return "", ""
	}
	return sniffByExt(b, ext)
}

func readSniffBytes(fileAbs string) ([]byte, error) {
	//nolint:gosec // Safe file open for sniffing
	f, err := os.Open(fileAbs)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()

	const max = 2 << 20
	head, err := io.ReadAll(io.LimitReader(f, max))
	if err != nil || len(head) == 0 {
		if err == nil {
			err = io.EOF
		}
		return nil, err
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
	return b, nil
}

func sniffByExt(b []byte, ext string) (string, string) {
	if ext == ".mkv" {
		return sniffMKV(b)
	}
	if ext == ".mp4" || ext == ".m4v" || ext == ".mov" {
		return sniffMP4(b)
	}
	return "", ""
}

func sniffMKV(b []byte) (string, string) {
	has := func(s string) bool { return bytes.Contains(b, []byte(s)) }
	video := firstSniffMatch(has, mkvVideoSniffs)
	audio := firstSniffMatch(has, mkvAudioSniffs)
	return video, audio
}

func sniffMP4(b []byte) (string, string) {
	has := func(s string) bool { return bytes.Contains(b, []byte(s)) }
	video := firstSniffMatch(has, mp4VideoSniffs)
	audio := firstSniffMatch(has, mp4AudioSniffs)
	return video, audio
}

type sniffPattern struct {
	pattern string
	label   string
}

var mkvVideoSniffs = []sniffPattern{
	{pattern: "V_MPEGH/ISO/HEVC", label: "H.265/HEVC"},
	{pattern: "V_MPEG4/ISO/AVC", label: "H.264/AVC"},
	{pattern: "V_AV1", label: "AV1"},
	{pattern: "V_VP9", label: "VP9"},
}

var mkvAudioSniffs = []sniffPattern{
	{pattern: "A_EAC3", label: "E-AC-3"},
	{pattern: "A_AC3", label: "AC-3"},
	{pattern: "A_OPUS", label: "Opus"},
	{pattern: "A_AAC", label: "AAC"},
	{pattern: "A_VORBIS", label: "Vorbis"},
	{pattern: "A_FLAC", label: "FLAC"},
	{pattern: "A_DTS", label: "DTS"},
	{pattern: "A_TRUEHD", label: "TrueHD"},
}

var mp4VideoSniffs = []sniffPattern{
	{pattern: "hvc1", label: "H.265/HEVC"},
	{pattern: "hev1", label: "H.265/HEVC"},
	{pattern: "avc1", label: "H.264/AVC"},
	{pattern: "av01", label: "AV1"},
	{pattern: "vp09", label: "VP9"},
}

var mp4AudioSniffs = []sniffPattern{
	{pattern: "ec-3", label: "E-AC-3"},
	{pattern: "ac-3", label: "AC-3"},
	{pattern: "mp4a", label: "AAC/MP4A"},
	{pattern: "opus", label: "Opus"},
}

func firstSniffMatch(has func(string) bool, patterns []sniffPattern) string {
	for _, p := range patterns {
		if has(p.pattern) {
			return p.label
		}
	}
	return ""
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
