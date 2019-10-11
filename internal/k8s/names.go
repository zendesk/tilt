package k8s

import (
	"fmt"
	"strings"
)

// Calculates names for entities by using the shortest uniquely matching identifiers
// Sometimes users have two resources with the same fullname. I.e., the same
// name/kind/namespace/group This usually means the user is trying to
// deploy the same resource twice. Kubernetes will not treat these as
// unique.
//
// We should surface a warning or error about this somewhere else that has
// more context on how to fix it.
// https://github.com/windmilleng/tilt/issues/1852
//
// But for now, append the index to the name to make it unique
func UniqueNames(es []K8sEntity) []string {
	_, unique, repeated := calculateNames(es)
	result := make([]string, len(es))
	for i, e := range es {
		fullname := Fullname(e)
		result[i] = fullname
		if j, ok := repeated[fullname]; ok {
			// just kidding; add an index to dedupe
			result[i] = fmt.Sprintf("%s:%d", fullname, j)
			repeated[fullname] = repeated[fullname] + 1
		}
	}

	return result
}

// We want to let users refer to objects precisely and succinctly.
// Kubernetes doesn't seem to have a good definition of this, so we have our own.
// First, precision.
// We use name, kind, namespace, and group, in that order to construct a "fullname".
// E.g., foo:deployment:default:apps
//
// The fullname is cumbersome for both lookup and display. We define a "shortname" for display, and a "key" for lookup.
//
// Every object has:
// * exactly 1 fullname
// * exactly 1 shortname (used for display)
// * 4 fragments (for lookups)
// * at least 1 unique fragment
// (Note: all of this only makes sense within some set of entities)
//
//
func CalculateNames(es []K8sEntity) (lookup map[string][]string, unique map[string]string) {
	lookup, unique, _ = calculateNames(es)
	return lookup, unique
}

func calculateNames(es []K8sEntity) (lookup map[string][]string, unique map[string]string, repeated map[string]int) {
	// lookup: fragment -> matching fullnames
	// fragments: fullname -> fragments
	// repeated: fullname -> 0
	// unique: fullname -> shortname
	lookup = make(map[string][]string)
	fragments := make(map[string][]string)
	repeated = make(map[string]int)
	for _, e := range es {
		fs := genFragments(e)
		fullname := fs[3]
		if len(fragments[fullname]) > 0 {
			// duplicate fullname
			repeated[fullname] = 0
			break
		}

		fragments[fullname] = fs
		for _, f := range fs {
			lookup[f] = append(lookup[f], fullname)
		}
	}

	unique = make(map[string]string)
	for fullname, fs := range fragments {
		// find the first fragment that only has one matching fullname
		for _, f := range fs {
			if len(lookup[f]) == 1 {
				unique[fullname] = f
			}
		}
	}

	return lookup, unique, repeated
}

func Fullname(e K8sEntity) string {
	return genFragments(e)[3]
}

// returns a list of potential names, in order of preference
func genFragments(e K8sEntity) []string {
	gvk := e.GVK()
	components := []string{
		e.Name(),
		gvk.Kind,
		e.Namespace().String(),
		gvk.Group,
	}
	var ret []string
	for i := 0; i < len(components); i++ {
		ret = append(ret, strings.ToLower(strings.Join(components[:i+1], ":")))
	}
	return ret
}
