package handlers

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/zorak1103/ha-mcp/internal/handlers/skills"
	"github.com/zorak1103/ha-mcp/internal/mcp"
)

// auditFileOwners maps every non-test handler file that reads a literal or
// const argument key (args["x"], params["x"], f(args, "x"), or an argReader
// call such as r.str("x")) - or hands
// args to a function declared in an auditSharedArgFiles file - to the tools
// whose handlers it serves. A key read in that file must be declared by at
// least one of those tools' InputSchema.Properties. A new handler file that
// reads args fails TestToolSchemas_DeclareEveryLiteralArgRead until it is
// listed here (or in auditSharedArgFiles).
var auditFileOwners = map[string][]string{
	"analysis.go":                         {"analyze_entity", "get_entity_dependencies"},
	"areas.go":                            {"manage_area"},
	"automations.go":                      {"manage_automation"},
	"automations_coverage.go":             {"manage_automation"},
	"automations_trigger.go":              {"manage_automation"},
	"blueprints.go":                       {"manage_blueprint"},
	"calendars.go":                        {"manage_calendar"},
	"cameras.go":                          {"manage_camera"},
	"config_entries.go":                   {"manage_config_entry"},
	"datetime.go":                         {"get_datetime"},
	"devices_consolidated.go":             {"query_devices"},
	"devices_health.go":                   {"query_devices"},
	"devices_manage.go":                   {"manage_device"},
	"entities.go":                         {"get_state"},
	"entity_id_args.go":                   {"get_logbook"},
	"entities_consolidated.go":            {"query_entities"},
	"entities_health.go":                  {"query_entities"},
	"entities_manage.go":                  {"manage_entity"},
	"entities_presence.go":                {"query_entities"},
	"find_references.go":                  {"find_references"},
	"floors.go":                           {"manage_floor"},
	"hacs.go":                             {"manage_hacs"},
	"helpers_config_builders_extended.go": {"manage_helper", "helper_action"},
	"helpers_consolidated.go":             {"manage_helper", "helper_action"},
	"helpers_template_types.go":           {"manage_helper", "helper_action"},
	"labels.go":                           {"manage_label"},
	"logbook.go":                          {"get_logbook"},
	"logbook_correlation.go":              {"get_logbook"},
	"lovelace.go":                         {"manage_dashboard"},
	"media.go":                            {"browse_media", "sign_media_path"},
	"persons.go":                          {"manage_person"},
	"registry.go":                         {"get_registry"},
	"registry_consolidated.go":            {"get_registry"},
	"scenes.go":                           {"manage_scene"},
	"scripts.go":                          {"manage_script", "call_service"},
	"services.go":                         {"list_services"},
	"skill.go":                            {"get_skill"},
	"statistics.go":                       {"manage_statistics"},
	"system_log.go":                       {"manage_system_log"},
	"tags.go":                             {"manage_tag"},
	"targets_consolidated.go":             {"analyze_target"},
	"templates.go":                        {"render_template"},
	"todos.go":                            {"manage_todo"},
	"traces.go":                           {"manage_trace"},
	"traces_debug.go":                     {"manage_trace"},
	"updates.go":                          {"manage_update"},
	"zones.go":                            {"manage_zone"},
}

// auditSharedArgFiles lists the files whose argument readers are shared by
// several tools, with the complete set of keys those readers consume. Every
// owned file that passes args to a function declared in one of these files
// must have all of that file's keys declared by its owners.
var auditSharedArgFiles = map[string][]string{
	"pagination.go":        {"cursor", "limit", "offset"},
	"patch.go":             {"operations"},
	"labels_validation.go": {"labels"},
}

// auditedUndeclaredArgs whitelists "tool:arg" reads that are intentionally
// absent from the schema, with the reason. Must stay empty unless a reviewer
// accepts the reason.
var auditedUndeclaredArgs = map[string]string{}

// auditArgIdents are the identifier names under which handlers hold the raw
// tool-argument map.
var auditArgIdents = map[string]bool{"args": true, "params": true}

