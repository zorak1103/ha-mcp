// Package access is the object-level access policy engine. It compiles the
// validated access section of the config file into an immutable Policy and
// answers which capabilities a Home Assistant object is granted. The package
// does no I/O; its only side effect is the WARN log of a recorded denial. A
// Policy is safe for concurrent use.
package access

// Capability is one permission an object can be granted.
type Capability string

// Capabilities. control and configure are independent and always imply read.
const (
	CapRead      Capability = "read"
	CapControl   Capability = "control"
	CapConfigure Capability = "configure"
)

// SystemValue names a class of object-less system operations (access.system).
type SystemValue string

// System values, matching config's valid access.system entries.
const (
	SystemReload        SystemValue = "reload"
	SystemRestart       SystemValue = "restart"
	SystemBackup        SystemValue = "backup"
	SystemUpdates       SystemValue = "updates"
	SystemConfigEntries SystemValue = "config_entries"
	SystemHACS          SystemValue = "hacs"
	SystemAddons        SystemValue = "addons"
	SystemBlueprints    SystemValue = "blueprints"
	SystemStatistics    SystemValue = "statistics"
	SystemLogs          SystemValue = "logs"
	SystemHomeAssistant SystemValue = "homeassistant"
	SystemAll           SystemValue = "all"
)

// Kind is the type of object a decision is about.
type Kind int

// Object kinds. KindGlobal objects (floors, zones, areas as objects, ...) have
// no matcher and are governed by the default only.
const (
	KindEntity Kind = iota
	KindDashboard
	KindGlobal
)

// DefaultDashboardID is the ID of the default (overview) dashboard.
const DefaultDashboardID = "lovelace"

// Object is what a decision is about. For entities ID is the entity id; for
// dashboards it is the url_path (DefaultDashboardID for the default one). The
// attribute slices hold lower-cased ids and names: Areas includes the area
// inherited from the device, Labels is the union of entity and device labels,
// Devices holds the device id and name_by_user.
type Object struct {
	Kind    Kind
	ID      string
	Areas   []string
	Labels  []string
	Devices []string
}

// Grant is the outcome of a decision. Create and Delete refine Configure and
// are never true without it.
type Grant struct {
	Read      bool
	Control   bool
	Configure bool
	Create    bool
	Delete    bool
}

// Hidden reports whether the object must be invisible.
func (g Grant) Hidden() bool { return !g.Read }

// Has reports whether c is granted. Unknown capabilities are never granted.
func (g Grant) Has(c Capability) bool {
	switch c {
	case CapRead:
		return g.Read
	case CapControl:
		return g.Control
	case CapConfigure:
		return g.Configure
	default:
		return false
	}
}
