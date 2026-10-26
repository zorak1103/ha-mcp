package access

import (
	"slices"
	"strings"
)

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
// the objects come from the call target. EntityDataKeys lists call-data keys
// whose values name further entities the call acts on (a string, comma string,
// list, or a map whose keys are the entity ids; see EntityIDsFromData).
//
// Contract for the caller: the objects of a call are the union of ParseTarget's
// result, the ids from EntityDataKeys and the ObjectFromData object. A caller
// MUST refuse (or evaluate against the policy default) a Control or Configure
// call whose resolved set is empty and not All, because a check over zero
// objects would pass vacuously; a Malformed target or malformed entity data
// MUST be refused.
type ServiceClass struct {
	Kind           ServiceKind
	System         SystemValue
	ObjectFromData string
	EntityDataKeys []string
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
	// These reach arbitrary entities or hardware without a matchable object.
	"conversation": SystemHomeAssistant,
	"mqtt":         SystemHomeAssistant,
	"zha":          SystemHomeAssistant,
	"zwave_js":     SystemHomeAssistant,
}

// indirectServices maps "domain.service" to the classes of services that are
// configure, or control with objects named in the call data. The table values
// are templates; ClassifyService returns a copy.
var indirectServices = map[string]ServiceClass{
	"scene.create":             {Kind: ServiceConfigure, ObjectFromData: "scene_id", EntityDataKeys: []string{"entities", "snapshot_entities"}},
	"group.set":                {Kind: ServiceConfigure, ObjectFromData: "object_id", EntityDataKeys: []string{"entities", "add_entities"}},
	"group.remove":             {Kind: ServiceConfigure, ObjectFromData: "object_id"},
	"input_select.set_options": {Kind: ServiceConfigure},
	"device_tracker.see":       {Kind: ServiceConfigure, ObjectFromData: "dev_id"},
	"scene.apply":              {Kind: ServiceControl, EntityDataKeys: []string{"entities"}},
	"media_player.join":        {Kind: ServiceControl, EntityDataKeys: []string{"group_members"}},
	"tts.speak":                {Kind: ServiceControl, EntityDataKeys: []string{"media_player_entity_id"}},
	"camera.play_stream":       {Kind: ServiceControl, EntityDataKeys: []string{"media_player"}},
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
// See ServiceClass for what the caller must do with the result; in particular a
// default-control call is only as strong as the objects the caller can resolve.
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
	if c, ok := indirectServices[key]; ok {
		c.EntityDataKeys = slices.Clone(c.EntityDataKeys)
		return c
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
