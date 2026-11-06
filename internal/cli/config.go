package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/codyconfer/pockit/data"
	"github.com/codyconfer/pockit/internal/config"
	uiadapter "github.com/codyconfer/pockit/internal/ui"
)

func runConfig(args []string, out, errOut io.Writer) int {
	if len(args) == 0 || (args[0] != "init" && args[0] != "schema") {
		fmt.Fprintln(errOut, "usage: pockit config init|schema [--out FILE] [--overwrite]")
		return Invalid
	}
	command := args[0]
	f := flag.NewFlagSet("config "+command, flag.ContinueOnError)
	var help bytes.Buffer
	f.SetOutput(&help)
	var output string
	var overwrite, machine, plain bool
	f.BoolVar(&overwrite, "overwrite", false, "replace existing regular files")
	f.BoolVar(&machine, "json", false, "machine-readable output (schema is always JSON)")
	f.BoolVar(&plain, "plain", false, "disable styling")
	if command == "schema" {
		f.StringVar(&output, "out", "", "write schema to a file instead of stdout")
	}
	if err := f.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			fmt.Fprint(out, help.String())
			return Success
		}
		uiadapter.Warning(errOut, err.Error(), plain || machine)
		return Invalid
	}
	if f.NArg() != 0 {
		uiadapter.Warning(errOut, "unexpected positional arguments", plain || machine)
		return Invalid
	}
	fail := func(err error) int {
		uiadapter.Warning(errOut, err.Error(), plain || machine)
		if errors.Is(err, config.ErrRefused) {
			return Refused
		}
		return Partial
	}
	var paths []string
	if command == "schema" {
		if output == "" {
			if _, err := out.Write(data.Schema); err != nil {
				return fail(err)
			}
			return Success
		}
		if err := config.ExportSchema(output, overwrite); err != nil {
			return fail(err)
		}
		paths = []string{output}
	} else {
		var err error
		paths, err = config.Install(overwrite)
		if err != nil {
			return fail(err)
		}
	}
	if machine {
		if err := json.NewEncoder(out).Encode(map[string]any{"version": 1, "ok": true, "paths": paths}); err != nil {
			return fail(err)
		}
	} else if err := uiadapter.WritePanel(out, "POCKIT · Configuration", paths, plain); err != nil {
		return fail(err)
	}
	return Success
}
