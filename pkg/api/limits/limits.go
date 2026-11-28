package limits

const (
	// MaxBodySize is the largest request body, cache value and store file.
	MaxBodySize = 10 << 20 // 10 MiB

	// MaxCacheKeySize is the longest cache key.
	MaxCacheKeySize = 1 << 10 // 1 KiB

	// MaxBackendResponseSize is the largest echo response read from a backend.
	MaxBackendResponseSize = 64 << 20 // 64 MiB

	// MaxLoggedPayloadSize is the largest payload logged at debug level;
	// larger ones are logged with their size only.
	MaxLoggedPayloadSize = 64 << 10 // 64 KiB
)
