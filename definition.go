package eclint

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/editorconfig/editorconfig-core-go/v2"
)

// ErrNotImplemented represents a missing feature.
var ErrNotImplemented = errors.New("not implemented yet, PRs are welcome")

// definition contains the fields that aren't native to EditorConfig.Definition.
type definition struct {
	editorconfig.Definition
	BlockCommentStart  []byte
	BlockComment       []byte
	BlockCommentEnd    []byte
	MaxLength          int
	TabWidth           int
	IndentSize         int
	LastLine           []byte
	LastIndex          int
	InsideBlockComment bool
}

func newDefinition(d *editorconfig.Definition) (*definition, error) { //nolint:cyclop,gocognit
	def := &definition{
		Definition: *d,
		TabWidth:   d.TabWidth,
	}

	if def.Charset == "utf-8-bom" {
		def.Charset = "utf-8 bom"
	}

	if d.IndentSize != "" && d.IndentSize != UnsetValue {
		is, err := strconv.Atoi(d.IndentSize)
		if err != nil {
			return nil, fmt.Errorf("cannot convert indentsize %q to int: %w", d.IndentSize, err)
		}

		def.IndentSize = is
	}

	if def.IndentStyle != "" && def.IndentStyle != UnsetValue { //nolint:nestif
		bs, ok := def.Raw["block_comment_start"]
		if ok && bs != "" && bs != UnsetValue {
			def.BlockCommentStart = []byte(bs)
			bc, ok := def.Raw["block_comment"]

			if ok && bc != "" && bs != UnsetValue {
				def.BlockComment = []byte(bc)
			}

			be, ok := def.Raw["block_comment_end"]
			if !ok || be == "" || be == UnsetValue {
				return nil, fmt.Errorf(
					"%w: .editorconfig: block_comment_end was expected, none were found",
					ErrConfiguration,
				)
			}

			def.BlockCommentEnd = []byte(be)
		}
	}

	if mll, ok := def.Raw["max_line_length"]; ok && mll != "off" && mll != UnsetValue {
		ml, er := strconv.Atoi(mll)
		if er != nil || ml < 0 {
			return nil, fmt.Errorf(
				"%w: .editorconfig: max_line_length expected a non-negative number, got %q",
				ErrConfiguration,
				mll,
			)
		}

		def.MaxLength = ml

		if def.TabWidth <= 0 {
			def.TabWidth = DefaultTabWidth
		}
	}

	return def, nil
}

// EOL returns the byte value of the given definition.
func (def *definition) EOL() ([]byte, error) {
	switch def.EndOfLine {
	case "cr":
		return []byte{cr}, nil
	case "crlf":
		return []byte{cr, lf}, nil
	case "lf":
		return []byte{lf}, nil
	default:
		return nil, fmt.Errorf("%w: unsupported EndOfLine value %s", ErrConfiguration, def.EndOfLine)
	}
}

// OverrideDefinitionUsingPrefix is an helper that takes the prefixed values.
//
// It replaces those values into the nominal ones. That way a tool could a
// different set of definition than the real editor would.
func OverrideDefinitionUsingPrefix(def *editorconfig.Definition, prefix string) error {
	for k, v := range def.Raw {
		if strings.HasPrefix(k, prefix) {
			nk := k[len(prefix):]
			def.Raw[nk] = v

			switch nk {
			case "indent_style":
				def.IndentStyle = v
			case "indent_size":
				def.IndentSize = v
			case "charset":
				def.Charset = v
			case "end_of_line":
				def.EndOfLine = v
			case "tab_width":
				i, err := strconv.Atoi(v)
				if err != nil {
					return fmt.Errorf("tab_width cannot be set. %w", err)
				}

				def.TabWidth = i
			case "trim_trailing_whitespace":
				return fmt.Errorf("%v cannot be overridden: %w", nk, ErrNotImplemented)
			case "insert_final_newline":
				return fmt.Errorf("%v cannot be overridden: %w", nk, ErrNotImplemented)
			}
		}
	}

	return nil
}
