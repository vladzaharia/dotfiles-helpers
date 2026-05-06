package orb

// Machine mirrors the JSON record returned by `orb list --format json`.
type Machine struct {
	ID      string  `json:"id"`
	Name    string  `json:"name"`
	Image   Image   `json:"image"`
	Config  MConfig `json:"config"`
	Builtin bool    `json:"builtin"`
	State   string  `json:"state"` // "running" | "stopped" | …
}

// Image is the distro/version/arch tuple.
type Image struct {
	Distro  string `json:"distro"`
	Version string `json:"version"`
	Arch    string `json:"arch"`
	Variant string `json:"variant"`
}

// MConfig captures the per-machine config block that orb returns.
type MConfig struct {
	Isolated        bool   `json:"isolated"`
	DefaultUsername string `json:"default_username"`
	HTTPPort        int    `json:"http_port"`
	HTTPSPort       int    `json:"https_port"`
}

// Info is the full record from `orb info <name> --format json`,
// which wraps Machine and adds runtime fields.
type Info struct {
	Record   Machine `json:"record"`
	DiskSize int64   `json:"disk_size"`
	IP4      string  `json:"ip4"`
	IP6      string  `json:"ip6"`
}

// CreateOptions captures the args for `orb create`.
type CreateOptions struct {
	Distro   string      // "ubuntu" (default)
	Version  string      // "" or e.g. "noble"
	Arch     string      // "arm64" / "amd64" (auto-detected if empty)
	Name     string      // machine name (required)
	User     string      // default user (defaults to host username)
	Isolated bool        // --isolated
	UserData []byte      // cloud-init payload (written to a temp file)
	Mounts   []MountSpec // selective host mounts (--mount, repeatable)
}

// IsRunning reports whether the machine is in a running state.
func (m Machine) IsRunning() bool { return m.State == "running" }
