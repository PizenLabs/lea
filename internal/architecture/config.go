// Package architecture defines the configuration and logic for architecture validation.
package architecture

// Config represents the architecture rules configuration.
type Config struct {
	Layers   []Layer  `yaml:"layers"`
	Settings Settings `yaml:"settings"`
}

// Layer defines a logical layer in the application and its allowed dependencies.
type Layer struct {
	Name     string   `yaml:"name"`
	Patterns []string `yaml:"patterns"`
	Allow    []string `yaml:"allow"`
	Deny     []string `yaml:"deny"`
}

// Settings contains global architecture validation settings.
type Settings struct {
	AllowUnknown    *bool `yaml:"allow_unknown"`
	AllowSelf       *bool `yaml:"allow_self"`
	DefaultAllowAll *bool `yaml:"default_allow_all"`
}
