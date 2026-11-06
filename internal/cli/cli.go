package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/codyconfer/pockit/internal/app"
	"github.com/codyconfer/pockit/internal/fetching"
	"github.com/codyconfer/pockit/internal/skills"
	uiadapter "github.com/codyconfer/pockit/internal/ui"
)

const (
	Success = 0
	Invalid = 2
	Refused = 3
	Partial = 4
)
const usage = `PocKit: offline catalog and role-based skill bundles
Usage (flags follow the command):
  pockit config init [--overwrite] [--json]
  pockit config schema [--out FILE] [--overwrite]
  pockit input --sources FILE --roles FILE [--out DIR] [--pi-extension | --opencode-plugin | --goose-plugin | --cursor-plugin | --claude-code-plugin | --codex-plugin | --agent-md] [--json]
  pockit sources validate|plan|fetch [--role ROLE] [--json]
  pockit skills generate [--out DIR] [--role ROLE] [--dry-run] [--check] [--overwrite] [--pi-extension | --opencode-plugin | --goose-plugin | --cursor-plugin | --claude-code-plugin | --codex-plugin | --agent-md] [--package-name NAME] [--package-version SEMVER] [--install] [--json]
Output: inline ViewKit panels on terminals; --json for automation; --plain for plain text
Inputs: --catalog FILE (--sources FILE) --schema FILE --inheritance FILE (--roles FILE) --cache DIR
Defaults: user config pockit/sources.json and roles.json, then embedded inputs.
Fetch: --concurrency 8 --timeout 30s --max-bytes 8388608 --refresh
Only sources fetch accesses remote content.
`

