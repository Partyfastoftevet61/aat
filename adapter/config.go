package adapter

// EnvironmentConfig holds the environment-specific settings that adapters
// need when building requests. A template reads values only from its step's
// inputs; an environment value reaches it through an input default such as
// "{{env.KEY}}".
type EnvironmentConfig struct {
	BaseURL string            // scheme+host (e.g., "https://api.example.com")
	Headers map[string]string // default headers (auth tokens, API keys)
}
