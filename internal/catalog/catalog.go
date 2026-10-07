package catalog

const (
	StackGo      = "go"
	StackNodeJS  = "nodejs"
	StackNestJS  = "nestjs"
	StackLaravel = "laravel"
	StackPHP     = "php"

	ShapeMonolith      = "monolith"
	ShapeMicroservices = "microservices"
	ShapeOneShotJob    = "one-shot-job"
	ShapeCLI           = "cli"
	ShapeWorker        = "worker"
	ShapeLibrary       = "library"

	WorkloadHTTPAPI = "http-api"
	WorkloadCLI     = "cli"
	WorkloadOneShot = "one-shot-job"
	WorkloadWorker  = "background-worker"
	WorkloadLibrary = "library"

	TopologyMonolith      = ShapeMonolith
	TopologyMicroservices = ShapeMicroservices

	PatternBackend = "standard-backend"
	PatternRAG     = "rag"
	PatternAgent   = "agent"
	PatternChatbot = "chatbot"

	TemplateGoCLI     = "go-cli"
	TemplateGoWorker  = "go-worker"
	TemplateGoLibrary = "go-library"

	ArchitectureLayered  = "layered"
	ArchitectureClean    = "clean-hexagonal"
	ArchitectureDDD      = "ddd"
	ArchitecturePipeline = "pipeline"

	TemplateGoMonolithLayered      = "go-monolith-layered"
	TemplateGoMonolithClean        = "go-monolith-clean-hexagonal"
	TemplateGoMonolithDDD          = "go-monolith-ddd"
	TemplateGoMicroservicesLayered = "go-microservices-layered"
	TemplateGoMicroservicesClean   = "go-microservices-clean-hexagonal"
	TemplateGoMicroservicesDDD     = "go-microservices-ddd"
	TemplateGoOneShotPipeline      = "go-one-shot-pipeline"

	CapabilitySQLite   = "database-sqlite"
	CapabilityPostgres = "database-postgresql"
	CapabilityMySQL    = "database-mysql"
	CapabilityMongoDB  = "database-mongodb"

	SettingStack            = "stack"
	SettingServiceFramework = "service-framework"
)

type AppShape struct {
	ID          string
	Name        string
	Description string
}

type Workload struct {
	ID          string
	Name        string
	Description string
}

type Pattern struct {
	ID          string
	Name        string
	Description string
}

type Capability struct {
	ID          string
	Name        string
	Category    string
	Description string
	Supported   bool
}

type Architecture struct {
	ID          string
	Name        string
	Description string
}

type Template struct {
	ID                 string
	Name               string
	Description        string
	StackID            string
	AppShapeID         string
	WorkloadID         string
	TopologyID         string
	ArchitectureID     string
	ServiceFrameworkID string
}

type SettingOption struct {
	ID          string
	Name        string
	Description string
	Supported   bool
}

type Setting struct {
	ID      string
	Name    string
	Options []SettingOption
}
