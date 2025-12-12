/*******************************************************************************
*  internal/config/config.go
*
*  Provides static configuration to the logging facility.
*******************************************************************************/

package config

// Config holds runtime configuration for the collector.
type Config struct {
	DataDir        string   // where to store logs
	BufferSize     int      // number of recent messages to keep per topic
	DescriptorSets []string // list of .desc files to load into the registry
}

// DefaultBufferSize is a sane default if not overridden.
const DefaultBufferSize = 1000

// DefaultConfig returns a baseline configuration.
func DefaultConfig() *Config {
	return &Config{
		BufferSize:     DefaultBufferSize,
		DescriptorSets: []string{"protolog/schema.desc", "/home/ec2-user/sandbox/nimbus/nimbus.desc", "/home/ec2-user/sandbox/nimbus/ascend_core/asend_core.desc"},
	}
}

