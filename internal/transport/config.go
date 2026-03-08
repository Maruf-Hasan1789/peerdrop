package transport

type SessionConfig struct {
	ChunkSize int
}

func DefaultConfig() SessionConfig {
	return SessionConfig{
		ChunkSize: 512 * 1024,
	}
}
