package generator

import (
	"strings"

	"projemble/internal/catalog"
	"projemble/internal/projectstore"
)

var databaseModulePaths = map[string]string{
	catalog.CapabilitySQLite:   "modernc.org/sqlite",
	catalog.CapabilityPostgres: "github.com/jackc/pgx/v5",
	catalog.CapabilityMySQL:    "github.com/go-sql-driver/mysql",
	catalog.CapabilityMongoDB:  "go.mongodb.org/mongo-driver/v2",
}

func projectModules(project projectstore.Project, microservices bool) []catalog.ModuleInfo {
	var paths []string
	if microservices {
		paths = append(paths, "go-micro.dev/v6")
	}
	for _, capability := range project.Capabilities {
		if path, ok := databaseModulePaths[capability]; ok {
			paths = append(paths, path)
		}
	}
	modules := make([]catalog.ModuleInfo, 0, len(paths))
	for _, path := range paths {
		if module, ok := catalog.ModuleByPath(path); ok {
			modules = append(modules, module)
		}
	}
	return modules
}

func databaseCapability(id string) bool {
	_, ok := databaseModulePaths[id]
	return ok
}

func patternPackage(id string) string {
	return strings.ReplaceAll(id, "-", "_")
}
