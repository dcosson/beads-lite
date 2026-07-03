package config

// Paths captures resolved locations for config.
type Paths struct {
	ConfigDir  string // path to .beads directory
	ConfigFile string // path to .beads/config.yaml
	// OverlayConfigFile is the config.yaml of a redirecting .beads directory,
	// when discovery followed a redirect and the redirecting directory has its
	// own config.yaml. Its keys override values from ConfigFile in memory
	// (e.g. a per-repo issue_prefix over a shared beads directory). Empty when
	// no redirect was followed or the redirecting directory has no config.yaml.
	OverlayConfigFile string
}