// auditFileScan is the per-file result of scanning a handler source file.
type auditFileScan struct {
	// reads maps each literal/const key read to its first line number.
	reads map[string]int
	// sharedCalls is the set of auditSharedArgFiles entries whose functions
	// this file calls with an args map.
	sharedCalls map[string]bool
	// funcReads maps each top-level function declared in this file to the
	// keys it reads directly.
	funcReads map[string]map[string]int
	// argCalls maps each plain-identifier callee this file passes an args map
	// to, to the first call's line number.
	argCalls map[string]int
}

var (
	auditRegistryOnce sync.Once
	auditRegistry     *mcp.Registry
)

// auditRegisteredTools returns every tool RegisterAllTools registers.
func auditRegisteredTools() []mcp.Tool {
	auditRegistryOnce.Do(func() {
		auditRegistry = mcp.NewRegistry()
		RegisterAllTools(auditRegistry)
	})
	return auditRegistry.ListTools()
}

// auditToolProperties returns tool name -> declared schema property names for
// every registered tool.
func auditToolProperties(t *testing.T) map[string]map[string]bool {
	t.Helper()
	props := make(map[string]map[string]bool)
	for _, tool := range auditRegisteredTools() {
		set := make(map[string]bool, len(tool.InputSchema.Properties))
		for k := range tool.InputSchema.Properties {
			set[k] = true
		}
		props[tool.Name] = set
	}
	return props
}

// collectStringConsts resolves package-level string constants, including
// consts defined as another const identifier.
func collectStringConsts(files map[string]*ast.File) map[string]string {
	consts := make(map[string]string)
	aliases := make(map[string]string)
	for _, f := range files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs := spec.(*ast.ValueSpec)
				for i, name := range vs.Names {
					if i >= len(vs.Values) {
						continue
					}
					switch v := vs.Values[i].(type) {
					case *ast.BasicLit:
						if s, ok := stringLit(v); ok {
							consts[name.Name] = s
						}
					case *ast.Ident:
						aliases[name.Name] = v.Name
					}
				}
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for name, target := range aliases {
			if s, ok := consts[target]; ok {
				consts[name] = s
				delete(aliases, name)
				changed = true
			}
		}
	}
	return consts
}

func stringLit(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

// resolveKey returns the string value of a key expression that is a string
// literal or a resolvable package-level const.
func resolveKey(e ast.Expr, consts map[string]string) (string, bool) {
	if s, ok := stringLit(e); ok {
		return s, true
	}
	if id, ok := e.(*ast.Ident); ok {
		s, found := consts[id.Name]
		return s, found
	}
	return "", false
}

func isArgIdent(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && auditArgIdents[id.Name]
}

// sharedFuncFiles maps each top-level function declared in a shared file to
// that file.
func sharedFuncFiles(files map[string]*ast.File) map[string]string {
	out := make(map[string]string)
	for name := range auditSharedArgFiles {
		f, ok := files[name]
		if !ok {
			continue
		}
		for _, decl := range f.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil {
				out[fd.Name.Name] = name
			}
		}
	}
	return out
}

// auditScanContext holds the package-wide facts every file scan needs.
type auditScanContext struct {
	fset        *token.FileSet
	consts      map[string]string
	sharedFuncs map[string]string
	// readerMethods is the set of *argReader methods whose first parameter
	// is the args key they read (str, strAs, num, ...).
	readerMethods map[string]bool
}

// isArgReaderType reports whether a type expression is argReader or
// *argReader.
func isArgReaderType(e ast.Expr) bool {
	if star, ok := e.(*ast.StarExpr); ok {
		e = star.X
	}
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "argReader"
}

// isNewArgReaderCall reports whether e is a newArgReader(...) call.
func isNewArgReaderCall(e ast.Expr) bool {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	fn, ok := call.Fun.(*ast.Ident)
	return ok && fn.Name == "newArgReader"
}

