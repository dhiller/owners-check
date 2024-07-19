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

package main

import (
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const robotName = "owners-check"

type Filter struct {
	Reviewers []string `yaml:"reviewers"`
	Approvers []string `yaml:"approvers"`
	Labels    []string `yaml:"labels"`
}
type Owners struct {
	Filters map[string]Filter `yaml:"filters"`
}

var (
	dir   string
	debug bool
)

func init() {
	logrus.SetFormatter(&logrus.JSONFormatter{})
}

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	fs.StringVar(&dir, "directory", "", "the directory for the OWNERS files to check")
	fs.BoolVar(&debug, "debug", false, "whether debug information should be printed")
	fs.Parse(os.Args[1:])

	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	if dir == "" {
		log().Fatalf("directory is required")
	}
	stat, err := os.Stat(dir)
	if os.IsNotExist(err) || !stat.IsDir() {
		log().Fatal("%q does not exist or is not a directory", dir)
	}

	err = filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
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
			err = filepath.WalkDir(pathDir, func(innerpath string, innerd os.DirEntry, innererr error) error {
				checkPath := strings.TrimPrefix(innerpath, pathDir)
				log().WithFields(logrus.Fields{
					"checkPath": checkPath,
					"regex":     fileFilterExpression,
				}).Debug("check match")
				if filterRegex.MatchString(checkPath) {
					matchingFiles = append(matchingFiles, checkPath)
				}
				return nil
			})
			if len(matchingFiles) == 0 {
				return fmt.Errorf("no files matched filter %s", fileFilterExpression)
			}
			log().WithField("filter", fileFilterExpression).Infof("files matched: %v", matchingFiles)
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

func log() *logrus.Entry {
	return logrus.StandardLogger().
		WithFields(logrus.Fields{
			"robot":     robotName,
			"directory": fmt.Sprintf("%q", dir),
		},
		)
}
