package scanner

import (
	"bytes"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"msp/internal/domain"
	"msp/internal/util"
)

func IsSubtitleExt(ext string) bool {
	ext = strings.ToLower(ext)
	return ext == ".vtt" || ext == ".srt" || ext == ".ass" || ext == ".ssa"
}

func IsLyricsExt(ext string) bool {
	return ext == ".lrc"
}

func FindSidecarSubtitles(mediaAbs string, idCodec *util.IDCodec) []domain.Subtitle {
	return FindSidecarSubtitlesCached(mediaAbs, make(map[string][]fs.DirEntry), idCodec)
}

func FindSidecarSubtitlesCached(mediaAbs string, cache map[string][]fs.DirEntry, idCodec *util.IDCodec) []domain.Subtitle {
	if cache == nil {
		cache = make(map[string][]fs.DirEntry)
	}
	dir := filepath.Dir(mediaAbs)
	base := strings.TrimSuffix(filepath.Base(mediaAbs), filepath.Ext(mediaAbs))
	ents, ok := cache[dir]
	if !ok {
		var err error
		ents, err = os.ReadDir(dir)
		if err != nil {
			log.Printf("[WARN] read dir error for subtitles: %v", err)
			return nil
		}
		cache[dir] = ents
	}

	out := collectSubtitles(dir, base, ents, idCodec)
	if len(out) == 0 {
		return nil
	}
	sortSubtitles(out)
	out[0].Default = true
	return out
}