func Run(ctx context.Context, args []string, out, errOut io.Writer) int {
	if len(args) == 0 {
		fmt.Fprint(out, usage)
		return Success
	}
	if args[0] == "--help" || args[0] == "help" || args[0] == "-h" {
		fmt.Fprint(out, usage)
		return Success
	}
	if args[0] == "config" {
		return runConfig(args[1:], out, errOut)
	}
	group, command, rest := args[0], "", args[1:]
	adhoc := group == "input"
	if adhoc {
		group, command = "skills", "generate"
	} else {
		if len(rest) == 0 {
			fmt.Fprint(errOut, usage)
			return Invalid
		}
		command, rest = rest[0], rest[1:]
	}
	var machine, dry, overwrite, plain, piExtension, openCodePlugin, goosePlugin, cursorPlugin, claudeCodePlugin, codexPlugin, agentMD, check, install bool
	for _, arg := range rest {
		if arg == "--json" || arg == "--json=true" {
			machine = true
		}
	}
	var progress *uiadapter.Progress
	warn := func(message string) {
		uiadapter.Warning(errOut, message, plain || machine)
	}
	finish := func(success bool) error {
		if progress == nil {
			progress = uiadapter.NewProgress(out, group+" / "+command, plain)
		}
		e := progress.Finish(success)
		progress = nil
		return e
	}
	emit := func(v any) bool {
		var e error
		if machine {
			e = json.NewEncoder(out).Encode(v)
		} else {
			complete := true
			switch report := v.(type) {
			case fetching.Report:
				complete = report.Failed == 0 && ctx.Err() == nil
			case skills.Report:
				complete = report.Failed == 0 && !report.NeedsRebuild
			}
			e = finish(complete)
			if e == nil && uiadapter.PanelsEnabled(out, plain) {
				e = uiadapter.WritePanel(out, "POCKIT · Results", reportSummary(v), plain)
			}
			switch report := v.(type) {
			case fetching.Report:
				for _, outcome := range report.Outcomes {
					if outcome.Outcome == "failure" {
						warn(outcome.ID + ": " + outcome.Detail)
					}
				}
			case skills.Report:
				if check {
					message := "Bundle is up to date with current cached inputs."
					if report.NeedsRebuild {
						message = "Rebuild needed: bundled output differs from current cached inputs."
					}
					if e == nil {
						e = uiadapter.WritePanel(out, "POCKIT · Bundle check", []string{message}, plain)
					}
				}
				for _, result := range report.Results {
					if result.Status == "failure" {
						warn(result.Role + " / " + result.ID + ": " + result.Detail)
					}
				}
			}
		}
		if e != nil {
			warn("output write failed")
			return false
		}
		return true
	}
	fail := func(e error, code int) int {
		if !machine && progress != nil {
			if finish(false) != nil {
				warn("output write failed")
			}
		}
		warn(e.Error())
		if machine {
			emit(map[string]any{"version": 1, "ok": false, "error": e.Error()})
		}
		return code
	}
	valid := (group == "sources" && (command == "validate" || command == "plan" || command == "fetch")) || (group == "skills" && command == "generate")
	if !valid {
		return fail(fmt.Errorf("unknown command"), Invalid)
	}
	f := flag.NewFlagSet("pockit", flag.ContinueOnError)
	var flagOutput bytes.Buffer
	f.SetOutput(&flagOutput)
	var in app.Inputs
	var role, cacheDir, output, packageName, packageVersion string
	opts := fetching.Options{}
	f.StringVar(&in.Catalog, "catalog", "", "catalog input (user config, then embedded by default)")
	f.StringVar(&in.Catalog, "sources", "", "alias for --catalog")
	f.StringVar(&in.Schema, "schema", "", "schema input (embedded by default)")
	f.StringVar(&in.Inheritance, "inheritance", "", "inheritance input (user config, then embedded by default)")
	f.StringVar(&in.Inheritance, "roles", "", "alias for --inheritance")
	f.StringVar(&role, "role", "", "selected role; omission selects all")
	f.StringVar(&cacheDir, "cache", "", "private cache outside project")
	f.BoolVar(&machine, "json", machine, "stable machine-readable JSON instead of styled output")
	f.BoolVar(&plain, "plain", false, "plain text without panels or terminal control sequences")
	if group == "skills" {
		f.StringVar(&output, "out", "", "output directory (default: user data/pockit/{pi-package,opencode-plugin,goose-plugin,cursor-plugin,claude-code-plugin,codex-plugin} for package output, otherwise private temp; required for --check)")
		f.BoolVar(&piExtension, "pi-extension", false, "include a Pi extension package and installation instructions")
		f.BoolVar(&openCodePlugin, "opencode-plugin", false, "include an OpenCode plugin package and configuration instructions")
		f.BoolVar(&goosePlugin, "goose-plugin", false, "include a Goose Open Plugins bundle and installation instructions")
		f.BoolVar(&cursorPlugin, "cursor-plugin", false, "include a Cursor plugin bundle and local installation instructions")
		f.BoolVar(&claudeCodePlugin, "claude-code-plugin", false, "include a Claude Code plugin bundle and local loading instructions")
		f.BoolVar(&codexPlugin, "codex-plugin", false, "include a Codex plugin bundle and local marketplace instructions")
		f.BoolVar(&agentMD, "agent-md", false, "write a single self-contained AGENT.md instead of role skill files")
		f.StringVar(&packageName, "package-name", "", "package name (default: "+skills.DefaultPackageName+"; requires a package output flag)")
		f.StringVar(&packageVersion, "package-version", "", "package SemVer (default: "+skills.DefaultPackageVersion+"; requires a package output flag)")
		f.BoolVar(&install, "install", false, "install the generated package for the current user (Claude Code/Codex stage local copies; updates require --overwrite)")
		f.BoolVar(&dry, "dry-run", false, "preview without writing")
		f.BoolVar(&check, "check", false, "check cached inputs against output without writing; exit 4 if rebuild needed")
		f.BoolVar(&overwrite, "overwrite", false, "explicit replacement permission for regular artifact files")
	}
	if command == "fetch" {
		f.IntVar(&opts.Concurrency, "concurrency", fetching.MaxConcurrency, "workers (1..8)")
		f.DurationVar(&opts.Timeout, "timeout", 30*time.Second, "per-source request timeout")
		f.Int64Var(&opts.MaxBytes, "max-bytes", 8<<20, "maximum bytes per response")
		f.BoolVar(&opts.Refresh, "refresh", false, "explicitly recollect cached successes")
	}
	if e := f.Parse(rest); e != nil {
		if errors.Is(e, flag.ErrHelp) {
			fmt.Fprint(out, flagOutput.String())
			return Success
		}
		return fail(e, Invalid)
	}
	if f.NArg() != 0 {
		return fail(fmt.Errorf("unexpected positional arguments"), Invalid)
	}
	if adhoc {
		if in.Catalog == "" || in.Inheritance == "" {
			return fail(fmt.Errorf("input requires --sources FILE and --roles FILE"), Invalid)
		}
		in.Embedded = true
	}
	versionSet, nameSet := false, false
	f.Visit(func(flag *flag.Flag) {
		if flag.Name == "package-name" {
			nameSet = true
		}
		if flag.Name == "package-version" {
			versionSet = true
		}
	})
	generateOpts := skills.Options{Out: output, DryRun: dry, Overwrite: overwrite, PiExtension: piExtension, OpenCodePlugin: openCodePlugin, GoosePlugin: goosePlugin, CursorPlugin: cursorPlugin, ClaudeCodePlugin: claudeCodePlugin, CodexPlugin: codexPlugin, AgentMD: agentMD, PackageName: packageName, PackageVersion: packageVersion, Check: check}
	adapter, err := skills.PackageAdapterFor(generateOpts)
	if err != nil {
		return fail(fmt.Errorf("--pi-extension, --opencode-plugin, --goose-plugin, --cursor-plugin, --claude-code-plugin, --codex-plugin and --agent-md are mutually exclusive"), Invalid)
	}
	packageOutput, err := newPackageOutput(adapter)
	if err != nil {
		return fail(err, Invalid)
	}
	if nameSet {
		if packageOutput == nil {
			return fail(fmt.Errorf("--package-name requires --pi-extension, --opencode-plugin, --goose-plugin, --cursor-plugin, --claude-code-plugin or --codex-plugin"), Invalid)
		}
		if err := skills.ValidatePackageName(packageName); err != nil {
			return fail(err, Invalid)
		}
	}
	if versionSet {
		if packageOutput == nil {
			return fail(fmt.Errorf("--package-version requires --pi-extension, --opencode-plugin, --goose-plugin, --cursor-plugin, --claude-code-plugin or --codex-plugin"), Invalid)
		}
		if err := skills.ValidatePackageVersion(packageVersion); err != nil {
			return fail(err, Invalid)
		}
	}
	if packageOutput != nil && nameSet {
		if err := packageOutput.ValidateName(packageName); err != nil {
			return fail(err, Invalid)
		}
	}
	if check && output == "" {
		return fail(fmt.Errorf("--out is required with --check"), Invalid)
	}
	if install && (packageOutput == nil || dry || check) {
		return fail(fmt.Errorf("--install requires --pi-extension, --opencode-plugin, --goose-plugin, --cursor-plugin, --claude-code-plugin or --codex-plugin and cannot be combined with --dry-run or --check"), Invalid)
	}
	if check && (dry || overwrite) {
		return fail(fmt.Errorf("--check cannot be combined with --dry-run or --overwrite"), Invalid)
	}
	if command == "fetch" {
		if opts.Concurrency < 1 || opts.Concurrency > fetching.MaxConcurrency || opts.Timeout <= 0 || opts.MaxBytes <= 0 {
			return fail(fmt.Errorf("fetch bounds must be positive; concurrency must be 1..8"), Invalid)
		}
	}
	// Print directory diagnostics before starting the terminal progress line.
	if group == "skills" && output == "" {
		var err error
		if packageOutput != nil {
			dataDir := os.Getenv("XDG_DATA_HOME")
			if dataDir == "" || !filepath.IsAbs(dataDir) {
				var home string
				home, err = os.UserHomeDir()
				if err == nil {
					dataDir = filepath.Join(home, ".local", "share")
				}
			}
			if err == nil {
				output, err = filepath.Abs(filepath.Join(dataDir, "pockit", string(packageOutput.Type())))
			}
		} else {
			output, err = os.MkdirTemp("", "pockit-skills-")
		}
		if err != nil {
			return fail(fmt.Errorf("resolve output directory: %w", err), Partial)
		}
		if err := uiadapter.WritePanel(errOut, "POCKIT · Output", []string{"Output directory: " + output}, plain || machine); err != nil {
			return fail(err, Partial)
		}
	}
	if !machine {
		progress = uiadapter.NewProgress(out, group+" / "+command, plain)
	}
	session, e := app.Open(in, cacheDir)
	if e != nil {
		return fail(e, Invalid)
	}
	plan, e := session.Plan(role)
	if e != nil {
		return fail(e, Invalid)
	}
	switch group {
	case "sources":
		switch command {
		case "validate":
			if !emit(map[string]any{"version": 1, "ok": true, "roles": session.Service.Catalog.Names(), "unique_sources": len(plan.Entries)}) {
				return Partial
			}
		case "plan":
			if !emit(plan) {
				return Partial
			}
		case "fetch":
			if progress != nil {
				opts.Progress = progress.Update
			}
			r := fetching.New(session.Store, opts).Collect(ctx, plan)
			if !emit(r) {
				return Partial
			}
			if r.Failed > 0 || ctx.Err() != nil {
				if machine {
					warn("collection contains failures; inspect reported outcomes")
				}
				if ctx.Err() != nil {
					warn(ctx.Err().Error())
				}
				return Partial
			}
		}
	case "skills":
		generateOpts.Out = output
		if progress != nil {
			generateOpts.Progress = progress.Update
		}
		r, e := skills.Generate(ctx, plan, session.Store, session.Roles(role), generateOpts)
		if e != nil && !machine {
			if finish(false) != nil {
				warn("output write failed")
			}
		} else if !emit(r) {
			return Partial
		}
		if e != nil {
			warn(e.Error())
			if errors.Is(e, skills.ErrRefused) {
				return Refused
			}
			return Partial
		}
		if r.Installation != nil && packageOutput != nil {
			printInstructions := func() error {
				return uiadapter.WritePanel(errOut, "POCKIT · "+packageOutput.Host(), packageOutput.instructions(r.Installation), plain || machine)
			}
			if install {
				message, err := packageOutput.install(ctx, output, packageName, overwrite, r.Installation, errOut)
				if err != nil {
					warn(packageOutput.Host() + " installation failed (generated files remain available): " + err.Error())
					if packageOutput.instructionsOnFailure {
						_ = printInstructions()
					}
					if errors.Is(err, skills.ErrRefused) {
						return Refused
					}
					return Partial
				}
				lines := []string{message}
				if packageOutput.noteAfterInstall {
					lines = append(lines, r.Installation.Note)
				}
				if err := uiadapter.WritePanel(errOut, "POCKIT · "+packageOutput.Host(), lines, plain || machine); err != nil {
					return fail(err, Partial)
				}
			} else if err := printInstructions(); err != nil {
				return fail(err, Partial)
			}
		}
		if r.NeedsRebuild {
			return Partial
		}
		if r.Failed > 0 {
			if machine {
				warn("generation contains unavailable or unsupported mappings; inspect reported results")
			}
			return Partial
		}
	}
	return Success
}
