package handler

import (
	"context"
	"net/http"
	"time"

	"msp/internal/config"
	"msp/internal/domain"
	"msp/internal/media"
	"msp/internal/service"
	"msp/internal/storage"
	"msp/internal/util"
)

type ConfigProvider interface {
	Config() config.Config
	UpdateConfig(func(*config.Config)) error
	GetPort() int
}

type MediaCacheProvider interface {
	GetOrBuildMediaCache(ctx context.Context, shares []domain.Share, blacklist config.BlacklistConfig, refresh bool) (domain.MediaResponse, string)
	PeekMediaETag() (string, bool)
	InvalidateMediaCache()
}

type SessionProvider interface {
	CreateSession() (string, error)
	ValidateSession(token string) bool
}

type Logger interface {
	Log(level string, msg string)
	LogRequest(r *http.Request, status int, start time.Time)
}

type Deps struct {
	Config    ConfigProvider
	Media     MediaCacheProvider
	Session   SessionProvider
	Logger    Logger
	Progress  storage.ProgressStore
	Prefs     storage.PrefsStore
	Favorites storage.FavoriteStore
	Processor *media.MediaProcessor
	IDCodec   *util.IDCodec
}

type Handler struct {
	config        ConfigProvider
	media         MediaCacheProvider
	session       SessionProvider
	logger        Logger
	progress      storage.ProgressStore
	prefs         storage.PrefsStore
	favorites     storage.FavoriteStore
	configService *service.ConfigService
	processor     *media.MediaProcessor
	idCodec       *util.IDCodec
}

const (
	defaultJSONBodyLimit   int64 = 1 << 20
	maxSubtitleConvertSize int64 = 8 << 20
)

func New(deps Deps) *Handler {
	return &Handler{
		config:        deps.Config,
		media:         deps.Media,
		session:       deps.Session,
		logger:        deps.Logger,
		progress:      deps.Progress,
		prefs:         deps.Prefs,
		favorites:     deps.Favorites,
		configService: service.NewConfigService(deps.Config, deps.Media, deps.Processor),
		processor:     deps.Processor,
		idCodec:       deps.IDCodec,
	}
}
