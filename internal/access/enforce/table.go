package enforce

// class says how the wrapper enforces one Client method. Every method starts
// as classUnclassified (fail closed); later tasks give each its real class as
// they replace the stub in stubs.go.
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
)

// methodClass has exactly one entry per homeassistant.Client method, keyed by
// method name. TestEveryClientMethodClassified keeps it in step with the
// interface, so a new method cannot be added without a decision here.
var methodClass = map[string]class{
	"GetStates":                 classReadFiltered,
	"GetState":                  classReadLookup,
	"SetState":                  classUnclassified,
	"GetHistory":                classReadFiltered,
	"ListAutomations":           classReadFiltered,
	"GetAutomation":             classReadLookup,
	"CreateAutomation":          classUnclassified,
	"UpdateAutomation":          classUnclassified,
	"DeleteAutomation":          classUnclassified,
	"ToggleAutomation":          classUnclassified,
	"ListHelpers":               classReadFiltered,
	"CreateHelper":              classUnclassified,
	"CreateHelperEntity":        classUnclassified,
	"UpdateHelper":              classUnclassified,
	"DeleteHelper":              classUnclassified,
	"SetHelperValue":            classUnclassified,
	"ListScripts":               classReadFiltered,
	"GetScript":                 classReadLookup,
	"CreateScript":              classUnclassified,
	"UpdateScript":              classUnclassified,
	"DeleteScript":              classUnclassified,
	"ListScenes":                classReadFiltered,
	"GetScene":                  classReadLookup,
	"CreateScene":               classUnclassified,
	"UpdateScene":               classUnclassified,
	"DeleteScene":               classUnclassified,
	"ConfigFileEntryExists":     classReadLookup,
	"CallService":               classUnclassified,
	"GetEntityRegistry":         classReadFiltered,
	"GetEntityRegistryEntry":    classReadLookup,
	"GetDeviceRegistry":         classReadFiltered,
	"GetAreaRegistry":           classReadFiltered,
	"CreateArea":                classUnclassified,
	"UpdateArea":                classUnclassified,
	"DeleteArea":                classUnclassified,
	"GetLabelRegistry":          classUnclassified,
	"CreateLabel":               classUnclassified,
	"UpdateLabel":               classUnclassified,
	"DeleteLabel":               classUnclassified,
	"GetFloorRegistry":          classUnclassified,
	"CreateFloor":               classUnclassified,
	"UpdateFloor":               classUnclassified,
	"DeleteFloor":               classUnclassified,
	"GetZones":                  classUnclassified,
	"CreateZone":                classUnclassified,
	"UpdateZone":                classUnclassified,
	"DeleteZone":                classUnclassified,
	"GetPersons":                classUnclassified,
	"CreatePerson":              classUnclassified,
	"UpdatePerson":              classUnclassified,
	"DeletePerson":              classUnclassified,
	"GetTags":                   classUnclassified,
	"CreateTag":                 classUnclassified,
	"UpdateTag":                 classUnclassified,
	"DeleteTag":                 classUnclassified,
	"RemoveEntityRegistryEntry": classUnclassified,
	"UpdateEntityRegistryEntry": classUnclassified,
	"RemoveDeviceConfigEntry":   classUnclassified,
	"UpdateDeviceRegistryEntry": classUnclassified,
	"SignPath":                  classReadRestricted,
	"GetCameraStream":           classReadLookup,
	"BrowseMedia":               classReadFiltered,
	"GetLovelaceConfig":         classReadLookup,
	"SaveLovelaceConfig":        classUnclassified,
	"ListDashboards":            classReadFiltered,
	"CreateDashboard":           classUnclassified,
	"UpdateDashboard":           classUnclassified,
	"DeleteDashboard":           classUnclassified,
	"GetStatistics":             classReadFiltered,
	"GetTriggersForTarget":      classReadFiltered,
	"GetConditionsForTarget":    classReadFiltered,
	"GetServicesForTarget":      classReadFiltered,
	"ExtractFromTarget":         classReadFiltered,
	"GetHelperConfig":           classReadLookup,
	"GetConfigEntries":          classUnclassified,
	"GetConfigEntry":            classUnclassified,
	"GetConfigEntryOptions":     classUnclassified,
	"DeleteConfigEntry":         classUnclassified,
	"GetServices":               classUnclassified,
	"GetConfig":                 classUnclassified,
	"RenderTemplate":            classReadRestricted,
	"GetLogbook":                classReadFiltered,
	"CheckConfig":               classUnclassified,
	"SendHACSCommand":           classUnclassified,
	"CallServiceWithResponse":   classUnclassified,
	"ListStatisticIDs":          classReadFiltered,
	"ValidateStatistics":        classUnclassified,
	"ClearStatistics":           classUnclassified,
	"GetCalendars":              classReadFiltered,
	"GetCalendarEvents":         classReadLookup,
	"GetCameraSnapshot":         classReadLookup,
	"GetSystemLog":              classUnclassified,
	"ClearSystemLog":            classUnclassified,
}
