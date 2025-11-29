/*******************************************************************************
*  internal/config/config.go
*
*  Provides static configuration to the logging facility.
*******************************************************************************/

package config

// Config holds runtime configuration for the collector.
type Config struct {
	DataDir        string // where to store logs
	BufferSize     int    // number of recent messages to keep per topic
}

// DefaultBufferSize is a sane default if not overridden.
const DefaultBufferSize = 1000
