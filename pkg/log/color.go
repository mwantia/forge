package log

func Color(l LogLevel) string {
	switch l {
	case Trace:
		return "\033[36m" // Cyan for trace
	case Debug:
		return "\033[34m" // Blue for debug
	case Info:
		return "\033[32m" // Green for info
	case Warn:
		return "\033[33m" // Yellow for warn
	case Error:
		return "\033[31m" // Red for error
	default:
		return "\033[0m"
	}
}