package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/editorconfig/editorconfig-core-go/v2"
	"github.com/go-logr/logr"
	"github.com/logrusorgru/aurora"
	"github.com/mattn/go-colorable"
	"k8s.io/klog/v2"
	"k8s.io/klog/v2/klogr"
)

var (
	version = "dev"
	log     logr.Logger
)

func walk(paths ...string) ([]string, error) {
	files := make([]string, 0)
	for _, path := range paths {
		err := filepath.Walk(path, func(p string, i os.FileInfo, e error) error {
			if e != nil {
				return e
			}
			mode := i.Mode()
			if mode.IsRegular() && !mode.IsDir() {
				log.V(4).Info("index %s", p)
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return files, err
		}
	}
	return files, nil
}

// listFiles returns the list of files based on the input.
//
// When its empty, it relies on `git ls-files` first, which
// whould fail if `git` is not present or the current working
// directory is not managed by it. In that case, it work the
// current working directory.
//
// When args are given, it recursively walks into them.
func listFiles(args ...string) ([]string, error) {
	if len(args) == 0 {
		fs, err := gitLsFiles(".")
		if err == nil {
			return fs, nil
		}

		log.Error(err, "git ls-files failure")
		args = append(args, ".")
	}

	return walk(args...)
}

func main() {
	var stdout io.Writer = os.Stdout
	isTerminal := terminal.IsTerminal(syscall.Stdout)
	if runtime.GOOS == "windows" {
		stdout = colorable.NewColorableStdout()
	}

	var flagVersion bool

	exclude := ""
	noColors := false
	summary := false
	showAllErrors := false
	showErrorQuantity := 10

	klog.InitFlags(nil)
	flag.BoolVar(&flagVersion, "version", false, "print the version number")
	flag.BoolVar(&noColors, "no_colors", false, "enable or disable colors")
	flag.BoolVar(&summary, "summary", false, "enable the summary view")
	flag.BoolVar(
		&showAllErrors,
		"show_all_errors",
		false,
		fmt.Sprintf("display all errors for each file (otherwise %d are kept)", showErrorQuantity),
	)
	flag.StringVar(&exclude, "exclude", "", "paths to exclude")
	flag.Parse()

	if flagVersion {
		fmt.Fprintf(stdout, "eclint %s\n", version)
		return
	}

	log = klogr.New()

	args := flag.Args()
	files, err := listFiles(args...)
	if err != nil {
		log.Error(err, "error while handling the arguments")
		flag.Usage()
		os.Exit(1)
		return
	}

	au := aurora.NewAurora(isTerminal && !noColors)
	log.V(1).Info("files", "count", len(files), "exclude", exclude)

	c := 0
	for _, filename := range files {
		// Skip excluded files
		if exclude != "" {
			ok, err := editorconfig.FnmatchCase(exclude, filename)
			if err != nil {
				log.Error(err, "exclude pattern failure", "exclude", exclude)
				fmt.Fprintf(stdout, "exclude pattern failure %s", err)
				c++
				break
			}
			if ok {
				continue
			}
		}

		d := 0
		errs := lint(filename, log)
		for _, err := range errs {
			if err != nil {
				if d == 0 && !summary {
					fmt.Fprintf(stdout, "%s:\n", au.Magenta(filename))
				}

				if ve, ok := err.(validationError); ok {
					log.V(4).Info("lint error", "error", ve)
					if !summary {
						vi := au.Green(strconv.Itoa(ve.index))
						vp := au.Green(strconv.Itoa(ve.position))
						fmt.Fprintf(stdout, "%s:%s: %s\n", vi, vp, ve.error)
						l, err := errorAt(au, ve.line, ve.position-1)
						if err != nil {
							log.Error(err, "line formating failure", "error", ve)
							continue
						}
						fmt.Fprintln(stdout, l)
					}
				} else {
					log.V(4).Info("lint error", "filename", filename, "error", err)
					fmt.Fprintln(stdout, err)
				}

				if d >= showErrorQuantity && len(errs) > d {
					fmt.Fprintln(
						stdout,
						fmt.Sprintf(" ... skipping at most %s errors", au.BrightRed(strconv.Itoa(len(errs)-d))),
					)
					break
				}

				d++
				c++
			}
		}
		if d > 0 {
			if !summary {
				fmt.Fprintln(stdout, "")
			} else {
				fmt.Fprintf(stdout, "%s: %d errors\n", au.Magenta(filename), d)
			}
		}
	}
	if c > 0 {
		log.V(1).Info("Some errors were found.", "count", c)
		os.Exit(1)
	}
}

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
		}
	}

	return b.String(), nil
}
