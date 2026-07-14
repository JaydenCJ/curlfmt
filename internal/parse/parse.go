// Package parse turns the lexed words of a curl invocation into a
// structured Command that the formatter and linter share. Parsing never
// fails hard: anything the spec table does not recognize is preserved as a
// passthrough item and surfaced as a Note for the linter.
package parse

import (
	"fmt"
	"strings"

	"github.com/JaydenCJ/curlfmt/internal/shell"
	"github.com/JaydenCJ/curlfmt/internal/spec"
)

// Item is one parsed argument.
type Item struct {
	// Opt is set for recognized options. For passthrough items it is the
	// zero Option and Raw holds the original text.
	Opt   spec.Option
	Known bool
	// Value is the option's argument (valid when Opt.TakesValue).
	Value shell.Word
	// Raw is the verbatim source for passthrough items.
	Raw string
	// EqualsForm records that the source spelled this as --name=value,
	// which curl does not accept (lint CF018; --fix splits it).
	EqualsForm bool
}

// NoteKind classifies parser observations for the linter.
type NoteKind int

const (
	NoteUnknownLong NoteKind = iota
	NoteUnknownShort
	NoteMissingValue
	NoteEqualsForm
)

// Note is a parser observation the linter turns into a finding.
type Note struct {
	Kind NoteKind
	Text string // the option spelling involved
}

// Command is a fully parsed curl invocation.
type Command struct {
	Items []Item       // options in source order
	URLs  []shell.Word // positional arguments and --url values, in order
	Notes []Note
	// Suffix is the raw trailing shell text (`| jq .`, `> out.json`, …)
	// carried through from the lexer, empty when absent.
	Suffix string
}

// IsCurl reports whether words begin a curl invocation.
func IsCurl(words []shell.Word) bool {
	return len(words) > 0 && words[0].Value == "curl"
}

// Parse consumes the words of one command (words[0] must be "curl").
func Parse(words []shell.Word, suffix string) (*Command, error) {
	if !IsCurl(words) {
		return nil, fmt.Errorf("parse: not a curl command")
	}
	cmd := &Command{Suffix: suffix}
	args := words[1:]
	for i := 0; i < len(args); i++ {
		w := args[i]
		v := w.Value
		switch {
		case !w.Literal && strings.HasPrefix(v, "-"):
			// An option whose text involves expansion ($FLAGS etc.):
			// keep it verbatim, exactly where it was.
			cmd.Items = append(cmd.Items, Item{Raw: w.Raw})
		case strings.HasPrefix(v, "--") && len(v) > 2:
			i += cmd.parseLong(v[2:], args, i)
		case strings.HasPrefix(v, "-") && len(v) > 1 && v != "--":
			i += cmd.parseShortCluster(w, args, i)
		default:
			cmd.URLs = append(cmd.URLs, w)
		}
	}
	return cmd, nil
}

// parseLong handles one --name argument; returns how many extra args were
// consumed (0 or 1).
func (c *Command) parseLong(name string, args []shell.Word, i int) int {
	if eq := strings.IndexByte(name, '='); eq >= 0 {
		base := name[:eq]
		if opt, ok := spec.ByLong(base); ok && opt.TakesValue {
			c.Items = append(c.Items, Item{
				Opt:        opt,
				Known:      true,
				Value:      shell.Word{Value: name[eq+1:], Literal: true},
				EqualsForm: true,
			})
			c.Notes = append(c.Notes, Note{Kind: NoteEqualsForm, Text: "--" + base})
			return 0
		}
		c.Items = append(c.Items, Item{Raw: args[i].Raw})
		c.Notes = append(c.Notes, Note{Kind: NoteUnknownLong, Text: "--" + name})
		return 0
	}
	opt, ok := spec.ByLong(name)
	if !ok {
		c.Items = append(c.Items, Item{Raw: args[i].Raw})
		c.Notes = append(c.Notes, Note{Kind: NoteUnknownLong, Text: "--" + name})
		return 0
	}
	if !opt.TakesValue {
		c.Items = append(c.Items, Item{Opt: opt, Known: true})
		return 0
	}
	if i+1 >= len(args) {
		c.Items = append(c.Items, Item{Raw: args[i].Raw})
		c.Notes = append(c.Notes, Note{Kind: NoteMissingValue, Text: "--" + name})
		return 0
	}
	c.addValued(opt, args[i+1])
	return 1
}

// parseShortCluster handles -sSL style clusters, including attached values
// (-XPOST, -d@body.json); returns how many extra args were consumed.
func (c *Command) parseShortCluster(w shell.Word, args []shell.Word, i int) int {
	body := w.Value[1:]
	for j := 0; j < len(body); j++ {
		opt, ok := spec.ByShort(body[j])
		if !ok {
			// Unknown short flag: preserve the whole original token so we
			// cannot mangle whatever curl (or a fork) would do with it.
			c.Items = append(c.Items, Item{Raw: w.Raw})
			c.Notes = append(c.Notes, Note{Kind: NoteUnknownShort, Text: "-" + string(body[j])})
			return 0
		}
		if !opt.TakesValue {
			c.Items = append(c.Items, Item{Opt: opt, Known: true})
			continue
		}
		if j+1 < len(body) {
			// Attached value: -XPOST, -ubob:pw. Attached values are part
			// of a literal token, so the value is literal too — unless the
			// source word carried an expansion, in which case the whole
			// cluster was already passed through above.
			val := shell.Word{Value: body[j+1:], Literal: w.Literal, Raw: body[j+1:]}
			if !w.Literal {
				c.Items = append(c.Items, Item{Raw: w.Raw})
				return 0
			}
			c.addValued(opt, val)
			return 0
		}
		if i+1 >= len(args) {
			c.Items = append(c.Items, Item{Raw: w.Raw})
			c.Notes = append(c.Notes, Note{Kind: NoteMissingValue, Text: "-" + string(body[j])})
			return 0
		}
		c.addValued(opt, args[i+1])
		return 1
	}
	return 0
}

func (c *Command) addValued(opt spec.Option, value shell.Word) {
	if opt.Long == "url" {
		c.URLs = append(c.URLs, value)
		return
	}
	c.Items = append(c.Items, Item{Opt: opt, Known: true, Value: value})
}

// Has reports whether the command carries the named long option.
func (c *Command) Has(long string) bool {
	for _, it := range c.Items {
		if it.Known && it.Opt.Long == long {
			return true
		}
	}
	return false
}

// First returns the first item for the named long option.
func (c *Command) First(long string) (Item, bool) {
	for _, it := range c.Items {
		if it.Known && it.Opt.Long == long {
			return it, true
		}
	}
	return Item{}, false
}

// HasData reports whether any request-body option is present.
func (c *Command) HasData() bool {
	for _, it := range c.Items {
		if it.Known && it.Opt.Group == spec.GroupData {
			return true
		}
	}
	return false
}
