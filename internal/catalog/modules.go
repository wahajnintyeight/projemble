package catalog

import (
	"strconv"
	"strings"
	"time"
)

const GoBaseline = "1.26.0"

const pinsVerified = "2026-10-22"

type ModuleInfo struct {
	Path       string
	Version    string
	License    string
	MinGo      string
	VerifiedAt string
}

func (module ModuleInfo) Requirement() string {
	return module.Path + " " + module.Version
}

var modules = []ModuleInfo{
	{Path: "go-micro.dev/v6", Version: "v6.15.1", License: "Apache-2.0", MinGo: "1.26.0", VerifiedAt: pinsVerified},
	{Path: "modernc.org/sqlite", Version: "v1.60.1", License: "BSD-3-Clause", MinGo: "1.26.0", VerifiedAt: pinsVerified},
	{Path: "github.com/jackc/pgx/v5", Version: "v5.11.0", License: "MIT", MinGo: "1.26.0", VerifiedAt: pinsVerified},
	{Path: "github.com/go-sql-driver/mysql", Version: "v1.10.1", License: "MPL-2.0", MinGo: "1.26.0", VerifiedAt: pinsVerified},
	{Path: "go.mongodb.org/mongo-driver/v2", Version: "v2.9.2", License: "Apache-2.0", MinGo: "1.26.0", VerifiedAt: pinsVerified},
}

func Modules() []ModuleInfo {
	return append([]ModuleInfo(nil), modules...)
}

func ModuleByPath(path string) (ModuleInfo, bool) {
	for _, module := range modules {
		if module.Path == path {
			return module, true
		}
	}
	return ModuleInfo{}, false
}

func StaleModules(now time.Time, maxAge time.Duration) []ModuleInfo {
	var stale []ModuleInfo
	for _, module := range modules {
		verified, err := time.Parse("2006-01-02", module.VerifiedAt)
		if err != nil || now.Sub(verified) > maxAge {
			stale = append(stale, module)
		}
	}
	return stale
}

func GoDirective(selected []ModuleInfo) string {
	result := GoBaseline
	for _, module := range selected {
		if module.MinGo != "" && CompareGoVersions(module.MinGo, result) > 0 {
			result = module.MinGo
		}
	}
	return result
}

func CompareGoVersions(a, b string) int {
	left, right := goVersionParts(a), goVersionParts(b)
	for i := range left {
		if left[i] != right[i] {
			if left[i] < right[i] {
				return -1
			}
			return 1
		}
	}
	return 0
}

func goVersionParts(version string) [3]int {
	var parts [3]int
	version = strings.TrimPrefix(version, "go")
	for i, field := range strings.SplitN(version, ".", 3) {
		digits := field
		if end := strings.IndexFunc(field, func(r rune) bool { return r < '0' || r > '9' }); end >= 0 {
			digits = field[:end]
		}
		parts[i], _ = strconv.Atoi(digits)
	}
	return parts
}
