package access

import (
	"path"
	"strings"
)

// globMeta are the characters with special meaning in path.Match.
const globMeta = `*?[\`

// DomainWide reports whether every attribute-less entity of domain, whatever
// its id, is granted c. It is conservative (false when unsure): any rule that
// could remove c from some entity of the domain makes it false. Such a rule is
// a deny of c (for configure also create: false, since callers use this for
// creates with unpredictable ids) or allow: none whose entity glob has a
// literal domain equal to domain or no literal domain part at all, or that has
// any area, label or device matcher. Without a baseline grant, an allow of c
// must cover the whole domain.
func (p *Policy) DomainWide(domain string, c Capability) bool {
	domain = strings.ToLower(domain)
	if domain == "" || !(Grant{Read: true, Control: true, Configure: true}).Has(c) {
		return false
	}
	for i := range p.rules {
		r := &p.rules[i]
		if r.restricts(c) && r.mayMatchDomain(domain) {
			return false
		}
	}
	if p.base.has(c) {
		return true
	}
	for i := range p.rules {
		r := &p.rules[i]
		if r.allow.has(c) && r.coversDomain(domain) {
			return true
		}
	}
	return false
}

// restricts reports whether the rule can take c away from an object it matches.
func (r *rule) restricts(c Capability) bool {
	return r.allowNone || r.deny.has(c) || (c == CapConfigure && r.noCreate)
}

// mayMatchDomain reports whether the rule could select some entity of domain.
// Area, label and device matchers can select an entity of any domain.
func (r *rule) mayMatchDomain(domain string) bool {
	if r.hasAttributeMatcher() {
		return true
	}
	for _, g := range r.globs {
		lit, ok := literalDomain(g)
		if !ok || lit == domain {
			return true
		}
	}
	return false
}

// coversDomain reports whether one of the rule's globs matches every entity id
// of domain.
func (r *rule) coversDomain(domain string) bool {
	for _, g := range r.globs {
		if globCoversDomain(g, domain) {
			return true
		}
	}
	return false
}

// literalDomain returns the part of glob before the first dot when it has no
// glob metacharacters. ok is false when there is no dot or the part is not
// literal: such a glob may match an entity of any domain.
func literalDomain(glob string) (lit string, ok bool) {
	d, _, found := strings.Cut(glob, ".")
	if !found || strings.ContainsAny(d, globMeta) {
		return "", false
	}
	return d, true
}

// globCoversDomain reports whether glob provably matches "<domain>.<x>" for
// every object id x. It holds when glob is Q followed by one or more '*' and Q
// matches "<domain>." or "<domain>" (the trailing star then consumes the rest,
// as entity ids contain no '/'), or Q is empty. Anything else is treated as not
// covering (fail closed).
func globCoversDomain(glob, domain string) bool {
	q := strings.TrimRight(glob, "*")
	if q == glob {
		return false
	}
	if q == "" {
		return true
	}
	if strings.HasSuffix(q, `\`) {
		return false // the last star was escaped
	}
	for _, s := range []string{domain + ".", domain} {
		if ok, err := path.Match(q, s); err == nil && ok {
			return true
		}
	}
	return false
}
