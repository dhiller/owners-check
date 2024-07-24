/*
 * MIT License
 *
 * Copyright 2024 Daniel Hiller
 *
 * Permission is hereby granted, free of charge, to any person obtaining a copy of this software and
 * associated documentation files (the “Software”), to deal in the Software without restriction,
 * including without limitation the rights to use, copy, modify, merge, publish, distribute,
 * sublicense, and/or sell copies of the Software, and to permit persons to whom the Software is
 * furnished to do so, subject to the following conditions:
 *
 * The above copyright notice and this permission notice shall be included in all copies or
 * substantial portions of the Software.
 *
 * THE SOFTWARE IS PROVIDED “AS IS”, WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT
 * NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
 * NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM,
 * DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
 * OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
 */

package owners_check

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type Filter struct {
	Reviewers []string `yaml:"reviewers"`
	Approvers []string `yaml:"approvers"`
	Labels    []string `yaml:"labels"`
}
type Owners struct {
	Filters map[string]Filter `yaml:"filters"`
}

type checkFiltersArgs struct {
	directory string
}

var (
	checkFiltersCommand = &cobra.Command{
		Use:   `filters`,
		Short: "Checks OWNERS filters expressions for matching files",
		Long: `Checks the filters expressions inside OWNERS files.

Checks that:
* the filter expressions compile to valid go regular expressions
* each expression matches at least one file

Will output a list of matching files for each of the expressions.
`,
		Run: func(cmd *cobra.Command, args []string) {
			err := cmd.InheritedFlags().Parse(args)
			if err != nil {
				log().WithError(err).Fatal("invalid args")
			}
			if commonArguments.debug {
				logrus.SetLevel(logrus.DebugLevel)
			}
			log().Debug("common: %v", commonArguments)

			err = cmd.PersistentFlags().Parse(args)
			if err != nil {
				log().WithError(err).Fatal("invalid args")
			}
			log().Debug("filters: %v", checkFiltersArguments)

			checkFilters(checkFiltersArguments)
		},
	}

	checkFiltersArguments = checkFiltersArgs{}
)

func init() {
	checkFiltersCommand.PersistentFlags().StringVar(&checkFiltersArguments.directory, "directory", "", "the directory to check the OWNERS files")
	rootCmd.AddCommand(checkFiltersCommand)
}

func checkFilters(args checkFiltersArgs) {
	if args.directory == "" {
		log().Fatalf("directory is required")
	}
	stat, err := os.Stat(args.directory)
	if os.IsNotExist(err) || !stat.IsDir() {
		log().Fatal("%q does not exist or is not a directory", args.directory)
	}

	err = filepath.WalkDir(args.directory, func(path string, d os.DirEntry, err error) error {
		if d.IsDir() || d.Name() != "OWNERS" || strings.Contains(path, "/vendor/") {
			return nil
		}
		var owners *Owners
		log().WithField("file", path).Debug("checking OWNERS file")
		file, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		err = yaml.Unmarshal(file, &owners)
		if err != nil {
			return err
		}
		if owners == nil {
			log().WithField("file", path).Info("does not have filters")
			return nil
		}
		for fileFilterExpression := range owners.Filters {
			if fileFilterExpression == ".*" {
				continue
			}
			filterRegex := regexp.MustCompile(fileFilterExpression)
			var matchingFiles []string
			pathDir := filepath.Dir(path) + "/"
			fileFilterExpressionsLogger := log().WithFields(logrus.Fields{
				"pathDir": pathDir,
				"regex":   fileFilterExpression,
			})
			err = filepath.WalkDir(pathDir, func(innerpath string, innerd os.DirEntry, innererr error) error {
				checkPath := strings.TrimPrefix(innerpath, pathDir)
				fileFilterExpressionsLogger.Debug("check match")
				if filterRegex.MatchString(checkPath) {
					matchingFiles = append(matchingFiles, checkPath)
				}
				return nil
			})
			if len(matchingFiles) == 0 {
				return fmt.Errorf("no files matched filter %s", fileFilterExpression)
			}
			fileFilterExpressionsLogger.Infof("files matched: %v", matchingFiles)
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		log().WithError(err).Fatal("failed to check owners")
	}
}
