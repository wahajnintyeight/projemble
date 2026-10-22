package catalog

var steps = []Step{
	{
		ID:       StepServiceFramework,
		Name:     "Service framework",
		Question: "Which framework connects your services?",
		AppliesTo: func(selection Selection) bool {
			return selection[ContextWorkload] == WorkloadHTTPAPI && selection[ContextTopology] == TopologyMicroservices
		},
		Options: []Option{
			{ID: "go-micro", Name: "go-micro", Summary: "Go framework for microservice discovery and communication", Status: StatusRecommended, Modules: []string{"go-micro.dev/v6"}},
		},
	},
	{
		ID:       StepDatabase,
		Name:     "Primary database",
		Question: "Which database stores your data?",
		AppliesTo: func(selection Selection) bool {
			return selection[ContextWorkload] == WorkloadHTTPAPI || selection[ContextWorkload] == WorkloadWorker
		},
		Default: func(Selection) string { return OptionNone },
		Options: databaseOptions(),
	},
}

var databaseModulePaths = map[string]string{
	CapabilitySQLite:   "modernc.org/sqlite",
	CapabilityPostgres: "github.com/jackc/pgx/v5",
	CapabilityMySQL:    "github.com/go-sql-driver/mysql",
	CapabilityMongoDB:  "go.mongodb.org/mongo-driver/v2",
}

func databaseOptions() []Option {
	options := []Option{{ID: OptionNone, Name: "None", Summary: "Keep data in memory", Status: StatusSupported}}
	for _, capability := range capabilities {
		if capability.Category != "Primary database" {
			continue
		}
		option := Option{ID: capability.ID, Name: capability.Name, Summary: capability.Description, Status: StatusSupported}
		if !capability.Supported {
			option.Status = StatusPlanned
		}
		if path, ok := databaseModulePaths[capability.ID]; ok {
			option.Modules = []string{path}
		}
		options = append(options, option)
	}
	return options
}

func Steps() []Step {
	return append([]Step(nil), steps...)
}

func StepByID(id string) (Step, bool) {
	for _, step := range steps {
		if step.ID == id {
			return step, true
		}
	}
	return Step{}, false
}

func SelectionFor(workloadID, topologyID string, choices map[string]string) Selection {
	selection := Selection{ContextWorkload: workloadID, ContextTopology: topologyID}
	for key, value := range choices {
		selection[key] = value
	}
	return selection
}