// 预编译的正则表达式模式
var (
	normalizePatterns = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\.\d{3,4}p$`),
		regexp.MustCompile(`(?i)\.\d{3,4}x\d{3,4}$`),
		regexp.MustCompile(`(?i)\.h\.?26[45]$`),
		regexp.MustCompile(`(?i)\.x\.?26[45]$`),
		regexp.MustCompile(`(?i)\.av1$`),
		regexp.MustCompile(`(?i)\.vp[89]$`),
		regexp.MustCompile(`(?i)\.mpeg$`),
		regexp.MustCompile(`(?i)\.divx$`),
		regexp.MustCompile(`(?i)\.xvid$`),
		regexp.MustCompile(`(?i)\.aac$`),
		regexp.MustCompile(`(?i)\.ac3$`),
		regexp.MustCompile(`(?i)\.dts$`),
		regexp.MustCompile(`(?i)\.eac3$`),
		regexp.MustCompile(`(?i)\.flac$`),
		regexp.MustCompile(`(?i)\.mp3$`),
		regexp.MustCompile(`(?i)\.blu-?ray$`),
		regexp.MustCompile(`(?i)\.bdrip$`),
		regexp.MustCompile(`(?i)\.brrip$`),
		regexp.MustCompile(`(?i)\.dvd$`),
		regexp.MustCompile(`(?i)\.dvdrip$`),
		regexp.MustCompile(`(?i)\.web-?dl$`),
		regexp.MustCompile(`(?i)\.webrip$`),
		regexp.MustCompile(`(?i)\.hdtv$`),
		regexp.MustCompile(`(?i)\.pdtv$`),
		regexp.MustCompile(`(?i)\.dsr$`),
		regexp.MustCompile(`(?i)\.tvrip$`),
		regexp.MustCompile(`(?i)\.hdr$`),
		regexp.MustCompile(`(?i)\.hdr10$`),
		regexp.MustCompile(`(?i)\.hdr10\+$`),
		regexp.MustCompile(`(?i)\.dv$`),
		regexp.MustCompile(`(?i)\.dolby$`),
		regexp.MustCompile(`(?i)\.vision$`),
		regexp.MustCompile(`(?i)\.repack$`),
		regexp.MustCompile(`(?i)\.proper$`),
		regexp.MustCompile(`(?i)\.extended$`),
		regexp.MustCompile(`(?i)\.directors?\.cut$`),
		regexp.MustCompile(`(?i)\.unrated$`),
		regexp.MustCompile(`(?i)\.remastered$`),
		regexp.MustCompile(`(?i)\.limited$`),
		regexp.MustCompile(`(?i)\.internal$`),
		regexp.MustCompile(`(?i)\.read\.nfo$`),
		regexp.MustCompile(`(?i)\.subbed$`),
		regexp.MustCompile(`(?i)\.dubbed$`),
		regexp.MustCompile(`(?i)\.[a-z0-9]+$`),
	}
)

func normalizeBaseForMatch(base string) string {
	result := strings.ToLower(base)
	for _, re := range normalizePatterns {
		result = re.ReplaceAllString(result, "")
	}
	return result
}

func extractBaseVariants(base string) []string {
	variants := []string{strings.ToLower(base)}
	normalized := normalizeBaseForMatch(base)
	if normalized != strings.ToLower(base) && normalized != "" {
		variants = append(variants, normalized)
	}
	return variants
}

func collectSubtitles(dir, base string, ents []fs.DirEntry, idCodec *util.IDCodec) []domain.Subtitle {
	baseVariants := extractBaseVariants(base)
	var out []domain.Subtitle

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
		id := idCodec.EncodeID(abs)
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
		out = append(out, domain.Subtitle{ID: id, Label: label, Lang: lang, Src: src})
	}
	return out
}

func sortSubtitles(out []domain.Subtitle) {
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
	"zh-chs":  "中文",
	"sc":      "简体中文",
	"chs":     "简体中文",
	"gb":      "简体中文",
	"cn":      "简体中文",
	"zh-tw":   "繁體",
	"zh-hant": "繁體",
	"zh-cht":  "繁體",
	"tc":      "繁體中文",
	"cht":     "繁體中文",
	"hk":      "繁體中文",
	"big5":    "繁體中文",
	"tw":      "繁體中文",
	"en":      "English",
	"en-us":   "English",
	"en-gb":   "English",
	"eng":     "English",
	"ja":      "日本語",
	"jp":      "日本語",
	"jpn":     "日本語",
	"ko":      "한국어",
	"kor":     "한국어",
	"kr":      "한국어",
	"fr":      "Français",
	"fra":     "Français",
	"de":      "Deutsch",
	"ger":     "Deutsch",
	"deu":     "Deutsch",
	"es":      "Español",
	"spa":     "Español",
	"ru":      "Русский",
	"rus":     "Русский",
	"th":      "ไทย",
	"tha":     "ไทย",
	"vi":      "Tiếng Việt",
	"vie":     "Tiếng Việt",
	"id":      "Bahasa Indonesia",
	"ind":     "Bahasa Indonesia",
	"ms":      "Bahasa Melayu",
	"may":     "Bahasa Melayu",
	"tl":      "Tagalog",
	"tgl":     "Tagalog",
	"it":      "Italiano",
	"ita":     "Italiano",
	"pt":      "Português",
	"por":     "Português",
	"pt-br":   "Português (Brasil)",
	"nl":      "Nederlands",
	"dut":     "Nederlands",
	"nld":     "Nederlands",
	"pl":      "Polski",
	"pol":     "Polski",
	"tr":      "Türkçe",
	"tur":     "Türkçe",
	"sv":      "Svenska",
	"swe":     "Svenska",
	"da":      "Dansk",
	"dan":     "Dansk",
	"no":      "Norsk",
	"nor":     "Norsk",
	"fi":      "Suomi",
	"fin":     "Suomi",
	"cs":      "Čeština",
	"cze":     "Čeština",
	"hu":      "Magyar",
	"hun":     "Magyar",
	"el":      "Ελληνικά",
	"gre":     "Ελληνικά",
	"ell":     "Ελληνικά",
	"ar":      "العربية",
	"ara":     "العربية",
	"he":      "עברית",
	"heb":     "עברית",
	"hi":      "हिन्दी",
	"hin":     "हिन्दी",
}

func FindAudioSidecarsCached(mediaAbs string, cache map[string][]fs.DirEntry) (coverAbs string, lyricsAbs string) {
	dir := filepath.Dir(mediaAbs)
	base := strings.TrimSuffix(filepath.Base(mediaAbs), filepath.Ext(mediaAbs))
	ents, ok := cache[dir]
	if !ok {
		var err error
		ents, err = os.ReadDir(dir)
		if err != nil {
			log.Printf("[WARN] read dir error for sidecars: %v", err)
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

func AssToVtt(in []byte) []byte {
	in = bytes.TrimPrefix(in, []byte{0xEF, 0xBB, 0xBF})
	s := strings.ReplaceAll(strings.ReplaceAll(string(in), "\r\n", "\n"), "\r", "\n")
	lines := strings.Split(s, "\n")

	var out strings.Builder
	out.WriteString("WEBVTT\n\n")

	var inEvents bool
	var formatFields []string

	convertTime := func(t string) string {
		t = strings.TrimSpace(t)
		t = strings.Replace(t, ",", ".", 1)
		parts := strings.Split(t, ":")
		if len(parts) == 3 {
			hours := parts[0]
			minutes := parts[1]
			seconds := parts[2]
			if len(hours) == 1 {
				hours = "0" + hours
			}
			if dotIdx := strings.Index(seconds, "."); dotIdx != -1 {
				cs := seconds[dotIdx+1:]
				if len(cs) == 2 {
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

	// 预编译清理文本的正则
	var cleanTextRegex = regexp.MustCompile(`\{[^}]*\}`)
	cleanText := func(text string) string {
		text = cleanTextRegex.ReplaceAllString(text, "")
		text = strings.ReplaceAll(text, "\\N", "\n")
		text = strings.ReplaceAll(text, "\\n", "\n")
		text = strings.ReplaceAll(text, "\\h", " ")
		text = strings.ReplaceAll(text, "\\t", "\t")
		return text
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

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

		if strings.HasPrefix(strings.ToUpper(trimmed), "FORMAT:") {
			formatStr := strings.TrimPrefix(line, "Format:")
			formatStr = strings.TrimPrefix(formatStr, "FORMAT:")
			formatFields = []string{}
			for _, f := range strings.Split(formatStr, ",") {
				formatFields = append(formatFields, strings.TrimSpace(strings.ToLower(f)))
			}
			continue
		}

		if strings.HasPrefix(strings.ToUpper(trimmed), "DIALOGUE:") {
			dialogueStr := strings.TrimPrefix(line, "Dialogue:")
			dialogueStr = strings.TrimPrefix(dialogueStr, "DIALOGUE:")

			parts := strings.SplitN(dialogueStr, ",", 10)
			if len(parts) < 10 {
				parts = strings.Split(dialogueStr, ",")
			}

			var startTime, endTime, text string

			if len(formatFields) >= 10 {
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
