package scanner

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"msp/internal/config"
	"msp/internal/constants"
	"msp/internal/domain"
	"msp/internal/util"
)

// 预编译的正则表达式
var blockedStringRegex = regexp.MustCompile(`^/(.*)/$`)

// regexpCache 缓存用户配置中的正则规则，避免每次匹配都重新编译
var regexpCache sync.Map

// getOrCompileRegexp 获取缓存的编译正则，或编译并缓存
func getOrCompileRegexp(pattern string) *regexp.Regexp {
	if v, ok := regexpCache.Load(pattern); ok {
		return v.(*regexp.Regexp)
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		log.Printf("[WARN] invalid regexp pattern %q: %v", pattern, err)
		return nil
	}
	regexpCache.Store(pattern, re)
	return re
}

// IsBlockedStringRegex 检查规则是否是正则表达式格式（以 / 开头和结尾）
func IsBlockedStringRegex(rule string) bool {
	return len(rule) > 2 && strings.HasPrefix(rule, "/") && strings.HasSuffix(rule, "/")
}

// GetBlockedStringPattern 提取正则表达式模式
func GetBlockedStringPattern(rule string) string {
	if matches := blockedStringRegex.FindStringSubmatch(rule); matches != nil {
		return matches[1]
	}
	return ""
}

type WalkCallback func(item domain.MediaItem, path string, root string) error

func WalkShares(ctx context.Context, shares []domain.Share, blacklist config.BlacklistConfig, maxItems int, cb WalkCallback, idCodec *util.IDCodec) error {
	limit := maxItems
	if limit <= 0 {
		limit = constants.DefaultScanLimit
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		mu         sync.Mutex
		globalSeen int
		limitHit   bool
		firstErr   error
		wg         sync.WaitGroup
	)

	for _, sh := range shares {
		root := util.NormalizePath(sh.Path)
		if root == "" || !util.IsExistingDir(root) {
			continue
		}

		wg.Add(1)
		go func(label, root string) {
			defer wg.Done()

			w := shareWalker{
				ctx:       ctx,
				blacklist: blacklist,
				dirCache:  make(map[string][]fs.DirEntry),
				cb: func(item domain.MediaItem, path string, rootPtr string) error {
					mu.Lock()
					defer mu.Unlock()
					if limitHit || globalSeen >= limit {
						if !limitHit {
							limitHit = true
							cancel()
						}
						return fs.SkipAll
					}
					globalSeen++
					return cb(item, path, rootPtr)
				},
				idCodec: idCodec,
			}

			if err := w.walkShare(root, label); err != nil {
				mu.Lock()
				if !limitHit && firstErr == nil {
					firstErr = fmt.Errorf("walk share %s: %w", label, err)
				} else if limitHit {
					// Limit was already hit; log suppressed errors for visibility.
					log.Printf("[WARN] walk suppressed after limit: share %s: %v", label, err)
				}
				mu.Unlock()
			}
		}(sh.Label, root)
	}

	wg.Wait()
	return firstErr
}

type shareWalker struct {
	ctx       context.Context
	blacklist config.BlacklistConfig
	dirCache  map[string][]fs.DirEntry
	cb        WalkCallback
	idCodec   *util.IDCodec
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
		log.Printf("[WARN] walk entry error: %v", err)
		return nil
	}

	if d.IsDir() {
		if ShouldSkipDir(d.Name(), w.blacklist) {
			return fs.SkipDir
		}
		return nil
	}

	if shouldSkipFile(d, w.blacklist) {
		return nil
	}

	item, err := w.buildMediaItem(p, d, shareLabel, root)
	if err != nil {
		log.Printf("[WARN] build media item error: %v", err)
		return nil
	}

	return w.cb(item, p, root)
}

func ShouldSkipDir(name string, blacklist config.BlacklistConfig) bool {
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
		log.Printf("[WARN] file info error: %v", err)
		return true
	}

	if IsBlockedSize(fi.Size(), blacklist.SizeRule) {
		return true
	}
	return false
}

func (w *shareWalker) buildMediaItem(path string, d fs.DirEntry, shareLabel string, root string) (domain.MediaItem, error) {
	fi, err := d.Info()
	if err != nil {
		return domain.MediaItem{}, err
	}

	ext := filepath.Ext(d.Name())
	kind := ClassifyExt(ext)
	relDir, _ := filepath.Rel(root, filepath.Dir(path))
	relPath := d.Name()
	if relDir != "." {
		relPath = filepath.ToSlash(relDir) + "/" + d.Name()
	}

	item := domain.MediaItem{
		ID:         w.idCodec.EncodeID(path),
		Name:       d.Name(),
		Ext:        strings.ToLower(ext),
		Kind:       kind,
		ShareLabel: shareLabel,
		Size:       fi.Size(),
		ModTime:    fi.ModTime().Unix(),
		RelPath:    relPath,
	}

	if kind == "video" {
		item.Subtitles = FindSidecarSubtitlesCached(path, w.dirCache, w.idCodec)
	}
	if kind == "audio" {
		cover, lyrics := FindAudioSidecarsCached(path, w.dirCache)
		if cover != "" {
			item.CoverID = w.idCodec.EncodeID(cover)
		}
		if lyrics != "" {
			item.LyricsID = w.idCodec.EncodeID(lyrics)
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

		if pattern := GetBlockedStringPattern(rule); pattern != "" {
			if re := getOrCompileRegexp(pattern); re != nil && re.MatchString(target) {
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
		if _, err := f.ReadAt(tail, st.Size()-tailSize); err != nil {
			log.Printf("[WARN] read tail bytes error: %v", err)
		}
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
