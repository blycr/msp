package constants

// 常见的错误消息常量
const (
	// ErrMsgInvalidJSON JSON 解析错误
	ErrMsgInvalidJSON = "JSON 解析失败"

	// ErrMsgWriteConfig 配置写入失败
	ErrMsgWriteConfig = "写入配置失败"

	// ErrMsgReadConfig 配置读取失败
	ErrMsgReadConfig = "读取配置失败"

	// ErrMsgReadPrefs 偏好设置读取失败
	ErrMsgReadPrefs = "读取偏好失败"

	// ErrMsgWritePrefs 偏好设置写入失败
	ErrMsgWritePrefs = "写入偏好失败"

	// ErrMsgReadProgress 播放进度读取失败
	ErrMsgReadProgress = "读取进度失败"

	// ErrMsgWriteProgress 播放进度保存失败
	ErrMsgWriteProgress = "保存进度失败"

	// ErrMsgMissingID 缺少 ID 参数
	ErrMsgMissingID = "缺少 id"

	// ErrMsgMissingPath 缺少路径参数
	ErrMsgMissingPath = "缺少 Path"

	// ErrMsgMissingPrefs 缺少偏好设置
	ErrMsgMissingPrefs = "缺少 prefs"

	// ErrMsgBadID ID 格式错误
	ErrMsgBadID = "bad id"

	// ErrMsgNotAllowed 不允许访问
	ErrMsgNotAllowed = "not allowed"

	// ErrMsgNotFound 资源不存在
	ErrMsgNotFound = "not found"

	// ErrMsgOpenFailed 打开文件失败
	ErrMsgOpenFailed = "open failed"

	// ErrMsgReadFailed 读取失败
	ErrMsgReadFailed = "read failed"

	// ErrMsgTranscodeDisabled 转码未启用
	ErrMsgTranscodeDisabled = "transcoding is disabled in configuration"

	// ErrMsgAccessDenied 访问被拒绝
	ErrMsgAccessDenied = "Access Denied"

	// ErrMsgUnauthorized 未授权
	ErrMsgUnauthorized = "Unauthorized"

	// ErrMsgInvalidRequest 无效请求
	ErrMsgInvalidRequest = "Invalid request"

	// ErrMsgMethodNotAllowed 方法不允许
	ErrMsgMethodNotAllowed = "Method Not Allowed"

	// ErrMsgUnsupportedFormat 不支持的格式
	ErrMsgUnsupportedFormat = "unsupported subtitle format"
)
