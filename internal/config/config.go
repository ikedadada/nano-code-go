package config

type Config struct {
	Sandbox        bool
	AllowedDomains []string
}

func Default() Config {
	return Config{
		Sandbox:        false,
		AllowedDomains: []string{"api.github.com", "github.com"},
	}
}

func (c Config) WithAllowedDomains(domains []string) Config {
	next := Config{
		Sandbox:        c.Sandbox,
		AllowedDomains: append([]string(nil), c.AllowedDomains...),
	}
	next.AllowedDomains = append(next.AllowedDomains, domains...)
	return next
}
