package enforce

// class says how the wrapper enforces one Client method. A method added to the
// interface starts as classUnclassified (fail closed, its body calls
// c.unclassified) until it is given its real class;
// TestNoMethodLeftUnclassified and TestNoClassifiedMethodCallsUnclassified keep
// the table and the method bodies in step.
type class int

const (
	// classUnclassified marks a method with no enforcement decision yet: it
	// records a denial and returns zero values.
	classUnclassified class = iota
	// classReadFiltered lists, or returns result sets of, objects: hidden
	// objects are dropped from the request and the result.
	classReadFiltered
	// classReadLookup looks up one object: a hidden object gets the error of
	// a missing one.
	classReadLookup
	// classReadRestricted reads an operation that is refused or checked as a
	// whole, because it can reach any object.
	classReadRestricted
	// classWriteChecked acts on objects: every object the call reaches,
	// explicit or expanded, must be granted the needed capability, else the
	// whole call is refused before the inner client sees it.
	classWriteChecked
	// classSystem is an object-less system operation gated by a system value.
	classSystem
	// classCommand is the WebSocket passthrough, decided per command name.
	classCommand
	// classPassThrough is an object-less read forwarded unchanged: it exposes
	// no per-object data and cannot write. Each such entry carries its
	// justification in methodClass.
	classPassThrough
)

// methodClass has exactly one entry per homeassistant.Client method, keyed by
// method name. TestEveryClientMethodClassified keeps it in step with the
// interface, so a new method cannot be added without a decision here.
var methodClass = map[string]class{
	"GetStates":                 classReadFiltered,
	"GetState":                  classReadLookup,
	"SetState":                  classWriteChecked,
	"GetHistory":                classReadFiltered,
	"ListAutomations":           classReadFiltered,
	"GetAutomation":             classReadLookup,
	"CreateAutomation":          classWriteChecked,
	"UpdateAutomation":          classWriteChecked,
	"DeleteAutomation":          classWriteChecked,
	"ToggleAutomation":          classWriteChecked,
	"ListHelpers":               classReadFiltered,
	"CreateHelper":              classWriteChecked,
	"CreateHelperEntity":        classWriteChecked,
	"UpdateHelper":              classWriteChecked,
	"DeleteHelper":              classWriteChecked,
	"SetHelperValue":            classWriteChecked,
	"ListScripts":               classReadFiltered,
	"GetScript":                 classReadLookup,
	"CreateScript":              classWriteChecked,
	"UpdateScript":              classWriteChecked,
	"DeleteScript":              classWriteChecked,
	"ListScenes":                classReadFiltered,
	"GetScene":                  classReadLookup,
	"CreateScene":               classWriteChecked,
	"UpdateScene":               classWriteChecked,
	"DeleteScene":               classWriteChecked,
	"ConfigFileEntryExists":     classReadLookup,
	"CallService":               classWriteChecked,
	"GetEntityRegistry":         classReadFiltered,
	"GetEntityRegistryEntry":    classReadLookup,
	"GetDeviceRegistry":         classReadFiltered,
	"GetAreaRegistry":           classReadFiltered,
	"CreateArea":                classWriteChecked,
	"UpdateArea":                classWriteChecked,
	"DeleteArea":                classWriteChecked,
	"GetLabelRegistry":          classReadFiltered,
	"CreateLabel":               classWriteChecked,
	"UpdateLabel":               classWriteChecked,
	"DeleteLabel":               classWriteChecked,
	"GetFloorRegistry":          classReadFiltered,
	"CreateFloor":               classWriteChecked,
	"UpdateFloor":               classWriteChecked,
	"DeleteFloor":               classWriteChecked,
	"GetZones":                  classReadFiltered,
	"CreateZone":                classWriteChecked,
	"UpdateZone":                classWriteChecked,
	"DeleteZone":                classWriteChecked,
	"GetPersons":                classReadFiltered,
	"CreatePerson":              classWriteChecked,
	"UpdatePerson":              classWriteChecked,
	"DeletePerson":              classWriteChecked,
	"GetTags":                   classReadFiltered,
	"CreateTag":                 classWriteChecked,
	"UpdateTag":                 classWriteChecked,
	"DeleteTag":                 classWriteChecked,
	"RemoveEntityRegistryEntry": classWriteChecked,
	"UpdateEntityRegistryEntry": classWriteChecked,
	"RemoveDeviceConfigEntry":   classWriteChecked,
	"UpdateDeviceRegistryEntry": classWriteChecked,
	"SignPath":                  classReadRestricted,
	"GetCameraStream":           classReadLookup,
	"BrowseMedia":               classReadFiltered,
	"GetLovelaceConfig":         classReadLookup,
	"SaveLovelaceConfig":        classWriteChecked,
	"ListDashboards":            classReadFiltered,
	"CreateDashboard":           classWriteChecked,
	"UpdateDashboard":           classWriteChecked,
	"DeleteDashboard":           classWriteChecked,
	"GetStatistics":             classReadFiltered,
	"GetTriggersForTarget":      classReadFiltered,
	"GetConditionsForTarget":    classReadFiltered,
	"GetServicesForTarget":      classReadFiltered,
	"ExtractFromTarget":         classReadFiltered,
	"GetHelperConfig":           classReadLookup,
	"GetConfigEntries":          classReadFiltered,
	"GetConfigEntry":            classReadLookup,
	"GetConfigEntryOptions":     classReadLookup,
	"DeleteConfigEntry":         classSystem,
	// Service descriptions (names, fields, selectors) of every integration:
	// no entity ids or states, and a read.
	"GetServices": classPassThrough,
	// Core configuration (location, units, version, loaded components): no
	// per-object data, and a read.
	"GetConfig":      classPassThrough,
	"RenderTemplate": classReadRestricted,
	"GetLogbook":     classReadFiltered,
	// Validation of configuration.yaml (valid/invalid plus messages): a
	// read-only check that names no entity state.
	"CheckConfig":             classPassThrough,
	"SendHACSCommand":         classCommand,
	"CallServiceWithResponse": classWriteChecked,
	"ListStatisticIDs":        classReadFiltered,
	"ValidateStatistics":      classReadFiltered,
	"ClearStatistics":         classSystem,
	"GetCalendars":            classReadFiltered,
	"GetCalendarEvents":       classReadLookup,
	"GetCameraSnapshot":       classReadLookup,
	"GetSystemLog":            classReadRestricted,
	"ClearSystemLog":          classSystem,
}
