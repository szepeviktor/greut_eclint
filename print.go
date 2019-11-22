package eclint

import (
	"bytes"
	"fmt"
	"strconv"

	"github.com/logrusorgru/aurora"
)

// PrintErrors is the rich output of the program.
func PrintErrors(opt Option, filename string, errors []error) error {
	counter := 0

	log := opt.Log
	stdout := opt.Stdout

	au := aurora.NewAurora(opt.IsTerminal && !opt.NoColors)

	for _, err := range errors {
		if err != nil {
			if counter == 0 && !opt.Summary {
				fmt.Fprintf(stdout, "%s:\n", au.Magenta(filename))
			}

			if ve, ok := err.(validationError); ok {
				log.V(4).Info("lint error", "error", ve)
				if !opt.Summary {
					vi := au.Green(strconv.Itoa(ve.index))
					vp := au.Green(strconv.Itoa(ve.position))
					fmt.Fprintf(stdout, "%s:%s: %s\n", vi, vp, ve.error)
					l, err := errorAt(au, ve.line, ve.position-1)
					if err != nil {
						log.Error(err, "line formating failure", "error", ve)
						return err
					}
					fmt.Fprintln(stdout, l)
				}
			} else {
				log.V(4).Info("lint error", "filename", filename, "error", err)
				fmt.Fprintln(stdout, err)
			}

			if counter >= opt.ShowErrorQuantity && len(errors) > counter {
				fmt.Fprintln(
					stdout,
					fmt.Sprintf(" ... skipping at most %s errors", au.BrightRed(strconv.Itoa(len(errors)-counter))),
				)
				break
			}

			counter++
		}
	}

	if counter > 0 {
		if !opt.Summary {
			fmt.Fprintln(stdout, "")
		} else {
			fmt.Fprintf(stdout, "%s: %d errors\n", au.Magenta(filename), counter)
		}
	}
	return nil
}

// errorAt highlights the validationError position within the line.
func errorAt(au aurora.Aurora, line []byte, position int) (string, error) {
	b := bytes.NewBuffer(make([]byte, len(line)))

	if position > len(line) {
		position = len(line)
	}

	for i := 0; i < position; i++ {
		if line[i] != cr && line[i] != lf {
			if err := b.WriteByte(line[i]); err != nil {
				return "", err
			}

			// skip 0x10xxxxxx that are UTF-8 continuation markers
			if (line[i] >> 6) == 0b10 {
				position++
			}
		}
	}

	// XXX this will break every non latin1 line.
	s := " "
	if position < len(line)-1 {
		s = string(line[position : position+1])
	}
	if _, err := b.WriteString(au.White(s).BgRed().String()); err != nil {
		return "", err
	}

	for i := position + 1; i < len(line); i++ {
		if line[i] != cr && line[i] != lf {
			if err := b.WriteByte(line[i]); err != nil {
				return "", err
			}

			if (line[i] >> 6) == 0b10 {
				position++
			}
		}
	}

	return b.String(), nil
}
