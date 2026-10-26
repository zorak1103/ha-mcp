package access

import (
	"errors"
	"fmt"
	"path"
	"slices"
	"strings"

	"github.com/zorak1103/ha-mcp/internal/config"
)

// Default levels and capability names as written in the access section.
const (
	levelRead  = "read"
	levelWrite = "write"
	levelNone  = "none"

	capNameControl   = "control"
	capNameConfigure = "configure"
)

// capSet is a set of the three capabilities.
type capSet struct {
	read, control, configure bool
}

func (s capSet) union(o capSet) capSet {
	return capSet{s.read || o.read, s.control || o.control, s.configure || o.configure}
}

func (s capSet) has(c Capability) bool {
	return Grant{Read: s.read, Control: s.control, Configure: s.configure}.Has(c)
}

// rule is a compiled access rule. All matcher values are trimmed and
// lower-cased. allow always includes read when it grants control or configure.
type rule struct {
	globs      []string
	dashboards []string
	areas      []string
	labels     []string
	devices    []string

	allow     capSet
	allowNone bool
	deny      capSet // read is never set: deny: read is invalid
	noCreate  bool
	noDelete  bool
}

// Policy is a compiled, immutable access policy, safe for concurrent use.
type Policy struct {
	level        string
	base         capSet
	rules        []rule
	systemAbsent bool
	systemAll    bool
	system       []SystemValue
}

// Compile validates cfg and compiles it into a Policy. An invalid config is an
// error, never a permissive policy. An empty default is read.
func Compile(cfg config.AccessConfig) (*Policy, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	level := cfg.Default
	if level == "" {
		level = levelRead
	}
	base, err := baseFor(level)
	if err != nil {
		return nil, err
	}
	p := &Policy{level: level, base: base, rules: make([]rule, 0, len(cfg.Rules))}
	for i, r := range cfg.Rules {
		cr, err := compileRule(r)
		if err != nil {
			return nil, fmt.Errorf("access.rules[%d]: %w", i, err)
		}
		p.rules = append(p.rules, cr)
	}
	p.compileSystem(cfg.System)
	return p, nil
}

func baseFor(level string) (capSet, error) {
	switch level {
	case levelRead:
		return capSet{read: true}, nil
	case levelWrite:
		return capSet{read: true, control: true, configure: true}, nil
	case levelNone:
		return capSet{}, nil
	default:
		return capSet{}, fmt.Errorf("access.default %q is invalid", level)
	}
}

func (p *Policy) compileSystem(list *config.CapList) {
	if list == nil {
		p.systemAbsent = true
		return
	}
	for _, v := range *list {
		sv := SystemValue(v)
		if sv == SystemAll {
			p.systemAll = true
		}
		p.system = append(p.system, sv)
	}
}

func compileRule(r config.AccessRule) (rule, error) {
	m := r.Match
	if entityKinds(m) > 1 {
		return rule{}, errors.New(msgSeveralEntityMatcherKinds)
	}
	var cr rule
	var err error
	if cr.globs, err = normalize("entities", m.Entities); err != nil {
		return rule{}, err
	}
	if cr.dashboards, err = normalize("dashboards", m.Dashboards); err != nil {
		return rule{}, err
	}
	if cr.areas, err = normalize("areas", m.Areas); err != nil {
		return rule{}, err
	}
	if cr.labels, err = normalize("labels", m.Labels); err != nil {
		return rule{}, err
	}
	if cr.devices, err = normalize("devices", m.Devices); err != nil {
		return rule{}, err
	}
	if cr.allow, cr.allowNone, err = compileAllow(r.Allow); err != nil {
		return rule{}, err
	}
	if cr.deny, err = compileDeny(r.Deny); err != nil {
		return rule{}, err
	}
	cr.noCreate = r.Create != nil && !*r.Create
	cr.noDelete = r.Delete != nil && !*r.Delete
	return cr, nil
}

// msgSeveralEntityMatcherKinds mirrors config's validation message. A rule
// with more than one entity-selecting matcher kind has no safe meaning (OR
// over-grants an allow, AND under-applies a deny), so it is rejected here too,
// independent of Validate.
const msgSeveralEntityMatcherKinds = "rule combines several entity matcher kinds " +
	"(entities, areas, labels, devices): split into separate rules"

// entityKinds counts the entity-selecting matcher kinds m uses.
func entityKinds(m config.AccessMatch) int {
	n := 0
	for _, vals := range [][]string{m.Entities, m.Areas, m.Labels, m.Devices} {
		if len(vals) > 0 {
			n++
		}
	}
	return n
}

// normalize trims and lower-cases matcher values. A blank value would silently
// match nothing (a deny that never applies), so it is an error.
func normalize(kind string, vals []string) ([]string, error) {
	out := make([]string, 0, len(vals))
	for _, v := range vals {
		n := strings.ToLower(strings.TrimSpace(v))
		if n == "" {
			return nil, fmt.Errorf("match.%s contains a blank value", kind)
		}
		out = append(out, n)
	}
	return out, nil
}

