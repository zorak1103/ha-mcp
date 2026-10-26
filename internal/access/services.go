package access

import "strings"

// ServiceKind is the kind of access a call needs.
type ServiceKind int

// Service kinds. ServiceRead needs read on the target objects, ServiceControl
// control, ServiceConfigure configure, ServiceSystem a system value, and
// ServiceScriptCall control on script.<id> (the script id is the service name).
const (
	ServiceRead ServiceKind = iota
	ServiceControl
	ServiceConfigure
	ServiceSystem
	ServiceScriptCall
)

// ServiceClass is the classification of one domain.service call. System is set
// for ServiceSystem. ObjectFromData, for ServiceConfigure, names the call-data
// key that carries the bare object id within the service domain (for example
// scene.create takes scene_id, so the object is scene.<scene_id>); empty means
// the objects come from the call target.
type ServiceClass struct {
	Kind           ServiceKind
	System         SystemValue
	ObjectFromData string
}

const (
	domainScript        = "script"
	serviceReload       = "reload"
	serviceReloadPrefix = "reload_"
)

// systemDomains maps domains whose every service is a system operation.
var systemDomains = map[string]SystemValue{
	"backup":        SystemBackup,
	"update":        SystemUpdates,
	"recorder":      SystemStatistics,
	"logger":        SystemLogs,
	"system_log":    SystemLogs,
	"shell_command": SystemHomeAssistant,
	"python_script": SystemHomeAssistant,
	"pyscript":      SystemHomeAssistant,
	"rest_command":  SystemHomeAssistant,
}

// configureServices maps "domain.service" to the call-data key naming the object.
var configureServices = map[string]string{
	"scene.create":             "scene_id",
	"group.set":                "object_id",
	"group.remove":             "object_id",
	"input_select.set_options": "",
}

// readServices are the services that only read.
var readServices = map[string]bool{
	"todo.get_items":        true,
	"calendar.get_events":   true,
	"weather.get_forecasts": true,
}

// ClassifyService classifies a service call by what it needs. Matching is
// case-insensitive and ignores surrounding whitespace. Precedence, first match
// wins: malformed input (empty domain or service) is a system operation needing
// SystemAll; then the system rules (whole system domains, homeassistant.* and
// hassio.* specifics, any reload service); then configure; then read; then
// script calls; everything else is control. Because system is checked first, a
// service that is both system and configure/read is always the more privileged.
func ClassifyService(domain, service string) ServiceClass {
	d := strings.ToLower(strings.TrimSpace(domain))
	s := strings.ToLower(strings.TrimSpace(service))
	if d == "" || s == "" {
		return ServiceClass{Kind: ServiceSystem, System: SystemAll}
	}
	if v, ok := classifySystemService(d, s); ok {
		return ServiceClass{Kind: ServiceSystem, System: v}
	}
	key := d + "." + s
	if from, ok := configureServices[key]; ok {
		return ServiceClass{Kind: ServiceConfigure, ObjectFromData: from}
	}
	if readServices[key] {
		return ServiceClass{Kind: ServiceRead}
	}
	if d == domainScript && !isScriptEntityService(s) {
		return ServiceClass{Kind: ServiceScriptCall}
	}
	return ServiceClass{Kind: ServiceControl}
}

// isScriptEntityService reports whether s is a script service that targets a
// script entity (as opposed to calling a script by its id).
func isScriptEntityService(s string) bool {
	return s == "turn_on" || s == "turn_off" || s == "toggle"
}

func classifySystemService(d, s string) (SystemValue, bool) {
	if v, ok := systemDomains[d]; ok {
		return v, true
	}
	switch d {
	case "homeassistant":
		if v, ok := homeAssistantSystem(s); ok {
			return v, true
		}
	case "hassio":
		return hassioSystem(s), true
	}
	if s == serviceReload || strings.HasPrefix(s, serviceReloadPrefix) {
		return SystemReload, true
	}
	return "", false
}

func homeAssistantSystem(s string) (SystemValue, bool) {
	switch s {
	case "restart", "stop":
		return SystemRestart, true
	case "set_location":
		return SystemHomeAssistant, true
	}
	return "", false
}

// hassioSystem classifies hassio.* services. Unknown hassio services fail
// closed to the homeassistant system value instead of defaulting to control.
func hassioSystem(s string) SystemValue {
	switch {
	case strings.HasPrefix(s, "host_"):
		return SystemRestart
	case strings.HasPrefix(s, "addon_"):
		return SystemAddons
	case strings.HasPrefix(s, "backup_"), strings.HasPrefix(s, "restore_"):
		return SystemBackup
	default:
		return SystemHomeAssistant
	}
}
