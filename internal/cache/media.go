package cache

import (
	"context"
	"encoding/json"
	"hash/fnv"
	"log"
	"os"
	"runtime/debug"
	"sort"
	"strings"
	"sync"
	"time"

	"msp/internal/config"
	"msp/internal/constants"
	"msp/internal/domain"
	"msp/internal/media"
	"msp/internal/util"
)

type MediaCache struct {
	mu            sync.Mutex
	cond          *sync.Cond
	key           string
	builtAt       time.Time
	ttl           time.Duration
	respJSON      []byte
	etag          string
	building      bool
	cacheFilePath string
	bg            sync.WaitGroup
}

func NewMediaCache(cacheFilePath string, ttl time.Duration) *MediaCache {
	c := &MediaCache{
		cacheFilePath: cacheFilePath,
		ttl:           ttl,
	}
	c.cond = sync.NewCond(&c.mu)
	return c
}

func (c *MediaCache) runBg(fn func()) {
	c.bg.Add(1)
	go func() {
		defer c.bg.Done()
		fn()
	}()
}

func (c *MediaCache) WaitForBackground() {
	c.bg.Wait()
}

func (c *MediaCache) unmarshalResp() domain.MediaResponse {
	var r domain.MediaResponse
	if len(c.respJSON) > 0 {
		if err := json.Unmarshal(c.respJSON, &r); err != nil {
			log.Printf("[WARN] cache unmarshal error: %v", err)
		}
	}
	return r
}

func (c *MediaCache) Invalidate() {
	c.mu.Lock()
	c.key = ""
	c.etag = ""
	c.builtAt = time.Time{}
	c.respJSON = nil
	c.mu.Unlock()
	_ = os.Remove(c.cacheFilePath)
}

func (c *MediaCache) GetOrBuild(ctx context.Context, shares []domain.Share, blacklist config.BlacklistConfig, refresh bool, maxItems int) (domain.MediaResponse, string) {
	key := CacheKey(shares, blacklist)

	c.mu.Lock()
	if c.key == key && !c.builtAt.IsZero() && !refresh {
		if time.Since(c.builtAt) >= c.ttl && !c.building {
			c.building = true
			c.mu.Unlock()
			c.runBg(func() { c.rebuild(context.Background(), key, shares, blacklist, maxItems) })
			c.mu.Lock()
		}
		r := c.unmarshalResp()
		etag := c.etag
		c.mu.Unlock()
		return r, etag
	}

	if c.building {
		r := c.unmarshalResp()
		r.Scanning = true
		etag := c.etag
		c.mu.Unlock()
		return r, etag
	}

	if refresh {
		c.building = true
		c.runBg(func() { c.rebuild(context.Background(), key, shares, blacklist, maxItems) })
		r := c.unmarshalResp()
		r.Scanning = true
		etag := c.etag
		c.mu.Unlock()
		return r, etag
	}

	if c.key != key {
		c.mu.Unlock()
		if resp, builtAt, ok, dbErr := media.LoadMediaFromDB(ctx, key, shares); ok && !builtAt.IsZero() {
			if dbErr != nil {
				log.Printf("[WARN] LoadMediaFromDB error: %v", dbErr)
			}
			etag := WeakETag(key, builtAt)
			c.mu.Lock()
			if b, err := json.Marshal(resp); err != nil {
				log.Printf("[WARN] cache marshal error: %v", err)
			} else {
				c.respJSON = b
			}
			c.key = key
			c.builtAt = builtAt
			c.etag = etag
			c.mu.Unlock()
			return resp, etag
		}
		c.mu.Lock()
	}

	c.building = true
	c.mu.Unlock()

	resp, etag := c.buildAndUpdate(ctx, key, shares, blacklist, maxItems)
	return resp, etag
}

func (c *MediaCache) buildAndUpdate(ctx context.Context, key string, shares []domain.Share, blacklist config.BlacklistConfig, maxItems int) (domain.MediaResponse, string) {
	var resp domain.MediaResponse
	builtAt := time.Now()
	if media.IsDBAvailable() {
		r, bt, err := media.ReindexAndLoadMedia(ctx, key, shares, blacklist, maxItems)
		if err == nil && !bt.IsZero() {
			resp = r
			builtAt = bt
		} else {
			var berr error
			resp, berr = media.BuildMediaResponse(ctx, shares, blacklist, maxItems)
			if berr != nil {
				log.Printf("[WARN] BuildMediaResponse error: %v", berr)
			}
			builtAt = time.Now()
		}
	} else {
		var err error
		resp, err = media.BuildMediaResponse(ctx, shares, blacklist, maxItems)
		if err != nil {
			log.Printf("[WARN] BuildMediaResponse error: %v", err)
		}
	}
	etag := WeakETag(key, builtAt)

	b, err := json.Marshal(resp)
	if err != nil {
		log.Printf("[WARN] cache marshal error: %v", err)
	}

	c.mu.Lock()
	c.respJSON = b
	c.key = key
	c.builtAt = builtAt
	c.etag = etag
	c.building = false
	c.cond.Broadcast()
	c.mu.Unlock()

	if !media.IsDBAvailable() {
		c.runBg(func() { c.saveToDisk(key, builtAt, etag, resp) })
	}

	go debug.FreeOSMemory()

	return resp, etag
}

