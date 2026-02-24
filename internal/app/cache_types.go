package app

import "time"

type cacheMode string

const (
	cacheModeBalanced cacheMode = "balanced"
	cacheModeFresh    cacheMode = "fresh"
	cacheModeSpeed    cacheMode = "speed"
)

const cacheVersion = 1

type cacheConfig struct {
	Enabled bool
	Dir     string
	Mode    cacheMode

	TTLAccounts  time.Duration
	TTLRoles     time.Duration
	TTLRegions   time.Duration
	TTLInstances time.Duration
}

type cacheEnvelope struct {
	Version   int            `json:"version"`
	CreatedAt time.Time      `json:"created_at"`
	ExpiresAt time.Time      `json:"expires_at"`
	Profile   string         `json:"profile"`
	Key       string         `json:"key"`
	Payload   jsonRawPayload `json:"payload"`
}

type jsonRawPayload []byte

func (p jsonRawPayload) MarshalJSON() ([]byte, error) {
	if len(p) == 0 {
		return []byte("null"), nil
	}
	return p, nil
}

func (p *jsonRawPayload) UnmarshalJSON(data []byte) error {
	*p = append((*p)[:0], data...)
	return nil
}

type cacheReadStatus int

const (
	cacheMiss cacheReadStatus = iota
	cacheHitFresh
	cacheHitStale
)
