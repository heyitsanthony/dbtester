// Copyright 2017 CoreOS, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package fileinspect

import (
	"os"
	"path/filepath"
	"strings"
)

// Walk walks all files in the target directory.
func Walk(targetDir string) (map[string]os.FileInfo, error) {
	rm := make(map[string]os.FileInfo)
	visit := func(path string, f os.FileInfo, err error) error {
		if f != nil {
			if !f.IsDir() {
				wd, err := os.Getwd()
				if err != nil {
					return err
				}
				rm[filepath.Join(wd, strings.Replace(path, wd, "", -1))] = f
			}
		}
		return nil
	}
	err := filepath.Walk(targetDir, visit)
	if err != nil {
		return nil, err
	}
	return rm, nil
}

// Size returns the size of target directory.
// Same as 'du -sh $DIR'.
func Size(targetDir string) (int64, error) {
	fm, err := Walk(targetDir)
	if err != nil {
		return 0, err
	}
	var size int64
	for _, v := range fm {
		size += v.Size()
	}
	return size, nil
}