func compileAllow(list config.CapList) (capSet, bool, error) {
	var s capSet
	for _, v := range list {
		switch v {
		case levelRead:
			s.read = true
		case capNameControl:
			s = s.union(capSet{read: true, control: true})
		case capNameConfigure:
			s = s.union(capSet{read: true, configure: true})
		case levelWrite:
			s = s.union(capSet{read: true, control: true, configure: true})
		case levelNone:
			return capSet{}, true, nil
		default:
			return capSet{}, false, fmt.Errorf("unknown capability %q in allow", v)
		}
	}
	return s, false, nil
}

func compileDeny(list config.CapList) (capSet, error) {
	var s capSet
	for _, v := range list {
		switch v {
		case capNameControl:
			s.control = true
		case capNameConfigure:
			s.configure = true
		case levelWrite:
			s = s.union(capSet{control: true, configure: true})
		default:
			return capSet{}, fmt.Errorf("unsupported capability %q in deny", v)
		}
	}
	return s, nil
}

// matches reports whether the rule selects o. Entity matchers select entities,
// dashboards select dashboards, and KindGlobal objects are never selected. A
// rule has at most one entity matcher kind (compileRule rejects more); several
// values of that kind are alternatives.
func (r *rule) matches(o Object) bool {
	switch o.Kind {
	case KindEntity:
		return r.matchesEntity(o)
	case KindDashboard:
		return containsFold(o.ID, r.dashboards)
	default:
		return false
	}
}

func (r *rule) matchesEntity(o Object) bool {
	id := strings.ToLower(o.ID)
	for _, g := range r.globs {
		if ok, err := path.Match(g, id); err == nil && ok {
			return true
		}
	}
	return anyFold(o.Areas, r.areas) || anyFold(o.Labels, r.labels) || anyFold(o.Devices, r.devices)
}

func (r *rule) hasAttributeMatcher() bool {
	return len(r.areas)+len(r.labels)+len(r.devices) > 0
}

func containsFold(v string, set []string) bool {
	return slices.ContainsFunc(set, func(s string) bool { return strings.EqualFold(s, v) })
}

func anyFold(vals, set []string) bool {
	return slices.ContainsFunc(vals, func(v string) bool { return containsFold(v, set) })
}

// Decide returns the capabilities granted on o: the default, plus every
// matching allow, minus every matching deny; a matching allow: none clears
// everything; create/delete are off when any matching rule turns them off.
func (p *Policy) Decide(o Object) Grant {
	allowed, denied := p.base, capSet{}
	var hidden, noCreate, noDelete bool
	for i := range p.rules {
		r := &p.rules[i]
		if !r.matches(o) {
			continue
		}
		allowed = allowed.union(r.allow)
		denied = denied.union(r.deny)
		hidden = hidden || r.allowNone
		noCreate = noCreate || r.noCreate
		noDelete = noDelete || r.noDelete
	}
	if hidden || !allowed.read {
		return Grant{}
	}
	configure := allowed.configure && !denied.configure
	return Grant{
		Read:      true,
		Control:   allowed.control && !denied.control,
		Configure: configure,
		Create:    configure && !noCreate,
		Delete:    configure && !noDelete,
	}
}

// SystemAllowed reports whether the system operation class v is permitted. An
// absent system section follows the default (write allows everything); a
// present one allows exactly the listed values, or everything with all.
func (p *Policy) SystemAllowed(v SystemValue) bool {
	if !validSystemValue(v) {
		return false
	}
	if p.systemAbsent {
		return p.level == levelWrite
	}
	return p.systemAll || slices.Contains(p.system, v)
}

func validSystemValue(v SystemValue) bool {
	switch v {
	case SystemReload, SystemRestart, SystemBackup, SystemUpdates, SystemConfigEntries, SystemHACS,
		SystemAddons, SystemBlueprints, SystemStatistics, SystemLogs, SystemHomeAssistant, SystemAll:
		return true
	default:
		return false
	}
}

// Unrestricted reports whether the policy grants everything: default write, no
// rules, and the system section absent or all.
func (p *Policy) Unrestricted() bool {
	return p.level == levelWrite && len(p.rules) == 0 && (p.systemAbsent || p.systemAll)
}

// HidesAnything reports whether any object can be hidden.
func (p *Policy) HidesAnything() bool {
	return p.level == levelNone || slices.ContainsFunc(p.rules, func(r rule) bool { return r.allowNone })
}

// NeedsRegistry reports whether any rule uses an area, label or device
// matcher, so decisions need registry data.
func (p *Policy) NeedsRegistry() bool {
	return slices.ContainsFunc(p.rules, func(r rule) bool { return r.hasAttributeMatcher() })
}

// DefaultIsWrite reports whether the default is write.
func (p *Policy) DefaultIsWrite() bool { return p.level == levelWrite }

// Summary is a one-line description for logs and `ha-mcp config`. It names the
// default, the rule count and the system section, never rule contents.
func (p *Policy) Summary() string {
	noun := "rules"
	if len(p.rules) == 1 {
		noun = "rule"
	}
	return fmt.Sprintf("access policy: default=%s, %d %s, system=%s", p.level, len(p.rules), noun, p.systemSummary())
}

func (p *Policy) systemSummary() string {
	switch {
	case p.systemAbsent:
		return "absent (follows default)"
	case len(p.system) == 0:
		return levelNone
	default:
		parts := make([]string, len(p.system))
		for i, v := range p.system {
			parts[i] = string(v)
		}
		return strings.Join(parts, ",")
	}
}
