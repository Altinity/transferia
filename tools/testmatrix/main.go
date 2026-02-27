package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

type suiteManifest struct {
	Name   string `yaml:"name"`
	Waves  []wave `yaml:"waves"`
	Matrix struct {
		SourceVariants []string `yaml:"source_variants"`
	} `yaml:"matrix"`
}

type wave struct {
	ID       string  `yaml:"id"`
	Suites   []suite `yaml:"suites"`
	Packages []pkg   `yaml:"packages"`
}

type suite struct {
	SuiteName  string `yaml:"suite_name"`
	SuiteGroup string `yaml:"suite_group"`
	SuitePath  string `yaml:"suite_path"`
	GoTestArgs string `yaml:"go_test_args"`
}

type pkg struct {
	Name       string `yaml:"name"`
	Pattern    string `yaml:"pattern"`
	GoTestArgs string `yaml:"go_test_args"`
}

type matrixContract struct {
	Scenarios []scenario `yaml:"scenarios"`
}

type scenario struct {
	ID      string                    `yaml:"id"`
	Wave    int                       `yaml:"wave"`
	Applies map[string]scenarioSource `yaml:"applies"`
}

type scenarioSource struct {
	Mode  string   `yaml:"mode"`
	Paths []string `yaml:"paths"`
}

func main() {
	if len(os.Args) < 2 {
		exitErr(errors.New("usage: testmatrix <suite|gate> ..."))
	}

	switch os.Args[1] {
	case "suite":
		exitErr(runSuite(os.Args[2:]))
	case "gate":
		exitErr(runGate(os.Args[2:]))
	default:
		exitErr(fmt.Errorf("unknown command %q", os.Args[1]))
	}
}

func runSuite(args []string) error {
	fs := flag.NewFlagSet("suite", flag.ContinueOnError)
	manifestPath := fs.String("manifest", "", "path to suite manifest")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *manifestPath == "" {
		return errors.New("--manifest is required")
	}
	m, err := loadSuiteManifest(*manifestPath)
	if err != nil {
		return err
	}
	rest := fs.Args()
	if len(rest) < 1 {
		return errors.New("suite subcommand is required")
	}

	switch rest[0] {
	case "list":
		for _, w := range m.Waves {
			fmt.Printf("wave=%s suites=%d packages=%d\n", w.ID, len(w.Suites), len(w.Packages))
		}
		return nil
	case "verify":
		if len(m.Waves) == 0 {
			return errors.New("manifest has no waves")
		}
		for _, w := range m.Waves {
			if w.ID == "" {
				return errors.New("wave id must not be empty")
			}
			if len(w.Suites)+len(w.Packages) == 0 {
				return fmt.Errorf("wave %q has no runnable items", w.ID)
			}
		}
		fmt.Println("cdc suite verification passed")
		return nil
	case "waves":
		for _, w := range m.Waves {
			fmt.Println(w.ID)
		}
		return nil
	case "emit-wave":
		emitFS := flag.NewFlagSet("emit-wave", flag.ContinueOnError)
		waveID := emitFS.String("wave", "", "wave id")
		if err := emitFS.Parse(rest[1:]); err != nil {
			return err
		}
		if *waveID == "" {
			return errors.New("--wave is required")
		}
		for _, w := range m.Waves {
			if w.ID != *waveID {
				continue
			}
			for _, s := range w.Suites {
				fmt.Printf("SUITE\t%s\t%s\t%s\t%s\n", s.SuiteGroup, s.SuitePath, s.SuiteName, s.GoTestArgs)
			}
			for _, p := range w.Packages {
				fmt.Printf("PKG\t%s\t%s\t%s\t\n", p.Pattern, p.Name, p.GoTestArgs)
			}
			return nil
		}
		return fmt.Errorf("wave %q not found", *waveID)
	case "emit-matrix":
		emitFS := flag.NewFlagSet("emit-matrix", flag.ContinueOnError)
		_ = emitFS.String("scope", "all", "matrix scope")
		if err := emitFS.Parse(rest[1:]); err != nil {
			return err
		}
		for _, v := range m.Matrix.SourceVariants {
			fmt.Println(v)
		}
		return nil
	default:
		return fmt.Errorf("unknown suite subcommand %q", rest[0])
	}
}

func runGate(args []string) error {
	fs := flag.NewFlagSet("gate", flag.ContinueOnError)
	matrixPath := fs.String("matrix", "", "path to matrix contract")
	waveN := fs.Int("wave", 0, "wave number")
	writeReport := fs.String("write-report", "", "output report path")
	enforce := fs.Bool("enforce", false, "enforce required paths")
	printRequired := fs.Bool("print-required-paths", false, "print required paths")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *matrixPath == "" {
		return errors.New("--matrix is required")
	}
	if *waveN == 0 {
		return errors.New("--wave is required")
	}

	c, err := loadMatrixContract(*matrixPath)
	if err != nil {
		return err
	}
	required := requiredPaths(c, *waveN)
	if *printRequired {
		for _, p := range required {
			fmt.Println(p)
		}
	}
	if *writeReport != "" {
		if err := writeCoverageReport(*writeReport, *waveN, required); err != nil {
			return err
		}
	}
	if *enforce {
		missing := make([]string, 0)
		for _, p := range required {
			if _, err := os.Stat(p); err != nil {
				missing = append(missing, p)
			}
		}
		if len(missing) > 0 {
			for _, m := range missing {
				fmt.Fprintf(os.Stderr, "missing required path: %s\n", m)
			}
			return fmt.Errorf("gate failed: %d required paths missing", len(missing))
		}
	}
	return nil
}

func loadSuiteManifest(path string) (*suiteManifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m suiteManifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func loadMatrixContract(path string) (*matrixContract, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c matrixContract
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func requiredPaths(c *matrixContract, waveN int) []string {
	set := map[string]struct{}{}
	for _, sc := range c.Scenarios {
		if sc.Wave != waveN {
			continue
		}
		for _, src := range sc.Applies {
			if strings.ToUpper(src.Mode) != "M" {
				continue
			}
			for _, p := range src.Paths {
				if p == "" {
					continue
				}
				set[p] = struct{}{}
			}
		}
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func writeCoverageReport(path string, wave int, required []string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	var b strings.Builder
	b.WriteString("# Core2CH Coverage Report\n\n")
	b.WriteString(fmt.Sprintf("Wave: %d\n\n", wave))
	b.WriteString(fmt.Sprintf("Required paths: %d\n\n", len(required)))
	for _, p := range required {
		b.WriteString("- `" + p + "`\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

func exitErr(err error) {
	if err == nil {
		return
	}
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