func (c *MediaCache) rebuild(ctx context.Context, key string, shares []domain.Share, blacklist config.BlacklistConfig, maxItems int) {
	c.buildAndUpdate(ctx, key, shares, blacklist, maxItems)
}

func (c *MediaCache) LoadFromDisk(key string) bool {
	if media.IsDBAvailable() {
		return false
	}
	c.mu.Lock()
	already := c.key == key && !c.builtAt.IsZero()
	need := c.key != key || c.builtAt.IsZero()
	c.mu.Unlock()
	if already || !need {
		return already
	}

	b, err := os.ReadFile(c.cacheFilePath)
	if err != nil || len(b) == 0 {
		return false
	}
	var v mediaCacheOnDisk
	if err := json.Unmarshal(b, &v); err != nil {
		return false
	}
	if v.Key != key || v.BuiltAt <= 0 {
		return false
	}

	c.mu.Lock()
	c.key = v.Key
	c.builtAt = time.Unix(0, v.BuiltAt)
	c.etag = v.ETag
	if b, err := json.Marshal(v.Resp); err != nil {
		log.Printf("[WARN] cache marshal error: %v", err)
	} else {
		c.respJSON = b
	}
	c.mu.Unlock()
	return true
}

func (c *MediaCache) saveToDisk(key string, builtAt time.Time, etag string, resp domain.MediaResponse) {
	v := mediaCacheOnDisk{
		Key:     key,
		BuiltAt: builtAt.UnixNano(),
		ETag:    etag,
		Resp:    resp,
	}
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	tmp := c.cacheFilePath + ".tmp"
	if err := os.WriteFile(tmp, b, constants.FilePerm); err != nil {
		return
	}
	_ = os.Rename(tmp, c.cacheFilePath)
}

type mediaCacheOnDisk struct {
	Key     string               `json:"key"`
	BuiltAt int64                `json:"builtAt"`
	ETag    string               `json:"etag"`
	Resp    domain.MediaResponse `json:"resp"`
}

func CacheKey(shares []domain.Share, blacklist config.BlacklistConfig) string {
	var b strings.Builder
	b.WriteString(sharesCacheKey(shares))

	exts := normRuleList(blacklist.Extensions)
	files := normRuleList(blacklist.Filenames)
	folders := normRuleList(blacklist.Folders)

	b.WriteString("blExt=")
	b.WriteString(strings.Join(exts, ","))
	b.WriteByte('\n')
	b.WriteString("blFile=")
	b.WriteString(strings.Join(files, ","))
	b.WriteByte('\n')
	b.WriteString("blFolder=")
	b.WriteString(strings.Join(folders, ","))
	b.WriteByte('\n')
	b.WriteString("blSize=")
	b.WriteString(strings.TrimSpace(strings.ToLower(blacklist.SizeRule)))
	b.WriteByte('\n')

	return b.String()
}

func normRuleList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		v := strings.TrimSpace(s)
		if v == "" {
			continue
		}
		out = append(out, strings.ToLower(v))
	}
	sort.Strings(out)
	return out
}

func sharesCacheKey(shares []domain.Share) string {
	s := append([]domain.Share(nil), shares...)
	for i := range s {
		s[i].Path = util.NormalizePath(s[i].Path)
	}
	sort.Slice(s, func(i, j int) bool {
		return strings.ToLower(s[i].Path) < strings.ToLower(s[j].Path)
	})
	var b strings.Builder
	for _, sh := range s {
		b.WriteString(strings.ToLower(sh.Path))
		b.WriteByte('|')
		b.WriteString(strings.TrimSpace(sh.Label))
		b.WriteByte('\n')
	}
	return b.String()
}

func WeakETag(key string, builtAt time.Time) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(key))
	var t [8]byte
	//nolint:gosec // int64 timestamp to uint64 hash input is safe for build time
	n := uint64(builtAt.UnixNano())
	for i := 0; i < 8; i++ {
		t[i] = byte(n)
		n >>= 8
	}
	_, _ = h.Write(t[:])
	return `W/` + util.U64Base36(h.Sum64()) + ``
}

func FormatMediaCachePath(cfgPath string) string {
	return cfgPath + ".media_cache.json"
}