// argReaderMethods derives the *argReader methods that read args by the key
// in their first parameter: a method indexing <recv>.args[<first param>]
// directly, or forwarding its first parameter as the first argument of
// another such method (str -> strAs), iterated to a fixed point. Helpers
// like fail/checkStringLen take a key only for messages and never qualify.
func argReaderMethods(files map[string]*ast.File) map[string]bool {
	type method struct{ recv, key string }
	methods := map[string]method{}
	bodies := map[string]*ast.BlockStmt{}
	for _, f := range files {
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Body == nil || len(fd.Recv.List) != 1 ||
				!isArgReaderType(fd.Recv.List[0].Type) || len(fd.Recv.List[0].Names) != 1 {
				continue
			}
			params := fd.Type.Params.List
			if len(params) == 0 || len(params[0].Names) == 0 {
				continue
			}
			methods[fd.Name.Name] = method{recv: fd.Recv.List[0].Names[0].Name, key: params[0].Names[0].Name}
			bodies[fd.Name.Name] = fd.Body
		}
	}
	isIdent := func(e ast.Expr, name string) bool {
		id, ok := e.(*ast.Ident)
		return ok && id.Name == name
	}
	readers := map[string]bool{}
	for changed := true; changed; {
		changed = false
		for name, m := range methods {
			if readers[name] {
				continue
			}
			ast.Inspect(bodies[name], func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.IndexExpr:
					if sel, ok := node.X.(*ast.SelectorExpr); ok && isIdent(sel.X, m.recv) &&
						sel.Sel.Name == "args" && isIdent(node.Index, m.key) {
						readers[name] = true
					}
				case *ast.CallExpr:
					if sel, ok := node.Fun.(*ast.SelectorExpr); ok && isIdent(sel.X, m.recv) &&
						readers[sel.Sel.Name] && len(node.Args) > 0 && isIdent(node.Args[0], m.key) {
						readers[name] = true
					}
				}
				return !readers[name]
			})
			changed = changed || readers[name]
		}
	}
	return readers
}

// argReaderIdents returns the identifiers that hold an *argReader anywhere in
// decl: receivers and parameters (of the function or any closure in it)
// typed argReader/*argReader, variables declared with that type, and
// variables assigned from newArgReader(...).
func argReaderIdents(decl ast.Decl) map[string]bool {
	idents := map[string]bool{}
	addFields := func(fl *ast.FieldList) {
		if fl == nil {
			return
		}
		for _, field := range fl.List {
			if isArgReaderType(field.Type) {
				for _, name := range field.Names {
					idents[name.Name] = true
				}
			}
		}
	}
	if fd, ok := decl.(*ast.FuncDecl); ok {
		addFields(fd.Recv)
	}
	ast.Inspect(decl, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.FuncType:
			addFields(node.Params)
		case *ast.ValueSpec:
			for i, name := range node.Names {
				if (node.Type != nil && isArgReaderType(node.Type)) ||
					(i < len(node.Values) && isNewArgReaderCall(node.Values[i])) {
					idents[name.Name] = true
				}
			}
		case *ast.AssignStmt:
			for i, lhs := range node.Lhs {
				if id, ok := lhs.(*ast.Ident); ok && i < len(node.Rhs) && isNewArgReaderCall(node.Rhs[i]) {
					idents[id.Name] = true
				}
			}
		}
		return true
	})
	return idents
}

// scanArgReads finds every literal/const argument key read in f: X[K] with X
// an args ident; a call whose argument i is an args ident and whose argument
// i+1 is K; or R.m(K, ...) with R an *argReader and m one of its reader
// methods. It also records which functions receive args (or an argReader),
// which shared reader files those calls reach, and the direct reads of each
// top-level function.
func scanArgReads(sc auditScanContext, f *ast.File) auditFileScan {
	scan := auditFileScan{
		reads:       map[string]int{},
		sharedCalls: map[string]bool{},
		funcReads:   map[string]map[string]int{},
		argCalls:    map[string]int{},
	}
	for _, decl := range f.Decls {
		var fnReads map[string]int
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Recv == nil {
			fnReads = map[string]int{}
			scan.funcReads[fd.Name.Name] = fnReads
		}
		record := func(key string, pos token.Pos) {
			line := sc.fset.Position(pos).Line
			if _, seen := scan.reads[key]; !seen {
				scan.reads[key] = line
			}
			if _, seen := fnReads[key]; fnReads != nil && !seen {
				fnReads[key] = line
			}
		}
		readers := argReaderIdents(decl)
		ast.Inspect(decl, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.IndexExpr:
				if isArgIdent(node.X) {
					if key, ok := resolveKey(node.Index, sc.consts); ok {
						record(key, node.Pos())
					}
				}
			case *ast.CallExpr:
				scanArgCall(sc, node, readers, &scan, record)
			}
			return true
		})
	}
	for name, keys := range scan.funcReads {
		if len(keys) == 0 {
			delete(scan.funcReads, name)
		}
	}
	return scan
}

