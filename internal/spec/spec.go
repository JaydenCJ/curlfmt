// Package spec is curlfmt's knowledge of the curl command line: which
// options exist, their short and long spellings, whether they take a value,
// which formatting group they belong to, and whether repeating them is a
// last-one-wins mistake. The table covers the options that appear in real
// documentation and CI scripts; unknown options are passed through untouched
// and reported by the linter.
package spec

// Group classifies an option for canonical output ordering (the print
// order itself lives in the formatter). GroupOther is deliberately the zero
// value so the big "everything else" block of the table needs no annotation.
type Group int

const (
	GroupOther  Group = iota // everything else (original order preserved)
	GroupMethod              // --request
	GroupAuth                // credentials and tokens
	GroupHeader              // --header
	GroupData                // request bodies and forms
	GroupOutput              // where the response goes
)

// Option describes one curl option.
type Option struct {
	Long       string // canonical long name, without the leading --
	Short      byte   // 0 when the option has no short form
	TakesValue bool
	Group      Group
	// SingleUse marks options where curl silently keeps only the last
	// occurrence, so repeating them is almost always a mistake.
	SingleUse bool
}

// table is the curated option list. Kept alphabetical by Long within each
// block for reviewability.
var table = []Option{
	// Method.
	{Long: "request", Short: 'X', TakesValue: true, Group: GroupMethod, SingleUse: true},

	// Auth.
	{Long: "oauth2-bearer", TakesValue: true, Group: GroupAuth, SingleUse: true},
	{Long: "user", Short: 'u', TakesValue: true, Group: GroupAuth, SingleUse: true},

	// Headers.
	{Long: "header", Short: 'H', TakesValue: true, Group: GroupHeader},

	// Bodies and forms.
	{Long: "data", Short: 'd', TakesValue: true, Group: GroupData},
	{Long: "data-ascii", TakesValue: true, Group: GroupData},
	{Long: "data-binary", TakesValue: true, Group: GroupData},
	{Long: "data-raw", TakesValue: true, Group: GroupData},
	{Long: "data-urlencode", TakesValue: true, Group: GroupData},
	{Long: "form", Short: 'F', TakesValue: true, Group: GroupData},
	{Long: "form-string", TakesValue: true, Group: GroupData},
	{Long: "json", TakesValue: true, Group: GroupData},

	// Output.
	{Long: "dump-header", Short: 'D', TakesValue: true, Group: GroupOutput},
	{Long: "output", Short: 'o', TakesValue: true, Group: GroupOutput},
	{Long: "output-dir", TakesValue: true, Group: GroupOutput, SingleUse: true},
	{Long: "stderr", TakesValue: true, Group: GroupOutput, SingleUse: true},
	{Long: "trace", TakesValue: true, Group: GroupOutput, SingleUse: true},
	{Long: "trace-ascii", TakesValue: true, Group: GroupOutput, SingleUse: true},
	{Long: "write-out", Short: 'w', TakesValue: true, Group: GroupOutput, SingleUse: true},

	// Everything else that takes a value.
	{Long: "aws-sigv4", TakesValue: true, SingleUse: true},
	{Long: "cacert", TakesValue: true, SingleUse: true},
	{Long: "capath", TakesValue: true, SingleUse: true},
	{Long: "cert", Short: 'E', TakesValue: true, SingleUse: true},
	{Long: "ciphers", TakesValue: true, SingleUse: true},
	{Long: "config", Short: 'K', TakesValue: true},
	{Long: "connect-timeout", TakesValue: true, SingleUse: true},
	{Long: "connect-to", TakesValue: true},
	{Long: "continue-at", Short: 'C', TakesValue: true, SingleUse: true},
	{Long: "cookie", Short: 'b', TakesValue: true},
	{Long: "cookie-jar", Short: 'c', TakesValue: true, SingleUse: true},
	{Long: "interface", TakesValue: true, SingleUse: true},
	{Long: "keepalive-time", TakesValue: true, SingleUse: true},
	{Long: "key", TakesValue: true, SingleUse: true},
	{Long: "limit-rate", TakesValue: true, SingleUse: true},
	{Long: "max-filesize", TakesValue: true, SingleUse: true},
	{Long: "max-redirs", TakesValue: true, SingleUse: true},
	{Long: "max-time", Short: 'm', TakesValue: true, SingleUse: true},
	{Long: "noproxy", TakesValue: true, SingleUse: true},
	{Long: "proxy", Short: 'x', TakesValue: true, SingleUse: true},
	{Long: "proxy-user", Short: 'U', TakesValue: true, SingleUse: true},
	{Long: "range", Short: 'r', TakesValue: true, SingleUse: true},
	{Long: "referer", Short: 'e', TakesValue: true, SingleUse: true},
	{Long: "request-target", TakesValue: true, SingleUse: true},
	{Long: "resolve", TakesValue: true},
	{Long: "retry", TakesValue: true, SingleUse: true},
	{Long: "retry-delay", TakesValue: true, SingleUse: true},
	{Long: "retry-max-time", TakesValue: true, SingleUse: true},
	{Long: "speed-limit", Short: 'Y', TakesValue: true, SingleUse: true},
	{Long: "speed-time", Short: 'y', TakesValue: true, SingleUse: true},
	{Long: "time-cond", Short: 'z', TakesValue: true, SingleUse: true},
	{Long: "tls-max", TakesValue: true, SingleUse: true},
	{Long: "unix-socket", TakesValue: true, SingleUse: true},
	{Long: "upload-file", Short: 'T', TakesValue: true},
	{Long: "url", TakesValue: true},
	{Long: "user-agent", Short: 'A', TakesValue: true, SingleUse: true},

	// Boolean flags.
	{Long: "anyauth"},
	{Long: "basic"},
	{Long: "compressed"},
	{Long: "create-dirs"},
	{Long: "digest"},
	{Long: "disable", Short: 'q'},
	{Long: "disallow-username-in-url"},
	{Long: "fail", Short: 'f'},
	{Long: "fail-early"},
	{Long: "fail-with-body"},
	{Long: "get", Short: 'G'},
	{Long: "globoff", Short: 'g'},
	{Long: "head", Short: 'I'},
	{Long: "http1.0", Short: '0'},
	{Long: "http1.1"},
	{Long: "http2"},
	{Long: "http3"},
	{Long: "include", Short: 'i'},
	{Long: "insecure", Short: 'k'},
	{Long: "ipv4", Short: '4'},
	{Long: "ipv6", Short: '6'},
	{Long: "junk-session-cookies", Short: 'j'},
	{Long: "location", Short: 'L'},
	{Long: "location-trusted"},
	{Long: "negotiate"},
	{Long: "netrc", Short: 'n'},
	{Long: "netrc-optional"},
	{Long: "no-buffer", Short: 'N'},
	{Long: "no-keepalive"},
	{Long: "no-progress-meter"},
	{Long: "ntlm"},
	{Long: "parallel", Short: 'Z'},
	{Long: "progress-bar", Short: '#'},
	{Long: "proxytunnel", Short: 'p'},
	{Long: "remote-header-name", Short: 'J'},
	{Long: "remote-name", Short: 'O'},
	{Long: "remote-time", Short: 'R'},
	{Long: "retry-all-errors"},
	{Long: "retry-connrefused"},
	{Long: "show-error", Short: 'S'},
	{Long: "silent", Short: 's'},
	{Long: "ssl-reqd"},
	{Long: "tcp-fastopen"},
	{Long: "tcp-nodelay"},
	{Long: "tlsv1", Short: '1'},
	{Long: "tlsv1.0"},
	{Long: "tlsv1.1"},
	{Long: "tlsv1.2"},
	{Long: "tlsv1.3"},
	{Long: "verbose", Short: 'v'},
}

var (
	byLong  = map[string]*Option{}
	byShort = map[byte]*Option{}
)

func init() {
	for i := range table {
		o := &table[i]
		byLong[o.Long] = o
		if o.Short != 0 {
			byShort[o.Short] = o
		}
	}
}

// ByLong resolves a long option name (without dashes). It also understands
// curl's --no-<flag> negation prefix for boolean flags, returning a
// synthetic boolean option that keeps the negated spelling.
func ByLong(name string) (Option, bool) {
	if o, ok := byLong[name]; ok {
		return *o, true
	}
	if rest, ok := cutPrefix(name, "no-"); ok {
		if o, exists := byLong[rest]; exists && !o.TakesValue {
			return Option{Long: name, Group: o.Group}, true
		}
	}
	return Option{}, false
}

// ByShort resolves a single-letter option.
func ByShort(c byte) (Option, bool) {
	if o, ok := byShort[c]; ok {
		return *o, true
	}
	return Option{}, false
}

// Count returns the number of options in the table (used by tests to guard
// against accidental deletions).
func Count() int { return len(table) }

func cutPrefix(s, prefix string) (string, bool) {
	if len(s) > len(prefix) && s[:len(prefix)] == prefix {
		return s[len(prefix):], true
	}
	return s, false
}
