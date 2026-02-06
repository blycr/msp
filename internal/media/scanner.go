package media

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"msp/internal/config"
	"msp/internal/constants"
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
		limit = constants.DefaultScanLimit
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
			return fmt.Errorf("walk share %s: %w", sh.Label, err)
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

	ext := filepath.Ext(d.Name())
	kind := ClassifyExt(ext)
	item := types.MediaItem{
		ID:         util.EncodeID(path),
		Name:       d.Name(),
		Ext:        strings.ToLower(ext),
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

// IsBlockedString 检查目标是否匹配黑名单规则（支持正则）。
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

// IsBlockedSize 检查文件大小是否匹配黑名单规则。
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

// ClassifyExt 根据扩展名分类媒体类型（video/audio/image/other）。
// 扩展名应为小写，但此函数会处理大小写不敏感的情况。
func ClassifyExt(ext string) string {
	extLower := strings.ToLower(ext)
	switch extLower {
	case ".mp4", ".webm", ".mkv", ".mov", ".avi", ".m4v", ".wmv":
		return "video"
	case ".mp3", ".aac", ".wav", ".flac", ".m4a", ".ogg", ".opus":
		return "audio"
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".svg":
		return "image"
	}
	return "other"
}

// IsSubtitleExt 检查扩展名是否是字幕文件。
func IsSubtitleExt(ext string) bool {
	ext = strings.ToLower(ext)
	return ext == ".vtt" || ext == ".srt" || ext == ".ass" || ext == ".ssa"
}

// IsLyricsExt 检查扩展名是否是歌词文件。
func IsLyricsExt(ext string) bool {
	return ext == ".lrc"
}

// FindSidecarSubtitles 查找媒体文件的外挂字幕。
func FindSidecarSubtitles(mediaAbs string) []types.Subtitle {
	return FindSidecarSubtitlesCached(mediaAbs, make(map[string][]fs.DirEntry))
}

// FindSidecarSubtitlesCached 查找外挂字幕，使用缓存避免重复读取目录。
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

// normalizeBaseForMatch 提取视频基础名称用于字幕匹配。
// 移除常见的质量标识、年份等，以便更灵活地匹配字幕。
func normalizeBaseForMatch(base string) string {
	// 常见需要移除的后缀模式（按优先级排序）
	patterns := []string{
		// 分辨率
		`\.\d{3,4}p`, `\.\d{3,4}x\d{3,4}`,
		// 视频编码
		`\.h\.?26[45]`, `\.x\.?26[45]`, `\.av1`, `\.vp[89]`, `\.mpeg`, `\.divx`, `\.xvid`,
		// 音频编码
		`\.aac`, `\.ac3`, `\.dts`, `\.eac3`, `\.flac`, `\.mp3`,
		// 来源/发布组相关
		`\.blu-?ray`, `\.bdrip`, `\.brrip`, `\.dvd`, `\.dvdrip`, `\.web-?dl`, `\.webrip`,
		`\.hdtv`, `\.pdtv`, `\.dsr`, `\.tvrip`,
		// HDR/DV
		`\.hdr`, `\.hdr10`, `\.hdr10\+`, `\.dv`, `\.dolby`, `\.vision`,
		// 其他常见标识
		`\.repack`, `\.proper`, `\.extended`, `\.directors?\.cut`, `\.unrated`, `\.remastered`,
		`\.limited`, `\.internal`, `\.read\.nfo`, `\.subbed`, `\.dubbed`,
		// 发布组（通常在最末尾）
		`\.[a-z0-9]+$`,
	}

	result := strings.ToLower(base)
	for _, pattern := range patterns {
		re := regexp.MustCompile(`(?i)` + pattern + `$`)
		result = re.ReplaceAllString(result, "")
	}
	return result
}

// extractBaseVariants 生成可能的视频基础名称变体。
func extractBaseVariants(base string) []string {
	variants := []string{strings.ToLower(base)}
	normalized := normalizeBaseForMatch(base)
	if normalized != strings.ToLower(base) && normalized != "" {
		variants = append(variants, normalized)
	}
	return variants
}

func collectSubtitles(dir, base string, ents []fs.DirEntry) []types.Subtitle {
	baseVariants := extractBaseVariants(base)
	var out []types.Subtitle

	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		low := strings.ToLower(name)
		ext := strings.ToLower(filepath.Ext(low))
		if !IsSubtitleExt(ext) {
			continue
		}
		stem := strings.TrimSuffix(low, ext)
		token := ""
		matched := false

		// 尝试所有基础名称变体
		for _, baseVariant := range baseVariants {
			if stem == baseVariant {
				token = ""
				matched = true
				break
			} else if strings.HasPrefix(stem, baseVariant+".") {
				token = strings.TrimPrefix(stem, baseVariant+".")
				matched = true
				break
			}
		}

		if !matched {
			continue
		}

		abs := filepath.Join(dir, name)
		id := util.EncodeID(abs)
		src := "/api/stream?id=" + id
		if ext == ".srt" || ext == ".ass" || ext == ".ssa" {
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

// SubtitleLabel 将语言代码转换为显示标签。
func SubtitleLabel(token string) string {
	t := strings.ToLower(strings.TrimSpace(token))
	if v, ok := subtitleLabelMap[t]; ok {
		return v
	}
	return token
}

var subtitleLabelMap = map[string]string{
	// 简体中文
	"zh":      "中文",
	"zh-cn":   "中文",
	"zh-hans": "中文",
	"zh-chs":  "中文",
	"sc":      "简体中文",
	"chs":     "简体中文",
	"gb":      "简体中文",
	"cn":      "简体中文",
	// 繁體中文
	"zh-tw":   "繁體",
	"zh-hant": "繁體",
	"zh-cht":  "繁體",
	"tc":      "繁體中文",
	"cht":     "繁體中文",
	"hk":      "繁體中文",
	"big5":    "繁體中文",
	"tw":      "繁體中文",
	// 英语
	"en":    "English",
	"en-us": "English",
	"en-gb": "English",
	"eng":   "English",
	// 日语
	"ja":  "日本語",
	"jp":  "日本語",
	"jpn": "日本語",
	// 韩语
	"ko":  "한국어",
	"kor": "한국어",
	"kr":  "한국어",
	// 欧洲语言
	"fr":  "Français",
	"fra": "Français",
	"de":  "Deutsch",
	"ger": "Deutsch",
	"deu": "Deutsch",
	"es":  "Español",
	"spa": "Español",
	"ru":  "Русский",
	"rus": "Русский",
	// 其他亚洲语言
	"th":  "ไทย",
	"tha": "ไทย",
	"vi":  "Tiếng Việt",
	"vie": "Tiếng Việt",
	"id":  "Bahasa Indonesia",
	"ind": "Bahasa Indonesia",
	"ms":  "Bahasa Melayu",
	"may": "Bahasa Melayu",
	"tl":  "Tagalog",
	"tgl": "Tagalog",
	// 其他欧洲语言
	"it":    "Italiano",
	"ita":   "Italiano",
	"pt":    "Português",
	"por":   "Português",
	"pt-br": "Português (Brasil)",
	"nl":    "Nederlands",
	"dut":   "Nederlands",
	"nld":   "Nederlands",
	"pl":    "Polski",
	"pol":   "Polski",
	"tr":    "Türkçe",
	"tur":   "Türkçe",
	"sv":    "Svenska",
	"swe":   "Svenska",
	"da":    "Dansk",
	"dan":   "Dansk",
	"no":    "Norsk",
	"nor":   "Norsk",
	"fi":    "Suomi",
	"fin":   "Suomi",
	"cs":    "Čeština",
	"cze":   "Čeština",
	"hu":    "Magyar",
	"hun":   "Magyar",
	"el":    "Ελληνικά",
	"gre":   "Ελληνικά",
	"ell":   "Ελληνικά",
	"ar":    "العربية",
	"ara":   "العربية",
	"he":    "עברית",
	"heb":   "עברית",
	"hi":    "हिन्दी",
	"hin":   "हिन्दी",
}

// FindAudioSidecarsCached 查找音频文件的封面和歌词文件。
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

// SrtToVtt 将 SRT 格式的字幕转换为 WebVTT 格式。
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

// AssToVtt 将 ASS/SSA 格式的字幕转换为 WebVTT 格式。
func AssToVtt(in []byte) []byte {
	in = bytes.TrimPrefix(in, []byte{0xEF, 0xBB, 0xBF})
	s := strings.ReplaceAll(strings.ReplaceAll(string(in), "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(s, "\n")

	var out strings.Builder
	out.WriteString("WEBVTT\n\n")

	var inEvents bool
	var formatFields []string

	// ASS 时间格式: H:MM:SS.cc 或 HH:MM:SS.cc
	// WebVTT 时间格式: HH:MM:SS.mmm
	convertTime := func(t string) string {
		t = strings.TrimSpace(t)
		// 处理小数点或冒号分隔的百分秒
		t = strings.Replace(t, ",", ".", 1)
		parts := strings.Split(t, ":")
		if len(parts) == 3 {
			hours := parts[0]
			minutes := parts[1]
			seconds := parts[2]
			// 确保小时是两位数
			if len(hours) == 1 {
				hours = "0" + hours
			}
			// 转换百分秒到毫秒
			if dotIdx := strings.Index(seconds, "."); dotIdx != -1 {
				cs := seconds[dotIdx+1:]
				if len(cs) == 2 {
					// 百分秒转毫秒
					ms := cs + "0"
					seconds = seconds[:dotIdx+1] + ms
				} else if len(cs) == 1 {
					ms := cs + "00"
					seconds = seconds[:dotIdx+1] + ms
				} else if len(cs) > 3 {
					seconds = seconds[:dotIdx+4]
				}
			}
			return hours + ":" + minutes + ":" + seconds
		}
		return t
	}

	// 处理 ASS 格式文本（去除样式标签）
	cleanText := func(text string) string {
		// 移除 ASS 样式标签 {\...}
		re := regexp.MustCompile(`\{[^}]*\}`)
		text = re.ReplaceAllString(text, "")
		// 转换硬换行符 \N 为 WebVTT 换行
		text = strings.ReplaceAll(text, "\\N", "\n")
		text = strings.ReplaceAll(text, "\\n", "\n")
		// 移除其他 ASS 特殊序列
		text = strings.ReplaceAll(text, "\\h", " ")  // 硬空格
		text = strings.ReplaceAll(text, "\\t", "\t") // 制表符
		return text
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 检测 Events 段落
		if strings.EqualFold(trimmed, "[events]") {
			inEvents = true
			continue
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			inEvents = false
			continue
		}

		if !inEvents {
			continue
		}

		// 解析 Format 行
		if strings.HasPrefix(strings.ToUpper(trimmed), "FORMAT:") {
			formatStr := strings.TrimPrefix(line, "Format:")
			formatStr = strings.TrimPrefix(formatStr, "FORMAT:")
			formatFields = []string{}
			for _, f := range strings.Split(formatStr, ",") {
				formatFields = append(formatFields, strings.TrimSpace(strings.ToLower(f)))
			}
			continue
		}

		// 解析 Dialogue 行
		if strings.HasPrefix(strings.ToUpper(trimmed), "DIALOGUE:") {
			dialogueStr := strings.TrimPrefix(line, "Dialogue:")
			dialogueStr = strings.TrimPrefix(dialogueStr, "DIALOGUE:")

			// 找到第一个逗号后的内容（跳过 Layer 字段）
			parts := strings.SplitN(dialogueStr, ",", 10)
			if len(parts) < 10 {
				// 尝试更宽松的解析
				parts = strings.Split(dialogueStr, ",")
			}

			var startTime, endTime, text string

			if len(formatFields) >= 10 {
				// 根据 Format 字段解析
				fieldMap := make(map[string]string)
				for i, field := range formatFields {
					if i < len(parts) {
						fieldMap[field] = parts[i]
					}
				}
				startTime = fieldMap["start"]
				endTime = fieldMap["end"]
				text = fieldMap["text"]
			} else {
				// 默认解析: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
				if len(parts) >= 3 {
					startTime = parts[1]
					endTime = parts[2]
					if len(parts) >= 10 {
						text = parts[9]
					} else if len(parts) > 3 {
						text = strings.Join(parts[3:], ",")
					}
				}
			}

			if startTime != "" && endTime != "" {
				start := convertTime(startTime)
				end := convertTime(endTime)
				cleanedText := cleanText(text)

				if cleanedText != "" {
					out.WriteString(start + " --> " + end + "\n")
					out.WriteString(cleanedText + "\n\n")
				}
			}
		}
	}

	return []byte(out.String())
}

// IsAllDigits 检查字符串是否全部由数字组成。
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