// scanArgCall handles one call expression for scanArgReads.
func scanArgCall(sc auditScanContext, call *ast.CallExpr, readers map[string]bool, scan *auditFileScan, record func(string, token.Pos)) {
	// R.m(K, ...) on an argReader: K is the args key (strAs's second
	// argument is the config key, not an args key, so only the first counts).
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sc.readerMethods[sel.Sel.Name] && len(call.Args) > 0 {
		if recv, ok := sel.X.(*ast.Ident); ok && readers[recv.Name] {
			if key, ok := resolveKey(call.Args[0], sc.consts); ok && key != "" {
				record(key, call.Pos())
			}
		}
	}
	passesArgs := false
	for i, a := range call.Args {
		if id, ok := a.(*ast.Ident); ok && readers[id.Name] {
			passesArgs = true
		}
		if !isArgIdent(a) {
			continue
		}
		passesArgs = true
		// An empty string after args is a plain value (e.g.
		// buildStatisticListPaginationParams(args, "")), never a key.
		if i+1 < len(call.Args) {
			if key, ok := resolveKey(call.Args[i+1], sc.consts); ok && key != "" {
				record(key, call.Pos())
			}
		}
	}
	fn, ok := call.Fun.(*ast.Ident)
	if !ok || !passesArgs {
		return
	}
	if _, seen := scan.argCalls[fn.Name]; !seen {
		scan.argCalls[fn.Name] = sc.fset.Position(call.Pos()).Line
	}
	if file, shared := sc.sharedFuncs[fn.Name]; shared {
		scan.sharedCalls[file] = true
	}
}

// scanAllHandlerSources parses every non-test .go file in the package
// directory exactly once and scans it for argument reads.
func scanAllHandlerSources(t *testing.T) map[string]auditFileScan {
	t.Helper()
	names, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob handler sources: %v", err)
	}
	fset := token.NewFileSet()
	files := make(map[string]*ast.File)
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files[name] = f
	}
	if len(files) == 0 {
		t.Fatal("no handler sources found; test must run from internal/handlers")
	}
	sc := auditScanContext{
		fset:          fset,
		consts:        collectStringConsts(files),
		sharedFuncs:   sharedFuncFiles(files),
		readerMethods: argReaderMethods(files),
	}
	if len(sc.readerMethods) == 0 {
		t.Fatal("no argReader reader methods found; argReader detection is broken")
	}
	scans := make(map[string]auditFileScan, len(files))
	for name, f := range files {
		scans[name] = scanArgReads(sc, f)
	}
	return scans
}

// ownersDeclare reports whether key is declared by at least one owner tool or
// whitelisted for one of them.
func ownersDeclare(owners []string, key string, props map[string]map[string]bool) bool {
	for _, tool := range owners {
		if props[tool][key] {
			return true
		}
		if _, ok := auditedUndeclaredArgs[tool+":"+key]; ok {
			return true
		}
	}
	return false
}

func TestToolSchemas_DeclareEveryLiteralArgRead(t *testing.T) {
	props := auditToolProperties(t)
	scans := scanAllHandlerSources(t)

	var failures []string
	failf := func(format string, a ...any) { failures = append(failures, fmt.Sprintf(format, a...)) }

	// (d) map hygiene: every listed file exists, owners are registered tools,
	// no file is in both maps.
	for file, owners := range auditFileOwners {
		if _, ok := scans[file]; !ok {
			failf("%s: listed in auditFileOwners but no such non-test file", file)
		}
		if _, dup := auditSharedArgFiles[file]; dup {
			failf("%s: listed in both auditFileOwners and auditSharedArgFiles", file)
		}
		if len(owners) == 0 {
			failf("%s: auditFileOwners entry has no owner tools", file)
		}
		for _, tool := range owners {
			if _, ok := props[tool]; !ok {
				failf("%s: owner %q is not a registered tool", file, tool)
			}
		}
	}
	for file := range auditSharedArgFiles {
		if _, ok := scans[file]; !ok {
			failf("%s: listed in auditSharedArgFiles but no such non-test file", file)
		}
	}
	for entry := range auditedUndeclaredArgs {
		tool, _, ok := strings.Cut(entry, ":")
		if _, known := props[tool]; !ok || !known {
			failf("auditedUndeclaredArgs: %q is not a registered \"tool:arg\"", entry)
		}
	}

	// readerFuncFile maps each top-level function with direct reads that is
	// declared in an owned file to that file.
	readerFuncFile := make(map[string]string)
	for file, scan := range scans {
		if _, owned := auditFileOwners[file]; !owned {
			continue
		}
		for name := range scan.funcReads {
			readerFuncFile[name] = file
		}
	}

	for file, scan := range scans {
		owners, owned := auditFileOwners[file]
		sharedKeys, shared := auditSharedArgFiles[file]
		switch {
		case shared:
			// (c) a shared file reads only its listed keys.
			for key, line := range scan.reads {
				if !slices.Contains(sharedKeys, key) {
					failf("%s:%d: shared reader reads %q, not listed in auditSharedArgFiles", file, line, key)
				}
			}
		case owned:
			// (a) every direct read is declared by an owner.
			for key, line := range scan.reads {
				if !ownersDeclare(owners, key, props) {
					failf("%s: %s read undeclared %q (line %d)", file, strings.Join(owners, ", "), key, line)
				}
			}
			// (b) every key of a shared reader this file calls is declared.
			for sharedFile := range scan.sharedCalls {
				for _, key := range auditSharedArgFiles[sharedFile] {
					if !ownersDeclare(owners, key, props) {
						failf("%s: %s read undeclared %q (via %s)", file, strings.Join(owners, ", "), key, sharedFile)
					}
				}
			}
			// (b') the same for a reader function declared in another owned
			// file (e.g. traces_debug.go calling logbook.go's getHoursFromArgs).
			for callee, line := range scan.argCalls {
				src, ok := readerFuncFile[callee]
				if !ok || src == file {
					continue
				}
				for key := range scans[src].funcReads[callee] {
					if !ownersDeclare(owners, key, props) {
						failf("%s: %s read undeclared %q (via %s %s, line %d)", file, strings.Join(owners, ", "), key, src, callee, line)
					}
				}
			}
			if len(scan.reads) == 0 && len(scan.sharedCalls) == 0 {
				failf("%s: listed in auditFileOwners but reads no tool arguments; remove the entry", file)
			}
		case len(scan.reads) > 0 || len(scan.sharedCalls) > 0:
			// (d) every file with a read must be classified.
			failf("%s: reads tool arguments but is in neither auditFileOwners nor auditSharedArgFiles", file)
		}
	}

	slices.Sort(failures)
	for _, f := range failures {
		t.Error(f)
	}
}

func TestManageHelperSchema_DeclaresEveryTableDrivenField(t *testing.T) {
	declared := auditToolProperties(t)["manage_helper"]
	if len(declared) == 0 {
		t.Fatal("manage_helper is not registered or has no properties")
	}

	var failures []string
	for typeName, meta := range helperTypes {
		for _, field := range slices.Concat(meta.requiredFields, meta.optionalFields) {
			if !declared[field] {
				failures = append(failures, fmt.Sprintf("helperTypes[%q]: field %q not declared by manage_helper", typeName, field))
			}
		}
	}
	for typeName, sub := range templateSubtypes {
		for _, f := range sub.fields {
			if !declared[f.arg] {
				failures = append(failures, fmt.Sprintf("templateSubtypes[%q]: arg %q not declared by manage_helper", typeName, f.arg))
			}
		}
	}
	slices.Sort(failures)
	for _, f := range failures {
		t.Error(f)
	}
}

// jsonFenceRe matches a fenced ```json code block and captures its body.
var jsonFenceRe = regexp.MustCompile("(?s)```json[ \\t]*\\r?\\n(.*?)```")

// checkDocExampleBlock validates one JSON example block. It returns the
// failures found and whether the block had a recognized tool-call shape.
func checkDocExampleBlock(source, block string, props map[string]map[string]bool) ([]string, bool) {
	var obj map[string]any
	if err := json.Unmarshal([]byte(block), &obj); err != nil {
		return nil, false
	}
	var tool string
	var args map[string]any
	if name, ok := obj["tool"].(string); ok {
		tool = name
		args = make(map[string]any, len(obj))
		for k, v := range obj {
			if k != "tool" {
				args[k] = v
			}
		}
	} else if p, ok := obj["params"].(map[string]any); ok {
		name, nameOK := p["name"].(string)
		arguments, argsOK := p["arguments"].(map[string]any)
		if !nameOK || !argsOK {
			return nil, false
		}
		tool, args = name, arguments
	} else {
		return nil, false
	}

	declared, known := props[tool]
	if !known {
		return []string{fmt.Sprintf("%s: example uses unknown tool %q", source, tool)}, true
	}
	var failures []string
	for key := range args {
		if !declared[key] {
			failures = append(failures, fmt.Sprintf("%s: %s example uses undeclared %q", source, tool, key))
		}
	}
	return failures, true
}

func TestDocExamples_UseDeclaredArguments(t *testing.T) {
	props := auditToolProperties(t)

	sources := make(map[string]string)
	for _, meta := range skills.Catalog {
		body, err := skills.Read(meta.Slug)
		if err != nil {
			t.Fatalf("read skill %q: %v", meta.Slug, err)
		}
		sources["skill "+meta.Slug] = body
	}
	for _, path := range []string{"../../docs/tools.md", "../../README.md"} {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		sources[strings.TrimPrefix(path, "../../")] = string(b)
	}

	var failures []string
	checked := 0
	for source, body := range sources {
		for _, m := range jsonFenceRe.FindAllStringSubmatch(body, -1) {
			fails, recognized := checkDocExampleBlock(source, m[1], props)
			if recognized {
				checked++
			}
			failures = append(failures, fails...)
		}
	}
	if checked == 0 {
		t.Fatal("no tool-call JSON examples found; extraction is broken")
	}
	slices.Sort(failures)
	for _, f := range failures {
		t.Error(f)
	}
}

// collectIntegrationCallToolKeys returns, per tool, the literal argument keys
// used by s.CallTool("tool", map[string]any{...}) calls in the integration
// test sources, each with its first file:line.
func collectIntegrationCallToolKeys(t *testing.T) map[string]map[string]string {
	t.Helper()
	names, err := filepath.Glob(filepath.Join("integration", "*_test.go"))
	if err != nil {
		t.Fatalf("glob integration sources: %v", err)
	}
	fset := token.NewFileSet()
	out := make(map[string]map[string]string)
	for _, name := range names {
		f, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "CallTool" {
				return true
			}
			if recv, ok := sel.X.(*ast.Ident); !ok || recv.Name != "s" {
				return true
			}
			tool, ok := stringLit(call.Args[0])
			if !ok {
				return true
			}
			lit, ok := call.Args[1].(*ast.CompositeLit)
			if !ok {
				return true
			}
			if out[tool] == nil {
				out[tool] = make(map[string]string)
			}
			for _, elt := range lit.Elts {
				kv, ok := elt.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if key, ok := stringLit(kv.Key); ok {
					if _, seen := out[tool][key]; !seen {
						pos := fset.Position(kv.Pos())
						out[tool][key] = fmt.Sprintf("%s:%d", filepath.ToSlash(pos.Filename), pos.Line)
					}
				}
			}
			return true
		})
	}
	return out
}

func TestIntegrationCallTool_UseDeclaredArguments(t *testing.T) {
	props := auditToolProperties(t)
	calls := collectIntegrationCallToolKeys(t)
	if len(calls) == 0 {
		t.Fatal("no s.CallTool literal calls found; extraction is broken")
	}

	var failures []string
	for tool, keys := range calls {
		declared, known := props[tool]
		if !known {
			failures = append(failures, fmt.Sprintf("integration: s.CallTool uses unknown tool %q", tool))
			continue
		}
		for key, where := range keys {
			if !declared[key] {
				failures = append(failures, fmt.Sprintf("%s: %s call uses undeclared %q", where, tool, key))
			}
		}
	}
	slices.Sort(failures)
	for _, f := range failures {
		t.Error(f)
	}
}
